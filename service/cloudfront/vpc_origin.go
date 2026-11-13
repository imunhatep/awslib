package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// VpcOriginList holds a list of VpcOrigin items.
type VpcOriginList struct {
	Items []VpcOrigin
}

// VpcOrigin lets a distribution reach an ALB, NLB or EC2 instance in a private
// subnet. OriginEndpointArn is the load balancer or instance it fronts; an origin
// references the VPC origin through Origin.VpcOriginConfig.VpcOriginId.
//
// VPC origins are taggable. Tags are not part of the list response;
// ListVpcOriginsAll reads them in one tagging sweep (see bulkTags).
type VpcOrigin struct {
	service.AbstractResource
	cftypes.VpcOriginSummary
	Tags []cftypes.Tag
}

func NewVpcOrigin(client AwsClient, summary cftypes.VpcOriginSummary, tags []cftypes.Tag) VpcOrigin {
	return VpcOrigin{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Id),
			ARN:       parseArn(summary.Arn),
			CreatedAt: createdOrEpoch(summary.CreatedTime),
			Type:      ccfg.ResourceTypeCloudFrontVpcOrigin,
		},
		VpcOriginSummary: summary,
		Tags:             tags,
	}
}

func (e VpcOrigin) GetName() string {
	return aws.ToString(e.Name)
}

func (e VpcOrigin) GetTags() map[string]string {
	out := make(map[string]string, len(e.Tags))

	for _, tag := range e.Tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}

func (e VpcOrigin) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

func (e VpcOrigin) GetStatus() string {
	return aws.ToString(e.Status)
}

// GetOriginEndpointArn returns the ARN of the ALB, NLB or EC2 instance the VPC origin fronts.
func (e VpcOrigin) GetOriginEndpointArn() string {
	return aws.ToString(e.OriginEndpointArn)
}
