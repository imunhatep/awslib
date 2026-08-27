// Package resourcegroupstagging reads and writes resource tags in bulk through
// the AWS Resource Groups Tagging API.
//
// It exists because the per-service tagging calls do not scale across a sweep:
// tags for N S3 buckets cost N GetBucketTagging calls, and a Cloud Control
// resource whose LIST handler omits tags costs one GetResource each. GetResources
// answers for up to 100 resources per page regardless of type, so a whole
// region's tags cost a handful of calls instead of one per resource.
//
// Three limits shape how it can be used, and none of them are fixable here:
//
//   - It is regional. There is no account-wide variant, so a cross-region answer
//     means one repository per region — which is what the client pools already
//     hand out.
//   - It is ARN-keyed. Resources whose ARN is unknown (most Cloud Control
//     resources) can only be matched heuristically; see TagIndex.
//   - It reports only resources that are currently tagged or that ever held a
//     tag. It is an enrichment source, never a resource lister: using it to
//     enumerate resources silently omits every never-tagged one.
//
// Callers also need the tag:GetResources / tag:TagResources / tag:UntagResources
// permissions, which are separate from the per-service ones.
package resourcegroupstagging

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/resourcegroupstaggingapi"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// maxArnsPerRead is the ResourceARNList limit on GetResources. Requests above it
// are rejected, so GetResourceTagsByArns chunks rather than let a caller find
// out at runtime.
const maxArnsPerRead = 100

// maxArnsPerWrite is the ARN-list limit AWS enforces on TagResources and
// UntagResources. It is five times smaller than the read limit, so a caller that
// reads a page and writes it back must re-chunk — which is why the write methods
// do it themselves.
const maxArnsPerWrite = 20

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type ResourceGroupsTaggingRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewResourceGroupsTaggingRepository(ctx context.Context, client *v3.Client) *ResourceGroupsTaggingRepository {
	repo := &ResourceGroupsTaggingRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *ResourceGroupsTaggingRepository) taggingClient(optFns ...func(*awsrgt.Options)) *awsrgt.Client {
	return resourcegroupstaggingapi.GetClient(r.client, optFns...)
}

// promLabels takes the resource type being asked about so a per-type lookup is
// attributable in metrics. Calls that span types (an unfiltered region sweep,
// the tag-key and tag-value listings) pass ResourceTypeTagMapping, which exists
// for exactly that.
func (r *ResourceGroupsTaggingRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *ResourceGroupsTaggingRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *ResourceGroupsTaggingRepository) GetAccountID() ptypes.AwsAccountID {
	return r.client.GetAccountID()
}

// chunk splits items into consecutive runs of at most size, so a caller can
// respect the API's per-request ARN limits without hand-rolling index maths.
func chunk[T any](items []T, size int) [][]T {
	if size <= 0 || len(items) == 0 {
		return nil
	}

	chunks := make([][]T, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}

		chunks = append(chunks, items[start:end])
	}

	return chunks
}
