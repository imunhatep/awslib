package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ContinuousDeploymentPolicyList holds a list of ContinuousDeploymentPolicy items.
type ContinuousDeploymentPolicyList struct {
	Items []ContinuousDeploymentPolicy
}

// ContinuousDeploymentPolicy routes part of a primary distribution's traffic to
// a staging distribution, by weight or by header. The primary references it
// through DistributionConfig.ContinuousDeploymentPolicyId. It has neither a name
// nor an ARN, so the ID is its name. Not taggable.
type ContinuousDeploymentPolicy struct {
	service.AbstractResource
	cftypes.ContinuousDeploymentPolicy
}

func NewContinuousDeploymentPolicy(client AwsClient, policy cftypes.ContinuousDeploymentPolicy) ContinuousDeploymentPolicy {
	return ContinuousDeploymentPolicy{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(policy.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontContinuousDeploymentPolicy,
		},
		ContinuousDeploymentPolicy: policy,
	}
}

func (e ContinuousDeploymentPolicy) GetName() string {
	return e.GetId()
}

// GetTags returns an empty map: continuous deployment policies are not taggable.
func (e ContinuousDeploymentPolicy) GetTags() map[string]string {
	return map[string]string{}
}

func (e ContinuousDeploymentPolicy) IsEnabled() bool {
	return e.ContinuousDeploymentPolicyConfig != nil && aws.ToBool(e.ContinuousDeploymentPolicyConfig.Enabled)
}

// GetStagingDomainNames returns the staging distributions' domain names.
func (e ContinuousDeploymentPolicy) GetStagingDomainNames() []string {
	if e.ContinuousDeploymentPolicyConfig == nil || e.ContinuousDeploymentPolicyConfig.StagingDistributionDnsNames == nil {
		return nil
	}

	return append([]string(nil), e.ContinuousDeploymentPolicyConfig.StagingDistributionDnsNames.Items...)
}

// GetTrafficType returns SingleWeight or SingleHeader.
func (e ContinuousDeploymentPolicy) GetTrafficType() string {
	if e.ContinuousDeploymentPolicyConfig == nil || e.ContinuousDeploymentPolicyConfig.TrafficConfig == nil {
		return ""
	}

	return string(e.ContinuousDeploymentPolicyConfig.TrafficConfig.Type)
}
