package glue

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsglue "github.com/aws/aws-sdk-go-v2/service/glue"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/glue"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type GlueRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewGlueRepository(ctx context.Context, client *v3.Client) *GlueRepository {
	repo := &GlueRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *GlueRepository) glueClient() *awsglue.Client {
	return glue.GetClient(r.client)
}

func (r *GlueRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *GlueRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
