package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

// NetworkInterface is an elastic network interface (ENI). It is the only place where every
// public IPv4 address in a VPC is visible in one shape: an instance's auto-assigned address,
// an Elastic IP, and the per-AZ frontend address of an internet-facing Network Load Balancer
// all live on an ENI's association. DescribeLoadBalancers in particular reports no address
// for an NLB that has no Elastic IP attached, so its ENIs are the only way to find them.
type NetworkInterface struct {
	service.AbstractResource
	types.NetworkInterface
}

func NewNetworkInterface(client AwsClient, eni types.NetworkInterface) NetworkInterface {
	return NetworkInterface{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(eni.NetworkInterfaceId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "network-interface/", eni.NetworkInterfaceId),
			CreatedAt: networkInterfaceCreatedAt(eni),
			Type:      cfg.ResourceTypeNetworkInterface,
		},
		NetworkInterface: eni,
	}
}

// networkInterfaceCreatedAt uses the attachment time when there is one. ENIs carry no
// creation time, and for the requester-managed interfaces (load balancers, NAT gateways,
// Lambda) the attachment is made at creation, so it is the closest thing available.
func networkInterfaceCreatedAt(eni types.NetworkInterface) time.Time {
	if eni.Attachment != nil && eni.Attachment.AttachTime != nil {
		return aws.ToTime(eni.Attachment.AttachTime)
	}

	return time.Unix(0, 0)
}

// GetName prefers the Name tag and falls back to the description, which is where AWS names
// the owner of a requester-managed interface ("ELB net/<name>/<id>", "Interface for NAT
// Gateway nat-…").
func (e NetworkInterface) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	if description := aws.ToString(e.Description); description != "" {
		return description
	}

	return "-"
}

func (e NetworkInterface) GetStatus() types.NetworkInterfaceStatus {
	return e.Status
}

func (e NetworkInterface) GetInterfaceType() types.NetworkInterfaceType {
	return e.InterfaceType
}

func (e NetworkInterface) GetDescription() string {
	return aws.ToString(e.Description)
}

// GetPrivateIpAddress returns the primary private IPv4 address.
func (e NetworkInterface) GetPrivateIpAddress() string {
	return aws.ToString(e.NetworkInterface.PrivateIpAddress)
}

// GetPublicIp returns the public IPv4 address associated with the primary private address,
// whether AWS auto-assigned it or it is an Elastic IP. Empty when the interface has none.
func (e NetworkInterface) GetPublicIp() string {
	if e.Association == nil {
		return ""
	}

	return aws.ToString(e.Association.PublicIp)
}

// GetAllocationId returns the Elastic IP allocation behind the primary public address, or an
// empty string when the address is auto-assigned (or there is none).
func (e NetworkInterface) GetAllocationId() string {
	if e.Association == nil {
		return ""
	}

	return aws.ToString(e.Association.AllocationId)
}

// IsElasticIp reports whether the primary public address is an Elastic IP rather than an
// address AWS assigned from its pool. Only the latter changes when the owner restarts.
func (e NetworkInterface) IsElasticIp() bool {
	return e.GetAllocationId() != ""
}

// GetPrivateIpAddresses returns every private IPv4 address on the interface, primary first.
func (e NetworkInterface) GetPrivateIpAddresses() []string {
	return e.collectIps(func(p types.NetworkInterfacePrivateIpAddress) string {
		return aws.ToString(p.PrivateIpAddress)
	})
}

// GetPublicIps returns every public IPv4 address on the interface: one per private address
// that has an association, primary first. An interface with secondary Elastic IPs carries
// more than one.
func (e NetworkInterface) GetPublicIps() []string {
	return e.collectIps(func(p types.NetworkInterfacePrivateIpAddress) string {
		if p.Association == nil {
			return ""
		}

		return aws.ToString(p.Association.PublicIp)
	})
}

func (e NetworkInterface) collectIps(pick func(types.NetworkInterfacePrivateIpAddress) string) []string {
	var primary, rest []string

	for _, p := range e.PrivateIpAddresses {
		ip := pick(p)
		if ip == "" {
			continue
		}

		if aws.ToBool(p.Primary) {
			primary = append(primary, ip)
		} else {
			rest = append(rest, ip)
		}
	}

	return append(primary, rest...)
}

// GetInstanceId returns the instance the interface is attached to, or an empty string for an
// unattached interface or one owned by a managed service (load balancer, NAT gateway, …).
func (e NetworkInterface) GetInstanceId() string {
	if e.Attachment == nil {
		return ""
	}

	return aws.ToString(e.Attachment.InstanceId)
}

// IsRequesterManaged reports whether an AWS service created and owns the interface. Such an
// interface cannot be modified or detached by the account.
func (e NetworkInterface) IsRequesterManaged() bool {
	return aws.ToBool(e.RequesterManaged)
}

func (e NetworkInterface) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.TagSet {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e NetworkInterface) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
