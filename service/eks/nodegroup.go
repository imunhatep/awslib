package eks

import (
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/dict"
)

// Nodegroup is an EKS managed node group as DescribeNodegroup returns it.
//
// A node group name is unique within its cluster only, so the ID is "<cluster>/<nodegroup>";
// GetName returns the bare node group name and GetClusterName the cluster.
type Nodegroup struct {
	service.AbstractResource
	types.Nodegroup
}

func NewNodegroup(client AwsClient, nodegroup types.Nodegroup) Nodegroup {
	var nodegroupArn *arn.ARN
	if parsed, err := arn.Parse(aws.ToString(nodegroup.NodegroupArn)); err == nil {
		nodegroupArn = &parsed
	}

	return Nodegroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(nodegroup.ClusterName) + "/" + aws.ToString(nodegroup.NodegroupName),
			ARN:       nodegroupArn,
			CreatedAt: aws.ToTime(nodegroup.CreatedAt),
			Type:      ccfg.ResourceTypeEKSNodegroup,
		},
		Nodegroup: nodegroup,
	}
}

func (e Nodegroup) GetName() string {
	return aws.ToString(e.NodegroupName)
}

func (e Nodegroup) GetTags() map[string]string {
	return dict.Copy(e.Nodegroup.Tags)
}

func (e Nodegroup) GetTagValue(tag string) string {
	return e.Nodegroup.Tags[tag]
}

func (e Nodegroup) GetClusterName() string {
	return aws.ToString(e.ClusterName)
}

func (e Nodegroup) GetStatus() string {
	return string(e.Status)
}

// GetCapacityType is ON_DEMAND, SPOT or CAPACITY_BLOCK.
func (e Nodegroup) GetCapacityType() string {
	return string(e.CapacityType)
}

func (e Nodegroup) GetInstanceTypes() []string {
	return slices.Clone(e.InstanceTypes)
}

func (e Nodegroup) GetAmiType() string {
	return string(e.AmiType)
}

// GetReleaseVersion is the AMI release version, e.g. "1.31.0-20241024".
func (e Nodegroup) GetReleaseVersion() string {
	return aws.ToString(e.ReleaseVersion)
}

// GetKubernetesVersion is the node group's Kubernetes version, which can trail the cluster's.
func (e Nodegroup) GetKubernetesVersion() string {
	return aws.ToString(e.Version)
}

func (e Nodegroup) GetMinSize() int32 {
	if e.ScalingConfig == nil {
		return 0
	}

	return aws.ToInt32(e.ScalingConfig.MinSize)
}

func (e Nodegroup) GetMaxSize() int32 {
	if e.ScalingConfig == nil {
		return 0
	}

	return aws.ToInt32(e.ScalingConfig.MaxSize)
}

func (e Nodegroup) GetDesiredSize() int32 {
	if e.ScalingConfig == nil {
		return 0
	}

	return aws.ToInt32(e.ScalingConfig.DesiredSize)
}

func (e Nodegroup) GetSubnets() []string {
	return slices.Clone(e.Subnets)
}

func (e Nodegroup) GetNodeRole() string {
	return aws.ToString(e.NodeRole)
}

// GetDiskSize is the root volume size in GiB; zero when a launch template sets it.
func (e Nodegroup) GetDiskSize() int32 {
	return aws.ToInt32(e.DiskSize)
}

func (e Nodegroup) GetLaunchTemplateId() string {
	if e.LaunchTemplate == nil {
		return ""
	}

	return aws.ToString(e.LaunchTemplate.Id)
}

func (e Nodegroup) GetLaunchTemplateName() string {
	if e.LaunchTemplate == nil {
		return ""
	}

	return aws.ToString(e.LaunchTemplate.Name)
}

func (e Nodegroup) GetLaunchTemplateVersion() string {
	if e.LaunchTemplate == nil {
		return ""
	}

	return aws.ToString(e.LaunchTemplate.Version)
}

// GetAutoScalingGroupNames are the Auto Scaling groups EKS manages for the node group, the
// join key to AWS::AutoScaling::AutoScalingGroup.
func (e Nodegroup) GetAutoScalingGroupNames() []string {
	if e.Resources == nil {
		return nil
	}

	names := make([]string, 0, len(e.Resources.AutoScalingGroups))
	for _, group := range e.Resources.AutoScalingGroups {
		names = append(names, aws.ToString(group.Name))
	}

	return names
}

func (e Nodegroup) GetHealthIssueCount() int {
	if e.Health == nil {
		return 0
	}

	return len(e.Health.Issues)
}

// GetHealthIssueCodes returns the distinct issue codes, sorted, e.g. "AsgInstanceLaunchFailures".
func (e Nodegroup) GetHealthIssueCodes() []string {
	if e.Health == nil {
		return nil
	}

	var codes []string
	for _, issue := range e.Health.Issues {
		codes = append(codes, string(issue.Code))
	}

	slices.Sort(codes)

	return slices.Compact(codes)
}
