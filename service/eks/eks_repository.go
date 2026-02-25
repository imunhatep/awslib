package eks

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/eks"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type EksRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewEksRepository(ctx context.Context, client *v3.Client) *EksRepository {
	repo := &EksRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EksRepository) eksClient() *awseks.Client {
	return eks.GetClient(r.client)
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
