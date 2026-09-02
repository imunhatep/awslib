package secretmanager

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecretName = "prod/app/db-password"
	testSecretArn  = "arn:aws:secretsmanager:eu-central-1:111111111111:secret:prod/app/db-password-AbCdEf"
)

type mockSecret struct {
	service.AbstractResource
	name string
	tags map[string]string
}

func (m mockSecret) GetName() string { return m.name }

func (m mockSecret) GetId() string { return m.name }

func (m mockSecret) GetTags() map[string]string { return m.tags }

func (m mockSecret) GetTagValue(key string) string { return m.tags[key] }

func secretWithTags(tags map[string]string) mockSecret {
	parsed := mustParseArn(testSecretArn)

	return mockSecret{
		AbstractResource: service.AbstractResource{ARN: &parsed},
		name:             testSecretName,
		tags:             tags,
	}
}

func TestBuildTagResourceInput(t *testing.T) {
	input := BuildTagResourceInput(
		map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"},
		secretWithTags(map[string]string{"env": "tools"}),
	)

	require.NotNil(t, input)
	assert.Equal(t, []types.Tag{
		{Key: aws.String("lifecycle:delete:ts"), Value: aws.String("2026-09-02T00:00:00Z")},
	}, input.Tags)
}

// The whole requested set goes out, not just the tags that differ: TagResource is an
// upsert, so re-sending a correct tag costs nothing and keeps the call idempotent.
func TestBuildTagResourceInputSendsWholeSetWhenAnyTagDiffers(t *testing.T) {
	input := BuildTagResourceInput(
		map[string]string{"a": "1", "b": "2"},
		secretWithTags(map[string]string{"a": "1"}),
	)

	require.NotNil(t, input)
	assert.Equal(t, []types.Tag{
		{Key: aws.String("a"), Value: aws.String("1")},
		{Key: aws.String("b"), Value: aws.String("2")},
	}, input.Tags)
}

// Nil is the "already in sync" answer, and it is what lets a reconciler call this every
// cycle without issuing a write.
func TestBuildTagResourceInputNilWhenInSync(t *testing.T) {
	assert.Nil(t, BuildTagResourceInput(
		map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"},
		secretWithTags(map[string]string{
			"lifecycle:delete:ts": "2026-09-02T00:00:00Z",
			"env":                 "tools",
		}),
	))
}

// A tag present with a different value is not in sync — the mark timestamp changing is
// exactly this case.
func TestBuildTagResourceInputUpdatesChangedValue(t *testing.T) {
	input := BuildTagResourceInput(
		map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"},
		secretWithTags(map[string]string{"lifecycle:delete:ts": "2025-01-01T00:00:00Z"}),
	)

	require.NotNil(t, input)
	assert.Equal(t, "2026-09-02T00:00:00Z", aws.ToString(input.Tags[0].Value))
}

func TestBuildTagResourceInputNilWhenNoTagsRequested(t *testing.T) {
	assert.Nil(t, BuildTagResourceInput(map[string]string{}, secretWithTags(nil)))
	assert.Nil(t, BuildTagResourceInput(nil, secretWithTags(map[string]string{"a": "1"})))
}

// An empty requested value means "remove this key whatever it holds", the same contract
// ec2.BuildDeleteTagsInput uses, so the lifecycle's map[tag]"" idiom carries over.
func TestBuildUntagResourceInputEmptyValueMatchesAnyValue(t *testing.T) {
	input := BuildUntagResourceInput(
		map[string]string{"lifecycle:delete:ts": ""},
		secretWithTags(map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"}),
	)

	require.NotNil(t, input)
	assert.Equal(t, []string{"lifecycle:delete:ts"}, input.TagKeys)
}

func TestBuildUntagResourceInputMatchesExactValue(t *testing.T) {
	secret := secretWithTags(map[string]string{"env": "tools", "keep": "yes"})

	matching := BuildUntagResourceInput(map[string]string{"env": "tools"}, secret)
	require.NotNil(t, matching)
	assert.Equal(t, []string{"env"}, matching.TagKeys)

	assert.Nil(t, BuildUntagResourceInput(map[string]string{"env": "live"}, secret),
		"a value that does not match must not be removed")
}

func TestBuildUntagResourceInputNilWhenTagAbsent(t *testing.T) {
	assert.Nil(t, BuildUntagResourceInput(
		map[string]string{"lifecycle:delete:ts": ""},
		secretWithTags(map[string]string{"env": "tools"}),
	))
	assert.Nil(t, BuildUntagResourceInput(map[string]string{"a": ""}, secretWithTags(nil)))
}

// Sorted output, so the same secret and tag set always build the same input.
func TestBuildUntagResourceInputSortsKeys(t *testing.T) {
	secret := secretWithTags(map[string]string{"c": "3", "a": "1", "b": "2"})

	for i := 0; i < 10; i++ {
		input := BuildUntagResourceInput(map[string]string{"c": "", "a": "", "b": ""}, secret)
		require.NotNil(t, input)
		assert.Equal(t, []string{"a", "b", "c"}, input.TagKeys)
	}
}

// Both builders address the secret by ARN, never by name. A secret's ARN carries a
// random six-character suffix and Secrets Manager resolves names and partial ARNs by
// prefix, so tagging by name can land on a different secret when names overlap.
func TestTagInputsAddressSecretByArnNotName(t *testing.T) {
	secret := secretWithTags(map[string]string{"env": "tools"})

	tagInput := BuildTagResourceInput(map[string]string{"a": "1"}, secret)
	require.NotNil(t, tagInput)
	assert.Equal(t, testSecretArn, aws.ToString(tagInput.SecretId))
	assert.NotEqual(t, testSecretName, aws.ToString(tagInput.SecretId))

	untagInput := BuildUntagResourceInput(map[string]string{"env": ""}, secret)
	require.NotNil(t, untagInput)
	assert.Equal(t, testSecretArn, aws.ToString(untagInput.SecretId))
}

func TestTagMapToTagsSortsByKey(t *testing.T) {
	assert.Equal(t, []types.Tag{
		{Key: aws.String("a"), Value: aws.String("1")},
		{Key: aws.String("b"), Value: aws.String("2")},
		{Key: aws.String("c"), Value: aws.String("3")},
	}, TagMapToTags(map[string]string{"c": "3", "a": "1", "b": "2"}))

	assert.Empty(t, TagMapToTags(nil))
}

func mustParseArn(value string) arn.ARN {
	parsed, err := arn.Parse(value)
	if err != nil {
		panic(err)
	}

	return parsed
}
