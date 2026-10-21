package cloudcontrol

import (
	"encoding/json"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
)

// Cloud Control has no tagging call: tags are an ordinary property of the
// resource model, edited with a JSON Patch like any other. Two things make
// that awkward enough to deserve helpers. The property's shape is per type —
// most model Tags as a list of {"Key","Value"} objects, a few as a plain
// object keyed by tag — and a list can only be patched positionally, so
// changing one tag by key means reading the current list first and computing
// the patch from it. SetResourceTags and UntagResource do both.
//
// For resources that have an ARN, resourcegroupstagging.TagResources is the
// better tool: synchronous, keyed, type-agnostic and batched. Reach for these
// when the resource has no ARN, or when tags change together with other
// properties and the caller wants to build on tagsPatch.

// mapShapedTagTypes lists the types known to model Tags as an object rather
// than a list. The current value of the property decides when the resource
// already carries tags; this table only decides the shape to create when it
// does not. An unlisted map-shaped type that is untagged gets a list, which
// the handler rejects with a schema error rather than mis-tagging.
var mapShapedTagTypes = map[cfg.ResourceType]bool{
	"AWS::SSM::Parameter":            true,
	"AWS::Glue::Job":                 true,
	"AWS::Glue::Crawler":             true,
	"AWS::Glue::Trigger":             true,
	"AWS::Glue::Workflow":            true,
	"AWS::Glue::MLTransform":         true,
	"AWS::Batch::ComputeEnvironment": true,
	"AWS::Batch::JobQueue":           true,
	"AWS::Batch::JobDefinition":      true,
	"AWS::Batch::SchedulingPolicy":   true,
}

// SetResourceTags adds or replaces the given tags on a resource, keeping every
// other tag it carries. It reads the current properties, merges, and submits
// one UpdateResource; the returned event's RequestToken feeds
// WaitForResourceRequest. An empty tags map is a no-op returning nil, nil.
func (r *CloudControlRepository) SetResourceTags(resourceType cfg.ResourceType, identifier string, tags map[string]string) (*types.ProgressEvent, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	return r.patchTags(resourceType, identifier, tags, nil)
}

// UntagResource removes the given tag keys from a resource, keeping the rest.
// Keys the resource does not carry are not an error. No keys is a no-op
// returning nil, nil.
func (r *CloudControlRepository) UntagResource(resourceType cfg.ResourceType, identifier string, keys ...string) (*types.ProgressEvent, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	return r.patchTags(resourceType, identifier, nil, keys)
}

func (r *CloudControlRepository) patchTags(resourceType cfg.ResourceType, identifier string, set map[string]string, remove []string) (*types.ProgressEvent, error) {
	detail, err := r.DescribeResource(resourceType, aws.String(identifier))
	if err != nil {
		return nil, err
	}

	var attributes map[string]interface{}
	if detail.ResourceDescription != nil {
		attributes, _, err = ParseAttributes(*detail.ResourceDescription)
		if err != nil {
			return nil, errors.New(err)
		}
	}

	patch, err := tagsPatch(resourceType, attributes, set, remove)
	if err != nil {
		return nil, err
	}

	return r.UpdateResource(resourceType, identifier, patch)
}

// tagsPatch builds the JSON Patch document that applies set and remove to the
// Tags property in attributes, preserving the property's shape and, for a
// list, the order of the tags already present. New keys go last, sorted, so
// the same change always yields the same document.
func tagsPatch(resourceType cfg.ResourceType, attributes map[string]interface{}, set map[string]string, remove []string) (string, error) {
	removed := map[string]bool{}
	for _, key := range remove {
		removed[key] = true
	}

	var value interface{}

	switch current := attributes["Tags"].(type) {
	case map[string]interface{}:
		value = mergeTagMap(current, set, removed)
	case []interface{}:
		value = mergeTagList(current, set, removed)
	case nil:
		if mapShapedTagTypes[resourceType] {
			value = mergeTagMap(nil, set, removed)
		} else {
			value = mergeTagList(nil, set, removed)
		}
	default:
		return "", errors.Errorf("cloudcontrol: %s Tags property has unexpected shape %T", resourceType, current)
	}

	// "add" on an existing member replaces it (RFC 6902 §4.1), and on a missing
	// one creates it, so a single op covers a tagged and an untagged resource.
	patch, err := json.Marshal([]map[string]interface{}{
		{"op": "add", "path": "/Tags", "value": value},
	})
	if err != nil {
		return "", errors.New(err)
	}

	return string(patch), nil
}

func mergeTagMap(current map[string]interface{}, set map[string]string, removed map[string]bool) map[string]interface{} {
	merged := make(map[string]interface{}, len(current)+len(set))
	for key, value := range current {
		if !removed[key] {
			merged[key] = value
		}
	}
	for key, value := range set {
		merged[key] = value
	}

	return merged
}

func mergeTagList(current []interface{}, set map[string]string, removed map[string]bool) []map[string]string {
	merged := make([]map[string]string, 0, len(current)+len(set))
	seen := map[string]bool{}

	for _, item := range current {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := entry["Key"].(string)
		value, _ := entry["Value"].(string)
		if key == "" || removed[key] || seen[key] {
			continue
		}
		seen[key] = true

		if replacement, ok := set[key]; ok {
			value = replacement
		}
		merged = append(merged, map[string]string{"Key": key, "Value": value})
	}

	added := make([]string, 0, len(set))
	for key := range set {
		if !seen[key] {
			added = append(added, key)
		}
	}
	sort.Strings(added)

	for _, key := range added {
		merged = append(merged, map[string]string{"Key": key, "Value": set[key]})
	}

	return merged
}
