package elb

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/imunhatep/awslib/service"
)

type LoadBalancerList struct {
	Items []LoadBalancer
}

type LoadBalancer struct {
	service.AbstractResource
	types.LoadBalancer
	Tags []types.Tag
}

func NewLoadBalancer(client AwsClient, lb types.LoadBalancer, tags []types.Tag) LoadBalancer {
	lbArn, _ := arn.Parse(aws.ToString(lb.LoadBalancerArn))

	return LoadBalancer{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        "",
			ARN:       &lbArn,
			CreatedAt: aws.ToTime(lb.CreatedTime),
			Type:      cfg.ResourceTypeLoadBalancerV2,
		},
		LoadBalancer: lb,
		Tags:         tags,
	}
}

func (e LoadBalancer) GetName() string {
	return aws.ToString(e.LoadBalancerName)
}

func (e LoadBalancer) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e LoadBalancer) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}

func init() {
	gob.Register(LoadBalancer{})
}
