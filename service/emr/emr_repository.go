package emr

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/emr"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	EMR() *emr.Client
}

type EmrRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewEmrRepository(ctx context.Context, client AwsClient) *EmrRepository {
	repo := &EmrRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EmrRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *EmrRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
