package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListOriginRequestPoliciesAll lists every origin request policy visible to the account, managed and
// custom: a request without a Type filter returns both.
func (r *CloudFrontRepository) ListOriginRequestPoliciesAll() ([]OriginRequestPolicy, error) {
	return r.ListOriginRequestPoliciesByInput(&awscf.ListOriginRequestPoliciesInput{})
}

// ListOriginRequestPoliciesByInput lists origin request policys, following NextMarker to the end. Set
// query.Type to "managed" or "custom" to narrow the listing.
func (r *CloudFrontRepository) ListOriginRequestPoliciesByInput(query *awscf.ListOriginRequestPoliciesInput) ([]OriginRequestPolicy, error) {
	start := time.Now()

	input := awscf.ListOriginRequestPoliciesInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListOriginRequestPolicies", ccfg.ResourceTypeCloudFrontOriginRequestPolicy, input.Marker,
		func(marker *string) ([]cftypes.OriginRequestPolicySummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListOriginRequestPolicies(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.OriginRequestPolicyList == nil {
				return nil, nil, nil
			}

			return output.OriginRequestPolicyList.Items, output.OriginRequestPolicyList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	policies := make([]OriginRequestPolicy, 0, len(summaries))
	for _, summary := range summaries {
		if summary.OriginRequestPolicy == nil {
			continue
		}

		policies = append(policies, NewOriginRequestPolicy(r.client, *summary.OriginRequestPolicy, summary.Type))
	}

	r.observeList("ListOriginRequestPolicies", "ListOriginRequestPoliciesByInput", ccfg.ResourceTypeCloudFrontOriginRequestPolicy, len(policies), start)

	return policies, nil
}
