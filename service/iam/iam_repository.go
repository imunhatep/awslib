package iam

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/iam"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type IamRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewIamRepository(ctx context.Context, client *v3.Client) *IamRepository {
	repo := &IamRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *IamRepository) iamClient() *awsiam.Client {
	return iam.GetClient(r.client)
}

func (r *IamRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *IamRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}
