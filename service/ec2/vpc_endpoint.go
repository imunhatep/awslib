package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type VpcEndpoint struct {
	service.AbstractResource
	types.VpcEndpoint
}

func NewVpcEndpoint(client AwsClient, endpoint types.VpcEndpoint) VpcEndpoint {
	return VpcEndpoint{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(endpoint.VpcEndpointId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "vpc-endpoint/", endpoint.VpcEndpointId),
			CreatedAt: aws.ToTime(endpoint.CreationTimestamp),
			Type:      cfg.ResourceTypeVPCEndpoint,
		},
		VpcEndpoint: endpoint,
	}
}

// GetName prefers the Name tag and falls back to the endpoint's service name, which is what
// identifies an untagged endpoint in the console.
func (e VpcEndpoint) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	if svc := aws.ToString(e.VpcEndpoint.ServiceName); svc != "" {
		return svc
	}

	return "-"
}

func (e VpcEndpoint) GetVpcId() string {
	return aws.ToString(e.VpcEndpoint.VpcId)
}

func (e VpcEndpoint) GetServiceName() string {
	return aws.ToString(e.VpcEndpoint.ServiceName)
}

func (e VpcEndpoint) GetEndpointType() types.VpcEndpointType {
	return e.VpcEndpoint.VpcEndpointType
}

func (e VpcEndpoint) GetState() types.State {
	return e.VpcEndpoint.State
}

func (e VpcEndpoint) GetSubnetIds() []string {
	return e.VpcEndpoint.SubnetIds
}

func (e VpcEndpoint) GetRouteTableIds() []string {
	return e.VpcEndpoint.RouteTableIds
}

func (e VpcEndpoint) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.VpcEndpoint.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e VpcEndpoint) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
