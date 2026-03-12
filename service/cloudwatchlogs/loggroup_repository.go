package cloudwatchlogs

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/cloudwatchlogs"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type CloudWatchLogsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewCloudWatchLogsRepository(ctx context.Context, client *v3.Client) *CloudWatchLogsRepository {
	repo := &CloudWatchLogsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *CloudWatchLogsRepository) cloudwatchlogsClient() *awscloudwatchlogs.Client {
	return cloudwatchlogs.GetClient(r.client)
}

func (r *CloudWatchLogsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *CloudWatchLogsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *CloudWatchLogsRepository) ListLogGroupsAll() ([]LogGroup, error) {
	return r.ListLogGroupsByInput(&awscloudwatchlogs.DescribeLogGroupsInput{})
}

func (r *CloudWatchLogsRepository) ListLogGroupsByInput(query *awscloudwatchlogs.DescribeLogGroupsInput) ([]LogGroup, error) {
	start := time.Now()
	logGroups := []LogGroup{}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeLogGroups", ccfg.ResourceTypeCloudWatchLogGroup)).Inc()
	}

	resp, err := r.cloudwatchlogsClient().DescribeLogGroups(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DescribeLogGroups", ccfg.ResourceTypeCloudWatchLogGroup)).
				Inc()
		}

		return logGroups, errors.New(err)
	}

	for _, logGroup := range resp.LogGroups {
		tags, _ := r.GetLogGroupTags(logGroup)
		table := NewLogGroup(r.client, logGroup, tags)
		logGroups = append(logGroups, table)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeLogGroups", ccfg.ResourceTypeCloudWatchLogGroup)).
			Add(float64(len(logGroups)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListLogGroupsByInput", ccfg.ResourceTypeCloudWatchLogGroup)).
			Observe(time.Since(start).Seconds())
	}

	return logGroups, nil
}

func (r *CloudWatchLogsRepository) GetLogGroupTags(logGroup types.LogGroup) (map[string]string, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetBucketTagging", ccfg.ResourceTypeCloudWatchLogGroup)).
			Inc()
	}

	query := &awscloudwatchlogs.ListTagsForResourceInput{ResourceArn: logGroup.LogGroupArn}
	tagsOutput, err := r.cloudwatchlogsClient().ListTagsForResource(r.ctx, query)

	if err != nil {
		log.Debug().Err(err).
			Str("logGroup", aws.ToString(logGroup.LogGroupName)).
			Msg("failed to fetch LogGroup tags")

		return map[string]string{}, errors.New(err)
	}

	return tagsOutput.Tags, nil
}
