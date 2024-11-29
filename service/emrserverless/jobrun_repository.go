package emrserverless

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *EMRServerlessRepository) ListJobRunsAll() ([]JobRun, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessJobRun)).
		Msg("[EMRServerlessRepository.ListJobRunsAll] searching for jobs")

	start := time.Now()
	var jobRuns []JobRun

	applications, err := r.ListApplicationsActive()
	if err != nil {
		return jobRuns, errors.New(err)
	}

	for _, app := range applications {
		query := &emrserverless.ListJobRunsInput{
			ApplicationId:  app.Application.ApplicationId,
			CreatedAtAfter: service.LastDays(7),
		}

		appJobRuns, err := r.ListJobRunsByInput(query)

		if err != nil {
			log.Error().Err(err).
				Str("application", app.GetArn()).
				Msg("[EMRServerlessRepository] failed to fetch jobruns by application")

			continue
		}

		jobRuns = append(jobRuns, appJobRuns...)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListJobRunsAll", cfg.ResourceTypeEmrServerlessJobRun)).
			Observe(time.Since(start).Seconds())
	}

	return jobRuns, nil
}

// ListJobRunsByInput returns all job runs for a given Input
func (r *EMRServerlessRepository) ListJobRunsByInput(query *emrserverless.ListJobRunsInput) ([]JobRun, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessJobRun)).
		Msg("[EMRServerlessRepository.ListJobRunsByInput] searching for jobruns")

	start := time.Now()
	var jobRuns []JobRun

	p := emrserverless.NewListJobRunsPaginator(r.client.EMRServerless(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListJobRuns", cfg.ResourceTypeEmrServerlessJobRun)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListJobRuns", cfg.ResourceTypeEmrServerlessJobRun)).
					Inc()
			}

			return jobRuns, errors.New(err)
		}

		for _, jobRunSummary := range resp.JobRuns {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequests.
					With(r.promLabels("GetJobRun", cfg.ResourceTypeEmrServerlessJobRun)).
					Inc()
			}

			emrApp, err := r.client.EMRServerless().GetJobRun(r.ctx, &emrserverless.GetJobRunInput{
				ApplicationId: jobRunSummary.ApplicationId,
				JobRunId:      jobRunSummary.Id,
			})

			if err != nil {
				log.Error().Err(err).
					Str("jobRun", aws.ToString(jobRunSummary.Arn)).
					Msg("[EMRServerlessRepository.DescribeResources] failed to fetch EMR Serverless JobRun details")

				if metrics.AwsMetricsEnabled {
					metrics.AwsApiRequestErrors.
						With(r.promLabels("GetJobRun", cfg.ResourceTypeEmrServerlessJobRun)).
						Inc()
				}

				continue
			}

			jobRun := NewJobRun(r.client, emrApp.JobRun)
			jobRuns = append(jobRuns, jobRun)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListJobRuns", cfg.ResourceTypeEmrServerlessJobRun)).
			Add(float64(len(jobRuns)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListJobRunsByInput", cfg.ResourceTypeEmrServerlessJobRun)).
			Observe(time.Since(start).Seconds())
	}

	return jobRuns, nil
}
