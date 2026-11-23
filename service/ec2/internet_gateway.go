package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type InternetGateway struct {
	service.AbstractResource
	types.InternetGateway
}

func NewInternetGateway(client AwsClient, internetGateway types.InternetGateway) InternetGateway {
	return InternetGateway{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(internetGateway.InternetGatewayId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "internet-gateway/", internetGateway.InternetGatewayId),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeInternetGateway,
		},
		InternetGateway: internetGateway,
	}
}

func (e InternetGateway) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e InternetGateway) GetOwnerId() string {
	return aws.ToString(e.InternetGateway.OwnerId)
}

// GetAttachedVpcIds returns the VPCs the gateway is attached to. An internet gateway takes
// at most one VPC; one being attached or detached is still listed, with its state in
// the attachment.
func (e InternetGateway) GetAttachedVpcIds() []string {
	vpcIds := []string{}
	for _, attachment := range e.InternetGateway.Attachments {
		if attachment.VpcId != nil {
			vpcIds = append(vpcIds, aws.ToString(attachment.VpcId))
		}
	}

	return vpcIds
}

// IsAttached reports whether the gateway is attached to a VPC. A detached internet gateway
// costs nothing but is usually leftover from a deleted VPC.
func (e InternetGateway) IsAttached() bool {
	return len(e.GetAttachedVpcIds()) > 0
}

func (e InternetGateway) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.InternetGateway.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e InternetGateway) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
