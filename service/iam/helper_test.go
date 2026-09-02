package iam

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockIamResource struct {
	service.AbstractResource
	name string
	tags map[string]string
}

func (m mockIamResource) GetName() string { return m.name }

func (m mockIamResource) GetId() string { return m.name }

func (m mockIamResource) GetTags() map[string]string { return m.tags }

func (m mockIamResource) GetTagValue(key string) string { return m.tags[key] }

func resourceWithTags(tags map[string]string) mockIamResource {
	return mockIamResource{name: "svc-deploy", tags: tags}
}

func TestTagsToApply(t *testing.T) {
	tags := TagsToApply(
		map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"},
		resourceWithTags(map[string]string{"env": "tools"}),
	)

	assert.Equal(t, []types.Tag{
		{Key: aws.String("lifecycle:delete:ts"), Value: aws.String("2026-09-02T00:00:00Z")},
	}, tags)
}

// Nil is the "already in sync" answer, and it is what lets a reconciler run every cycle
// without issuing a write.
func TestTagsToApplyNilWhenInSync(t *testing.T) {
	assert.Nil(t, TagsToApply(
		map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"},
		resourceWithTags(map[string]string{
			"lifecycle:delete:ts": "2026-09-02T00:00:00Z",
			"env":                 "tools",
		}),
	))

	assert.Nil(t, TagsToApply(nil, resourceWithTags(map[string]string{"a": "1"})))
	assert.Nil(t, TagsToApply(map[string]string{}, resourceWithTags(nil)))
}

func TestTagsToApplyDetectsChangedValue(t *testing.T) {
	tags := TagsToApply(
		map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"},
		resourceWithTags(map[string]string{"lifecycle:delete:ts": "2025-01-01T00:00:00Z"}),
	)

	require.Len(t, tags, 1)
	assert.Equal(t, "2026-09-02T00:00:00Z", aws.ToString(tags[0].Value))
}

// The whole requested set is sent, not just the difference: IAM's Tag* calls are
// upserts, so re-sending a correct tag is free and keeps the call idempotent.
func TestTagsToApplySendsWholeSet(t *testing.T) {
	tags := TagsToApply(
		map[string]string{"a": "1", "b": "2"},
		resourceWithTags(map[string]string{"a": "1"}),
	)

	assert.Equal(t, []types.Tag{
		{Key: aws.String("a"), Value: aws.String("1")},
		{Key: aws.String("b"), Value: aws.String("2")},
	}, tags)
}

// An empty requested value means "remove this key whatever it holds", the same contract
// ec2.BuildDeleteTagsInput uses.
func TestTagKeysToRemoveEmptyValueMatchesAnyValue(t *testing.T) {
	assert.Equal(t, []string{"lifecycle:delete:ts"}, TagKeysToRemove(
		map[string]string{"lifecycle:delete:ts": ""},
		resourceWithTags(map[string]string{"lifecycle:delete:ts": "2026-09-02T00:00:00Z"}),
	))
}

func TestTagKeysToRemoveMatchesExactValue(t *testing.T) {
	resource := resourceWithTags(map[string]string{"env": "tools"})

	assert.Equal(t, []string{"env"}, TagKeysToRemove(map[string]string{"env": "tools"}, resource))
	assert.Nil(t, TagKeysToRemove(map[string]string{"env": "live"}, resource),
		"a value that does not match must not be removed")
}

func TestTagKeysToRemoveNilWhenAbsent(t *testing.T) {
	assert.Nil(t, TagKeysToRemove(
		map[string]string{"lifecycle:delete:ts": ""},
		resourceWithTags(map[string]string{"env": "tools"}),
	))
	assert.Nil(t, TagKeysToRemove(nil, resourceWithTags(map[string]string{"a": "1"})))
}

// Sorted, so the same resource and tag set always produce the same call.
func TestTagKeysToRemoveSorts(t *testing.T) {
	resource := resourceWithTags(map[string]string{"c": "3", "a": "1", "b": "2"})

	for i := 0; i < 10; i++ {
		assert.Equal(t, []string{"a", "b", "c"},
			TagKeysToRemove(map[string]string{"c": "", "a": "", "b": ""}, resource))
	}
}

func TestIamTagMapToTagsSortsByKey(t *testing.T) {
	assert.Equal(t, []types.Tag{
		{Key: aws.String("a"), Value: aws.String("1")},
		{Key: aws.String("b"), Value: aws.String("2")},
	}, TagMapToTags(map[string]string{"b": "2", "a": "1"}))

	assert.Empty(t, TagMapToTags(nil))
}

// GetRole takes a RoleName and rejects an ARN, and the path is not part of the name.
// Passing the ARN through was the bug this replaces.
func TestRoleNameFromArn(t *testing.T) {
	cases := map[string]string{
		"arn:aws:iam::111111111111:role/deploy":             "deploy",
		"arn:aws:iam::111111111111:role/svc/nested/deploy":  "deploy",
		"arn:aws:iam::111111111111:role/aws-service-role/x": "x",
		"deploy":                "deploy",
		"":                      "",
		"not-an-arn/with-slash": "not-an-arn/with-slash",
	}

	for input, want := range cases {
		assert.Equal(t, want, RoleNameFromArn(input), "input %q", input)
	}
}

// A virtual MFA device's serial is its ARN and the device is an IAM resource that leaks
// if only deactivated; a hardware device's serial is a manufacturer string and the
// device belongs to the account.
func TestIsVirtualMfaDevice(t *testing.T) {
	assert.True(t, isVirtualMfaDevice("arn:aws:iam::111111111111:mfa/svc-deploy"))
	assert.False(t, isVirtualMfaDevice("GAHT12345678"))
	assert.False(t, isVirtualMfaDevice(""))
}

// Service-linked roles need DeleteServiceLinkedRole, so the teardown has to refuse them
// rather than fail at DeleteRole with a message about the wrong thing.
func TestIsServiceLinkedRoleName(t *testing.T) {
	assert.True(t, isServiceLinkedRoleName("AWSServiceRoleForECS"))
	assert.True(t, isServiceLinkedRoleName("arn:aws:iam::1:role/aws-service-role/ecs.amazonaws.com/x"))
	assert.False(t, isServiceLinkedRoleName("deploy"))
	assert.False(t, isServiceLinkedRoleName("MyAWSServiceRole"))
}
