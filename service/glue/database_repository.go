package glue

import (
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *GlueRepository) ListDatabaseAll() ([]Database, error) {
	return r.ListDatabaseByInput(&glue.GetDatabasesInput{})
}

func (r *GlueRepository) ListDatabaseByInput(query *glue.GetDatabasesInput) ([]Database, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeGlueDatabase)).
		Msg("[GlueRepository::ListDatabaseByInput] searching for databases")

	start := time.Now()
	var databases []Database

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueDatabase)).
			Inc()
	}

	resp, err := r.client.Glue().GetDatabases(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueDatabase)).
				Inc()
		}

		return databases, errors.New(err)
	}

	for _, db := range resp.DatabaseList {
		databases = append(databases, NewDatabase(r.client, db, map[string]string{}))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueDatabase)).
			Add(float64(len(databases)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDatabaseByInput", cfg.ResourceTypeGlueDatabase)).
			Observe(time.Since(start).Seconds())
	}

	return databases, nil
}
