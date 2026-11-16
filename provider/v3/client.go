package v3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
)

// Client represents an AWS client - acts as config holder and service cache
type Client struct {
	callerIdentity *sts.GetCallerIdentityOutput
	accountID      types.AwsAccountID
	region         types.AwsRegion

	cfg   aws.Config
	mu    sync.Mutex
	cache sync.Map // Cache for instantiated service clients

	// failureHook is called by ReportFailure with errors that make the whole
	// (account, region) pair unusable; see OnRegionFailure.
	failureHook func(err error)

	// cached sts client for metadata
	stsClient *sts.Client
}

// NewClient creates a new AWS client
func NewClient(ctx context.Context, configProviders ...func(*config.LoadOptions) error) (*Client, error) {
	clientConf, err := config.LoadDefaultConfig(ctx, configProviders...)
	if err != nil {
		return nil, errors.New(err)
	}

	client := &Client{
		cfg: clientConf,
	}
	client.region = types.AwsRegion(clientConf.Region)

	err = client.updateAccountID(ctx)
	if err != nil {
		return nil, errors.New(err)
	}

	return client, nil
}

// NewClientForAccount creates a client for an account the caller already knows,
// without the STS GetCallerIdentity call NewClient makes to learn it.
//
// That call is per client, so per (account, region), and it is network I/O to the
// region's STS endpoint: a pool sweeping every region of every account paid one
// round trip per pair before fetching anything, and a region whose endpoint does
// not route held the whole sweep for its timeouts. The account is the same in
// every region and every pool already knows it — the role ARN's account in
// assume-role mode, the default client's account in local mode — so building a
// client this way is local: loading the config reads files, and credentials are
// fetched on first use.
func NewClientForAccount(ctx context.Context, accountID types.AwsAccountID, configProviders ...func(*config.LoadOptions) error) (*Client, error) {
	if accountID == "" {
		return NewClient(ctx, configProviders...)
	}

	clientConf, err := config.LoadDefaultConfig(ctx, configProviders...)
	if err != nil {
		return nil, errors.New(err)
	}

	return newClientFromConfig(clientConf, accountID), nil
}

// newClientFromConfig wraps a loaded config for a known account. No I/O.
func newClientFromConfig(cfg aws.Config, accountID types.AwsAccountID) *Client {
	return &Client{
		cfg:       cfg,
		accountID: accountID,
		region:    types.AwsRegion(cfg.Region),
	}
}

// OnRegionFailure registers what to do when a request made with this client
// fails in a way that says the (account, region) pair is unusable — see
// RegionFailure. Pools use it to remember the pair in their FailureCache once a
// client is built without any network call, since building it no longer proves
// the region answers. Set it before the client is shared.
func (c *Client) OnRegionFailure(hook func(err error)) *Client {
	c.failureHook = hook

	return c
}

// ReportFailure tells the client's pool that a request failed. It reports true
// when the error marks the whole (account, region) pair unusable and a hook was
// registered to remember it; any other error — access denied for one service, a
// service absent from the region, throttling — is the caller's alone and is
// ignored here.
func (c *Client) ReportFailure(err error) bool {
	if c == nil || c.failureHook == nil || !RegionFailure(err) {
		return false
	}

	c.failureHook(err)

	return true
}

// Config returns the AWS config for service creation
func (c *Client) Config() aws.Config {
	return c.cfg
}

// CacheService stores a service client (used internally by service packages)
func (c *Client) CacheService(name string, svc interface{}) {
	c.cache.Store(name, svc)
}

// GetCachedService retrieves a cached service (used internally by service packages)
func (c *Client) GetCachedService(name string) (interface{}, bool) {
	return c.cache.Load(name)
}

// GetRegion returns the AWS region
func (c *Client) GetRegion() types.AwsRegion {
	return c.region
}

// GetAccountID returns the AWS account ID
func (c *Client) GetAccountID() types.AwsAccountID {
	return c.accountID
}

// GetConfig returns the underlying AWS config
func (c *Client) GetConfig() aws.Config {
	return c.cfg
}

// Sts returns the STS client (always available)
func (c *Client) Sts() *sts.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stsClient == nil {
		c.stsClient = sts.NewFromConfig(c.cfg)
	}

	return c.stsClient
}

// GetCallerIdentity returns the caller identity
func (c *Client) GetCallerIdentity(ctx context.Context) (*sts.GetCallerIdentityOutput, error) {
	if c.callerIdentity != nil {
		return c.callerIdentity, nil
	}

	var err error
	c.callerIdentity, err = c.Sts().GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, errors.New(err)
	}

	return c.callerIdentity, nil
}

func (c *Client) updateAccountID(ctx context.Context) error {
	if c.accountID != "" {
		return nil
	}

	callerIdentity, err := c.GetCallerIdentity(ctx)
	if err != nil {
		return errors.New(err)
	}

	c.accountID = types.AwsAccountID(aws.ToString(callerIdentity.Account))
	return nil
}
