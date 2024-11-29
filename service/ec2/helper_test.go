package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"testing"
)

type MockEntity struct {
	service.AbstractResource
	id   string
	tags map[string]string
}

func (m MockEntity) GetName() string {
	return m.id
}

func (m MockEntity) GetId() string {
	return m.id
}

func (m MockEntity) GetTags() map[string]string {
	return m.tags
}

func (m MockEntity) GetTagValue(key string) string {
	return m.tags[key]
}

func TestBuildDeleteTagsInput(t *testing.T) {
	tags := map[string]string{
		"env": "production",
		"app": "myapp",
	}

	resources := []service.EntityInterface{
		MockEntity{id: "i-1234567890abcdef0", tags: map[string]string{"env": "production", "app": "myapp"}},
		MockEntity{id: "i-0987654321fedcba0", tags: map[string]string{"env": "staging", "app": "myapp"}},
		MockEntity{id: "i-11223344556677889", tags: map[string]string{"env": "production", "app": "otherapp"}},
	}

	input := BuildDeleteTagsInput(tags, resources...)

	expectedResourceIds := []string{"i-1234567890abcdef0", "i-0987654321fedcba0", "i-11223344556677889"}
	expectedTags := []types.Tag{
		{Key: aws.String("env"), Value: aws.String("production")},
		{Key: aws.String("app"), Value: aws.String("myapp")},
	}

	assert.ElementsMatch(t, expectedResourceIds, input.Resources)
	assert.ElementsMatch(t, expectedTags, input.Tags)
}

func TestBuildDeleteTagsInput_EmptyTags(t *testing.T) {
	tags := map[string]string{}

	resources := []service.EntityInterface{
		MockEntity{id: "i-1234567890abcdef0", tags: map[string]string{"env": "production", "app": "myapp"}},
	}

	input := BuildDeleteTagsInput(tags, resources...)

	assert.Nil(t, input)
}

func TestBuildDeleteTagsInput_NoMatchingTags(t *testing.T) {
	tags := map[string]string{
		"env": "development",
	}

	resources := []service.EntityInterface{
		MockEntity{id: "i-1234567890abcdef0", tags: map[string]string{"env": "production", "app": "myapp"}},
	}

	input := BuildDeleteTagsInput(tags, resources...)

	assert.Nil(t, input)
}
