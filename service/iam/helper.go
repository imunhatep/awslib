package iam

import (
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/imunhatep/awslib/service"
)

// Tag helpers for IAM.
//
// IAM cannot reuse the EC2 shape. ec2.BuildCreateTagsInput batches many resource ids
// into one CreateTags call; IAM has a separate operation per resource kind — TagUser,
// TagRole, TagPolicy — each taking exactly one resource, keyed on UserName, RoleName or
// PolicyArn rather than on a generic id. So there is one tag set to compute and three
// places to send it, which is why these return the tag slice and the key set instead of
// a per-operation input struct.
//
// Both keep the EC2 contract that matters to callers: TagsToApply returns nil when the
// tags are already in place, and TagKeysToRemove returns nil when none of them are
// present, so a reconciler can call them every cycle and only write when something has
// actually changed.

// TagsToApply returns the tags to send for the resource, or nil when every requested tag
// already holds the requested value.
//
// The whole requested set is returned rather than only the difference: IAM's Tag* calls
// are upserts, so re-sending a correct tag is free and keeps the call idempotent.
func TagsToApply(tags map[string]string, resource service.ResourceInterface) []types.Tag {
	if len(tags) == 0 {
		return nil
	}

	resourceTags := resource.GetTags()

	inSync := true
	for tag, value := range tags {
		if current, ok := resourceTags[tag]; !ok || current != value {
			inSync = false
			break
		}
	}

	if inSync {
		return nil
	}

	return TagMapToTags(tags)
}

// TagKeysToRemove returns the tag keys to untag from the resource, or nil when it
// carries none of them.
//
// A key is returned when the resource has it and either the requested value matches the
// current one or the requested value is empty, which reads as "remove this key whatever
// it holds". Same contract as ec2.BuildDeleteTagsInput, so a caller's map[tag]"" idiom
// means the same thing across resource types.
func TagKeysToRemove(tags map[string]string, resource service.ResourceInterface) []string {
	if len(tags) == 0 {
		return nil
	}

	resourceTags := resource.GetTags()

	var tagKeys []string
	for tag, value := range tags {
		current, ok := resourceTags[tag]
		if !ok {
			continue
		}

		if value == "" || current == value {
			tagKeys = append(tagKeys, tag)
		}
	}

	if len(tagKeys) == 0 {
		return nil
	}

	// Sorted so the same resource and tag set always produce the same call, which keeps
	// logs stable and tests independent of map iteration order.
	sort.Strings(tagKeys)

	return tagKeys
}

// TagMapToTags converts a tag map into the SDK tag slice, sorted by key.
func TagMapToTags(tags map[string]string) []types.Tag {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tagList := make([]types.Tag, 0, len(keys))
	for _, key := range keys {
		tagList = append(tagList, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(tags[key]),
		})
	}

	return tagList
}
