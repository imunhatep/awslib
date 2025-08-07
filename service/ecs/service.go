package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type ServiceList struct {
	Items []Service
}

type Service struct {
	service.AbstractResource
	types.Service
}

func NewService(client AwsClient, svc types.Service) Service {
	serviceArn, _ := arn.Parse(aws.ToString(svc.ServiceArn))

	return Service{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(svc.ServiceName),
			ARN:       &serviceArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeECSService,
		},
		Service: svc,
	}
}

func (e Service) GetName() string {
	return aws.ToString(e.Service.ServiceName)
}

func (e Service) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Service.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Service) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
