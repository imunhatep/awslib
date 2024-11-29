package athena

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	Athena() *athena.Client
}

type AthenaRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewAthenaRepository(ctx context.Context, client AwsClient) *AthenaRepository {
	repo := &AthenaRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *AthenaRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *AthenaRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
