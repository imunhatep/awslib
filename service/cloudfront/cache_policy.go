package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// CachePolicyList holds a list of CachePolicy items.
type CachePolicyList struct {
	Items []CachePolicy
}

// CachePolicy is a CloudFront cache policy, managed or custom.
//
// Managed policies are AWS-owned and identical in every account, but they are
// listed too: distributions reference them by ID, and resolving that ID to a name
// such as "Managed-CachingOptimized" is the question asked of them.
//
// The list item's Type field is carried as PolicyType instead of embedding the
// summary: AbstractResource has a Type field of its own, and two Type fields at
// the same depth are ambiguous, which encoding/json resolves by dropping both.
// Policies have no ARN in any API response, so none is set.
type CachePolicy struct {
	service.AbstractResource
	cftypes.CachePolicy
	PolicyType cftypes.CachePolicyType
}

func NewCachePolicy(client AwsClient, policy cftypes.CachePolicy, policyType cftypes.CachePolicyType) CachePolicy {
	return CachePolicy{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(policy.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontCachePolicy,
		},
		CachePolicy: policy,
		PolicyType:  policyType,
	}
}

func (e CachePolicy) GetName() string {
	if e.CachePolicyConfig == nil {
		return e.GetId()
	}

	return aws.ToString(e.CachePolicyConfig.Name)
}

// GetTags returns an empty map: CloudFront policies are not taggable.
func (e CachePolicy) GetTags() map[string]string {
	return map[string]string{}
}

// GetPolicyType returns "managed" or "custom".
func (e CachePolicy) GetPolicyType() string {
	return string(e.PolicyType)
}

// IsManaged reports whether the policy is AWS-owned.
func (e CachePolicy) IsManaged() bool {
	return e.PolicyType == cftypes.CachePolicyTypeManaged
}

func (e CachePolicy) GetComment() string {
	if e.CachePolicyConfig == nil {
		return ""
	}

	return aws.ToString(e.CachePolicyConfig.Comment)
}
