package v2

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

const AwsRetryAttempts = 5
const AwsRetryMaxBackoffDelay = 1 * time.Second

type ClientBuilder struct {
	sync.Mutex

	ctx         context.Context
	client      *Client
	providers   []func(*config.LoadOptions) error
	credentials map[types.RoleArn]*aws.CredentialsCache
}

func NewClientBuilder(ctx context.Context, providers ...func(*config.LoadOptions) error) *ClientBuilder {
	builder := &ClientBuilder{
		ctx:         ctx,
		providers:   providers,
		credentials: map[types.RoleArn]*aws.CredentialsCache{},
	}

	return builder
}

func (c *ClientBuilder) DefaultClient() (*Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	log.Debug().
		Str("region", types.DefaultAwsRegion.String()).
		Msg("[ClientBuilder.DefaultClient] creating default client")

	client, err := NewClient(c.ctx, c.getProviders(config.WithRegion(types.DefaultAwsRegion.String()))...)
	if err != nil {
		return nil, errors.New(err)
	}

	c.client = client

	return client, nil
}

func (c *ClientBuilder) getRoleCredentials(role types.RoleArn) (*aws.CredentialsCache, error) {
	if creds, ok := c.credentials[role]; ok {
		return creds, nil
	}

	log.Trace().Str("role", role.String()).Msg("[ClientBuilder.getRoleCredentials] getting assumed role credentials")

	client, err := c.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	c.Lock()
	defer c.Unlock()

	roleCredentials := stscreds.NewAssumeRoleProvider(client.Sts(), role.String())
	c.credentials[role] = aws.NewCredentialsCache(roleCredentials)

	return c.credentials[role], nil
}

func (c *ClientBuilder) getProviders(providers ...func(*config.LoadOptions) error) []func(*config.LoadOptions) error {
	cfgProviders := slice.Copy(c.providers)
	return append(cfgProviders, providers...)
}

func (c *ClientBuilder) AssumeClient(role types.RoleArn, region types.AwsRegion) (*Client, error) {
	log.Debug().Str("role", role.String()).Str("region", region.String()).Msg("[ClientBuilder.AssumeClient] assuming client")

	roleCredentials, err := c.getRoleCredentials(role)
	if err != nil {
		return nil, errors.New(err)
	}

	cfgProviders := c.getProviders(config.WithCredentialsProvider(roleCredentials), config.WithRegion(region.String()))
	client, err := NewClient(c.ctx, cfgProviders...)
	if err != nil {
		return nil, errors.New(err)
	}

	return client, nil
}

func (c *ClientBuilder) LocalClient(region types.AwsRegion) (*Client, error) {
	log.Debug().Str("region", region.String()).Msg("[ClientBuilder.AssumeClient] assuming client")

	cfgProviders := c.getProviders(config.WithRegion(region.String()))
	client, err := NewClient(c.ctx, cfgProviders...)
	if err != nil {
		return nil, errors.New(err)
	}

	return client, nil
}
