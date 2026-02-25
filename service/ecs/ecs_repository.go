package ecs

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/ecs"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type EcsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewEcsRepository(ctx context.Context, client *v3.Client) *EcsRepository {
	repo := &EcsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EcsRepository) ecsClient() *awsecs.Client {
	return ecs.GetClient(r.client)
}

func (r *EcsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *EcsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
