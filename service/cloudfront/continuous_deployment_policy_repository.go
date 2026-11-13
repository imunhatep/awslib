package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListContinuousDeploymentPoliciesAll lists every continuous deployment policy in the account.
func (r *CloudFrontRepository) ListContinuousDeploymentPoliciesAll() ([]ContinuousDeploymentPolicy, error) {
	return r.ListContinuousDeploymentPoliciesByInput(&awscf.ListContinuousDeploymentPoliciesInput{})
}

// ListContinuousDeploymentPoliciesByInput lists continuous deployment policys, following NextMarker to the end.
func (r *CloudFrontRepository) ListContinuousDeploymentPoliciesByInput(query *awscf.ListContinuousDeploymentPoliciesInput) ([]ContinuousDeploymentPolicy, error) {
	start := time.Now()

	input := awscf.ListContinuousDeploymentPoliciesInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListContinuousDeploymentPolicies", ccfg.ResourceTypeCloudFrontContinuousDeploymentPolicy, input.Marker,
		func(marker *string) ([]cftypes.ContinuousDeploymentPolicySummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListContinuousDeploymentPolicies(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.ContinuousDeploymentPolicyList == nil {
				return nil, nil, nil
			}

			return output.ContinuousDeploymentPolicyList.Items, output.ContinuousDeploymentPolicyList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]ContinuousDeploymentPolicy, 0, len(summaries))
	for _, summary := range summaries {
		if summary.ContinuousDeploymentPolicy == nil {
			continue
		}

		items = append(items, NewContinuousDeploymentPolicy(r.client, *summary.ContinuousDeploymentPolicy))
	}

	r.observeList("ListContinuousDeploymentPolicies", "ListContinuousDeploymentPoliciesByInput", ccfg.ResourceTypeCloudFrontContinuousDeploymentPolicy, len(items), start)

	return items, nil
}
