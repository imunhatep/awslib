package emrserverless

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsemr "github.com/aws/aws-sdk-go-v2/service/emrserverless"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/emrserverless"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type EMRServerlessRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewEMRServerlessRepository(ctx context.Context, client *v3.Client) *EMRServerlessRepository {
	repo := &EMRServerlessRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EMRServerlessRepository) emrserverlessClient() *awsemr.Client {
	return emrserverless.GetClient(r.client)
}

func (r *EMRServerlessRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *EMRServerlessRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
