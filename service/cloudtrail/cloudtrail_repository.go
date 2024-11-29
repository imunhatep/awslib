package cloudtrail

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	CloudTrail() *cloudtrail.Client
}

type CloudTrailRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewCloudTrailRepository(ctx context.Context, client AwsClient) *CloudTrailRepository {
	repo := &CloudTrailRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *CloudTrailRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *CloudTrailRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
