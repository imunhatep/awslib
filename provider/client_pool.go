package provider

import (
	"context"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/gocollection/dict"
	"github.com/rs/zerolog/log"
)

// ClientPool is a concurrent map implementation to store multiple AWS clients.
type ClientPool struct {
	sync.Mutex
	ctx     context.Context
	builder *v3.ClientBuilder
	clients map[types.AwsAccountID]map[types.AwsRegion]*v3.Client

	// failures remembers (account, region) pairs that could not produce a
	// client, so a disabled or unroutable region is probed once per TTL rather
	// than on every request.
	failures *v3.FailureCache
}

// NewClientPool creates an AWS client for each permutation of the given profiles and regions.
// If profiles, regions, or both are empty, credentials and regions are picked up via the usual default provider chain,
// respectively. For example, if regions are empty, the region is first looked for via the according region environment variable
// or second the default region for each profile is used from `~/.aws/config`.
func NewClientPool(ctx context.Context, clientBuilder *v3.ClientBuilder) *ClientPool {
	clientPool := &ClientPool{
		ctx:      ctx,
		builder:  clientBuilder,
		clients:  map[types.AwsAccountID]map[types.AwsRegion]*v3.Client{},
		failures: v3.NewFailureCache(v3.DefaultClientFailureTTL),
	}

	return clientPool
}

// WithFailureTTL sets how long a client-creation failure is remembered. See
// v3.DefaultClientFailureTTL for the trade-off; a non-positive ttl restores the
// default.
func (p *ClientPool) WithFailureTTL(ttl time.Duration) *ClientPool {
	p.Lock()
	defer p.Unlock()

	p.failures = v3.NewFailureCache(ttl)

	return p
}

func (p *ClientPool) GetContext() context.Context {
	return p.ctx
}

func (p *ClientPool) GetClients(regions ...types.AwsRegion) ([]*v3.Client, error) {
	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	return p.accountClients(defaultClient.GetAccountID(), regions), nil
}

// GetAccountClients returns clients for one account only. This pool serves a
// single account — whichever the default credential chain resolves to — so any
// other account is an error rather than an empty slice, which a caller would
// otherwise read as "that account holds no resources".
func (p *ClientPool) GetAccountClients(accountID types.AwsAccountID, regions ...types.AwsRegion) ([]*v3.Client, error) {
	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	if local := defaultClient.GetAccountID(); local != accountID {
		return nil, errors.Errorf("account %s is not reachable by this client pool, which serves account %s", accountID, local)
	}

	return p.accountClients(accountID, regions), nil
}

// PoolAccountIDs reports the single account this pool serves. Unlike
// ListAccountIDs it does not depend on a client having been created first.
func (p *ClientPool) PoolAccountIDs() ([]types.AwsAccountID, error) {
	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	return []types.AwsAccountID{defaultClient.GetAccountID()}, nil
}

func (p *ClientPool) accountClients(accountID types.AwsAccountID, regions []types.AwsRegion) []*v3.Client {
	clients := []*v3.Client{}

	for _, region := range regions {
		client, err := p.GetClient(accountID, region)
		if err != nil {
			log.Error().Err(err).
				Str("accountID", string(accountID)).
				Str("region", string(region)).
				Msg("[LocalClientPool.accountClients] failed to get client, skipping")

			continue
		}

		clients = append(clients, client)
	}

	return clients
}

func (p *ClientPool) GetClient(accountID types.AwsAccountID, region types.AwsRegion) (*v3.Client, error) {
	if client, ok := p.cachedClient(accountID, region); ok {
		return client, nil
	}

	// A region this account has not enabled rejects every STS call, and one
	// whose endpoint does not route costs a full TCP timeout. Replaying the
	// remembered failure keeps a wide multi-region sweep from paying that again
	// on every request.
	if err := p.failures.Err(accountID, region); err != nil {
		log.Debug().Err(err).
			Stringer("accountID", accountID).
			Stringer("region", region).
			Msg("[LocalClientPool.GetClient] replaying cached client failure")

		return nil, err
	}

	log.Trace().
		Stringer("accountID", accountID).
		Stringer("region", region).
		Msg("[LocalClientPool.GetClient] fetching assumable roles from local iam role policies")

	client, err := p.builder.LocalClient(region)
	if err != nil {
		wrapped := errors.New(err)
		p.failures.Add(accountID, region, wrapped)

		return nil, wrapped
	}

	p.failures.Forget(accountID, region)
	p.setClient(client.GetAccountID(), region, client)

	return client, nil
}

// cachedClient reads the client map under the lock setClient writes it with.
func (p *ClientPool) cachedClient(accountID types.AwsAccountID, region types.AwsRegion) (*v3.Client, bool) {
	p.Lock()
	defer p.Unlock()

	clients, ok := p.clients[accountID]
	if !ok {
		return nil, false
	}

	client, ok := clients[region]

	return client, ok
}

func (p *ClientPool) ListAssumableRoleArns() ([]types.RoleArn, error) {
	return []types.RoleArn{}, nil
}

func (p *ClientPool) ListAccountIDs() ([]types.AwsAccountID, error) {
	log.Trace().Msg("[LocalClientPool.ListAccountIDs] fetching assumable role account ids")

	return dict.Keys(p.clients), nil
}

func (p *ClientPool) setClient(accountID types.AwsAccountID, region types.AwsRegion, client *v3.Client) {
	p.Lock()
	if _, ok := p.clients[accountID]; !ok {
		p.clients[accountID] = map[types.AwsRegion]*v3.Client{}
	}

	p.clients[accountID][region] = client
	p.Unlock()
}
