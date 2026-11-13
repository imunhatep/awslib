package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListResponseHeadersPoliciesAll lists every response headers policy visible to the account, managed and
// custom: a request without a Type filter returns both.
func (r *CloudFrontRepository) ListResponseHeadersPoliciesAll() ([]ResponseHeadersPolicy, error) {
	return r.ListResponseHeadersPoliciesByInput(&awscf.ListResponseHeadersPoliciesInput{})
}

// ListResponseHeadersPoliciesByInput lists response headers policys, following NextMarker to the end. Set
// query.Type to "managed" or "custom" to narrow the listing.
func (r *CloudFrontRepository) ListResponseHeadersPoliciesByInput(query *awscf.ListResponseHeadersPoliciesInput) ([]ResponseHeadersPolicy, error) {
	start := time.Now()

	input := awscf.ListResponseHeadersPoliciesInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListResponseHeadersPolicies", ccfg.ResourceTypeCloudFrontResponseHeadersPolicy, input.Marker,
		func(marker *string) ([]cftypes.ResponseHeadersPolicySummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListResponseHeadersPolicies(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.ResponseHeadersPolicyList == nil {
				return nil, nil, nil
			}

			return output.ResponseHeadersPolicyList.Items, output.ResponseHeadersPolicyList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	policies := make([]ResponseHeadersPolicy, 0, len(summaries))
	for _, summary := range summaries {
		if summary.ResponseHeadersPolicy == nil {
			continue
		}

		policies = append(policies, NewResponseHeadersPolicy(r.client, *summary.ResponseHeadersPolicy, summary.Type))
	}

	r.observeList("ListResponseHeadersPolicies", "ListResponseHeadersPoliciesByInput", ccfg.ResourceTypeCloudFrontResponseHeadersPolicy, len(policies), start)

	return policies, nil
}
