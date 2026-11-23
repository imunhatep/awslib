package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type TransitGateway struct {
	service.AbstractResource
	types.TransitGateway
}

func NewTransitGateway(client AwsClient, transitGateway types.TransitGateway) TransitGateway {
	// DescribeTransitGateways returns TransitGatewayArn; fall back to a synthesized ARN when it is absent.
	tgwArn := parseArn(transitGateway.TransitGatewayArn)
	if tgwArn == nil {
		tgwArn = helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "transit-gateway/", transitGateway.TransitGatewayId)
	}

	return TransitGateway{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(transitGateway.TransitGatewayId),
			ARN:       tgwArn,
			CreatedAt: aws.ToTime(transitGateway.CreationTime),
			Type:      cfg.ResourceTypeTransitGateway,
		},
		TransitGateway: transitGateway,
	}
}

func (e TransitGateway) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

// GetOwnerId is the account that owns the transit gateway; for one shared through RAM it
// differs from the account that listed it.
func (e TransitGateway) GetOwnerId() string {
	return aws.ToString(e.TransitGateway.OwnerId)
}

func (e TransitGateway) GetState() string {
	return string(e.TransitGateway.State)
}

func (e TransitGateway) GetDescription() string {
	return aws.ToString(e.TransitGateway.Description)
}

func (e TransitGateway) GetAmazonSideAsn() int64 {
	if e.TransitGateway.Options == nil {
		return 0
	}

	return aws.ToInt64(e.TransitGateway.Options.AmazonSideAsn)
}

func (e TransitGateway) GetCidrBlocks() []string {
	if e.TransitGateway.Options == nil {
		return []string{}
	}

	return append([]string{}, e.TransitGateway.Options.TransitGatewayCidrBlocks...)
}

func (e TransitGateway) GetAssociationDefaultRouteTableId() string {
	if e.TransitGateway.Options == nil {
		return ""
	}

	return aws.ToString(e.TransitGateway.Options.AssociationDefaultRouteTableId)
}

func (e TransitGateway) GetPropagationDefaultRouteTableId() string {
	if e.TransitGateway.Options == nil {
		return ""
	}

	return aws.ToString(e.TransitGateway.Options.PropagationDefaultRouteTableId)
}

// IsAutoAcceptSharedAttachments reports whether attachment requests from other accounts
// are accepted without review.
func (e TransitGateway) IsAutoAcceptSharedAttachments() bool {
	if e.TransitGateway.Options == nil {
		return false
	}

	return e.TransitGateway.Options.AutoAcceptSharedAttachments == types.AutoAcceptSharedAttachmentsValueEnable
}

func (e TransitGateway) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.TransitGateway.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e TransitGateway) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
