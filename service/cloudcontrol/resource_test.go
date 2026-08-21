package cloudcontrol

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct{}

func (fakeClient) GetRegion() ptypes.AwsRegion       { return "eu-central-1" }
func (fakeClient) GetAccountID() ptypes.AwsAccountID { return "123456789012" }

// properties mirrors a realistic Cloud Control payload: a nested object, an
// array of objects, a JSON null, and scalars of each kind.
const properties = `{
	"BucketName": "example",
	"Arn": "arn:aws:s3:::example",
	"VersioningConfiguration": {"Status": "Enabled"},
	"Tags": [{"Key": "Name", "Value": "example-bucket"}, {"Key": "env", "Value": "prod"}],
	"AccelerateConfiguration": null,
	"ObjectLockEnabled": false,
	"ReplicaCount": 3
}`

func testResource(t *testing.T) Resource {
	t.Helper()

	desc := cc.ResourceDescription{
		Identifier: aws.String("example"),
		Properties: aws.String(properties),
	}

	attributes, tags, err := ParseAttributes(desc)
	require.NoError(t, err)

	return NewResource(fakeClient{}, cfg.ResourceTypeBucket, desc, attributes, tags)
}

// TestResourceSurvivesGobRoundTrip pins the reason Attributes and Tags are
// exported. The cache handlers serialize with encoding/gob, which ignores
// unexported fields without erroring — so an unexported attribute map returns
// empty from a cache hit and the resource silently loses every property.
func TestResourceSurvivesGobRoundTrip(t *testing.T) {
	in := testResource(t)

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))

	var out Resource
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))

	assert.Equal(t, "example", out.GetId())
	assert.Equal(t, cfg.ResourceTypeBucket, out.GetType())
	assert.Equal(t, ptypes.AwsAccountID("123456789012"), out.GetAccountID())

	// nested object and array of objects both survive
	assert.Equal(t, map[string]interface{}{"Status": "Enabled"}, out.Attributes["VersioningConfiguration"])
	assert.Len(t, out.Attributes["Tags"], 2)

	// a JSON null round-trips as a present key with a nil value
	require.Contains(t, out.Attributes, "AccelerateConfiguration")
	assert.Nil(t, out.Attributes["AccelerateConfiguration"])

	assert.Equal(t, false, out.Attributes["ObjectLockEnabled"])
	assert.Equal(t, float64(3), out.Attributes["ReplicaCount"])

	assert.Equal(t, map[string]string{"Name": "example-bucket", "env": "prod"}, out.GetTags())
}

// TestResourceSurvivesGobRoundTripAsInterface covers the path a consumer takes
// when it caches []service.ResourceInterface through the public DataCache API:
// the value then sits in an interface field, which needs the gob registration
// from gob_register_gen.go.
func TestResourceSurvivesGobRoundTripAsInterface(t *testing.T) {
	in := []service.ResourceInterface{testResource(t)}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(&in))

	var out []service.ResourceInterface
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))

	require.Len(t, out, 1)
	assert.Equal(t, "example", out[0].GetId())
	assert.Equal(t, "example-bucket", out[0].GetName())
}

func TestArnFromAttributes(t *testing.T) {
	tests := map[string]struct {
		attributes map[string]interface{}
		want       string
	}{
		"lifted from Arn":         {map[string]interface{}{"Arn": "arn:aws:s3:::example"}, "arn:aws:s3:::example"},
		"lifted from ARN":         {map[string]interface{}{"ARN": "arn:aws:sqs:eu-central-1:1:q"}, "arn:aws:sqs:eu-central-1:1:q"},
		"absent":                  {map[string]interface{}{"BucketName": "x"}, ""},
		"unparsable is ignored":   {map[string]interface{}{"Arn": "not-an-arn"}, ""},
		"non-string is ignored":   {map[string]interface{}{"Arn": 42}, ""},
		"empty string is ignored": {map[string]interface{}{"Arn": ""}, ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := Resource{AbstractResource: service.AbstractResource{ARN: arnFromAttributes(tc.attributes)}}
			assert.Equal(t, tc.want, got.GetArn())
		})
	}
}

// TestResourceGetIdOrArnFallsBackToIdentifier documents why a nil ARN is
// acceptable for a generic resource: the identifier is always set.
func TestResourceGetIdOrArnFallsBackToIdentifier(t *testing.T) {
	r := NewResource(
		fakeClient{},
		cfg.ResourceTypeQueue,
		cc.ResourceDescription{Identifier: aws.String("q-1"), Properties: aws.String(`{}`)},
		map[string]interface{}{},
		map[string]string{},
	)

	assert.Empty(t, r.GetArn())
	assert.Equal(t, "q-1", r.GetIdOrArn())
}

func TestResourceGetName(t *testing.T) {
	tests := map[string]struct {
		attributes map[string]interface{}
		tags       map[string]string
		want       string
	}{
		"name tag wins over property": {
			map[string]interface{}{"Name": "from-property"},
			map[string]string{"Name": "from-tag"},
			"from-tag",
		},
		"falls back to Name property": {
			map[string]interface{}{"Name": "from-property"},
			map[string]string{},
			"from-property",
		},
		"falls back to DisplayName": {
			map[string]interface{}{"DisplayName": "shown"},
			map[string]string{},
			"shown",
		},
		"empty when nothing is nameable": {
			map[string]interface{}{"BucketName": "x"},
			map[string]string{},
			"",
		},
		"empty tag value does not win": {
			map[string]interface{}{"Name": "from-property"},
			map[string]string{"Name": ""},
			"from-property",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := Resource{Attributes: tc.attributes, Tags: tc.tags}
			assert.Equal(t, tc.want, r.GetName())
		})
	}
}

// TestResourceCreatedAtIsZero guards the deliberate choice not to use
// time.Unix(0, 0) as a placeholder: Cloud Control reports no creation time, and
// a 1970 timestamp would be serialized to callers as if it were real.
func TestResourceCreatedAtIsZero(t *testing.T) {
	assert.True(t, testResource(t).GetCreatedAt().IsZero())
}

// TestParseAttributesRejectsNonListTags documents the tag-parsing contract the
// generic path relies on: a malformed Tags value is an error, not a panic, and
// the attributes are still usable.
func TestParseAttributesRejectsNonListTags(t *testing.T) {
	desc := cc.ResourceDescription{
		Identifier: aws.String("x"),
		Properties: aws.String(`{"Tags": {"env": "prod"}}`),
	}

	attributes, tags, err := ParseAttributes(desc)

	assert.Error(t, err)
	assert.Empty(t, tags)
	assert.Contains(t, attributes, "Tags")
}

// TestPropertiesAreValidJSON is a guard on the fixture itself.
func TestPropertiesAreValidJSON(t *testing.T) {
	var v map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(properties), &v))
}
