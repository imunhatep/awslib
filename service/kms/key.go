package kms

import (
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/gocollection/dict"
)

// aliasPrefix starts every alias name.
const aliasPrefix = "alias/"

type KeyList struct {
	Items []Key
}

// Key is a KMS key as described by DescribeKey, with its aliases and, for
// customer managed keys, its tags.
//
// A key whose key policy denies DescribeKey is still listed: KeyMetadata then
// holds only the KeyId and Arn that ListKeys returned, and DescribeError says
// why the rest is missing. Its key manager, state and usage are unknown, not
// zero — check IsDescribed before reading them.
//
// Tags are read only for customer managed keys; AWS managed keys carry none
// that the account controls. TagsError is set when they were not read for a
// customer managed key, or could not be read for a key that was not described,
// so that a failed read is never presented as an untagged key.
type Key struct {
	service.AbstractResource
	kmstypes.KeyMetadata

	// Aliases are the full alias names (alias/...) that target the key, sorted.
	Aliases []string

	Tags map[string]string

	// DescribeError is the DescribeKey failure for this key, empty when the
	// key was described.
	DescribeError string

	// TagsError is why the tags of this key are not known, empty when they
	// were read or the key is not customer managed.
	TagsError string
}

// NewKey builds a key from its DescribeKey metadata.
func NewKey(client AwsClient, metadata kmstypes.KeyMetadata, aliases []string, tags map[string]string) Key {
	return Key{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(metadata.KeyId),
			ARN:       parseArn(metadata.Arn),
			CreatedAt: aws.ToTime(metadata.CreationDate),
			Type:      cfg.ResourceTypeKey,
		},
		KeyMetadata: metadata,
		Aliases:     sortedAliases(aliases),
		Tags:        tags,
	}
}

// NewUndescribedKey builds a key DescribeKey could not read, from its ListKeys
// entry and the error. Its tags are reported as unread.
func NewUndescribedKey(client AwsClient, entry kmstypes.KeyListEntry, aliases []string, describeErr error) Key {
	key := NewKey(client, kmstypes.KeyMetadata{KeyId: entry.KeyId, Arn: entry.KeyArn}, aliases, nil)

	key.DescribeError = errorString(describeErr, "DescribeKey failed")
	key.TagsError = "tags not read: the key could not be described"

	return key
}

// GetName returns the first alias without its alias/ prefix, or the key ID for
// a key without aliases.
func (e Key) GetName() string {
	if len(e.Aliases) > 0 {
		return strings.TrimPrefix(e.Aliases[0], aliasPrefix)
	}

	return e.GetId()
}

func (e Key) GetTags() map[string]string {
	return dict.Copy(e.Tags)
}

func (e Key) GetTagValue(tag string) string {
	return e.Tags[tag]
}

// GetAliases returns a copy of the full alias names (alias/...), sorted.
func (e Key) GetAliases() []string {
	return slices.Clone(e.Aliases)
}

// IsDescribed reports whether DescribeKey answered for the key, i.e. whether
// the metadata beyond KeyId and Arn is known.
func (e Key) IsDescribed() bool {
	return e.DescribeError == ""
}

// TagsKnown reports whether GetTags is the key's real tag set: true for a
// customer managed key whose tags were read and for an AWS managed key, false
// when the tags could not be read.
func (e Key) TagsKnown() bool {
	return e.TagsError == ""
}

// GetKeyManager returns CUSTOMER or AWS, empty for a key that was not described.
func (e Key) GetKeyManager() string {
	return string(e.KeyManager)
}

// IsCustomerManaged reports whether the account manages the key. False for a
// key that was not described.
func (e Key) IsCustomerManaged() bool {
	return e.KeyManager == kmstypes.KeyManagerTypeCustomer
}

// IsAwsManaged reports whether an AWS service created and manages the key.
func (e Key) IsAwsManaged() bool {
	return e.KeyManager == kmstypes.KeyManagerTypeAws
}

// GetKeyState returns the key state (Enabled, Disabled, PendingDeletion, ...),
// empty for a key that was not described.
func (e Key) GetKeyState() string {
	return string(e.KeyState)
}

// GetKeyUsage returns what the key is for (ENCRYPT_DECRYPT, SIGN_VERIFY,
// GENERATE_VERIFY_MAC, KEY_AGREEMENT), empty for a key that was not described.
func (e Key) GetKeyUsage() string {
	return string(e.KeyUsage)
}

// GetKeySpec returns the key material type (SYMMETRIC_DEFAULT, RSA_2048, ...),
// empty for a key that was not described.
func (e Key) GetKeySpec() string {
	return string(e.KeySpec)
}

// IsEnabled reports the key's Enabled flag. False for a key that was not
// described, so read it together with IsDescribed.
func (e Key) IsEnabled() bool {
	return e.Enabled
}

// IsMultiRegion reports whether the key is a multi-Region primary or replica.
func (e Key) IsMultiRegion() bool {
	return aws.ToBool(e.MultiRegion)
}

// GetMultiRegionKeyType returns PRIMARY or REPLICA for a multi-Region key,
// empty otherwise.
func (e Key) GetMultiRegionKeyType() string {
	if e.MultiRegionConfiguration == nil {
		return ""
	}

	return string(e.MultiRegionConfiguration.MultiRegionKeyType)
}

// GetOrigin returns where the key material came from (AWS_KMS, EXTERNAL,
// AWS_CLOUDHSM, EXTERNAL_KEY_STORE), empty for a key that was not described.
func (e Key) GetOrigin() string {
	return string(e.Origin)
}

// GetDescription returns the key's description.
func (e Key) GetDescription() string {
	return aws.ToString(e.Description)
}

// GetDeletionDate returns when a key pending deletion will be deleted, nil for
// a key that is not scheduled for deletion.
func (e Key) GetDeletionDate() *time.Time {
	return e.DeletionDate
}

// IsPendingDeletion reports whether the key is scheduled for deletion. A
// multi-Region primary waiting for its replicas to be deleted counts too.
func (e Key) IsPendingDeletion() bool {
	return e.KeyState == kmstypes.KeyStatePendingDeletion ||
		e.KeyState == kmstypes.KeyStatePendingReplicaDeletion
}

func sortedAliases(aliases []string) []string {
	if len(aliases) == 0 {
		return nil
	}

	out := slices.Clone(aliases)
	slices.Sort(out)

	return out
}

func parseArn(value *string) *arn.ARN {
	if value == nil || *value == "" {
		return nil
	}

	parsed, err := arn.Parse(*value)
	if err != nil {
		return nil
	}

	return &parsed
}

func tagsToMap(tags []kmstypes.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[aws.ToString(tag.TagKey)] = aws.ToString(tag.TagValue)
	}

	return out
}

func errorString(err error, fallback string) string {
	if err == nil || err.Error() == "" {
		return fallback
	}

	return err.Error()
}
