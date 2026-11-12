package cloudfront

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
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

// distributionTags reads the tags of every distribution, index-aligned with summaries, in one
// tagging sweep (see bulkTags).
func (r *CloudFrontRepository) distributionTags(summaries []cftypes.DistributionSummary) ([][]cftypes.Tag, error) {
	arns := make([]string, len(summaries))
	for i, summary := range summaries {
		arns[i] = aws.ToString(summary.ARN)
	}

	return r.bulkTags(arns, distributionTagSource)
}

// GetDistributionTags reads the tags of one distribution by ARN.
func (r *CloudFrontRepository) GetDistributionTags(distributionArn string) ([]cftypes.Tag, error) {
	if distributionArn == "" {
		return nil, errors.New("distribution ARN cannot be empty")
	}

	return r.listTagsForResource(distributionArn, cfg.ResourceTypeDistribution)
}
