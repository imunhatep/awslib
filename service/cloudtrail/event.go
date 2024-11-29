package cloudtrail

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"time"
)

type Event struct {
	service.AbstractResource
	types.Event
}

func NewEvent(client AwsClient, event types.Event) Event {
	ebs := Event{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(event.EventId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "cloudtrail", "event/", event.EventId),
			CreatedAt: aws.ToTime(event.EventTime),
			Type:      cfg.ResourceTypeTrail,
		},
		Event: event,
	}

	return ebs
}

func (e Event) GetName() string {
	return aws.ToString(e.Event.EventName)
}

func (e Event) GetTags() map[string]string {
	return map[string]string{}
}

func (e Event) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}

func (e Event) GetUsername() string {
	return aws.ToString(e.Event.Username)
}

func (e Event) GetSource() string {
	return aws.ToString(e.Event.EventSource)
}

func (e Event) GetResources() []types.Resource {
	return e.Event.Resources
}

func (e Event) GetResourcesByType(resourceType cfg.ResourceType) []string {
	var names []string
	for _, r := range e.Event.Resources {
		if r.ResourceType == nil {
			continue
		}
		if cfg.ResourceType(aws.ToString(r.ResourceType)) != resourceType {
			continue
		}

		names = append(names, aws.ToString(r.ResourceName))
	}

	return names
}

func (e Event) GetTime() time.Time {
	return aws.ToTime(e.Event.EventTime)
}

func (e Event) GetReadOnly() string {
	return *e.Event.ReadOnly
}

func (e Event) IsReadOnly() bool {
	return e.GetReadOnly() == "true"
}

func init() {
	gob.Register(Event{})
}
