package sqs

import (
	"context"
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/sqs"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type SqsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewSqsRepository(ctx context.Context, client *v3.Client) *SqsRepository {
	repo := &SqsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *SqsRepository) sqsClient() *awssqs.Client {
	return sqs.GetClient(r.client)
}

func (r *SqsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *SqsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *SqsRepository) ListQueuesAll() ([]Queue, error) {
	return r.ListQueuesByInput(&awssqs.ListQueuesInput{})
}

func (r *SqsRepository) ListQueuesByInput(query *awssqs.ListQueuesInput) ([]Queue, error) {
	start := time.Now()
	var queues []Queue

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListQueues", cfg.ResourceTypeQueue)).Inc()
	}

	resp, err := r.sqsClient().ListQueues(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListQueues", cfg.ResourceTypeQueue)).Inc()
		}

		return queues, errors.New(err)
	}

	for _, queueUrl := range resp.QueueUrls {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetQueueAttributes", cfg.ResourceTypeQueue)).Inc()
		}

		queueQuery := &awssqs.GetQueueAttributesInput{
			QueueUrl:       &queueUrl,
			AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameAll},
		}

		attrsOutput, err := r.sqsClient().GetQueueAttributes(r.ctx, queueQuery)

		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetQueueAttributes", cfg.ResourceTypeQueue)).Inc()
			}

			return queues, errors.New(err)
		}

		tags, _ := r.GetQueueTags(queueUrl)
		queue := NewQueue(r.client, queueUrl, attrsOutput.Attributes, tags)
		queues = append(queues, queue)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListQueues", cfg.ResourceTypeQueue)).
			Add(float64(len(queues)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListQueuesByInput", cfg.ResourceTypeQueue)).
			Observe(time.Since(start).Seconds())
	}

	return queues, nil
}

func (r *SqsRepository) GetQueueTags(queueUrl string) (map[string]string, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeQueue)).Inc()
	}

	tagOutput, err := r.sqsClient().ListQueueTags(r.ctx, &awssqs.ListQueueTagsInput{QueueUrl: &queueUrl})

	if err != nil {
		log.Debug().Str("queue", queueUrl).Err(err).Msg("failed to fetch sqs.QueueUrl tags")
		return map[string]string{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}
