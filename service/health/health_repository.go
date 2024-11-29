package health

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/health"
	"github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"time"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	Health() *health.Client
}

type HealthRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewHealthRepository(ctx context.Context, client AwsClient) *HealthRepository {
	repo := &HealthRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *HealthRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *HealthRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *HealthRepository) ListEventsDetailsByInput(query *health.DescribeEventsInput) ([]types.EventDetails, error) {
	start := time.Now()
	var eventDetails []types.EventDetails

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeEvents", ccfg.ResourceTypeHealthEvent)).
			Inc()
	}

	output, err := r.client.Health().DescribeEvents(r.ctx, query)
	if err != nil {
		return eventDetails, errors.New(err)
	}

	// request details by chunks
	for _, events := range service.ChunkSlice(output.Events, 20) {
		eventArns := slice.Map(events, func(e types.Event) string { return *e.Arn })

		// Define the input parameters for the DescribeEventDetails operation
		detailsInput := &health.DescribeEventDetailsInput{EventArns: eventArns}

		// Call the DescribeEventDetails operation
		details, err := r.client.Health().DescribeEventDetails(r.ctx, detailsInput)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeEventDetails", ccfg.ResourceTypeHealthEvent)).
					Inc()
			}

			log.Error().Err(err).Msg("[ListEventsByInput] failed to describe event details")

			continue
		}

		for _, eventInfo := range details.SuccessfulSet {
			eventDetails = append(eventDetails, eventInfo)
		}

		for _, failedEvent := range details.FailedSet {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeEventDetails", ccfg.ResourceTypeHealthEvent)).
					Inc()
			}

			log.Error().
				Str("arn", *failedEvent.EventArn).
				Str("event", *failedEvent.ErrorName).
				Str("message", *failedEvent.ErrorMessage).
				Msg("[ListEventsByInput] failed to describe event details")
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeEvents", ccfg.ResourceTypeHealthEvent)).
			Add(float64(len(eventDetails)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListEventsDetailsByInput", ccfg.ResourceTypeHealthEvent)).
			Observe(time.Since(start).Seconds())
	}

	return eventDetails, nil
}
