package cloudtrail

import (
	"context"

	awscloudtrail "github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	cloudtrailopts "github.com/imunhatep/awslib/provider/v3/clients/cloudtrail"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type CloudTrailRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewCloudTrailRepository(ctx context.Context, client *v3.Client) *CloudTrailRepository {
	repo := &CloudTrailRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *CloudTrailRepository) cloudtrailClient() *awscloudtrail.Client {
	return cloudtrailopts.GetClient(r.client)
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
