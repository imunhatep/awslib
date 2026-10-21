package cloudcontrol

// Hand-written companion to the generated cached_repository.go. Its name must
// not match cached_*_repository.go: generate-cached deletes those on every run.
//
// The generated wrapper caches Get*/List* results and knows nothing about
// writes. These methods make CloudControlRepositoryCached usable for writes
// without dropping to the inner repository: each delegates, and on an accepted
// request evicts every cached Cloud Control list for this account and region.
//
// Eviction is by method-name prefix, not by resource type, because the cache
// key folds the type into a one-way hash alongside whatever other parameters
// the list took, so "the entries for AWS::Logs::LogGroup" cannot be named. The
// namespace is per account and region, so the blast radius is other types'
// lists in the same place, refetched on their next read. That is the cheaper
// mistake: a stale list still showing a deleted resource is an answer that is
// wrong, a refetched one is an answer that is slow.
//
// Eviction happens when the request is accepted, not when it completes, since a
// consumer polling the list to watch the change land needs the first poll to
// miss. It happens again after a wait, because the *AndWait variants hold the
// cache for the whole operation and a list fetched mid-way is itself stale.

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	awscloudcontrol "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/rs/zerolog/log"
)

// listCachePrefix is the common prefix of ListResourcesByType,
// ListResourcesByTypeDetailed and ListResourcesByInput cache keys.
const listCachePrefix = "ListResources"

// Repository returns the inner, uncached repository.
func (c *CloudControlRepositoryCached) Repository() *CloudControlRepository {
	return c.repo
}

// InvalidateLists evicts every cached list in this repository's namespace. The
// write methods call it; it is exported for a caller that changed a resource
// through some other path and knows the cache is stale.
func (c *CloudControlRepositoryCached) InvalidateLists() {
	if err := c.cache.Invalidate(listCachePrefix); err != nil {
		log.Warn().Err(err).Msg("[CloudControlRepositoryCached.InvalidateLists] cache eviction failed, lists stay stale until TTL")
	}
}

func (c *CloudControlRepositoryCached) afterWrite(event *types.ProgressEvent, err error) (*types.ProgressEvent, error) {
	if event != nil {
		c.InvalidateLists()
	}

	return event, err
}

// CreateResource delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) CreateResource(resourceType cfg.ResourceType, desiredState string) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.CreateResource(resourceType, desiredState))
}

// CreateResourceByInput delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) CreateResourceByInput(query *awscloudcontrol.CreateResourceInput) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.CreateResourceByInput(query))
}

// UpdateResource delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) UpdateResource(resourceType cfg.ResourceType, identifier string, patchDocument string) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.UpdateResource(resourceType, identifier, patchDocument))
}

// UpdateResourceByInput delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) UpdateResourceByInput(query *awscloudcontrol.UpdateResourceInput) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.UpdateResourceByInput(query))
}

// DeleteResource delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) DeleteResource(resourceType cfg.ResourceType, identifier string) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.DeleteResource(resourceType, identifier))
}

// DeleteResourceByInput delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) DeleteResourceByInput(query *awscloudcontrol.DeleteResourceInput) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.DeleteResourceByInput(query))
}

// SetResourceTags delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) SetResourceTags(resourceType cfg.ResourceType, identifier string, tags map[string]string) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.SetResourceTags(resourceType, identifier, tags))
}

// UntagResource delegates and evicts cached lists once the request is accepted.
func (c *CloudControlRepositoryCached) UntagResource(resourceType cfg.ResourceType, identifier string, keys ...string) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.UntagResource(resourceType, identifier, keys...))
}

// CreateResourceAndWait delegates, evicting cached lists both when the request
// is accepted and after it completes.
func (c *CloudControlRepositoryCached) CreateResourceAndWait(resourceType cfg.ResourceType, desiredState string, maxWait time.Duration) (*Resource, error) {
	event, err := c.CreateResource(resourceType, desiredState)
	if err != nil {
		return nil, err
	}
	defer c.InvalidateLists()

	return c.repo.waitAndDescribe(resourceType, event, maxWait)
}

// UpdateResourceAndWait delegates, evicting cached lists both when the request
// is accepted and after it completes.
func (c *CloudControlRepositoryCached) UpdateResourceAndWait(resourceType cfg.ResourceType, identifier string, patchDocument string, maxWait time.Duration) (*Resource, error) {
	event, err := c.UpdateResource(resourceType, identifier, patchDocument)
	if err != nil {
		return nil, err
	}
	defer c.InvalidateLists()

	return c.repo.waitAndDescribe(resourceType, event, maxWait)
}

// DeleteResourceAndWait delegates, evicting cached lists both when the request
// is accepted and after it completes.
func (c *CloudControlRepositoryCached) DeleteResourceAndWait(resourceType cfg.ResourceType, identifier string, maxWait time.Duration) (*types.ProgressEvent, error) {
	event, err := c.DeleteResource(resourceType, identifier)
	if err != nil {
		return nil, err
	}
	defer c.InvalidateLists()

	return c.repo.WaitForResourceRequest(aws.ToString(event.RequestToken), maxWait)
}

// Pass-throughs with no cache interaction: a request status must never be
// served from cache, and the inner repository's waiter options apply.

func (c *CloudControlRepositoryCached) DescribeResourceRequest(requestToken string) (*awscloudcontrol.GetResourceRequestStatusOutput, error) {
	return c.repo.DescribeResourceRequest(requestToken)
}

func (c *CloudControlRepositoryCached) WaitForResourceRequest(requestToken string, maxWait time.Duration) (*types.ProgressEvent, error) {
	return c.repo.WaitForResourceRequest(requestToken, maxWait)
}

func (c *CloudControlRepositoryCached) CancelResourceRequest(requestToken string) (*types.ProgressEvent, error) {
	return c.afterWrite(c.repo.CancelResourceRequest(requestToken))
}

func (c *CloudControlRepositoryCached) FindResourceRequests(statuses ...types.OperationStatus) ([]types.ProgressEvent, error) {
	return c.repo.FindResourceRequests(statuses...)
}
