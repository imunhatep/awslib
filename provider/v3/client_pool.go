package v3

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/gocollection/dict"
	"github.com/rs/zerolog/log"
)

// ClientPool is a concurrent map implementation to store multiple AWS clients.
type ClientPool struct {
	sync.Mutex
	ctx     context.Context
	builder *ClientBuilder

	// lists
	clients map[types.AwsAccountID]map[types.AwsRegion]*Client
	roles   map[types.AwsAccountID]types.RoleArn

	// failures remembers (account, region) pairs that could not produce a
	// client, so they are not re-probed on every request.
	failures *FailureCache
}

// NewClientPool creates an AWS client for each permutation of the given profiles and regions.
// If profiles, regions, or both are empty, credentials and regions are picked up via the usual default provider chain,
// respectively. For example, if regions are empty, the region is first looked for via the according region environment variable
// or second the default region for each profile is used from `~/.aws/config`.
func NewClientPool(ctx context.Context, clientBuilder *ClientBuilder, assumableRoles map[types.AwsAccountID]types.RoleArn) *ClientPool {
	clientPool := &ClientPool{
		ctx:      ctx,
		builder:  clientBuilder,
		clients:  map[types.AwsAccountID]map[types.AwsRegion]*Client{},
		roles:    maps.Clone(assumableRoles),
		failures: NewFailureCache(DefaultClientFailureTTL),
	}

	return clientPool
}

// WithFailureTTL sets how long a client-creation failure is remembered. See
// DefaultClientFailureTTL for the trade-off; a non-positive ttl restores the
// default.
func (p *ClientPool) WithFailureTTL(ttl time.Duration) *ClientPool {
	p.Lock()
	defer p.Unlock()

	p.failures = NewFailureCache(ttl)

	return p
}

func (p *ClientPool) GetContext() context.Context {
	return p.ctx
}

func (p *ClientPool) GetClients(regions ...types.AwsRegion) ([]*Client, error) {
	accountIDs, err := p.PoolAccountIDs()
	if err != nil {
		return nil, errors.New(err)
	}

	return p.collectClients(accountIDs, regions), nil
}

// GetAccountClients returns clients for one account only.
//
// This is not GetClients plus a filter: building a client assumes that
// account's role, so filtering afterwards would already have reached into every
// other account in the pool. Callers that scope work to one account must scope
// it here, before any client exists.
//
// An account the pool cannot reach is an error rather than an empty slice — a
// caller asking for the wrong account should not be told the account is empty.
func (p *ClientPool) GetAccountClients(accountID types.AwsAccountID, regions ...types.AwsRegion) ([]*Client, error) {
	accountIDs, err := p.PoolAccountIDs()
	if err != nil {
		return nil, errors.New(err)
	}

	if !slices.Contains(accountIDs, accountID) {
		return nil, errors.Errorf("account %s is not reachable by this client pool", accountID)
	}

	return p.collectClients([]types.AwsAccountID{accountID}, regions), nil
}

// PoolAccountIDs reports every account this pool can build a client for: the
// configured assumable roles, or the default credentials' own account when no
// roles are set. Unlike ListAccountIDs it does not depend on a client having
// been created first.
func (p *ClientPool) PoolAccountIDs() ([]types.AwsAccountID, error) {
	p.Lock()
	roles := maps.Clone(p.roles)
	p.Unlock()

	if len(roles) > 0 {
		return dict.Keys(roles), nil
	}

	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	return []types.AwsAccountID{defaultClient.GetAccountID()}, nil
}

// collectClients builds the account × region matrix concurrently, logging and
// skipping the pairs that fail so one disabled region cannot fail the query.
func (p *ClientPool) collectClients(accountIDs []types.AwsAccountID, regions []types.AwsRegion) []*Client {
	clients := []*Client{}

	for _, accountID := range accountIDs {
		wg := sync.WaitGroup{}

		for _, region := range regions {
			wg.Add(1)

			go func(accID types.AwsAccountID, reg types.AwsRegion) {
				defer wg.Done()

				client, err := p.GetClient(accID, reg)
				if err != nil {
					log.Warn().Err(err).
						Str("accountID", string(accID)).
						Str("region", string(reg)).
						Msg("[ClientPool.collectClients] failed to init aws client. IAM role access issue or region might not be enabled. Skipping..")

					return
				}

				p.Lock()
				clients = append(clients, client)
				p.Unlock()
			}(accountID, region)
		}

		wg.Wait()
	}

	return clients
}

