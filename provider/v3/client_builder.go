package v3

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
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

// Connection timeouts applied to every client this package builds.
//
// The SDK's default dialer waits 30s for a connection, and the retryer repeats
// it up to AwsRetryAttempts times, so a region whose endpoint does not route —
// me-south-1 from some networks — held a request for minutes. A working endpoint
// connects in milliseconds; these bound only the dead ones. A slow answer from a
// working endpoint is not affected: neither limits how long a response may take.
const (
	AwsDialTimeout         = 5 * time.Second
	AwsTLSHandshakeTimeout = 5 * time.Second
)

// DefaultHTTPClient is the HTTP client the builder gives every config: the SDK's
// buildable client with AwsDialTimeout and AwsTLSHandshakeTimeout. A provider
// passed to NewClientBuilder may still replace it with config.WithHTTPClient.
func DefaultHTTPClient() *awshttp.BuildableClient {
	return awshttp.NewBuildableClient().
		WithDialerOptions(func(d *net.Dialer) { d.Timeout = AwsDialTimeout }).
		WithTransportOptions(func(t *http.Transport) { t.TLSHandshakeTimeout = AwsTLSHandshakeTimeout })
}

// NewRetryer is the retryer every client this package builds uses: the SDK's
// standard retryer with AwsRetryAttempts and AwsRetryMaxBackoffDelay, except that
// a failure to connect (ConnectFailure) is not retried. An endpoint that has not
// connected within AwsDialTimeout is dead, not slow, and retrying it only
// multiplies the wait — five attempts plus backoff held each unreachable region
// for over half a minute. The pool's FailureCache remembers the failure instead.
//
// Pass it to config.WithRetryer when building a config outside ClientBuilder.
func NewRetryer() aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = AwsRetryAttempts
		o.MaxBackoff = AwsRetryMaxBackoffDelay
		o.Retryables = append([]retry.IsErrorRetryable{noRetryOnConnectFailure}, o.Retryables...)
	})
}

// noRetryOnConnectFailure goes first in the retryer's chain, where the first
// definite answer wins, so it overrides the SDK's own "connection errors are
// retryable" rule for exactly the failures ConnectFailure names.
var noRetryOnConnectFailure = retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
	if ConnectFailure(err) {
		return aws.FalseTernary
	}

	return aws.UnknownTernary
})

type ClientBuilder struct {
	sync.Mutex

	ctx         context.Context
	client      *Client
	providers   []func(*config.LoadOptions) error
	credentials map[types.RoleArn]*aws.CredentialsCache

	// bases holds one loaded config per credential source — "" for the
	// builder's own credentials, else the role ARN — from which region clients
	// are copied. See regionClient.
	bases map[types.RoleArn]aws.Config
	// loadMu serialises loading a base: collectClients builds regions
	// concurrently, and getRoleCredentials and DefaultClient are not safe to
	// enter from several goroutines at once.
	loadMu sync.Mutex
	// defaultMu guards client, which DefaultClient builds once and pools then
	// read from concurrent goroutines.
	defaultMu sync.Mutex
}

func NewClientBuilder(ctx context.Context, providers ...func(*config.LoadOptions) error) *ClientBuilder {
	builder := &ClientBuilder{
		ctx:         ctx,
		providers:   providers,
		credentials: map[types.RoleArn]*aws.CredentialsCache{},
		bases:       map[types.RoleArn]aws.Config{},
	}

	return builder
}

func (c *ClientBuilder) DefaultClient() (*Client, error) {
	c.defaultMu.Lock()
	defer c.defaultMu.Unlock()

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
	c.Lock()
	creds, ok := c.credentials[role]
	c.Unlock()

	if ok {
		return creds, nil
	}

	log.Trace().Str("role", role.String()).Msg("[ClientBuilder.getRoleCredentials] getting assumed role credentials")

	client, err := c.DefaultClient()
	if err != nil {
		return nil, errors.New(err)
	}

	c.Lock()
	defer c.Unlock()

	// Concurrent callers may race here; keep the first cache so every client
	// of the role shares it.
	if creds, ok := c.credentials[role]; ok {
		return creds, nil
	}

	roleCredentials := stscreds.NewAssumeRoleProvider(client.Sts(), role.String())
	c.credentials[role] = aws.NewCredentialsCache(roleCredentials)

	return c.credentials[role], nil
}

