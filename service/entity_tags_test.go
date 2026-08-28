package service_test

import (
	"testing"

	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/batch"
	"github.com/imunhatep/awslib/service/cloudcontrol"
	"github.com/imunhatep/awslib/service/cloudwatchlogs"
	"github.com/imunhatep/awslib/service/ec2"
	"github.com/imunhatep/awslib/service/glue"
	"github.com/imunhatep/awslib/service/lambda"
	"github.com/imunhatep/awslib/service/sns"
	"github.com/imunhatep/awslib/service/sqs"
	"github.com/stretchr/testify/assert"
)

// Entities that store a ready-made tag map used to return it, so a caller
// writing into the result of GetTags rewrote the resource itself — and with it
// every middleware stage, the resource pool and anything else holding that
// resource. Entities that build their tags per call from an SDK tag slice never
// had the problem, which made the same interface call safe on one type and
// shared state on another, with no way for the caller to tell which it had.
//
// This lives here, next to the ResourceInterface it is a property of, and is a
// table over concrete entities on purpose: the leak is per entity type, so a new
// entity that returns its stored map has to be added here to be caught.
func TestGetTagsReturnsCopy(t *testing.T) {
	tags := func() map[string]string { return map[string]string{"Team": "platform"} }

	resources := map[string]service.ResourceInterface{
		"batch.ComputeEnvironment": batch.ComputeEnvironment{Tags: tags()},
		"batch.JobQueue":           batch.JobQueue{Tags: tags()},
		"cloudcontrol.Resource":    cloudcontrol.Resource{Tags: tags()},
		"cloudwatchlogs.LogGroup":  cloudwatchlogs.LogGroup{Tags: tags()},
		"glue.Database":            glue.Database{Tags: tags()},
		"glue.Job":                 glue.Job{Tags: tags()},
		"glue.Table":               glue.Table{Tags: tags()},
		"lambda.Function":          lambda.Function{Tags: tags()},
		"sqs.Queue":                sqs.Queue{Tags: tags()},
	}

	for name, resource := range resources {
		t.Run(name, func(t *testing.T) {
			first := resource.GetTags()
			assert.Equal(t, "platform", first["Team"])

			delete(first, "Team")
			first["Injected"] = "value"

			second := resource.GetTags()
			assert.Equal(t, "platform", second["Team"], "GetTags handed out the entity's own map")
			assert.NotContains(t, second, "Injected")
		})
	}
}

// The flat attribute maps are the same story. Cloud Control's nested attributes
// are covered by service/cloudcontrol/resource_test.go, where the deep copy lives.
func TestGetAttributesReturnsCopy(t *testing.T) {
	queue := sqs.Queue{Attributes: map[string]string{"QueueArn": "arn:aws:sqs:eu-central-1:1:q"}}
	queue.GetAttributes()["QueueArn"] = "mutated"
	assert.Equal(t, "arn:aws:sqs:eu-central-1:1:q", queue.GetAttributes()["QueueArn"])

	topic := sns.Topic{Attributes: map[string]string{"Policy": "{}"}}
	topic.GetAttributes()["Policy"] = "mutated"
	assert.Equal(t, "{}", topic.GetAttributes()["Policy"])
}

// A nil map has to copy to an empty one rather than nil, so a caller sees the
// same thing an entity that builds its tags per call returns (ec2.Instance here).
// Ranging over a nil map works, but writing to one panics.
func TestTagsOfEmptyEntityAreUsable(t *testing.T) {
	assert.NotNil(t, sqs.Queue{}.GetTags())
	assert.NotNil(t, cloudcontrol.Resource{}.GetAttributes())
	assert.NotNil(t, ec2.Instance{}.GetTags())
}
