package v3

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

// Retry settings applied to every client this package builds.
//
// Both are passed to retry.NewStandard as options rather than through
// config.WithRetryMaxAttempts, and that is not a style choice — see the comment in
// DefaultAwsClientProviders for why the config-level option cannot work here.
const (
	// AwsRetryAttempts is the total number of attempts per request, the first
	// included.
	AwsRetryAttempts = 5
	// AwsRetryMaxBackoffDelay caps the delay between attempts.
	//
	// This was 3s, which is too short for the case retries exist for. A throttled
	// request answered with ThrottlingException needs the backoff to grow past the
	// window the service is rate-limiting over; capped at 3s, all attempts land inside
	// that window, fail, and the request is abandoned having waited under 10 seconds
	// in total. 20s is the SDK's own default and leaves room for the exponential
	// growth to do its job.
	AwsRetryMaxBackoffDelay = 20 * time.Second
)

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

func DefaultAwsClientProviders(providers ...func(*config.LoadOptions) error) ([]func(options *config.LoadOptions) error, error) {
	log.Debug().Msg("[client.GetAwsClientProviders] creating aws client with env creds")

	// AWS retry.
	//
	// Both settings go inside retry.NewStandard, and config.WithRetryMaxAttempts is
	// deliberately not used alongside it. Supplying a Retryer makes the SDK skip the
	// retry options entirely — config.resolveRetryer assigns cfg.Retryer and returns
	// before it reads them, commented there as "Only load the retry options if a
	// custom retryer has not be specified". So cfg.RetryMaxAttempts stays 0, the
	// service client's finalizeRetryMaxAttempts sees 0 and does nothing, and the
	// retryer keeps retry.DefaultMaxAttempts, which is 3.
	//
	// That is what the pair of options here used to do: AwsRetryAttempts was passed
	// in, silently discarded, and every client retried 3 times. Errors read
	// "exceeded maximum number of attempts, 3" while the constant said 5.
	commonProviders := []func(*config.LoadOptions) error{
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = AwsRetryAttempts
				o.MaxBackoff = AwsRetryMaxBackoffDelay
			})
		}),
	}

	// aws config credsProvider
	envConf, err := config.NewEnvConfig()
	if err != nil {
		return providers, err
	}

	if envConf.SharedConfigProfile != "" {
		log.Debug().Str("aws_profile", envConf.SharedConfigProfile).Msg("[client.GetAwsClientProviders] aws credentials with shared profile")

		commonProviders = append(commonProviders, config.WithSharedConfigProfile(envConf.SharedConfigProfile))
	}

	if envConf.Credentials.HasKeys() {
		log.Debug().Str("aws_access_key_id", envConf.Credentials.AccessKeyID).Msg("[client.GetAwsClientProviders] aws credentials with static credentials")

		commonProviders = append(commonProviders, config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: envConf.Credentials,
		}))
	}

	if envConf.RoleARN != "" {
		log.Debug().Str("aws_role_arn", envConf.RoleARN).Msg("[client.GetAwsClientProviders] aws credentials with web identity")

		commonProviders = append(commonProviders, config.WithWebIdentityRoleCredentialOptions(func(options *stscreds.WebIdentityRoleOptions) {
			options.RoleSessionName = "aws_reporting@" + os.Getenv("HOSTNAME")
		}))
	}

	providers = append(commonProviders, providers...)

	return providers, nil
}
