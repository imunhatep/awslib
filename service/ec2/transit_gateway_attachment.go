package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type TransitGatewayAttachment struct {
	service.AbstractResource
	types.TransitGatewayAttachment
}

func NewTransitGatewayAttachment(client AwsClient, attachment types.TransitGatewayAttachment) TransitGatewayAttachment {
	return TransitGatewayAttachment{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(attachment.TransitGatewayAttachmentId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "transit-gateway-attachment/", attachment.TransitGatewayAttachmentId),
			CreatedAt: aws.ToTime(attachment.CreationTime),
			Type:      cfg.ResourceTypeTransitGatewayAttachment,
		},
		TransitGatewayAttachment: attachment,
	}
}

func (e TransitGatewayAttachment) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e TransitGatewayAttachment) GetTransitGatewayId() string {
	return aws.ToString(e.TransitGatewayAttachment.TransitGatewayId)
}

func (e TransitGatewayAttachment) GetTransitGatewayOwnerId() string {
	return aws.ToString(e.TransitGatewayAttachment.TransitGatewayOwnerId)
}

// GetResourceType is what the attachment connects: vpc, vpn, direct-connect-gateway,
// peering, connect, tgw-peering and so on.
func (e TransitGatewayAttachment) GetResourceType() string {
	return string(e.TransitGatewayAttachment.ResourceType)
}

// GetResourceId is the attached resource: a VPC ID, VPN connection ID, peer attachment...
func (e TransitGatewayAttachment) GetResourceId() string {
	return aws.ToString(e.TransitGatewayAttachment.ResourceId)
}

func (e TransitGatewayAttachment) GetResourceOwnerId() string {
	return aws.ToString(e.TransitGatewayAttachment.ResourceOwnerId)
}

func (e TransitGatewayAttachment) GetState() string {
	return string(e.TransitGatewayAttachment.State)
}

// GetRouteTableId is the transit gateway route table the attachment is associated with,
// empty when it is not associated.
func (e TransitGatewayAttachment) GetRouteTableId() string {
	if e.TransitGatewayAttachment.Association == nil {
		return ""
	}

	return aws.ToString(e.TransitGatewayAttachment.Association.TransitGatewayRouteTableId)
}

func (e TransitGatewayAttachment) GetAssociationState() string {
	if e.TransitGatewayAttachment.Association == nil {
		return ""
	}

	return string(e.TransitGatewayAttachment.Association.State)
}

// IsCrossAccount reports whether the attached resource and the transit gateway belong to
// different accounts (a RAM-shared transit gateway).
func (e TransitGatewayAttachment) IsCrossAccount() bool {
	owner, resourceOwner := e.GetTransitGatewayOwnerId(), e.GetResourceOwnerId()

	return owner != "" && resourceOwner != "" && owner != resourceOwner
}

func (e TransitGatewayAttachment) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.TransitGatewayAttachment.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e TransitGatewayAttachment) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
