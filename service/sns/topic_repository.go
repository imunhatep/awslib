package sns

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/sns"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type SnsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewSnsRepository(ctx context.Context, client *v3.Client) *SnsRepository {
	repo := &SnsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *SnsRepository) snsClient() *awssns.Client {
	return sns.GetClient(r.client)
}

func (r *SnsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *SnsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *SnsRepository) ListTopicsAll() ([]Topic, error) {
	return r.ListTopicsByInput(&awssns.ListTopicsInput{})
}

func (r *SnsRepository) ListTopicsByInput(query *awssns.ListTopicsInput) ([]Topic, error) {
	start := time.Now()
	var topics []Topic

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTopics", cfg.ResourceTypeTopic)).Inc()
	}

	resp, err := r.snsClient().ListTopics(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListTopics", cfg.ResourceTypeTopic)).Inc()
		}

		return topics, errors.New(err)
	}

	for _, v := range resp.Topics {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetTopicAttributes", cfg.ResourceTypeTopic)).Inc()
		}

		attrsOutput, err := r.snsClient().GetTopicAttributes(r.ctx, &awssns.GetTopicAttributesInput{TopicArn: v.TopicArn})
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetTopicAttributes", cfg.ResourceTypeTopic)).Inc()
			}

			return topics, errors.New(err)
		}

		tags, _ := r.GetTopicTags(v)
		topic := NewTopic(r.client, v, attrsOutput.Attributes, tags)
		topics = append(topics, topic)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListTopics", cfg.ResourceTypeTopic)).
			Add(float64(len(topics)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTopicsByInput", cfg.ResourceTypeTopic)).
			Observe(time.Since(start).Seconds())
	}

	return topics, nil
}

func (r *SnsRepository) GetTopicTags(topic types.Topic) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeTopic)).Inc()
	}

	tagOutput, err := r.snsClient().ListTagsForResource(r.ctx, &awssns.ListTagsForResourceInput{ResourceArn: topic.TopicArn})

	if err != nil {
		log.Debug().Str("topic", aws.ToString(topic.TopicArn)).Err(err).Msg("failed to fetch sns.Topic tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}
