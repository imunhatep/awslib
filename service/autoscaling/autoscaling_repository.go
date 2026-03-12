package autoscaling

import (
	"context"

	awsautoscaling "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/autoscaling"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type AutoscalingRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewAsgRepository(ctx context.Context, client *v3.Client) *AutoscalingRepository {
	repo := &AutoscalingRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *AutoscalingRepository) autoscalingClient() *awsautoscaling.Client {
	return autoscaling.GetClient(r.client)
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
