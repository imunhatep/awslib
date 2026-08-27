package resourcegroupstagging

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// GetResourceTagsAll returns the tags of every tagged resource in this client's
// region, across all resource types.
//
// One sweep serves any number of resource types, which is what makes it the
// right call for a multi-type fetch: the alternative is one filtered sweep per
// type over the same pages.
//
// It reports only resources that are currently tagged or that ever held a tag,
// so it cannot be used to enumerate resources — a never-tagged resource is
// absent, not empty.
func (r *ResourceGroupsTaggingRepository) GetResourceTagsAll() ([]TagMapping, error) {
	return r.getResourceTags(&awsrgt.GetResourcesInput{}, ccfg.ResourceTypeTagMapping)
}

// GetResourceTagsByType returns the tags of every tagged resource of one type.
//
// A type with no known filter falls back to the unfiltered region sweep rather
// than to a guessed filter, so the result is a superset instead of a silent
// under-report. See ResourceTypeFilter for why the distinction matters.
func (r *ResourceGroupsTaggingRepository) GetResourceTagsByType(resourceType cfg.ResourceType) ([]TagMapping, error) {
	return r.GetResourceTagsByTypes([]cfg.ResourceType{resourceType})
}

// GetResourceTagsByTypes returns the tags of every tagged resource of the given
// types.
//
// If any type has no known filter the whole call goes unfiltered, because a
// filtered request would answer about the mapped types only and the caller would
// have no way to tell. That trades pages for correctness, and it is logged.
func (r *ResourceGroupsTaggingRepository) GetResourceTagsByTypes(resourceTypes []cfg.ResourceType) ([]TagMapping, error) {
	filters, unmapped := ResourceTypeFilters(resourceTypes)

	// A single resource type gets its own metric label; a mixed request has no
	// one type to attribute the call to.
	labelType := ccfg.ResourceTypeTagMapping
	if len(resourceTypes) == 1 {
		labelType = resourceTypes[0]
	}

	query := &awsrgt.GetResourcesInput{ResourceTypeFilters: filters}
	if len(unmapped) > 0 {
		log.Debug().
			Str("accountID", r.client.GetAccountID().String()).
			Str("region", r.client.GetRegion().String()).
			Interface("types", unmapped).
			Msg("[ResourceGroupsTaggingRepository.GetResourceTagsByTypes] no tagging filter for resource types, falling back to an unfiltered region sweep")

		query.ResourceTypeFilters = nil
	}

	return r.getResourceTags(query, labelType)
}

// GetResourceTagsByArns returns the tags of specific resources.
//
// Use it to fill gaps in a known set; use GetResourceTagsAll to build an index
// for a whole region. An ARN that does not exist, or that belongs to another
// region, is left out of the response rather than raising an error, so a shorter
// result than the input is normal and not a failure.
func (r *ResourceGroupsTaggingRepository) GetResourceTagsByArns(arns []string) ([]TagMapping, error) {
	start := time.Now()
	mappings := []TagMapping{}

	for _, batch := range chunk(arns, maxArnsPerRead) {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("GetResources", ccfg.ResourceTypeTagMapping)).
				Inc()
		}

		// GetResources rejects ResourceARNList alongside any pagination parameter,
		// so this calls the operation directly instead of paginating: a request
		// bounded by maxArnsPerRead answers in one page by construction.
		resp, err := r.taggingClient().GetResources(r.ctx, &awsrgt.GetResourcesInput{ResourceARNList: batch})
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("GetResources", ccfg.ResourceTypeTagMapping)).
					Inc()
			}

			return mappings, errors.New(err)
		}

		for _, mapping := range resp.ResourceTagMappingList {
			mappings = append(mappings, newTagMapping(mapping))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetResources", ccfg.ResourceTypeTagMapping)).
			Add(float64(len(mappings)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetResourceTagsByArns", ccfg.ResourceTypeTagMapping)).
			Observe(time.Since(start).Seconds())
	}

	return mappings, nil
}

// GetResourceTagsByInput is the workhorse behind the methods above, exposed so
// callers can reach the parameters they do not cover: TagFilters to ask which
// resources carry a given tag, ResourcesPerPage, and the tag-policy compliance
// fields.
func (r *ResourceGroupsTaggingRepository) GetResourceTagsByInput(query *awsrgt.GetResourcesInput) ([]TagMapping, error) {
	return r.getResourceTags(query, ccfg.ResourceTypeTagMapping)
}

// getResourceTags paginates GetResources and converts each page.
//
// resourceType only labels metrics; the query decides what is fetched. Partial
// results are returned alongside an error, so a sweep that fails on page nine
// still hands back the first eight — the caller decides whether an incomplete
// tag index is worth using.
func (r *ResourceGroupsTaggingRepository) getResourceTags(query *awsrgt.GetResourcesInput, resourceType cfg.ResourceType) ([]TagMapping, error) {
	start := time.Now()
	mappings := []TagMapping{}

	p := awsrgt.NewGetResourcesPaginator(r.taggingClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("GetResources", resourceType)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("GetResources", resourceType)).
					Inc()
			}

			return mappings, errors.New(err)
		}

		for _, mapping := range resp.ResourceTagMappingList {
			mappings = append(mappings, newTagMapping(mapping))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetResources", resourceType)).
			Add(float64(len(mappings)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetResourceTags", resourceType)).
			Observe(time.Since(start).Seconds())
	}

	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Strs("filters", query.ResourceTypeFilters).
		Msgf("[ResourceGroupsTaggingRepository.getResourceTags] tag mappings found: %d", len(mappings))

	return mappings, nil
}

// ListTagKeys returns every tag key in use in this client's region.
func (r *ResourceGroupsTaggingRepository) ListTagKeys() ([]string, error) {
	keys := []string{}

	p := awsrgt.NewGetTagKeysPaginator(r.taggingClient(), &awsrgt.GetTagKeysInput{})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("GetTagKeys", ccfg.ResourceTypeTagMapping)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("GetTagKeys", ccfg.ResourceTypeTagMapping)).
					Inc()
			}

			return keys, errors.New(err)
		}

		keys = append(keys, resp.TagKeys...)
	}

	return keys, nil
}

