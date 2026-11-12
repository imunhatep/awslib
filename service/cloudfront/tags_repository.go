package cloudfront

import (
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	rgtclient "github.com/imunhatep/awslib/provider/v3/clients/resourcegroupstaggingapi"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// Tags are part of no CloudFront list response — not distributions, not tenants, not
// connection groups — and CloudFront leaves them out of the Get responses too. Listings read
// them in bulk from the Resource Groups Tagging API, which answers for up to 100 resources per
// call; ListTagsForResource is one call per resource and is kept for single reads and as the
// fallback for a failed sweep.

// tagRegion is where the Resource Groups Tagging API reports CloudFront resources. Asked from
// any other region it returns none of them — which would read as "every resource is untagged" —
// so the sweep pins it per operation, whatever region the client was built for.
const tagRegion = "us-east-1"

// tagFetchConcurrency and tagFetchMaxAttempts shape the per-resource fallback.
// CloudFront throttles ListTagsForResource hard: at a concurrency of 8 an account with ~65
// distributions exhausted the SDK's default 3 attempts and failed the whole listing.
const (
	tagFetchConcurrency = 2
	tagFetchMaxAttempts = 10
)

// tagSource says how the tags of one resource type are read in bulk.
type tagSource struct {
	// filter is the GetResources resource type filter.
	filter string

	// resourceType labels metrics.
	resourceType cfg.ResourceType

	// arnMarker is set for a type the tagging API is not known to index. A resource absent
	// from the sweep then counts as untagged only once the sweep has reported at least one ARN
	// containing the marker; until then the tags are read per resource.
	arnMarker string
}

var (
	distributionTagSource = tagSource{
		filter:       "cloudfront:distribution",
		resourceType: cfg.ResourceTypeDistribution,
	}

	// Tenants are indexed: a us-east-1 sweep filtered on "cloudfront" reports their
	// distribution-tenant ARNs. A typed filter's spelling is not established, and a wrong one
	// answers with an empty list that reads as "untagged", so the sweep uses the bare service
	// filter and matches by ARN.
	distributionTenantTagSource = tagSource{
		filter:       "cloudfront",
		resourceType: ccfg.ResourceTypeCloudFrontDistributionTenantSummary,
	}

	// Whether connection groups are indexed has not been observed, so their absence from the
	// sweep is not trusted on its own. An account holds a handful of them, which keeps the
	// per-group path cheap when it is taken.
	connectionGroupTagSource = tagSource{
		filter:       "cloudfront",
		resourceType: ccfg.ResourceTypeCloudFrontConnectionGroup,
		arnMarker:    ":connection-group/",
	}
)

// bulkTags reads the tags of the resources with the given ARNs, index-aligned with arns.
//
// One tagging sweep answers for the whole account. A resource absent from it has no tags —
// the API returns every currently tagged resource — so absences are not confirmed one by one.
// Only a failed sweep (most likely a role without tag:GetResources), or a sweep that cannot
// vouch for the type (see tagSource.arnMarker), falls back to ListTagsForResource per resource.
//
// A resource whose tags cannot be read fails the read instead of being returned untagged: an
// empty tag map is indistinguishable from "this resource is untagged", and the tags are what
// cost attribution and ownership questions are answered from.
func (r *CloudFrontRepository) bulkTags(arns []string, src tagSource) ([][]cftypes.Tag, error) {
	if len(arns) == 0 {
		return nil, nil
	}

	byArn, err := r.sweepTags(src)
	if err != nil {
		log.Warn().Err(err).
			Str("accountID", r.client.GetAccountID().String()).
			Str("resourceType", ccfg.ResourceTypeToString(src.resourceType)).
			Int("resources", len(arns)).
			Msg("[CloudFrontRepository.bulkTags] resource groups tagging sweep failed, falling back to ListTagsForResource per resource")

		return r.tagsPerResource(arns, src.resourceType)
	}

	if src.arnMarker != "" && !anyArnContains(byArn, src.arnMarker) {
		log.Debug().
			Str("accountID", r.client.GetAccountID().String()).
			Str("resourceType", ccfg.ResourceTypeToString(src.resourceType)).
			Int("resources", len(arns)).
			Msg("[CloudFrontRepository.bulkTags] tagging sweep reported no resource of this type, reading tags per resource")

		return r.tagsPerResource(arns, src.resourceType)
	}

	tags := make([][]cftypes.Tag, len(arns))
	for i, resourceArn := range arns {
		tags[i] = byArn[resourceArn]
	}

	return tags, nil
}

func anyArnContains(byArn map[string][]cftypes.Tag, marker string) bool {
	for resourceArn := range byArn {
		if strings.Contains(resourceArn, marker) {
			return true
		}
	}

	return false
}

// sweepTags returns the tags of every tagged resource matching the source's filter, keyed by
// ARN.
func (r *CloudFrontRepository) sweepTags(src tagSource) (map[string][]cftypes.Tag, error) {
	inUsEast1 := func(o *awsrgt.Options) { o.Region = tagRegion }

	out := map[string][]cftypes.Tag{}

	p := awsrgt.NewGetResourcesPaginator(rgtclient.GetClient(r.client), &awsrgt.GetResourcesInput{
		ResourceTypeFilters: []string{src.filter},
	})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetResources", src.resourceType)).Inc()
		}

		page, err := p.NextPage(r.ctx, inUsEast1)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetResources", src.resourceType)).Inc()
			}

			return nil, errors.New(err)
		}

		for _, mapping := range page.ResourceTagMappingList {
			tags := make([]cftypes.Tag, 0, len(mapping.Tags))
			for _, tag := range mapping.Tags {
				tags = append(tags, cftypes.Tag{Key: tag.Key, Value: tag.Value})
			}

			out[aws.ToString(mapping.ResourceARN)] = tags
		}
	}

	return out, nil
}

