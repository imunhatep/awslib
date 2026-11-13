package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// OriginRequestPolicyList holds a list of OriginRequestPolicy items.
type OriginRequestPolicyList struct {
	Items []OriginRequestPolicy
}

// OriginRequestPolicy is a CloudFront origin request policy, managed or custom.
//
// Managed policies are AWS-owned and identical in every account, but they are
// listed too: distributions reference them by ID, and resolving that ID to a name
// such as "Managed-CachingOptimized" is the question asked of them.
//
// The list item's Type field is carried as PolicyType instead of embedding the
// summary: AbstractResource has a Type field of its own, and two Type fields at
// the same depth are ambiguous, which encoding/json resolves by dropping both.
// Policies have no ARN in any API response, so none is set.
type OriginRequestPolicy struct {
	service.AbstractResource
	cftypes.OriginRequestPolicy
	PolicyType cftypes.OriginRequestPolicyType
}

func NewOriginRequestPolicy(client AwsClient, policy cftypes.OriginRequestPolicy, policyType cftypes.OriginRequestPolicyType) OriginRequestPolicy {
	return OriginRequestPolicy{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(policy.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontOriginRequestPolicy,
		},
		OriginRequestPolicy: policy,
		PolicyType:          policyType,
	}
}

func (e OriginRequestPolicy) GetName() string {
	if e.OriginRequestPolicyConfig == nil {
		return e.GetId()
	}

	return aws.ToString(e.OriginRequestPolicyConfig.Name)
}

// GetTags returns an empty map: CloudFront policies are not taggable.
func (e OriginRequestPolicy) GetTags() map[string]string {
	return map[string]string{}
}

// GetPolicyType returns "managed" or "custom".
func (e OriginRequestPolicy) GetPolicyType() string {
	return string(e.PolicyType)
}

// IsManaged reports whether the policy is AWS-owned.
func (e OriginRequestPolicy) IsManaged() bool {
	return e.PolicyType == cftypes.OriginRequestPolicyTypeManaged
}

func (e OriginRequestPolicy) GetComment() string {
	if e.OriginRequestPolicyConfig == nil {
		return ""
	}

	return aws.ToString(e.OriginRequestPolicyConfig.Comment)
}