// ListTagValues returns every value in use for one tag key in this client's
// region.
func (r *ResourceGroupsTaggingRepository) ListTagValues(key string) ([]string, error) {
	values := []string{}

	query := &awsrgt.GetTagValuesInput{Key: aws.String(key)}

	p := awsrgt.NewGetTagValuesPaginator(r.taggingClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("GetTagValues", ccfg.ResourceTypeTagMapping)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("GetTagValues", ccfg.ResourceTypeTagMapping)).
					Inc()
			}

			return values, errors.New(err)
		}

		values = append(values, resp.TagValues...)
	}

	return values, nil
}

// TagResources applies tags to resources by ARN, adding keys that are absent and
// overwriting the values of keys already present.
//
// **A nil error does not mean every resource was tagged.** The API answers a
// partially successful request with HTTP 200 and names the resources it could
// not tag in the returned map — an unsupported service, a resource in another
// region, a denied permission. Callers must check the map; an empty one is the
// only proof that everything succeeded.
//
// ARNs are written in batches of maxArnsPerWrite. A batch that fails outright
// returns the failures collected so far together with the error, because the
// earlier batches were really applied and pretending otherwise would invite a
// caller to retry a write that already landed.
func (r *ResourceGroupsTaggingRepository) TagResources(arns []string, tags map[string]string) (map[string]rgttypes.FailureInfo, error) {
	failures := map[string]rgttypes.FailureInfo{}

	if len(arns) == 0 || len(tags) == 0 {
		return failures, nil
	}

	for _, batch := range chunk(arns, maxArnsPerWrite) {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("TagResources", ccfg.ResourceTypeTagMapping)).
				Inc()
		}

		query := &awsrgt.TagResourcesInput{ResourceARNList: batch, Tags: tags}

		resp, err := r.taggingClient().TagResources(r.ctx, query)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("TagResources", ccfg.ResourceTypeTagMapping)).
					Inc()
			}

			return failures, errors.New(err)
		}

		for arn, failure := range resp.FailedResourcesMap {
			failures[arn] = failure
		}
	}

	if len(failures) > 0 {
		log.Warn().
			Str("accountID", r.client.GetAccountID().String()).
			Str("region", r.client.GetRegion().String()).
			Msgf("[ResourceGroupsTaggingRepository.TagResources] resources could not be tagged: %d", len(failures))
	}

	return failures, nil
}

// UntagResources removes tag keys from resources by ARN. Keys a resource does
// not carry are not an error.
//
// The same warning as TagResources applies: a nil error with a non-empty map
// means some resources were left untouched.
func (r *ResourceGroupsTaggingRepository) UntagResources(arns []string, tagKeys []string) (map[string]rgttypes.FailureInfo, error) {
	failures := map[string]rgttypes.FailureInfo{}

	if len(arns) == 0 || len(tagKeys) == 0 {
		return failures, nil
	}

	for _, batch := range chunk(arns, maxArnsPerWrite) {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("UntagResources", ccfg.ResourceTypeTagMapping)).
				Inc()
		}

		query := &awsrgt.UntagResourcesInput{ResourceARNList: batch, TagKeys: tagKeys}

		resp, err := r.taggingClient().UntagResources(r.ctx, query)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("UntagResources", ccfg.ResourceTypeTagMapping)).
					Inc()
			}

			return failures, errors.New(err)
		}

		for arn, failure := range resp.FailedResourcesMap {
			failures[arn] = failure
		}
	}

	if len(failures) > 0 {
		log.Warn().
			Str("accountID", r.client.GetAccountID().String()).
			Str("region", r.client.GetRegion().String()).
			Msgf("[ResourceGroupsTaggingRepository.UntagResources] resources could not be untagged: %d", len(failures))
	}

	return failures, nil
}
