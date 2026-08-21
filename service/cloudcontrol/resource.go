package cloudcontrol

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/service"
)

// Resource is a schema-less AWS resource fetched through the Cloud Control API.
//
// Unlike the typed entities in this package (Instance, Bucket, Volume) it embeds
// no service-specific SDK struct: the resource's own fields stay in Attributes,
// exactly as Cloud Control returned them. That is what lets one repository
// method serve any AWS::*::* type whose registry entry implements the LIST
// handler, with no per-type Go code — the point of the generic path.
//
// Attributes and Tags are exported deliberately. The cache handlers serialize
// with encoding/gob, which silently ignores unexported fields, so an unexported
// map here would come back empty from a cache hit rather than erroring — the
// resource would look real and its properties would have vanished.
type Resource struct {
	service.AbstractResource
	cc.ResourceDescription

	// Attributes is the parsed Properties JSON object. Nested objects and
	// arrays are preserved as map[string]interface{} and []interface{}; both
	// are gob-registered in this package so they survive the cache.
	Attributes map[string]interface{}

	// Tags is lifted out of Attributes when the type carries a CloudFormation
	// style Tags list. Empty for types that do not.
	Tags map[string]string
}

// NewResource builds a generic resource from one Cloud Control list/get result.
// resourceType is passed in rather than derived because ResourceDescription does
// not carry the type it belongs to.
func NewResource(
	client AwsClient,
	resourceType cfg.ResourceType,
	resource cc.ResourceDescription,
	attributes map[string]interface{},
	tags map[string]string,
) Resource {
	return Resource{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(resource.Identifier),
			ARN:       arnFromAttributes(attributes),
			Type:      resourceType,
			// CreatedAt is left at the zero value on purpose: Cloud Control
			// reports no creation time, and a placeholder like time.Unix(0, 0)
			// would serialize as a real 1970 timestamp to callers.
		},
		ResourceDescription: resource,
		Attributes:          attributes,
		Tags:                tags,
	}
}

// GetName resolves a display name without type-specific knowledge: the Name tag
// first (the convention most AWS consoles use), then a Name-ish property, then
// nothing. Callers still have GetId for identity — this is for humans.
func (e Resource) GetName() string {
	if name, ok := e.Tags["Name"]; ok && name != "" {
		return name
	}

	for _, key := range []string{"Name", "ResourceName", "DisplayName"} {
		if name, ok := e.Attributes[key].(string); ok && name != "" {
			return name
		}
	}

	return ""
}

// GetAttributes exposes the parsed Cloud Control properties. Consumers that
// curate attributes per resource type (e.g. an MCP server's summary view) can
// type-assert for this method to get a generic fallback for free.
func (e Resource) GetAttributes() map[string]interface{} {
	return e.Attributes
}

func (e Resource) GetTags() map[string]string {
	return e.Tags
}

func (e Resource) GetTagValue(tag string) string {
	return e.Tags[tag]
}

// arnFromAttributes recovers an ARN from the resource's own properties.
//
// A correct ARN cannot be synthesized generically: ResourceDescription carries
// only an opaque Identifier, and the service/resource-path segments differ per
// type. Many Cloud Control types do expose their ARN as a property, so this
// lifts one when present and returns nil otherwise — GetIdOrArn then falls back
// to the identifier, which is always set.
func arnFromAttributes(attributes map[string]interface{}) *arn.ARN {
	for _, key := range []string{"Arn", "ARN", "ResourceArn", "ResourceARN"} {
		raw, ok := attributes[key].(string)
		if !ok || raw == "" {
			continue
		}

		if parsed, err := arn.Parse(raw); err == nil {
			return &parsed
		}
	}

	return nil
}
