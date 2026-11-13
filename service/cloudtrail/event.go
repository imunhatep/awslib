package cloudtrail

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

type Event struct {
	service.AbstractResource
	types.Event
	EventData CloudTrailEvent
}

// UserIdentity represents the user identity information in a CloudTrail event
type UserIdentity struct {
	Type        string `json:"type"`
	PrincipalID string `json:"principalId"`
	ARN         string `json:"arn"`
	AccountID   string `json:"accountId"`
	AccessKeyID string `json:"accessKeyId"`
	UserName    string `json:"userName"`
	InvokedBy   string `json:"invokedBy"`
}

// CloudTrailEvent represents the structure of a CloudTrail event. It holds the fields worth
// filtering on; the full record stays available as the raw JSON in Event.CloudTrailEvent.
type CloudTrailEvent struct {
	EventID            string       `json:"eventID"`
	EventType          string       `json:"eventType"`
	AwsRegion          string       `json:"awsRegion"`
	UserIdentity       UserIdentity `json:"userIdentity"`
	SourceIPAddress    string       `json:"sourceIPAddress"`
	UserAgent          string       `json:"userAgent"`
	ErrorCode          string       `json:"errorCode"`
	ErrorMessage       string       `json:"errorMessage"`
	RecipientAccountID string       `json:"recipientAccountId"`
}

func NewEvent(client AwsClient, event types.Event, eventData CloudTrailEvent) Event {
	ebs := Event{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(event.EventId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "cloudtrail", "event/", event.EventId),
			CreatedAt: aws.ToTime(event.EventTime),
			Type:      ccfg.ResourceTypeTrailEvent,
		},
		Event:     event,
		EventData: eventData,
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

// GetPrincipalArn is the ARN of the identity that made the call. For an assumed role it names the
// role session, which GetUsername reduces to the session name alone.
func (e Event) GetPrincipalArn() string {
	return e.EventData.UserIdentity.ARN
}

func (e Event) GetSource() string {
	return aws.ToString(e.Event.EventSource)
}

func (e Event) GetSourceIPAddress() string {
	return e.EventData.SourceIPAddress
}

func (e Event) GetUserAgent() string {
	return e.EventData.UserAgent
}

// GetErrorCode is the error the call failed with, e.g. AccessDenied; empty for a call that succeeded.
func (e Event) GetErrorCode() string {
	return e.EventData.ErrorCode
}

func (e Event) GetErrorMessage() string {
	return e.EventData.ErrorMessage
}

func (e Event) IsFailed() bool {
	return e.EventData.ErrorCode != ""
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

// GetReadOnly is "true" or "false" as CloudTrail reports it, empty when the event carries no value.
func (e Event) GetReadOnly() string {
	return aws.ToString(e.Event.ReadOnly)
}

func (e Event) IsReadOnly() bool {
	return e.GetReadOnly() == "true"
}
