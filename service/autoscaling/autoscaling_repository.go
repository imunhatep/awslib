package autoscaling

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	Autoscaling() *autoscaling.Client
}

type AutoscalingRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewAsgRepository(ctx context.Context, client AwsClient) *AutoscalingRepository {
	repo := &AutoscalingRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *AutoscalingRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *AutoscalingRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
