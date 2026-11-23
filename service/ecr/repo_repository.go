package ecr

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsecr "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

func (r *EcrRepository) ListRepositoriesAll() ([]Repo, error) {
	return r.ListRepositoriesByInput(&awsecr.DescribeRepositoriesInput{})
}

// ListRepositoriesByInput lists repositories together with their tags.
//
// This is 1+N API calls by necessity — DescribeRepositories does not return
// tags. The per-repository reads run concurrently, bounded by
// tagFetchConcurrency. Missing tags would make a repository look unowned to a
// caller selecting by tag, so any failed tag read fails the whole listing; a
// repository deleted between the listing and its tag read is dropped.
func (r *EcrRepository) ListRepositoriesByInput(query *awsecr.DescribeRepositoriesInput) ([]Repo, error) {
	start := time.Now()

	repositories, err := r.describeRepositories(query)
	if err != nil {
		return nil, err
	}

	tags := make([]map[string]string, len(repositories))
	errs := make([]error, len(repositories))
	sem := make(chan struct{}, tagFetchConcurrency)

	var wg sync.WaitGroup
	for i := range repositories {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tags[idx], errs[idx] = r.repositoryTags(aws.ToString(repositories[idx].RepositoryArn))
		}(i)
	}

	wg.Wait()

	result := make([]Repo, 0, len(repositories))
	for i, repository := range repositories {
		if err := errs[i]; err != nil {
			if errors.Is(err, ErrNotFound) {
				log.Debug().
					Str("repository", aws.ToString(repository.RepositoryArn)).
					Msg("[EcrRepository.ListRepositoriesByInput] repository deleted during listing")

				continue
			}

			return nil, errors.Errorf("reading tags of %s: %w", aws.ToString(repository.RepositoryArn), err)
		}

		result = append(result, NewRepo(r.client, repository, tags[i]))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeRepositories", cfg.ResourceTypeECRRepository)).
			Add(float64(len(result)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListRepositoriesByInput", cfg.ResourceTypeECRRepository)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}

func (r *EcrRepository) describeRepositories(query *awsecr.DescribeRepositoriesInput) ([]ecrtypes.Repository, error) {
	var repositories []ecrtypes.Repository

	p := awsecr.NewDescribeRepositoriesPaginator(r.ecrClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeRepositories", cfg.ResourceTypeECRRepository)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeRepositories", cfg.ResourceTypeECRRepository)).Inc()
			}

			return nil, classifyErr(err)
		}

		repositories = append(repositories, resp.Repositories...)
	}

	return repositories, nil
}

func (r *EcrRepository) repositoryTags(repositoryArn string) (map[string]string, error) {
	if repositoryArn == "" {
		return nil, errors.New("repository ARN cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeECRRepository)).Inc()
	}

	output, err := r.ecrClient().ListTagsForResource(r.ctx, &awsecr.ListTagsForResourceInput{
		ResourceArn: aws.String(repositoryArn),
	}, func(o *awsecr.Options) { o.RetryMaxAttempts = tagFetchMaxAttempts })
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeECRRepository)).Inc()
		}

		return nil, classifyErr(err)
	}

	return tagsToMap(output.Tags), nil
}
