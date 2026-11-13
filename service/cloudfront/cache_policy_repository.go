package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListCachePoliciesAll lists every cache policy visible to the account, managed and
// custom: a request without a Type filter returns both.
func (r *CloudFrontRepository) ListCachePoliciesAll() ([]CachePolicy, error) {
	return r.ListCachePoliciesByInput(&awscf.ListCachePoliciesInput{})
}

// ListCachePoliciesByInput lists cache policys, following NextMarker to the end. Set
// query.Type to "managed" or "custom" to narrow the listing.
func (r *CloudFrontRepository) ListCachePoliciesByInput(query *awscf.ListCachePoliciesInput) ([]CachePolicy, error) {
	start := time.Now()

	input := awscf.ListCachePoliciesInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListCachePolicies", ccfg.ResourceTypeCloudFrontCachePolicy, input.Marker,
		func(marker *string) ([]cftypes.CachePolicySummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListCachePolicies(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.CachePolicyList == nil {
				return nil, nil, nil
			}

			return output.CachePolicyList.Items, output.CachePolicyList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	policies := make([]CachePolicy, 0, len(summaries))
	for _, summary := range summaries {
		if summary.CachePolicy == nil {
			continue
		}

		policies = append(policies, NewCachePolicy(r.client, *summary.CachePolicy, summary.Type))
	}

	r.observeList("ListCachePolicies", "ListCachePoliciesByInput", ccfg.ResourceTypeCloudFrontCachePolicy, len(policies), start)

	return policies, nil
}
