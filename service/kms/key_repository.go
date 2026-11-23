package kms

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

func (r *KmsRepository) ListKeysAll() ([]Key, error) {
	return r.ListKeysByInput(&awskms.ListKeysInput{})
}

// ListKeysByInput lists the keys of the region with their metadata, aliases
// and tags.
//
// ListKeys returns IDs only, so every key costs a DescribeKey, and a customer
// managed key a ListResourceTags as well, run concurrently and bounded by
// keyDetailConcurrency. Aliases come from one ListAliases sweep. A failure of
// ListKeys or ListAliases fails the listing; a per-key failure does not — the
// key is kept with the error recorded in DescribeError or TagsError, because a
// key policy can deny one key's reads while allowing the rest, and dropping the
// key would understate the inventory. A key deleted between the listing and
// its reads is dropped.
func (r *KmsRepository) ListKeysByInput(query *awskms.ListKeysInput) ([]Key, error) {
	start := time.Now()

	entries, err := r.listKeyEntries(query)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return []Key{}, nil
	}

	aliases, err := r.keyAliases()
	if err != nil {
		return nil, err
	}

	keys := make([]Key, len(entries))
	deleted := make([]bool, len(entries))
	sem := make(chan struct{}, keyDetailConcurrency)

	var wg sync.WaitGroup
	for i := range entries {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			keys[idx], deleted[idx] = r.readKey(entries[idx], aliases[aws.ToString(entries[idx].KeyId)])
		}(i)
	}

	wg.Wait()

	// A cancelled context fails every per-key read the same way; recording that
	// on each key would pass off "the caller gave up" as "these keys are unreadable".
	if err := r.ctx.Err(); err != nil {
		return nil, errors.New(err)
	}

	result := make([]Key, 0, len(keys))
	for i, key := range keys {
		if !deleted[i] {
			result = append(result, key)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListKeys", cfg.ResourceTypeKey)).
			Add(float64(len(result)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListKeysByInput", cfg.ResourceTypeKey)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}

// readKey describes one key and reads its tags. deleted reports a key that no
// longer exists, which the listing drops.
func (r *KmsRepository) readKey(entry kmstypes.KeyListEntry, aliases []string) (key Key, deleted bool) {
	keyID := aws.ToString(entry.KeyId)

	metadata, err := r.describeKey(keyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.Debug().Str("keyID", keyID).Msg("[KmsRepository.ListKeysByInput] key deleted during listing")

			return Key{}, true
		}

		log.Debug().Err(err).Str("keyID", keyID).Msg("[KmsRepository.ListKeysByInput] key could not be described")

		return NewUndescribedKey(r.client, entry, aliases, err), false
	}

	key = NewKey(r.client, *metadata, aliases, nil)
	if !key.IsCustomerManaged() {
		return key, false
	}

	tags, err := r.keyTags(keyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.Debug().Str("keyID", keyID).Msg("[KmsRepository.ListKeysByInput] key deleted during listing")

			return Key{}, true
		}

		log.Debug().Err(err).Str("keyID", keyID).Msg("[KmsRepository.ListKeysByInput] key tags could not be read")

		key.TagsError = errorString(err, "ListResourceTags failed")

		return key, false
	}

	key.Tags = tags

	return key, false
}

func (r *KmsRepository) listKeyEntries(query *awskms.ListKeysInput) ([]kmstypes.KeyListEntry, error) {
	var entries []kmstypes.KeyListEntry

	p := awskms.NewListKeysPaginator(r.kmsClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListKeys", cfg.ResourceTypeKey)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListKeys", cfg.ResourceTypeKey)).Inc()
			}

			return nil, classifyErr(err)
		}

		entries = append(entries, resp.Keys...)
	}

	return entries, nil
}

// keyAliases returns the alias names of the region keyed by the ID of the key
// they target. AWS reserves aliases for AWS managed keys a service has not
// created yet; those target no key and are left out.
func (r *KmsRepository) keyAliases() (map[string][]string, error) {
	aliases := map[string][]string{}

	p := awskms.NewListAliasesPaginator(r.kmsClient(), &awskms.ListAliasesInput{})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListAliases", cfg.ResourceTypeKey)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListAliases", cfg.ResourceTypeKey)).Inc()
			}

			return nil, classifyErr(err)
		}

		for _, alias := range resp.Aliases {
			target := aws.ToString(alias.TargetKeyId)
			if target == "" {
				continue
			}

			aliases[target] = append(aliases[target], aws.ToString(alias.AliasName))
		}
	}

	return aliases, nil
}

func (r *KmsRepository) describeKey(keyID string) (*kmstypes.KeyMetadata, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeKey", cfg.ResourceTypeKey)).Inc()
	}

	output, err := r.kmsClient().DescribeKey(r.ctx, &awskms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	}, withDetailRetries)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DescribeKey", cfg.ResourceTypeKey)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.KeyMetadata == nil {
		return nil, errors.Errorf("DescribeKey returned no metadata for %s", keyID)
	}

	return output.KeyMetadata, nil
}

func (r *KmsRepository) keyTags(keyID string) (map[string]string, error) {
	var tags []kmstypes.Tag

	p := awskms.NewListResourceTagsPaginator(r.kmsClient(), &awskms.ListResourceTagsInput{
		KeyId: aws.String(keyID),
	})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListResourceTags", cfg.ResourceTypeKey)).Inc()
		}

		resp, err := p.NextPage(r.ctx, withDetailRetries)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListResourceTags", cfg.ResourceTypeKey)).Inc()
			}

			return nil, classifyErr(err)
		}

		tags = append(tags, resp.Tags...)
	}

	return tagsToMap(tags), nil
}
