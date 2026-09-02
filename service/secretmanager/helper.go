package secretmanager

import (
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/imunhatep/awslib/service"
)

// Tag input builders for Secrets Manager, the counterparts of ec2.BuildCreateTagsInput
// and ec2.BuildDeleteTagsInput.
//
// Two differences from the EC2 pair, both forced by the API rather than chosen:
//
// TagResource and UntagResource address a single secret, so these take one resource
// instead of a variadic list — there is no batch form to build for.
//
// The secret is addressed by its **ARN**, never by its name. A secret's ARN carries a
// random six-character suffix, and Secrets Manager resolves a name or a partial ARN by
// prefix matching, which can land on a different secret when names overlap. Since
// SecretEntry.GetId() is the name, taking the ARN here is what keeps callers from
// tagging or untagging the wrong secret.
//
// Both return nil when the write would be a no-op, so a caller can hand the result
// straight to the repository — CreateSecretTags and DeleteSecretTags treat nil as
// "nothing to do". That is what lets a reconciler stay idempotent without doing its own
// read-modify-write.

// BuildTagResourceInput builds the input that brings the secret's tags in line with
// tags, or nil when every requested tag already holds the requested value.
//
// When an update is needed the input carries the whole requested set, not just the
// difference: TagResource is an upsert, so sending a tag that is already correct is
// free and keeps the call idempotent.
func BuildTagResourceInput(tags map[string]string, secret service.ResourceInterface) *sm.TagResourceInput {
	if len(tags) == 0 {
		return nil
	}

	secretTags := secret.GetTags()

	inSync := true
	for tag, value := range tags {
		if current, ok := secretTags[tag]; !ok || current != value {
			inSync = false
			break
		}
	}

	if inSync {
		return nil
	}

	return &sm.TagResourceInput{
		SecretId: aws.String(secret.GetArn()),
		Tags:     TagMapToTags(tags),
	}
}

// BuildUntagResourceInput builds the input removing the requested tags from the secret,
// or nil when the secret carries none of them.
//
// A tag is removed when the secret has the key and either the requested value matches
// the current one or the requested value is empty, which reads as "remove this key
// whatever it holds". That is the same contract as ec2.BuildDeleteTagsInput, so the
// lifecycle's map[TagLifecycleDeleteTS]"" idiom means the same thing for both resource
// types.
func BuildUntagResourceInput(tags map[string]string, secret service.ResourceInterface) *sm.UntagResourceInput {
	if len(tags) == 0 {
		return nil
	}

	secretTags := secret.GetTags()

	var tagKeys []string
	for tag, value := range tags {
		current, ok := secretTags[tag]
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

	// Sorted so the same secret and tag set always produce the same input: it keeps the
	// logs and the cache keys of anything wrapping this stable, and makes the tests
	// independent of map iteration order.
	sort.Strings(tagKeys)

	return &sm.UntagResourceInput{
		SecretId: aws.String(secret.GetArn()),
		TagKeys:  tagKeys,
	}
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
