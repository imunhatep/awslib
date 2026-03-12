package batch

import (
	"context"

	awsbatch "github.com/aws/aws-sdk-go-v2/service/batch"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	batchopts "github.com/imunhatep/awslib/provider/v3/clients/batch"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type BatchRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewBatchRepository(ctx context.Context, client *v3.Client) *BatchRepository {
	repo := &BatchRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *BatchRepository) batchClient() *awsbatch.Client {
	return batchopts.GetClient(r.client)
}

func (r *BatchRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *BatchRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
