package v2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service/iam"
	"github.com/imunhatep/gocollection/dict"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"strings"
	"sync"
)

// ClientPool is a concurrent map implementation to store multiple AWS clients.
type ClientPool struct {
	sync.Mutex
	ctx     context.Context
	builder *ClientBuilder

	// lists
	clients map[types.AwsAccountID]map[types.AwsRegion]*Client
	roles   map[types.AwsAccountID]iam.RoleArn
}

// NewClientPool creates an AWS client for each permutation of the given profiles and regions.
// If profiles, regions, or both are empty, credentials and regions are picked up via the usual default provider chain,
// respectively. For example, if regions are empty, the region is first looked for via the according region environment variable
// or second the default region for each profile is used from `~/.aws/config`.
func NewClientPool(ctx context.Context, clientBuilder *ClientBuilder) *ClientPool {
	clientPool := &ClientPool{
		ctx:     ctx,
		builder: clientBuilder,
		clients: map[types.AwsAccountID]map[types.AwsRegion]*Client{},
		roles:   map[types.AwsAccountID]iam.RoleArn{},
	}

	return clientPool
}

func (p *ClientPool) GetContext() context.Context {
	return p.ctx
}

func (p *ClientPool) GetClients(regions ...types.AwsRegion) ([]*Client, error) {
	roles, err := p.getAssumableRoles()
	if err != nil {
		return nil, errors.New(err)
	}

	clients := []*Client{}
	for accountID, _ := range roles {
		wg := sync.WaitGroup{}

		for _, region := range regions {
			wg.Add(1)

			go func() {
				defer wg.Done()

				client, err := p.GetClient(accountID, region)
				if err != nil {
					log.Warn().Err(err).
						Str("accountID", string(accountID)).
						Str("region", string(region)).
						Msg("[ClientPool.GetClients] failed to init aws client. IAM role access issue or region might not be enabled. Skipping..")

					return
				}

				clients = append(clients, client)
			}()
		}

		wg.Wait()
	}

	return clients, nil
}

func (p *ClientPool) GetClient(accountID types.AwsAccountID, region types.AwsRegion) (*Client, error) {
	if clients, ok := p.clients[accountID]; ok {
		if client, ok := clients[region]; ok {
			return client, nil
		}
	}

	log.Trace().
		Stringer("accountID", accountID).
		Stringer("region", region).
		Msg("[ClientPool.GetClient] fetching assumable roles from local iam role policies")

	roles, err := p.getAssumableRoles()
	if err != nil {
		return nil, errors.New(err)
	}

	roleArn, ok := roles[accountID]
	if !ok {
		return nil, errors.New("role not found")
	}

	client, err := p.builder.AssumeClient(roleArn, region)
	if err != nil {
		return nil, errors.New(err)
	}

	p.setClient(accountID, region, client)

	return client, nil
}

func (p *ClientPool) ListAssumableRoleArns() ([]iam.RoleArn, error) {
	log.Trace().Msg("[ClientPool.ListAssumableRoleArns] fetching assumable role arns")

	roles, err := p.getAssumableRoles()
	if err != nil {
		return []iam.RoleArn{}, errors.New(err)
	}

	return dict.Values(roles), nil
}

func (p *ClientPool) ListAccountIDs() ([]types.AwsAccountID, error) {
	log.Trace().Msg("[ClientPool.ListAccountIDs] fetching assumable role account ids")

	roles, err := p.getAssumableRoles()
	if err != nil {
		return []types.AwsAccountID{}, errors.New(err)
	}

	return dict.Keys(roles), nil
}

func (p *ClientPool) setClient(accountID types.AwsAccountID, region types.AwsRegion, client *Client) {
	p.Lock()
	if _, ok := p.clients[accountID]; !ok {
		p.clients[accountID] = map[types.AwsRegion]*Client{}
	}

	p.clients[accountID][region] = client
	p.Unlock()
}

// getAssumableRoles fetches the assumable roles by parsing the local IAM role policies
func (p *ClientPool) getAssumableRoles() (map[types.AwsAccountID]iam.RoleArn, error) {
	p.Lock()
	defer p.Unlock()

	if len(p.roles) > 0 {
		return p.roles, nil
	}

	log.Trace().Msg("[ClientPool.getAssumableRoles] fetching assumable roles from local iam role policies")

	defaultClient, err := p.builder.DefaultClient()
	if err != nil {
		return p.roles, errors.New(err)
	}

	// Get the caller identity
	callerIdentity, err := defaultClient.GetCallerIdentity(p.ctx)
	if err != nil {
		return p.roles, errors.New(err)
	}

	// Parse the callerIdentity to fetch role ARN
	roleArn, err := arn.Parse(aws.ToString(callerIdentity.Arn))
	if err != nil {
		return p.roles, errors.New(err)
	}

	// Assuming the role name is in the format "role/RoleName"
	var roleName string
	if parts := strings.Split(roleArn.Resource, "/"); len(parts) > 1 {
		roleName = parts[1]
	} else {
		log.Error().
			Str("roleArn", aws.ToString(callerIdentity.Arn)).
			Msg("[ClientPool.getAssumableRoles] invalid role ARN format")

		return p.roles, errors.New("invalid role ARN format")
	}

	log.Debug().
		Stringer("roleArn", roleArn).
		Str("roleName", roleName).
		Msg("[ClientPool.getAssumableRoles] fetching assumable roles from local iam role policies")

	iamRepo := iam.NewIamRepository(p.ctx, defaultClient)
	versions, err := iamRepo.ListAttachedRolePolicyVersionsByRoleName(roleName)
	if err != nil {
		return p.roles, errors.New(err)
	}
	roleArnList := []iam.RoleArn{}

	// fetch
	for _, version := range versions {
		assumedRoles := iamRepo.ListAssumedRoleArn(version)
		roleArnList = append(roleArnList, assumedRoles...)
	}

	log.Trace().
		Strs("roleArnList", slice.Map(roleArnList, func(v iam.RoleArn) string { return string(v) })).
		Msg("[ClientPool.getAssumableRoles] fetched assumable roles from local iam role policies")

	for _, assumedRoleArn := range roleArnList {
		arnRoleArn, err := arn.Parse(assumedRoleArn.String())
		if err != nil {
			return p.roles, errors.New(err)
		}

		log.Trace().
			Str("accountID", arnRoleArn.AccountID).
			Str("role", assumedRoleArn.String()).
			Msg("[ClientPool.getAssumableRoles] adding assumed role mapping")

		p.roles[types.AwsAccountID(arnRoleArn.AccountID)] = assumedRoleArn
	}

	return p.roles, nil
}
