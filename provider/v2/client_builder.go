package v2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service/iam"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"os"
	"sync"
	"time"
)

const AwsRetryAttempts = 5
const AwsRetryMaxBackoffDelay = 1 * time.Second

type ClientBuilder struct {
	sync.Mutex

	ctx         context.Context
	client      *Client
	providers   []func(*config.LoadOptions) error
	credentials map[iam.RoleArn]*aws.CredentialsCache
}

func NewClientBuilder(ctx context.Context, providers ...func(*config.LoadOptions) error) *ClientBuilder {
	builder := &ClientBuilder{
		ctx:         ctx,
		providers:   providers,
		credentials: map[iam.RoleArn]*aws.CredentialsCache{},
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

func (c *ClientBuilder) getRoleCredentials(role iam.RoleArn) (*aws.CredentialsCache, error) {
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

func (c *ClientBuilder) AssumeClient(role iam.RoleArn, region types.AwsRegion) (*Client, error) {
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

func DefaultAwsClientProviders(providers ...func(*config.LoadOptions) error) ([]func(options *config.LoadOptions) error, error) {
	log.Debug().Msg("[client.GetAwsClientProviders] creating aws client with env creds")

	// aws retry
	providers = append(providers,
		config.WithRetryMaxAttempts(AwsRetryAttempts),
		config.WithRetryer(func() aws.Retryer { return retry.AddWithMaxBackoffDelay(retry.NewStandard(), AwsRetryMaxBackoffDelay) }),
	)

	// aws config credsProvider
	envConf, err := config.NewEnvConfig()
	if err != nil {
		return providers, err
	}

	if envConf.SharedConfigProfile != "" {
		log.Debug().Str("aws_profile", envConf.SharedConfigProfile).Msg("[client.GetAwsClientProviders] aws credentials with shared profile")

		providers = append(providers, config.WithSharedConfigProfile(envConf.SharedConfigProfile))
	}

	if envConf.Credentials.HasKeys() {
		log.Debug().Str("aws_access_key_id", envConf.Credentials.AccessKeyID).Msg("[client.GetAwsClientProviders] aws credentials with static credentials")

		providers = append(providers, config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: envConf.Credentials,
		}))
	}

	if envConf.RoleARN != "" {
		log.Debug().Str("aws_role_arn", envConf.RoleARN).Msg("[client.GetAwsClientProviders] aws credentials with web identity")

		providers = append(providers, config.WithWebIdentityRoleCredentialOptions(func(options *stscreds.WebIdentityRoleOptions) {
			options.RoleSessionName = "aws_reporting@" + os.Getenv("HOSTNAME")
		}))
	}

	return providers, nil
}
