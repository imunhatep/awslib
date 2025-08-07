package sqs

import (
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type Queue struct {
	service.AbstractResource
	QueueUrl   string
	Attributes map[string]string
	Tags       map[string]string
}

func NewQueue(client AwsClient, queueUrl string, attributes, tags map[string]string) Queue {
	tArn, _ := arn.Parse(attributes[string(types.QueueAttributeNameQueueArn)])
	createdAt, _ := time.Parse("2006-01-02T15:04:05-0700", attributes[string(types.QueueAttributeNameCreatedTimestamp)])

	return Queue{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        queueUrl,
			ARN:       &tArn,
			CreatedAt: createdAt,
			Type:      cfg.ResourceTypeQueue,
		},
		QueueUrl:   queueUrl,
		Attributes: attributes,
		Tags:       tags,
	}
}

func (e Queue) GetName() string {
	return e.QueueUrl
}

func (e Queue) GetAttributes() map[string]string {
	return e.Attributes
}

func (e Queue) GetTags() map[string]string {
	return e.Tags
}

func (e Queue) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
