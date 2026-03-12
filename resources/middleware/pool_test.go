package middleware

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/resources"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
)

type MockEntity struct {
	service.AbstractResource

	id   string
	tags map[string]string
}

func (m MockEntity) GetId() string {
	return m.id
}
func (m MockEntity) GetName() string {
	return m.id
}
func (m MockEntity) GetTags() map[string]string {
	return m.tags
}
func (m MockEntity) GetTagValue(name string) string {
	return m.tags[name]
}

type ResourceReaderMock struct {
	Type      types.ResourceType
	Resources []service.ResourceInterface
}

func (r ResourceReaderMock) ResourceType() types.ResourceType {
	return r.Type
}

func (r ResourceReaderMock) Read() []service.ResourceInterface {
	return r.Resources
}

func TestNewResourcePoolMiddleware(t *testing.T) {
	middleware := NewResourcePoolMiddleware()
	assert.NotNil(t, middleware)
	assert.Empty(t, middleware.GetResources())
}

func TestResourcePoolMiddleware_GetResources(t *testing.T) {
	middleware := NewResourcePoolMiddleware()
	resource := MockEntity{id: "1", tags: map[string]string{"key": "value"}}
	middleware.flush(types.ResourceTypeInstance, []service.ResourceInterface{resource})

	resources := middleware.GetResources()
	assert.Len(t, resources, 1)
	assert.Equal(t, "1", resources[0].GetId())
}

func TestResourcePoolMiddleware_GetResourcesByType(t *testing.T) {
	middleware := NewResourcePoolMiddleware()
	resource := MockEntity{id: "1", tags: map[string]string{"key": "value"}}
	middleware.flush(types.ResourceTypeInstance, []service.ResourceInterface{resource})

	resources := middleware.GetResourcesByType(types.ResourceTypeInstance)
	assert.Len(t, resources, 1)
	assert.Equal(t, "1", resources[0].GetId())

	resources = middleware.GetResourcesByType(types.ResourceTypeBucket)
	assert.Empty(t, resources)
}

func TestResourcePoolMiddleware_HandleResourceReader(t *testing.T) {
	middleware := NewResourcePoolMiddleware()
	resource := MockEntity{id: "1", tags: map[string]string{"key": "value"}}
	reader := ResourceReaderMock{types.ResourceTypeInstance, []service.ResourceInterface{resource}}

	handler := middleware.HandleResourceReader(func(reader resources.ResourceReaderInterface) error {
		return nil
	})

	err := handler(reader)
	assert.NoError(t, err)

	resources := middleware.GetResourcesByType(types.ResourceTypeInstance)
	assert.Len(t, resources, 1)
	assert.Equal(t, "1", resources[0].GetId())
}
