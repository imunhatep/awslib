package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type Subnet struct {
	service.AbstractResource
	types.Subnet
}

func NewSubnet(client AwsClient, subnet types.Subnet) Subnet {
	// DescribeSubnets returns SubnetArn; fall back to a synthesized ARN when it is absent.
	subnetArn := parseArn(subnet.SubnetArn)
	if subnetArn == nil {
		subnetArn = helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "subnet/", subnet.SubnetId)
	}

	return Subnet{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(subnet.SubnetId),
			ARN:       subnetArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeSubnet,
		},
		Subnet: subnet,
	}
}

func (e Subnet) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Subnet) GetVpcId() string {
	return aws.ToString(e.Subnet.VpcId)
}

func (e Subnet) GetCidrBlock() string {
	return aws.ToString(e.Subnet.CidrBlock)
}

func (e Subnet) GetAvailabilityZone() string {
	return aws.ToString(e.Subnet.AvailabilityZone)
}

func (e Subnet) GetState() types.SubnetState {
	return e.Subnet.State
}

// IsPublic reports whether instances launched into the subnet get a public IPv4 address
// by default. It is a launch-time property, not a routing check.
func (e Subnet) IsPublic() bool {
	return aws.ToBool(e.Subnet.MapPublicIpOnLaunch)
}

func (e Subnet) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Subnet.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Subnet) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
