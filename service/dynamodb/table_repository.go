package dynamodb

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"time"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	DynamoDB() *dynamodb.Client
}

type DynamoDBRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewDynamoDBRepository(ctx context.Context, client AwsClient) *DynamoDBRepository {
	repo := &DynamoDBRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *DynamoDBRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *DynamoDBRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *DynamoDBRepository) ListTablesAll() ([]Table, error) {
	return r.ListTablesByInput(&dynamodb.ListTablesInput{})
}

func (r *DynamoDBRepository) ListTablesByInput(query *dynamodb.ListTablesInput) ([]Table, error) {
	start := time.Now()
	var tables []Table

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTables", cfg.ResourceTypeTable)).Inc()
	}

	resp, err := r.client.DynamoDB().ListTables(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("ListTables", cfg.ResourceTypeTable)).
				Inc()
		}

		return tables, errors.New(err)
	}

	for _, tableName := range resp.TableNames {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeTable", cfg.ResourceTypeTable)).
				Inc()
		}

		tableOutput, err := r.client.DynamoDB().DescribeTable(r.ctx, &dynamodb.DescribeTableInput{TableName: &tableName})
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeTable", cfg.ResourceTypeTable)).Inc()
			}

			return tables, errors.New(err)
		}

		tags, _ := r.GetTableTags(tableOutput.Table)
		table := NewTable(r.client, tableOutput.Table, tags)
		tables = append(tables, table)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("ListTables", cfg.ResourceTypeTable)).Add(float64(len(tables)))
		metrics.AwsRepoCallDuration.With(r.promLabels("ListTablesByInput", cfg.ResourceTypeTable)).Observe(time.Since(start).Seconds())
	}

	return tables, nil
}

func (r *DynamoDBRepository) GetTableTags(table *types.TableDescription) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("ListTagsOfResource", cfg.ResourceTypeTable)).
			Inc()
	}

	tagOutput, err := r.client.DynamoDB().ListTagsOfResource(r.ctx, &dynamodb.ListTagsOfResourceInput{ResourceArn: table.TableArn})
	if err != nil {
		log.Debug().Str("table", aws.ToString(table.TableArn)).Err(err).Msg("failed to fetch dynamodb table tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}
