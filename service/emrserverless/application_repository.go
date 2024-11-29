package emrserverless

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *EMRServerlessRepository) ListApplicationsAll(maxResults *int32) ([]Application, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessApplication)).
		Msg("[EMRServerlessRepository.ListApplicationsAll] searching for applications")

	applications, err := r.ListApplicationsByInput(&emrserverless.ListApplicationsInput{MaxResults: maxResults})
	if err != nil {
		return applications, err
	}

	return applications, nil
}

func (r *EMRServerlessRepository) ListApplicationsActive() ([]Application, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessApplication)).
		Msg("[EMRServerlessRepository.ListApplicationsActive] searching for applications")

	start := time.Now()

	query := &emrserverless.ListApplicationsInput{
		States: []types.ApplicationState{
			types.ApplicationStateCreating,
			types.ApplicationStateCreated,
			types.ApplicationStateStarting,
			types.ApplicationStateStarted,
			types.ApplicationStateStopping,
			types.ApplicationStateStopped,
		},
	}

	applications, err := r.ListApplicationsByInput(query)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListApplicationsActive", cfg.ResourceTypeEmrServerlessApplication)).
			Observe(time.Since(start).Seconds())
	}

	return applications, err
}

func (r *EMRServerlessRepository) ListApplicationsByInput(query *emrserverless.ListApplicationsInput) ([]Application, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessApplication)).
		Msg("[EMRServerlessRepository.ListApplicationsByInput] searching for applications")

	start := time.Now()
	var applications []Application

	p := emrserverless.NewListApplicationsPaginator(r.client.EMRServerless(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListApplications", cfg.ResourceTypeEmrServerlessApplication)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListApplications", cfg.ResourceTypeEmrServerlessApplication)).
					Inc()
			}

			return applications, errors.New(err)
		}

		log.Trace().
			Str("accountID", r.client.GetAccountID().String()).
			Str("region", r.client.GetRegion().String()).
			Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessApplication)).
			Msgf("[EMRServerlessRepository.ListApplicationsByInput] applications on page %d", len(resp.Applications))

		for _, appSummary := range resp.Applications {
			application, err := r.DescribeApplication(appSummary.Id)
			if err != nil {
				applications = append(applications, *application)
			}
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListApplications", cfg.ResourceTypeEmrServerlessApplication)).
			Add(float64(len(applications)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListApplicationsByInput", cfg.ResourceTypeEmrServerlessApplication)).
			Observe(time.Since(start).Seconds())
	}

	return applications, nil
}

func (r *EMRServerlessRepository) DescribeApplication(applicationId *string) (*Application, error) {
	query := &emrserverless.GetApplicationInput{ApplicationId: applicationId}
	return r.DescribeApplicationByInput(query)
}

func (r *EMRServerlessRepository) DescribeApplicationByInput(query *emrserverless.GetApplicationInput) (*Application, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeEmrServerlessApplication)).
		Msg("[EMRServerlessRepository.DescribeApplication] searching for application")

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetApplication", cfg.ResourceTypeEmrServerlessApplication)).
			Inc()
	}

	emrApp, err := r.client.EMRServerless().GetApplication(r.ctx, query)
	if err != nil {
		log.Error().Err(err).
			Str("application", aws.ToString(query.ApplicationId)).
			Msg("EMRServerlessRepository.DescribeApplication] failed to fetch EMR Serverless Application details")

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("GetApplication", cfg.ResourceTypeEmrServerlessApplication)).
				Inc()
		}

		return nil, err
	}

	application := NewApplication(r.client, emrApp.Application)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DescribeApplication", cfg.ResourceTypeEmrServerlessApplication)).
			Observe(time.Since(start).Seconds())
	}

	return &application, nil
}
