package sns

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type Topic struct {
	service.AbstractResource
	types.Topic
	Attributes map[string]string
	Tags       []types.Tag
}

func NewTopic(client AwsClient, topic types.Topic, attributes map[string]string, tags []types.Tag) Topic {
	tArn, _ := arn.Parse(aws.ToString(topic.TopicArn))

	return Topic{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(topic.TopicArn),
			ARN:       &tArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeTopic,
		},
		Topic:      topic,
		Attributes: attributes,
		Tags:       tags,
	}
}

func (e Topic) GetName() string {
	if name, ok := e.Attributes["DisplayName"]; ok {
		return name
	}

	return aws.ToString(e.Topic.TopicArn)
}

func (e Topic) GetOwner() (string, bool) {
	if owner, ok := e.Attributes["Owner"]; ok {
		return owner, true
	}

	return "", false
}

func (e Topic) GetPolicy() (string, bool) {
	if policy, ok := e.Attributes["Policy"]; ok {
		return policy, true
	}

	return "", false
}

func (e Topic) GetAttributes() map[string]string {
	return e.Attributes
}

func (e Topic) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Topic) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
