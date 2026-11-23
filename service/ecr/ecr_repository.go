// Package ecr lists ECR repositories with their tags.
//
// The entity is named Repo, not Repository: the cache and gob generators treat
// every exported struct whose name ends in "Repository" as a repository — the
// first would wrap its Get* methods in a cached type, the second would skip
// registering it.
package ecr

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsecr "github.com/aws/aws-sdk-go-v2/service/ecr"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	ecrclient "github.com/imunhatep/awslib/provider/v3/clients/ecr"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// tagFetchConcurrency and tagFetchMaxAttempts shape the per-repository tag
// reads of a listing. DescribeRepositories does not return tags, so a listing
// is unavoidably 1+N calls. The values follow the WAFv2 per-object reads: a
// modest fan-out with a retry budget large enough that throttling does not
// fail the listing.
const (
	tagFetchConcurrency = 4
	tagFetchMaxAttempts = 10
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type EcrRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewEcrRepository(ctx context.Context, client *v3.Client) *EcrRepository {
	repo := &EcrRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EcrRepository) ecrClient() *awsecr.Client {
	return ecrclient.GetClient(r.client)
}

func (r *EcrRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *EcrRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}
