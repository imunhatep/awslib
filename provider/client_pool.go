package provider

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/provider/v2"
	"github.com/imunhatep/awslib/service/iam"
	"github.com/imunhatep/gocollection/dict"
	"github.com/rs/zerolog/log"
	"sync"
)

// ClientPool is a concurrent map implementation to store multiple AWS clients.
type ClientPool struct {
	sync.Mutex
	ctx     context.Context
	builder *v2.ClientBuilder
	clients map[types.AwsAccountID]map[types.AwsRegion]*v2.Client
}

// NewClientPool creates an AWS client for each permutation of the given profiles and regions.
// If profiles, regions, or both are empty, credentials and regions are picked up via the usual default provider chain,
// respectively. For example, if regions are empty, the region is first looked for via the according region environment variable
// or second the default region for each profile is used from `~/.aws/config`.
func NewClientPool(ctx context.Context, clientBuilder *v2.ClientBuilder) *ClientPool {
	clientPool := &ClientPool{
		ctx:     ctx,
		builder: clientBuilder,
		clients: map[types.AwsAccountID]map[types.AwsRegion]*v2.Client{},
	}

	return clientPool
}

func (p *ClientPool) GetContext() context.Context {
	return p.ctx
}

func (p *ClientPool) GetClients(regions ...types.AwsRegion) ([]*v2.Client, error) {
	clients := []*v2.Client{}

	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	accountID := defaultClient.GetAccountID()
	for _, region := range regions {
		client, err := p.GetClient(accountID, region)
		if err != nil {
			log.Error().Err(err).
				Str("accountID", string(accountID)).
				Str("region", string(region)).
				Msg("[LocalClientPool.GetClients] failed to get client, skipping")

			continue
		}

		clients = append(clients, client)
	}

	return clients, nil
}

func (p *ClientPool) GetClient(accountID types.AwsAccountID, region types.AwsRegion) (*v2.Client, error) {
	if clients, ok := p.clients[accountID]; ok {
		if client, ok := clients[region]; ok {
			return client, nil
		}
	}

	log.Trace().
		Stringer("accountID", accountID).
		Stringer("region", region).
		Msg("[LocalClientPool.GetClient] fetching assumable roles from local iam role policies")

	client, err := p.builder.LocalClient(region)
	if err != nil {
		return nil, errors.New(err)
	}

	p.setClient(client.GetAccountID(), region, client)

	return client, nil
}

func (p *ClientPool) ListAssumableRoleArns() ([]iam.RoleArn, error) {
	return []iam.RoleArn{}, nil
}

func (p *ClientPool) ListAccountIDs() ([]types.AwsAccountID, error) {
	log.Trace().Msg("[LocalClientPool.ListAccountIDs] fetching assumable role account ids")

	return dict.Keys(p.clients), nil
}

func (p *ClientPool) setClient(accountID types.AwsAccountID, region types.AwsRegion, client *v2.Client) {
	p.Lock()
	if _, ok := p.clients[accountID]; !ok {
		p.clients[accountID] = map[types.AwsRegion]*v2.Client{}
	}

	p.clients[accountID][region] = client
	p.Unlock()
}
