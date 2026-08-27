package resourcegroupstagging

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTagMappingConvertsTagList(t *testing.T) {
	mapping := newTagMapping(rgttypes.ResourceTagMapping{
		ResourceARN: aws.String("arn:aws:ec2:eu-central-1:123456789012:instance/i-0123456789abcdef0"),
		Tags: []rgttypes.Tag{
			{Key: aws.String("Name"), Value: aws.String("web-1")},
			{Key: aws.String("env"), Value: aws.String("prod")},
		},
	})

	assert.Equal(t, "arn:aws:ec2:eu-central-1:123456789012:instance/i-0123456789abcdef0", mapping.Arn)
	assert.Equal(t, map[string]string{"Name": "web-1", "env": "prod"}, mapping.Tags)
}

// TestNewTagMappingHandlesEmptyTagSet covers the resource that was tagged once
// and is not any more: the API returns it with "Tags": [], which must become an
// empty map rather than a nil one a caller has to guard against.
func TestNewTagMappingHandlesEmptyTagSet(t *testing.T) {
	mapping := newTagMapping(rgttypes.ResourceTagMapping{
		ResourceARN: aws.String("arn:aws:s3:::example"),
	})

	assert.Equal(t, "arn:aws:s3:::example", mapping.Arn)
	assert.NotNil(t, mapping.Tags)
	assert.Empty(t, mapping.Tags)
}

// TestTagMappingSurvivesGobRoundTrip pins the reason Arn and Tags are exported.
// The cache handlers serialize with encoding/gob, which drops unexported fields
// without erroring — a cache hit would return mappings whose tags had silently
// vanished, which looks exactly like a resource that carries no tags.
func TestTagMappingSurvivesGobRoundTrip(t *testing.T) {
	original := TagMapping{
		Arn:  "arn:aws:ec2:eu-central-1:123456789012:volume/vol-0123",
		Tags: map[string]string{"env": "prod", "team": "platform"},
	}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded TagMapping
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))

	assert.Equal(t, original.Arn, decoded.Arn)
	assert.Equal(t, original.Tags, decoded.Tags)
}

func TestIdentifierFromArn(t *testing.T) {
	cases := map[string]string{
		// slash-separated resource path
		"arn:aws:ec2:eu-central-1:123456789012:instance/i-0123": "i-0123",
		// colon-separated resource path
		"arn:aws:lambda:eu-central-1:123456789012:function:my-fn": "my-fn",
		// no resource path at all, and no region or account either
		"arn:aws:s3:::example-bucket": "example-bucket",
		// bare name in the resource position
		"arn:aws:sns:eu-central-1:123456789012:my-topic": "my-topic",
		// nested path: only the last segment survives, which is why
		// NewTagIndex refuses to trust a colliding identifier
		"arn:aws:iam::123456789012:role/service-role/my-role": "my-role",
		// not an ARN
		"i-0123":            "",
		"arn:aws:ec2:eu:12": "",
		"":                  "",
	}

	for arn, expected := range cases {
		assert.Equal(t, expected, identifierFromArn(arn), "identifier of %q", arn)
	}
}

func TestNewTagIndexIndexesByArnAndId(t *testing.T) {
	index := NewTagIndex([]TagMapping{
		{Arn: "arn:aws:ec2:eu-central-1:123456789012:instance/i-0123", Tags: map[string]string{"env": "prod"}},
		{Arn: "arn:aws:s3:::example", Tags: map[string]string{"env": "dev"}},
	})

	assert.Equal(t, 2, index.Len())
	assert.Equal(t, map[string]string{"env": "prod"}, index.LookupArn("arn:aws:ec2:eu-central-1:123456789012:instance/i-0123"))
	assert.Equal(t, map[string]string{"env": "prod"}, index.LookupId("i-0123"))
	assert.Equal(t, map[string]string{"env": "dev"}, index.LookupId("example"))
	assert.Nil(t, index.LookupArn("arn:aws:ec2:eu-central-1:123456789012:instance/i-9999"))
	assert.Nil(t, index.LookupId("i-9999"))
}

// TestNewTagIndexSkipsMappingsWithoutArn guards the index against an entry that
// carries no key: without the ARN there is nothing to look it up by, and ""
// would collide with every other keyless entry.
func TestNewTagIndexSkipsMappingsWithoutArn(t *testing.T) {
	index := NewTagIndex([]TagMapping{
		{Arn: "", Tags: map[string]string{"env": "prod"}},
	})

	assert.Equal(t, 0, index.Len())
	assert.Nil(t, index.LookupArn(""))
	assert.Nil(t, index.LookupId(""))
}

