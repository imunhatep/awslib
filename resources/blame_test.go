package resources

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service/cloudtrail"
	"github.com/stretchr/testify/assert"
)

func blameEvent(at time.Time, user, readOnly, errorCode string) cloudtrail.Event {
	return cloudtrail.NewEvent(&v3.Client{}, types.Event{
		EventId:   aws.String(user + at.String()),
		EventTime: aws.Time(at),
		Username:  aws.String(user),
		ReadOnly:  aws.String(readOnly),
	}, cloudtrail.CloudTrailEvent{ErrorCode: errorCode})
}

// LookupEvents lists the newest event first; the creator is the earliest successful write.
func TestResourceEventsCreatorIsEarliestSuccessfulWrite(t *testing.T) {
	created := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	events := NewResourceEvents(nil,
		blameEvent(created.Add(5*time.Second), "tagging-lambda", "false", ""),
		blameEvent(created.Add(2*time.Second), "auditor", "true", ""),
		blameEvent(created, "alice@example.com", "false", ""),
		blameEvent(created.Add(-3*time.Second), "bob@example.com", "false", "AccessDenied"),
	)

	assert.Equal(t, "alice@example.com", events.GetUsername())
	assert.Equal(t, "alice", events.GetUser())
}

func TestResourceEventsCreatorUnknown(t *testing.T) {
	created := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	assert.Equal(t, ResourceCreatorUnknown, NewResourceEvents(nil).GetUsername())
	assert.Equal(t, ResourceCreatorUnknown,
		NewResourceEvents(nil, blameEvent(created, "auditor", "true", "")).GetUsername())
}