func (p *ClientPool) GetClient(accountID types.AwsAccountID, region types.AwsRegion) (*Client, error) {
	if client, ok := p.cachedClient(accountID, region); ok {
		return client, nil
	}

	// A recent failure for this pair is replayed without an AWS call. A region
	// the account has not enabled rejects every STS call, and one whose endpoint
	// does not route costs a full TCP timeout — without this, a wide sweep pays
	// that for every dead region on every request.
	if err := p.failures.Err(accountID, region); err != nil {
		log.Debug().Err(err).
			Stringer("accountID", accountID).
			Stringer("region", region).
			Msg("[ClientPool.GetClient] replaying cached client failure")

		return nil, err
	}

	client, err := p.buildClient(accountID, region)
	if err != nil {
		p.failures.Add(accountID, region, err)
		return nil, err
	}

	p.failures.Forget(accountID, region)
	p.setClient(accountID, region, client)

	return client, nil
}

// cachedClient reads the client map under the same lock setClient writes it
// with. collectClients calls GetClient from one goroutine per region, so an
// unsynchronised read here races the writes and can abort the process with
// "concurrent map read and map write".
func (p *ClientPool) cachedClient(accountID types.AwsAccountID, region types.AwsRegion) (*Client, bool) {
	p.Lock()
	defer p.Unlock()

	clients, ok := p.clients[accountID]
	if !ok {
		return nil, false
	}

	client, ok := clients[region]

	return client, ok
}

func (p *ClientPool) buildClient(accountID types.AwsAccountID, region types.AwsRegion) (*Client, error) {
	p.Lock()
	roleArn, assume := p.roles[accountID]
	p.Unlock()

	// If a role is configured for this account, use it
	if assume {
		log.Trace().
			Stringer("accountID", accountID).
			Stringer("region", region).
			Str("roleArn", roleArn.String()).
			Msg("[ClientPool.buildClient] creating client with assumed role")

		client, err := p.builder.AssumeClient(roleArn, region)
		if err != nil {
			return nil, errors.New(err)
		}

		return client, nil
	}

	// Use default credentials (no role assumption)
	log.Trace().
		Stringer("accountID", accountID).
		Stringer("region", region).
		Msg("[ClientPool.buildClient] creating client with default credentials")

	client, err := p.builder.LocalClient(region)
	if err != nil {
		return nil, errors.New(err)
	}

	// Verify the accountID matches
	if client.GetAccountID() != accountID {
		return nil, errors.Errorf("accountID mismatch: requested %s but got %s", accountID, client.GetAccountID())
	}

	return client, nil
}

func (p *ClientPool) ListAssumableRoleArns() ([]types.RoleArn, error) {
	p.Lock()
	defer p.Unlock()

	return dict.Values(p.roles), nil
}

func (p *ClientPool) ListAccountIDs() ([]types.AwsAccountID, error) {
	p.Lock()
	defer p.Unlock()

	if len(p.roles) > 0 {
		return dict.Keys(p.roles), nil
	}

	// If no roles configured, return the default client's account
	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return []types.AwsAccountID{}, errors.New(err)
	}

	return []types.AwsAccountID{defaultClient.GetAccountID()}, nil
}

func (p *ClientPool) setClient(accountID types.AwsAccountID, region types.AwsRegion, client *Client) {
	p.Lock()
	if _, ok := p.clients[accountID]; !ok {
		p.clients[accountID] = map[types.AwsRegion]*Client{}
	}

	p.clients[accountID][region] = client
	p.Unlock()
}
