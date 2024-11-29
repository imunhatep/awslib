package ecs

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	ECS() *ecs.Client
}

type EcsRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewEcsRepository(ctx context.Context, client AwsClient) *EcsRepository {
	repo := &EcsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
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
