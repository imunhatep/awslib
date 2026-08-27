package resourcegroupstagging

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/imunhatep/awslib/service"
)

// TagMapping pairs one resource ARN with its tags, as the Resource Groups
// Tagging API reports them.
//
// It is deliberately not a service.ResourceInterface: the API answers about tags
// on a resource, it does not return the resource. An ARN and a tag map is all
// there is, and dressing that up as a resource would put entries in a result set
// that carry no type, no name and no creation time.
//
// Both fields are exported because the repository's Get* methods are wrapped by
// a generated cached repository and the cache handlers serialize with gob, which
// drops unexported fields without erroring — a cache hit would return mappings
// with no tags at all.
type TagMapping struct {
	Arn  string
	Tags map[string]string
}

func newTagMapping(mapping rgttypes.ResourceTagMapping) TagMapping {
	tags := make(map[string]string, len(mapping.Tags))
	for _, tag := range mapping.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return TagMapping{
		Arn:  aws.ToString(mapping.ResourceARN),
		Tags: tags,
	}
}

// TagIndex is a lookup table built from one region's tag mappings.
//
// It is a view over data the repository already returned, not a repository
// result itself: the cache stores []TagMapping and an index is rebuilt from it,
// so there is one representation on the wire and no chance of the two drifting.
// The fields are still exported, because generate-gob registers every exported
// struct in this package and a caller who does cache a TagIndex would otherwise
// get one back with both maps silently empty.
type TagIndex struct {
	// ByArn maps a resource ARN to its tags. This is the exact join: the ARN came
	// from AWS on both sides.
	ByArn map[string]map[string]string

	// ById maps a resource identifier — the ARN's last path or colon segment — to
	// the same tag maps, for resources whose ARN a caller does not know. Only
	// unambiguous identifiers appear: see NewTagIndex.
	ById map[string]map[string]string
}

// NewTagIndex indexes mappings by ARN and by identifier.
//
// The identifier index exists for Cloud Control resources, which carry an opaque
// identifier and usually no ARN, so an exact join is impossible. Deriving the
// identifier from the ARN is a heuristic and is treated as one: an identifier
// produced by more than one ARN is dropped from ById entirely rather than
// resolved to an arbitrary winner. A missing tag lookup is recoverable; tags
// attributed to the wrong resource are not.
//
// Narrowing the mappings to a single resource type before indexing (see
// GetResourceTagsByType) makes collisions unlikely, since the identifier is only
// ambiguous across types.
func NewTagIndex(mappings []TagMapping) *TagIndex {
	index := &TagIndex{
		ByArn: make(map[string]map[string]string, len(mappings)),
		ById:  make(map[string]map[string]string, len(mappings)),
	}

	ambiguous := map[string]bool{}
	for _, mapping := range mappings {
		if mapping.Arn == "" {
			continue
		}

		index.ByArn[mapping.Arn] = mapping.Tags

		id := identifierFromArn(mapping.Arn)
		if id == "" {
			continue
		}

		if _, seen := index.ById[id]; seen || ambiguous[id] {
			// Two ARNs, one identifier: neither answer can be trusted.
			delete(index.ById, id)
			ambiguous[id] = true

			continue
		}

		index.ById[id] = mapping.Tags
	}

	return index
}

// Len reports how many ARNs the index holds.
func (e *TagIndex) Len() int {
	if e == nil {
		return 0
	}

	return len(e.ByArn)
}

// LookupArn returns the tags for an ARN, or nil when the index holds none.
func (e *TagIndex) LookupArn(arn string) map[string]string {
	if e == nil || arn == "" {
		return nil
	}

	return e.ByArn[arn]
}

// LookupId returns the tags for a resource identifier, or nil when the index
// holds none or the identifier is ambiguous.
func (e *TagIndex) LookupId(id string) map[string]string {
	if e == nil || id == "" {
		return nil
	}

	return e.ById[id]
}

// Lookup returns the tags for a resource, preferring the exact ARN join and
// falling back to the identifier only when the resource reports no ARN.
//
// The fallback is what makes this usable for Cloud Control resources, whose ARN
// is empty for most types. Note it is a lookup heuristic and nothing more: the
// worst case is an unfilled tag map, never an ARN synthesized onto a resource.
func (e *TagIndex) Lookup(resource service.ResourceInterface) map[string]string {
	if e == nil || resource == nil {
		return nil
	}

	if tags := e.LookupArn(resource.GetArn()); tags != nil {
		return tags
	}

	return e.LookupId(resource.GetId())
}

// identifierFromArn recovers the resource identifier from an ARN.
//
// An ARN is arn:partition:service:region:account:resource, and the resource part
// spells identity differently per service: "instance/i-0123", "function:name",
// or a bare "bucket-name". Taking the segment after the last separator covers
// all three. It does not cover a nested path (an IAM role at "role/path/name"
// yields "name"), which is one of the reasons NewTagIndex drops collisions
// instead of trusting this.
func identifierFromArn(arn string) string {
	const arnPrefixParts = 5

	parts := strings.SplitN(arn, ":", arnPrefixParts+1)
	if len(parts) < arnPrefixParts+1 {
		return ""
	}

	resource := parts[arnPrefixParts]
	if idx := strings.LastIndexAny(resource, "/:"); idx >= 0 {
		resource = resource[idx+1:]
	}

	return resource
}
