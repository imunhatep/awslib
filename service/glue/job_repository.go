package glue

import (
	"time"

	awsglue "github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

func (r *GlueRepository) ListJobsAll() ([]Job, error) {
	return r.ListJobsByInput(&awsglue.GetJobsInput{})
}

func (r *GlueRepository) ListJobsByInput(query *awsglue.GetJobsInput) ([]Job, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeGlueJob)).
		Msg("[GlueRepository::ListJobsByInput] searching for glue jobs")

	start := time.Now()
	var databases []Job

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetJobs", cfg.ResourceTypeGlueJob)).
			Inc()
	}

	resp, err := r.glueClient().GetJobs(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueJob)).
				Inc()
		}

		return databases, errors.New(err)
	}

	for _, job := range resp.Jobs {
		databases = append(databases, NewJob(r.client, job, map[string]string{}))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueJob)).
			Add(float64(len(databases)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListJobsByInput", cfg.ResourceTypeGlueJob)).
			Observe(time.Since(start).Seconds())
	}

	return databases, nil
}
