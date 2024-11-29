package sqs

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
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
	SQS() *sqs.Client
}

type SqsRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewSqsRepository(ctx context.Context, client AwsClient) *SqsRepository {
	repo := &SqsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
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
	return r.ListQueuesByInput(&sqs.ListQueuesInput{})
}

func (r *SqsRepository) ListQueuesByInput(query *sqs.ListQueuesInput) ([]Queue, error) {
	start := time.Now()
	var queues []Queue

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListQueues", cfg.ResourceTypeQueue)).Inc()
	}

	resp, err := r.client.SQS().ListQueues(r.ctx, query)
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

		queueQuery := &sqs.GetQueueAttributesInput{
			QueueUrl:       &queueUrl,
			AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameAll},
		}

		attrsOutput, err := r.client.SQS().GetQueueAttributes(r.ctx, queueQuery)

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

	tagOutput, err := r.client.SQS().ListQueueTags(r.ctx, &sqs.ListQueueTagsInput{QueueUrl: &queueUrl})

	if err != nil {
		log.Debug().Str("queue", queueUrl).Err(err).Msg("failed to fetch sqs.QueueUrl tags")
		return map[string]string{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}
