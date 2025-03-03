package cloudtrail

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *CloudTrailRepository) ListEventsByInput(query *cloudtrail.LookupEventsInput) ([]Event, error) {
	source := make(chan Event, 50)
	errChan := make(chan *errors.Error, 50)
	defer close(source)
	defer close(errChan)

	r.ListEventsByInputAsync(query, source, errChan)

	return service.ReadChannels(r.ctx, source, errChan)
}

func (r *CloudTrailRepository) ListEventsByInputAsync(
	query *cloudtrail.LookupEventsInput,
	events chan<- Event,
	errChan chan<- *errors.Error,
) {

	DebugQuery("[CloudTrailRepository.FindBy] debug query", query)

	start := time.Now()

	p := cloudtrail.NewLookupEventsPaginator(r.client.CloudTrail(), query)

	// reach end of pages or max results
	eventsFetchedCount := 0
	for p.HasMorePages() && (query.MaxResults == nil || eventsFetchedCount < int(*query.MaxResults)) {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).Inc()
		}

		select {
		case <-r.ctx.Done():
			break
		default:
		}

		log.Trace().Int("count", eventsFetchedCount).Msg("[CloudTrailRepository.FindBy] fetching events...")

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).Inc()
			}

			errChan <- errors.New(err)
			continue
		}

		eventsFetchedCount += len(resp.Events)

		for _, event := range resp.Events {
			var cloudTrailEvent CloudTrailEvent
			if err := json.Unmarshal([]byte(*event.CloudTrailEvent), &cloudTrailEvent); err != nil {
				errChan <- errors.New(err)
				continue
			}

			select {
			case <-r.ctx.Done():
				break
			default:
				service.WriteToChan(events, NewEvent(r.client, event, cloudTrailEvent))
			}
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).
			Add(float64(eventsFetchedCount))

		// metrics
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListEventsByInput", ccfg.ResourceTypeTrailEvent)).
			Observe(time.Since(start).Seconds())
	}
}

func (r *CloudTrailRepository) ListEventsByLookupAsync(
	lookup *LookupMiddleware,
	events chan<- Event,
	errChan chan<- *errors.Error,
) {
	if errs, ok := lookup.Errors(); !ok {
		errChan <- errors.New(errs[0])
		return
	}

	r.ListEventsByInputAsync(lookup.Get(), events, errChan)
}

func (r *CloudTrailRepository) ListEventsByLookup(lookup *LookupMiddleware) ([]Event, error) {
	if errs, ok := lookup.Errors(); !ok {
		return []Event{}, errors.New(slice.Head(errs).OrEmpty())
	}

	// get cloudtrail events by lookup query
	events, err := r.ListEventsByInput(lookup.Get())
	if err != nil {
		return events, errors.New(err)
	}

	return events, nil
}

// ListEventsByLookupCached a wrapper of EventsByResource method with reading and writing results into a cache
func (r *CloudTrailRepository) ListEventsByLookupCached(cache *cache.DataCache, lookup *LookupMiddleware) (items []Event, err error) {
	namespace := fmt.Sprintf("%s:%s:%s", r.client.GetAccountID().String(), r.client.GetRegion().String(), lookup.Hash())
	cacheNs := cache.WithNamespace(namespace)

	resourceTypeKey := ccfg.ResourceTypeToString(ccfg.ResourceTypeTrailEvent)
	if found := cacheNs.Read(resourceTypeKey, &items); !found {
		items, err = r.ListEventsByLookup(lookup)

		if err == nil {
			err = cacheNs.Write(resourceTypeKey, items)
		}
	}

	log.Trace().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", ccfg.ResourceTypeToString(ccfg.ResourceTypeTrailEvent)).
		Msgf("[CloudTrailRepository.ListEventsByLookupCached] events found: %d", len(items))

	return items, err
}
