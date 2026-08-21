package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type SecurityGroup struct {
	service.AbstractResource
	types.SecurityGroup
}

func NewSecurityGroup(client AwsClient, group types.SecurityGroup) SecurityGroup {
	// DescribeSecurityGroups returns SecurityGroupArn; fall back to a synthesized ARN.
	groupArn := parseArn(group.SecurityGroupArn)
	if groupArn == nil {
		groupArn = helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "security-group/", group.GroupId)
	}

	return SecurityGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(group.GroupId),
			ARN:       groupArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeSecurityGroup,
		},
		SecurityGroup: group,
	}
}

// GetName prefers the Name tag and falls back to the security group name, which — unlike
// most EC2 resources — is always set.
func (e SecurityGroup) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	if name := aws.ToString(e.SecurityGroup.GroupName); name != "" {
		return name
	}

	return "-"
}

func (e SecurityGroup) GetGroupName() string {
	return aws.ToString(e.SecurityGroup.GroupName)
}

func (e SecurityGroup) GetVpcId() string {
	return aws.ToString(e.SecurityGroup.VpcId)
}

func (e SecurityGroup) GetDescription() string {
	return aws.ToString(e.SecurityGroup.Description)
}

func (e SecurityGroup) GetIngressRules() []types.IpPermission {
	return e.SecurityGroup.IpPermissions
}

func (e SecurityGroup) GetEgressRules() []types.IpPermission {
	return e.SecurityGroup.IpPermissionsEgress
}

func (e SecurityGroup) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.SecurityGroup.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e SecurityGroup) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
