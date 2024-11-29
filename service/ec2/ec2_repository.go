package ec2

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	EC2() *ec2.Client
	Pricing() *pricing.Client
}

type Ec2Repository struct {
	ctx    context.Context
	client AwsClient
}

func NewEc2Repository(ctx context.Context, client AwsClient) *Ec2Repository {
	repo := &Ec2Repository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *Ec2Repository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *Ec2Repository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}
