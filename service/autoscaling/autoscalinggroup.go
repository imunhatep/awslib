package autoscaling

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/service"
)

type AutoScalingGroupList struct {
	Items []AutoScalingGroup
}

type AutoScalingGroup struct {
	service.AbstractResource
	types.AutoScalingGroup
}

func NewAutoScalingGroup(client AwsClient, group types.AutoScalingGroup) AutoScalingGroup {
	groupArn, _ := arn.Parse(aws.ToString(group.AutoScalingGroupARN))
	ec2 := AutoScalingGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(group.AutoScalingGroupName),
			ARN:       &groupArn,
			CreatedAt: aws.ToTime(group.CreatedTime),
			Type:      cfg.ResourceTypeAutoScalingGroup,
		},
		AutoScalingGroup: group,
	}

	return ec2
}

func (e AutoScalingGroup) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e AutoScalingGroup) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e AutoScalingGroup) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}

func init() {
	gob.Register(AutoScalingGroup{})
}
