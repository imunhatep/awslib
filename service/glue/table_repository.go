package glue

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *GlueRepository) ListTablesAll() ([]Table, error) {
	databases, err := r.ListDatabaseAll()
	if err != nil {
		return nil, errors.New(err)
	}

	tables := []Table{}
	for _, db := range databases {
		query := &glue.GetTablesInput{DatabaseName: aws.String(db.GetId())}

		dbTables, err := r.ListTablesByInput(query)
		if err != nil {
			log.Error().Err(err).Msg("[GlueRepository.ListTablesAll] failed to list tables")
			continue
		}

		tables = append(tables, dbTables...)
	}

	return tables, nil
}

func (r *GlueRepository) ListTablesByInput(query *glue.GetTablesInput) ([]Table, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(cfg.ResourceTypeGlueTable)).
		Msg("[GlueRepository::ListTablesByInput] searching for glue tables")

	start := time.Now()
	var tables []Table

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetTables", cfg.ResourceTypeGlueTable)).
			Inc()
	}

	resp, err := r.client.Glue().GetTables(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueTable)).
				Inc()
		}

		return tables, errors.New(err)
	}

	for _, table := range resp.TableList {
		tables = append(tables, NewTable(r.client, table, map[string]string{}))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDatabases", cfg.ResourceTypeGlueTable)).
			Add(float64(len(tables)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTablesByInput", cfg.ResourceTypeGlueTable)).
			Observe(time.Since(start).Seconds())
	}

	return tables, nil
}
