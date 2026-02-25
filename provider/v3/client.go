package v3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
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

// GetIAMClient returns a cached or new IAM client
func (c *Client) GetIAMClient(optFns ...func(*iam.Options)) *iam.Client {
	const serviceName = "iam"

	// Check cache first
	if cached, ok := c.GetCachedService(serviceName); ok {
		return cached.(*iam.Client)
	}

	// Create new client with lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring lock
	if cached, ok := c.GetCachedService(serviceName); ok {
		return cached.(*iam.Client)
	}

	svc := iam.NewFromConfig(c.cfg, optFns...)
	c.CacheService(serviceName, svc)

	return svc
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
