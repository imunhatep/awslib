package cloudfront

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	rgtclient "github.com/imunhatep/awslib/provider/v3/clients/resourcegroupstaggingapi"
	"github.com/rs/zerolog/log"
)

// distributionTagRegion is where the Resource Groups Tagging API reports CloudFront
// distributions. Asked from any other region it returns none of them — which would read as
// "every distribution is untagged" — so the sweep pins it per operation, whatever region
// the client was built for.
const distributionTagRegion = "us-east-1"

// distributionTagFilter is the GetResources filter for classic distributions.
const distributionTagFilter = "cloudfront:distribution"

// tagFetchConcurrency and tagFetchMaxAttempts shape the per-distribution fallback.
// CloudFront throttles ListTagsForResource hard: at a concurrency of 8 an account with ~65
// distributions exhausted the SDK's default 3 attempts and failed the whole listing.
const (
	tagFetchConcurrency = 2
	tagFetchMaxAttempts = 10
)

// ListDistributionsAll lists every classic distribution in the account, with its tags.
func (r *CloudFrontRepository) ListDistributionsAll() ([]Distribution, error) {
	return r.ListDistributionsByInput(&awscf.ListDistributionsInput{})
}

// ListDistributionsByInput lists distributions and reads each one's tags.
//
// A distribution whose tags cannot be read fails the listing instead of being returned
// untagged: an empty tag map is indistinguishable from "this distribution is untagged", and
// the tags are what cost attribution and ownership questions are answered from.
func (r *CloudFrontRepository) ListDistributionsByInput(query *awscf.ListDistributionsInput) ([]Distribution, error) {
	start := time.Now()

	summaries, err := r.listDistributionSummaries(query)
	if err != nil {
		return nil, err
	}

	tags, err := r.distributionTags(summaries)
	if err != nil {
		return nil, err
	}

	distributions := make([]Distribution, 0, len(summaries))
	for i, summary := range summaries {
		distributions = append(distributions, NewDistribution(r.client, summary, tags[i]))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListDistributions", cfg.ResourceTypeDistribution)).
			Add(float64(len(distributions)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDistributionsByInput", cfg.ResourceTypeDistribution)).
			Observe(time.Since(start).Seconds())
	}

	return distributions, nil
}

func (r *CloudFrontRepository) listDistributionSummaries(query *awscf.ListDistributionsInput) ([]cftypes.DistributionSummary, error) {
	var summaries []cftypes.DistributionSummary

	p := awscf.NewListDistributionsPaginator(r.cloudFrontClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListDistributions", cfg.ResourceTypeDistribution)).Inc()
		}

		output, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListDistributions", cfg.ResourceTypeDistribution)).Inc()
			}

			return nil, classifyErr(err)
		}

		if output.DistributionList != nil {
			summaries = append(summaries, output.DistributionList.Items...)
		}
	}

	return summaries, nil
}

// distributionTags reads the tags of every distribution, index-aligned with summaries.
//
// One Resource Groups Tagging API sweep answers for up to 100 distributions per call. A
// distribution absent from the sweep has no tags — the API returns every currently tagged
// resource — so absences are not confirmed one by one. Only a failed sweep (most likely a
// role without tag:GetResources) falls back to ListTagsForResource per distribution.
func (r *CloudFrontRepository) distributionTags(summaries []cftypes.DistributionSummary) ([][]cftypes.Tag, error) {
	if len(summaries) == 0 {
		return nil, nil
	}

	byArn, err := r.sweepDistributionTags()
	if err != nil {
		log.Warn().Err(err).
			Str("accountID", r.client.GetAccountID().String()).
			Int("distributions", len(summaries)).
			Msg("[CloudFrontRepository.distributionTags] resource groups tagging sweep failed, falling back to ListTagsForResource per distribution")

		return r.distributionTagsPerDistribution(summaries)
	}

	tags := make([][]cftypes.Tag, len(summaries))
	for i, summary := range summaries {
		tags[i] = byArn[aws.ToString(summary.ARN)]
	}

	return tags, nil
}

// sweepDistributionTags returns the tags of every tagged distribution in the account, keyed
// by ARN.
func (r *CloudFrontRepository) sweepDistributionTags() (map[string][]cftypes.Tag, error) {
	inUsEast1 := func(o *awsrgt.Options) { o.Region = distributionTagRegion }

	out := map[string][]cftypes.Tag{}

	p := awsrgt.NewGetResourcesPaginator(rgtclient.GetClient(r.client), &awsrgt.GetResourcesInput{
		ResourceTypeFilters: []string{distributionTagFilter},
	})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetResources", cfg.ResourceTypeDistribution)).Inc()
		}

		page, err := p.NextPage(r.ctx, inUsEast1)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetResources", cfg.ResourceTypeDistribution)).Inc()
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

// distributionTagsPerDistribution is the fallback for a failed sweep: one
// ListTagsForResource per distribution, index-aligned with summaries.
func (r *CloudFrontRepository) distributionTagsPerDistribution(summaries []cftypes.DistributionSummary) ([][]cftypes.Tag, error) {
	tags := make([][]cftypes.Tag, len(summaries))
	errs := make([]error, len(summaries))
	sem := make(chan struct{}, tagFetchConcurrency)

	var wg sync.WaitGroup
	for i, summary := range summaries {
		wg.Add(1)

		go func(idx int, arn *string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tags[idx], errs[idx] = r.GetDistributionTags(aws.ToString(arn))
		}(i, summary.ARN)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, errors.Errorf("reading tags of distribution %s: %w", aws.ToString(summaries[i].Id), err)
		}
	}

	return tags, nil
}

// GetDistributionTags reads the tags of one distribution by ARN.
func (r *CloudFrontRepository) GetDistributionTags(distributionArn string) ([]cftypes.Tag, error) {
	if distributionArn == "" {
		return nil, errors.New("distribution ARN cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeDistribution)).Inc()
	}

	output, err := r.cloudFrontClient().ListTagsForResource(r.ctx, &awscf.ListTagsForResourceInput{
		Resource: aws.String(distributionArn),
	}, func(o *awscf.Options) { o.RetryMaxAttempts = tagFetchMaxAttempts })
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeDistribution)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.Tags == nil {
		return nil, nil
	}

	return output.Tags.Items, nil
}
