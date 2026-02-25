package v3

import (
	"context"
	"maps"
	"sync"

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
}

// NewClientPool creates an AWS client for each permutation of the given profiles and regions.
// If profiles, regions, or both are empty, credentials and regions are picked up via the usual default provider chain,
// respectively. For example, if regions are empty, the region is first looked for via the according region environment variable
// or second the default region for each profile is used from `~/.aws/config`.
func NewClientPool(ctx context.Context, clientBuilder *ClientBuilder, assumableRoles map[types.AwsAccountID]types.RoleArn) *ClientPool {
	clientPool := &ClientPool{
		ctx:     ctx,
		builder: clientBuilder,
		clients: map[types.AwsAccountID]map[types.AwsRegion]*Client{},
		roles:   maps.Clone(assumableRoles),
	}

	return clientPool
}

func (p *ClientPool) GetContext() context.Context {
	return p.ctx
}

func (p *ClientPool) GetClients(regions ...types.AwsRegion) ([]*Client, error) {
	// If no roles are set, use default client only
	if len(p.roles) == 0 {
		defaultClient, err := p.builder.DefaultClient()
		if err != nil {
			return nil, errors.New(err)
		}

		clients := []*Client{}
		accountID := defaultClient.GetAccountID()

		for _, region := range regions {
			client, err := p.GetClient(accountID, region)
			if err != nil {
				log.Warn().Err(err).
					Str("accountID", string(accountID)).
					Str("region", string(region)).
					Msg("[ClientPool.GetClients] failed to init aws client. Skipping..")
				continue
			}
			clients = append(clients, client)
		}

		return clients, nil
	}

	// Use configured roles for cross-account access
	clients := []*Client{}
	for accountID := range p.roles {
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
						Msg("[ClientPool.GetClients] failed to init aws client. IAM role access issue or region might not be enabled. Skipping..")

					return
				}

				p.Lock()
				clients = append(clients, client)
				p.Unlock()
			}(accountID, region)
		}

		wg.Wait()
	}

	return clients, nil
}

func (p *ClientPool) GetClient(accountID types.AwsAccountID, region types.AwsRegion) (*Client, error) {
	// Check cache first
	if clients, ok := p.clients[accountID]; ok {
		if client, ok := clients[region]; ok {
			return client, nil
		}
	}

	p.Lock()
	roles := p.roles
	p.Unlock()

	var client *Client
	var err error

	// If a role is configured for this account, use it
	if roleArn, ok := roles[accountID]; ok {
		log.Trace().
			Stringer("accountID", accountID).
			Stringer("region", region).
			Str("roleArn", roleArn.String()).
			Msg("[ClientPool.GetClient] creating client with assumed role")

		client, err = p.builder.AssumeClient(roleArn, region)
		if err != nil {
			return nil, errors.New(err)
		}
	} else {
		// Use default credentials (no role assumption)
		log.Trace().
			Stringer("accountID", accountID).
			Stringer("region", region).
			Msg("[ClientPool.GetClient] creating client with default credentials")

		client, err = p.builder.LocalClient(region)
		if err != nil {
			return nil, errors.New(err)
		}

		// Verify the accountID matches
		if client.GetAccountID() != accountID {
			return nil, errors.Errorf("accountID mismatch: requested %s but got %s", accountID, client.GetAccountID())
		}
	}

	p.setClient(accountID, region, client)

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
