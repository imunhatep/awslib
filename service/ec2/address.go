package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

// Address is an Elastic IP allocation. It carries both sides of the NAT: the public address
// AWS assigned and, when the EIP is associated, the private address it maps onto.
type Address struct {
	service.AbstractResource
	types.Address
}

func NewAddress(client AwsClient, address types.Address) Address {
	// VPC EIPs are identified by AllocationId; EC2-Classic addresses only ever have a PublicIp.
	id := aws.ToString(address.AllocationId)
	if id == "" {
		id = aws.ToString(address.PublicIp)
	}

	return Address{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        id,
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "elastic-ip/", aws.String(id)),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeEip,
		},
		Address: address,
	}
}

// GetName prefers the Name tag and falls back to the public IP, which is how an untagged
// Elastic IP is identified everywhere else.
func (e Address) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	if ip := aws.ToString(e.Address.PublicIp); ip != "" {
		return ip
	}

	return "-"
}

// GetPublicIp returns the public IPv4 address of the Elastic IP.
func (e Address) GetPublicIp() string {
	return aws.ToString(e.Address.PublicIp)
}

// GetPrivateIpAddress returns the private IPv4 address the Elastic IP is mapped onto, or an
// empty string when the address is unassociated.
func (e Address) GetPrivateIpAddress() string {
	return aws.ToString(e.Address.PrivateIpAddress)
}

func (e Address) GetAllocationId() string {
	return aws.ToString(e.Address.AllocationId)
}

func (e Address) GetAssociationId() string {
	return aws.ToString(e.Address.AssociationId)
}

func (e Address) GetInstanceId() string {
	return aws.ToString(e.Address.InstanceId)
}

func (e Address) GetNetworkInterfaceId() string {
	return aws.ToString(e.Address.NetworkInterfaceId)
}

// IsAssociated reports whether the Elastic IP is attached to an instance or network interface.
// An unassociated EIP still bills, so this is the interesting property.
func (e Address) IsAssociated() bool {
	return aws.ToString(e.Address.AssociationId) != "" ||
		aws.ToString(e.Address.InstanceId) != "" ||
		aws.ToString(e.Address.NetworkInterfaceId) != ""
}

func (e Address) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Address.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Address) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
