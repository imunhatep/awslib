package savingsplans

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awssp "github.com/aws/aws-sdk-go-v2/service/savingsplans"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/savingsplans"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// SavingsPlansRepository reads Savings Plans inventory (what an account has bought)
// and offering rates (what a commitment would cost).
//
// The Savings Plans API is partition-global — every client resolves to
// savingsplans.amazonaws.com — so unlike a regional repository this one's client
// region tells you nothing about the answer. Inventory is account-wide and offering
// rates take the region as a query filter (see OfferingRatesQuery.Region). Nothing
// here reads r.client.GetRegion() to decide what to ask for, and callers must not
// infer a region from the repository they happen to hold.
//
// Nothing in this package is a service.ResourceInterface: a purchased plan is closer
// to a contract than a resource, and a rate is a price. They are deliberately absent
// from proxy.RepoProxy.FindAll and from cfg.ResourceTypeList — the resource types
// below exist only to label metrics.
type SavingsPlansRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewSavingsPlansRepository(ctx context.Context, client *v3.Client) *SavingsPlansRepository {
	repo := &SavingsPlansRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *SavingsPlansRepository) savingsPlansClient() *awssp.Client {
	return savingsplans.GetClient(r.client)
}

func (r *SavingsPlansRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *SavingsPlansRepository) GetAccountID() ptypes.AwsAccountID {
	return r.client.GetAccountID()
}

func (r *SavingsPlansRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}