// TestNewTagIndexDropsAmbiguousIdentifiers is the load-bearing test for the
// identifier fallback. Two resource types can share an identifier, and picking
// whichever mapping arrived first would attribute one resource's tags to
// another. The ARN index keeps both; only the guess is withdrawn.
func TestNewTagIndexDropsAmbiguousIdentifiers(t *testing.T) {
	first := "arn:aws:ec2:eu-central-1:123456789012:instance/shared-name"
	second := "arn:aws:ecs:eu-central-1:123456789012:cluster/shared-name"

	index := NewTagIndex([]TagMapping{
		{Arn: first, Tags: map[string]string{"env": "prod"}},
		{Arn: second, Tags: map[string]string{"env": "dev"}},
	})

	assert.Nil(t, index.LookupId("shared-name"), "an ambiguous identifier must resolve to nothing, not to a winner")
	assert.Equal(t, map[string]string{"env": "prod"}, index.LookupArn(first))
	assert.Equal(t, map[string]string{"env": "dev"}, index.LookupArn(second))
}

// TestNewTagIndexKeepsAmbiguousIdentifierWithdrawn covers the third collision:
// once an identifier is known to be ambiguous it must not be re-admitted by a
// later mapping.
func TestNewTagIndexKeepsAmbiguousIdentifierWithdrawn(t *testing.T) {
	index := NewTagIndex([]TagMapping{
		{Arn: "arn:aws:ec2:eu-central-1:123456789012:instance/dup", Tags: map[string]string{"n": "1"}},
		{Arn: "arn:aws:ecs:eu-central-1:123456789012:cluster/dup", Tags: map[string]string{"n": "2"}},
		{Arn: "arn:aws:eks:eu-central-1:123456789012:cluster/dup", Tags: map[string]string{"n": "3"}},
	})

	assert.Nil(t, index.LookupId("dup"))
	assert.Equal(t, 3, index.Len())
}

// stubResource stands in for any service.ResourceInterface. The Cloud Control
// case — an identifier and no ARN — is the one the identifier fallback exists
// for, so it is exercised by leaving arn empty.
type stubResource struct {
	service.AbstractResource
	id  string
	arn string
}

func (e stubResource) GetId() string                     { return e.id }
func (e stubResource) GetArn() string                    { return e.arn }
func (e stubResource) GetIdOrArn() string                { return e.id }
func (e stubResource) GetName() string                   { return e.id }
func (e stubResource) GetTags() map[string]string        { return nil }
func (e stubResource) GetType() cfg.ResourceType         { return cfg.ResourceTypeInstance }
func (e stubResource) GetRegion() ptypes.AwsRegion       { return "eu-central-1" }
func (e stubResource) GetAccountID() ptypes.AwsAccountID { return "123456789012" }

func TestLookupPrefersArnOverIdentifier(t *testing.T) {
	arn := "arn:aws:ec2:eu-central-1:123456789012:instance/i-0123"

	index := NewTagIndex([]TagMapping{
		{Arn: arn, Tags: map[string]string{"via": "arn"}},
		{Arn: "arn:aws:s3:::i-0123", Tags: map[string]string{"via": "id-collision"}},
	})

	// Both mappings yield the identifier "i-0123", so ById withdrew it. The exact
	// ARN join still answers, which is the whole reason it is tried first.
	var resource service.ResourceInterface = stubResource{id: "i-0123", arn: arn}

	assert.Equal(t, map[string]string{"via": "arn"}, index.Lookup(resource))
}

// TestLookupFallsBackToIdentifier is the Cloud Control path: no ARN on the
// resource, so the identifier is all there is to match on.
func TestLookupFallsBackToIdentifier(t *testing.T) {
	index := NewTagIndex([]TagMapping{
		{Arn: "arn:aws:ec2:eu-central-1:123456789012:instance/i-0123", Tags: map[string]string{"env": "prod"}},
	})

	var resource service.ResourceInterface = stubResource{id: "i-0123"}

	assert.Equal(t, map[string]string{"env": "prod"}, index.Lookup(resource))
}

// TestLookupOnNilIndexIsSafe lets a caller treat "no tag index available" as an
// empty one instead of branching on nil at every call site.
func TestLookupOnNilIndexIsSafe(t *testing.T) {
	var index *TagIndex

	assert.Equal(t, 0, index.Len())
	assert.Nil(t, index.LookupArn("arn:aws:s3:::example"))
	assert.Nil(t, index.LookupId("example"))
	assert.Nil(t, index.Lookup(stubResource{id: "example"}))
}

func TestChunk(t *testing.T) {
	assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5}}, chunk([]int{1, 2, 3, 4, 5}, 2))
	assert.Equal(t, [][]int{{1, 2, 3}}, chunk([]int{1, 2, 3}, 3))
	assert.Equal(t, [][]int{{1, 2, 3}}, chunk([]int{1, 2, 3}, 10))
	assert.Nil(t, chunk([]int{}, 2))
	assert.Nil(t, chunk([]int{1}, 0))
}
