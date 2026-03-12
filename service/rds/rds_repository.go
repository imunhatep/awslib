package rds

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/rds"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type RdsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewRdsRepository(ctx context.Context, client *v3.Client) *RdsRepository {
	repo := &RdsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *RdsRepository) rdsClient() *awsrds.Client {
	return rds.GetClient(r.client)
}

func (r *RdsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *RdsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
