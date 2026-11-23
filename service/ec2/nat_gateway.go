package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type NatGateway struct {
	service.AbstractResource
	types.NatGateway
}

func NewNatGateway(client AwsClient, natGateway types.NatGateway) NatGateway {
	return NatGateway{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(natGateway.NatGatewayId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "natgateway/", natGateway.NatGatewayId),
			CreatedAt: aws.ToTime(natGateway.CreateTime),
			Type:      cfg.ResourceTypeNatGateway,
		},
		NatGateway: natGateway,
	}
}

func (e NatGateway) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e NatGateway) GetVpcId() string {
	return aws.ToString(e.NatGateway.VpcId)
}

func (e NatGateway) GetSubnetId() string {
	return aws.ToString(e.NatGateway.SubnetId)
}

func (e NatGateway) GetState() string {
	return string(e.NatGateway.State)
}

// GetConnectivityType is "public" (egress to the internet through an Elastic IP) or
// "private" (egress to other VPCs or on-premises only).
func (e NatGateway) GetConnectivityType() string {
	return string(e.NatGateway.ConnectivityType)
}

func (e NatGateway) IsPublic() bool {
	return e.NatGateway.ConnectivityType == types.ConnectivityTypePublic
}

// GetPublicIps returns the public addresses of the gateway, primary first as the API lists
// them. A private NAT gateway has none.
func (e NatGateway) GetPublicIps() []string {
	return e.addressValues(func(a types.NatGatewayAddress) *string { return a.PublicIp })
}

func (e NatGateway) GetPrivateIps() []string {
	return e.addressValues(func(a types.NatGatewayAddress) *string { return a.PrivateIp })
}

// GetAllocationIds returns the Elastic IP allocations backing the public addresses.
func (e NatGateway) GetAllocationIds() []string {
	return e.addressValues(func(a types.NatGatewayAddress) *string { return a.AllocationId })
}

// GetNetworkInterfaceIds returns the ENIs the gateway's addresses live on; the interface
// is requester-managed, so this is the join key to AWS::EC2::NetworkInterface.
func (e NatGateway) GetNetworkInterfaceIds() []string {
	return e.addressValues(func(a types.NatGatewayAddress) *string { return a.NetworkInterfaceId })
}

// addressValues collects one field from every address, skipping unset values and
// duplicates (several secondary addresses share one network interface).
func (e NatGateway) addressValues(field func(types.NatGatewayAddress) *string) []string {
	values := []string{}
	seen := map[string]bool{}
	for _, address := range e.NatGateway.NatGatewayAddresses {
		value := aws.ToString(field(address))
		if value == "" || seen[value] {
			continue
		}

		seen[value] = true
		values = append(values, value)
	}

	return values
}

func (e NatGateway) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.NatGateway.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e NatGateway) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
