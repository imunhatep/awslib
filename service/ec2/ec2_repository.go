package ec2

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/ec2"
	pricingopts "github.com/imunhatep/awslib/provider/v3/clients/pricing"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type Ec2Repository struct {
	ctx    context.Context
	client *v3.Client
}

func NewEc2Repository(ctx context.Context, client *v3.Client) *Ec2Repository {
	repo := &Ec2Repository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *Ec2Repository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *Ec2Repository) ec2Client() *awsec2.Client {
	return ec2.GetClient(r.client)
}

func (r *Ec2Repository) pricingClient() *pricing.Client {
	return pricingopts.GetClient(r.client)
}

func (r *Ec2Repository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}
