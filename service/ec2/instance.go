package ec2

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"strings"
)

func init() {
	gob.Register(Instance{})
}

type InstanceList struct {
	Items []Instance
}

type Instance struct {
	service.AbstractResource
	types.Instance
}

func NewInstance(client AwsClient, instance types.Instance) Instance {
	ec2 := Instance{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(instance.InstanceId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "instance/", instance.InstanceId),
			CreatedAt: aws.ToTime(instance.LaunchTime),
			Type:      cfg.ResourceTypeInstance,
		},
		Instance: instance,
	}

	return ec2
}

func (e Instance) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Instance) GetInstanceType() string {
	return string(e.InstanceType)
}

func (e Instance) GetInstanceFamily() string {
	instanceType := e.GetInstanceType()
	typeParts := strings.Split(instanceType, ".")

	return typeParts[0]
}

func (e Instance) GetState() string {
	return string(e.State.Name)
}

func (e Instance) GetPrivateIpAddress() string {
	return aws.ToString(e.PrivateIpAddress)
}

func (e Instance) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Instance) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
