package route53

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsr53 "github.com/aws/aws-sdk-go-v2/service/route53"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/route53"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type Route53Repository struct {
	ctx    context.Context
	client *v3.Client
}

func NewRoute53Repository(ctx context.Context, client *v3.Client) *Route53Repository {
	repo := &Route53Repository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *Route53Repository) route53Client() *awsr53.Client {
	return route53.GetClient(r.client)
}

func (r *Route53Repository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *Route53Repository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