// tagsPerResource is the fallback for a sweep that cannot answer: one ListTagsForResource per
// resource, index-aligned with arns.
func (r *CloudFrontRepository) tagsPerResource(arns []string, resourceType cfg.ResourceType) ([][]cftypes.Tag, error) {
	tags := make([][]cftypes.Tag, len(arns))
	errs := make([]error, len(arns))
	sem := make(chan struct{}, tagFetchConcurrency)

	var wg sync.WaitGroup
	for i, resourceArn := range arns {
		wg.Add(1)

		go func(idx int, resourceArn string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tags[idx], errs[idx] = r.listTagsForResource(resourceArn, resourceType)
		}(i, resourceArn)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, errors.Errorf("reading tags of %s: %w", arns[i], err)
		}
	}

	return tags, nil
}

// listTagsForResource reads the tags of one resource by ARN.
func (r *CloudFrontRepository) listTagsForResource(resourceArn string, resourceType cfg.ResourceType) ([]cftypes.Tag, error) {
	if resourceArn == "" {
		return nil, errors.New("resource ARN cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForResource", resourceType)).Inc()
	}

	output, err := r.cloudFrontClient().ListTagsForResource(r.ctx, &awscf.ListTagsForResourceInput{
		Resource: aws.String(resourceArn),
	}, func(o *awscf.Options) { o.RetryMaxAttempts = tagFetchMaxAttempts })
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListTagsForResource", resourceType)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.Tags == nil {
		return nil, nil
	}

	return output.Tags.Items, nil
}

// completeTags returns the tags of a single resource read with a Get. The API reference lists
// Tags in the Get responses, but CloudFront leaves them out, so they are read with
// ListTagsForResource whenever the response carries none. name only labels errors.
//
// A resource deleted between the two calls is ErrNotFound, like the read itself.
func (r *CloudFrontRepository) completeTags(name, resourceArn string, current *cftypes.Tags, resourceType cfg.ResourceType) (*cftypes.Tags, error) {
	if hasTags(current) {
		return current, nil
	}

	if resourceArn == "" {
		return current, errors.Errorf("%s has no ARN to read its tags by", name)
	}

	items, err := r.listTagsForResource(resourceArn, resourceType)
	if err != nil {
		return current, err
	}

	return &cftypes.Tags{Items: items}, nil
}

func hasTags(tags *cftypes.Tags) bool {
	return tags != nil && len(tags.Items) > 0
}

// carryTags returns current when it holds tags, and known otherwise. Writes use it: a create
// applies exactly the tags it was sent and an update leaves tags alone, so a response without
// tags is completed from what the caller already has instead of being read again.
func carryTags(current, known *cftypes.Tags) *cftypes.Tags {
	if hasTags(current) {
		return current
	}

	return known
}