func (c *ClientBuilder) getProviders(providers ...func(*config.LoadOptions) error) []func(*config.LoadOptions) error {
	// The default HTTP client and retryer go first so a caller's own
	// WithHTTPClient or WithRetryer, in the builder's providers or these, still
	// wins.
	cfgProviders := []func(*config.LoadOptions) error{
		config.WithHTTPClient(DefaultHTTPClient()),
		config.WithRetryer(NewRetryer),
	}
	cfgProviders = append(cfgProviders, c.providers...)

	return append(cfgProviders, providers...)
}

// LocalClientForAccount is LocalClient for an account the caller already knows:
// no STS call, no network I/O. Every region shares one loaded config, and with
// it one credentials cache, so a profile's credentials are fetched once rather
// than once per region.
//
// accountID is taken on trust. A pool should pass the account its default
// client resolved, which is the account these credentials belong to.
func (c *ClientBuilder) LocalClientForAccount(accountID types.AwsAccountID, region types.AwsRegion) (*Client, error) {
	log.Debug().Str("account", accountID.String()).Str("region", region.String()).Msg("[ClientBuilder.LocalClientForAccount] building client")

	return c.regionClient("", accountID, region)
}

// AssumeClientForAccount is AssumeClient for the account the role belongs to,
// without an STS call per region: the role's credentials are assumed on first
// use and shared by every region.
//
// accountID must be the account in the role's ARN. AssumeClient learned the
// account from GetCallerIdentity under the role, so it reported the account it
// really reached; with no call to learn it from, the ARN is the only authority,
// and a mismatch — one typo in an accountID=roleArn list — would otherwise label
// every resource fetched with the client as another account's.
func (c *ClientBuilder) AssumeClientForAccount(accountID types.AwsAccountID, role types.RoleArn, region types.AwsRegion) (*Client, error) {
	log.Debug().Str("account", accountID.String()).Str("role", role.String()).Str("region", region.String()).Msg("[ClientBuilder.AssumeClientForAccount] building client")

	if role == "" {
		return nil, errors.New("AssumeClientForAccount needs a role")
	}

	if roleAccount := roleArnAccount(role); roleAccount != accountID.String() {
		return nil, errors.Errorf("role %s belongs to account %q, not %s", role, roleAccount, accountID)
	}

	return c.regionClient(role, accountID, region)
}

// roleArnAccount returns the account segment of a role ARN
// (arn:<partition>:iam::<account>:role/<name>), or "" when it has none.
func roleArnAccount(role types.RoleArn) string {
	parts := strings.SplitN(role.String(), ":", 6)
	if len(parts) != 6 || parts[0] != "arn" {
		return ""
	}

	return parts[4]
}

// regionClient copies the credential source's base config into region. Copying
// keeps the credentials cache, the retryer and the HTTP client shared, and only
// the region — which decides every service client's endpoint — differs.
func (c *ClientBuilder) regionClient(role types.RoleArn, accountID types.AwsAccountID, region types.AwsRegion) (*Client, error) {
	base, err := c.baseConfig(role)
	if err != nil {
		return nil, errors.New(err)
	}

	cfg := base.Copy()
	cfg.Region = region.String()

	return newClientFromConfig(cfg, accountID), nil
}

// baseConfig loads, once per credential source, the config region clients are
// copied from. Loading reads the shared config files; it makes no AWS call.
func (c *ClientBuilder) baseConfig(role types.RoleArn) (aws.Config, error) {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()

	c.Lock()
	base, ok := c.bases[role]
	c.Unlock()

	if ok {
		return base, nil
	}

	providers := []func(*config.LoadOptions) error{config.WithRegion(types.DefaultAwsRegion.String())}

	if role != "" {
		roleCredentials, err := c.getRoleCredentials(role)
		if err != nil {
			return aws.Config{}, errors.New(err)
		}

		providers = append(providers, config.WithCredentialsProvider(roleCredentials))
	}

	base, err := config.LoadDefaultConfig(c.ctx, c.getProviders(providers...)...)
	if err != nil {
		return aws.Config{}, errors.New(err)
	}

	c.Lock()
	c.bases[role] = base
	c.Unlock()

	return base, nil
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
		config.WithRetryer(NewRetryer),
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
