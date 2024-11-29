package eks

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	EKS() *eks.Client
}

type EksRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewEksRepository(ctx context.Context, client AwsClient) *EksRepository {
	repo := &EksRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EksRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *EksRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
