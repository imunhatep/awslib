package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type VpcPeeringConnection struct {
	service.AbstractResource
	types.VpcPeeringConnection
}

func NewVpcPeeringConnection(client AwsClient, peering types.VpcPeeringConnection) VpcPeeringConnection {
	return VpcPeeringConnection{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(peering.VpcPeeringConnectionId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "vpc-peering-connection/", peering.VpcPeeringConnectionId),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeVPCPeeringConnection,
		},
		VpcPeeringConnection: peering,
	}
}

func (e VpcPeeringConnection) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

// GetStatusCode is the peering state: pending-acceptance, active, rejected, expired,
// deleted, failed and so on.
func (e VpcPeeringConnection) GetStatusCode() string {
	if e.VpcPeeringConnection.Status == nil {
		return ""
	}

	return string(e.VpcPeeringConnection.Status.Code)
}

func (e VpcPeeringConnection) GetStatusMessage() string {
	if e.VpcPeeringConnection.Status == nil {
		return ""
	}

	return aws.ToString(e.VpcPeeringConnection.Status.Message)
}

func (e VpcPeeringConnection) GetRequesterVpcId() string {
	return vpcInfoValue(e.VpcPeeringConnection.RequesterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.VpcId })
}

func (e VpcPeeringConnection) GetRequesterOwnerId() string {
	return vpcInfoValue(e.VpcPeeringConnection.RequesterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.OwnerId })
}

func (e VpcPeeringConnection) GetRequesterRegion() string {
	return vpcInfoValue(e.VpcPeeringConnection.RequesterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.Region })
}

// GetRequesterCidrBlock is the requester VPC's primary IPv4 CIDR.
func (e VpcPeeringConnection) GetRequesterCidrBlock() string {
	return vpcInfoValue(e.VpcPeeringConnection.RequesterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.CidrBlock })
}

func (e VpcPeeringConnection) GetAccepterVpcId() string {
	return vpcInfoValue(e.VpcPeeringConnection.AccepterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.VpcId })
}

func (e VpcPeeringConnection) GetAccepterOwnerId() string {
	return vpcInfoValue(e.VpcPeeringConnection.AccepterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.OwnerId })
}

func (e VpcPeeringConnection) GetAccepterRegion() string {
	return vpcInfoValue(e.VpcPeeringConnection.AccepterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.Region })
}

// GetAccepterCidrBlock is the accepter VPC's primary IPv4 CIDR; the API omits it until the
// peering is accepted.
func (e VpcPeeringConnection) GetAccepterCidrBlock() string {
	return vpcInfoValue(e.VpcPeeringConnection.AccepterVpcInfo, func(i *types.VpcPeeringConnectionVpcInfo) *string { return i.CidrBlock })
}

// IsCrossAccount reports whether the two VPCs belong to different accounts.
func (e VpcPeeringConnection) IsCrossAccount() bool {
	requester, accepter := e.GetRequesterOwnerId(), e.GetAccepterOwnerId()

	return requester != "" && accepter != "" && requester != accepter
}

// IsCrossRegion reports whether the two VPCs are in different regions (inter-region
// peering, billed as inter-region data transfer).
func (e VpcPeeringConnection) IsCrossRegion() bool {
	requester, accepter := e.GetRequesterRegion(), e.GetAccepterRegion()

	return requester != "" && accepter != "" && requester != accepter
}

func vpcInfoValue(info *types.VpcPeeringConnectionVpcInfo, field func(*types.VpcPeeringConnectionVpcInfo) *string) string {
	if info == nil {
		return ""
	}

	return aws.ToString(field(info))
}

func (e VpcPeeringConnection) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.VpcPeeringConnection.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e VpcPeeringConnection) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
