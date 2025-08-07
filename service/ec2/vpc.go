package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"time"
)

type Vpc struct {
	service.AbstractResource
	types.Vpc
}

func NewVpc(client AwsClient, vpc types.Vpc) Vpc {
	ebs := Vpc{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(vpc.VpcId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "vpc/", vpc.VpcId),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeVpc,
		},
		Vpc: vpc,
	}

	return ebs
}

func (e Vpc) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Vpc) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Vpc) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
