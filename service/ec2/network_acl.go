package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type NetworkAcl struct {
	service.AbstractResource
	types.NetworkAcl
}

func NewNetworkAcl(client AwsClient, networkAcl types.NetworkAcl) NetworkAcl {
	return NetworkAcl{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(networkAcl.NetworkAclId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "network-acl/", networkAcl.NetworkAclId),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeNetworkAcl,
		},
		NetworkAcl: networkAcl,
	}
}

func (e NetworkAcl) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e NetworkAcl) GetVpcId() string {
	return aws.ToString(e.NetworkAcl.VpcId)
}

// IsDefault reports whether the network ACL is the VPC's default network ACL.
func (e NetworkAcl) IsDefault() bool {
	return aws.ToBool(e.NetworkAcl.IsDefault)
}

func (e NetworkAcl) GetEntries() []types.NetworkAclEntry {
	return e.NetworkAcl.Entries
}

// GetIngressEntries returns the inbound rules, in the order the API returned them.
func (e NetworkAcl) GetIngressEntries() []types.NetworkAclEntry {
	return e.entries(false)
}

// GetEgressEntries returns the outbound rules, in the order the API returned them.
func (e NetworkAcl) GetEgressEntries() []types.NetworkAclEntry {
	return e.entries(true)
}

func (e NetworkAcl) entries(egress bool) []types.NetworkAclEntry {
	entries := []types.NetworkAclEntry{}
	for _, entry := range e.NetworkAcl.Entries {
		if aws.ToBool(entry.Egress) == egress {
			entries = append(entries, entry)
		}
	}

	return entries
}

func (e NetworkAcl) GetAssociations() []types.NetworkAclAssociation {
	return e.NetworkAcl.Associations
}

// GetSubnetIds returns the subnets associated with the network ACL.
func (e NetworkAcl) GetSubnetIds() []string {
	subnetIds := []string{}
	for _, association := range e.NetworkAcl.Associations {
		if association.SubnetId != nil {
			subnetIds = append(subnetIds, aws.ToString(association.SubnetId))
		}
	}

	return subnetIds
}

func (e NetworkAcl) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.NetworkAcl.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e NetworkAcl) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
