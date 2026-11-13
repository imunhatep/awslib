package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ResponseHeadersPolicyList holds a list of ResponseHeadersPolicy items.
type ResponseHeadersPolicyList struct {
	Items []ResponseHeadersPolicy
}

// ResponseHeadersPolicy is a CloudFront response headers policy, managed or custom.
//
// Managed policies are AWS-owned and identical in every account, but they are
// listed too: distributions reference them by ID, and resolving that ID to a name
// such as "Managed-CachingOptimized" is the question asked of them.
//
// The list item's Type field is carried as PolicyType instead of embedding the
// summary: AbstractResource has a Type field of its own, and two Type fields at
// the same depth are ambiguous, which encoding/json resolves by dropping both.
// Policies have no ARN in any API response, so none is set.
type ResponseHeadersPolicy struct {
	service.AbstractResource
	cftypes.ResponseHeadersPolicy
	PolicyType cftypes.ResponseHeadersPolicyType
}

func NewResponseHeadersPolicy(client AwsClient, policy cftypes.ResponseHeadersPolicy, policyType cftypes.ResponseHeadersPolicyType) ResponseHeadersPolicy {
	return ResponseHeadersPolicy{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(policy.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontResponseHeadersPolicy,
		},
		ResponseHeadersPolicy: policy,
		PolicyType:            policyType,
	}
}

func (e ResponseHeadersPolicy) GetName() string {
	if e.ResponseHeadersPolicyConfig == nil {
		return e.GetId()
	}

	return aws.ToString(e.ResponseHeadersPolicyConfig.Name)
}

// GetTags returns an empty map: CloudFront policies are not taggable.
func (e ResponseHeadersPolicy) GetTags() map[string]string {
	return map[string]string{}
}

// GetPolicyType returns "managed" or "custom".
func (e ResponseHeadersPolicy) GetPolicyType() string {
	return string(e.PolicyType)
}

// IsManaged reports whether the policy is AWS-owned.
func (e ResponseHeadersPolicy) IsManaged() bool {
	return e.PolicyType == cftypes.ResponseHeadersPolicyTypeManaged
}

func (e ResponseHeadersPolicy) GetComment() string {
	if e.ResponseHeadersPolicyConfig == nil {
		return ""
	}

	return aws.ToString(e.ResponseHeadersPolicyConfig.Comment)
}
