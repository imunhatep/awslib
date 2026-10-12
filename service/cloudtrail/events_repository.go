package cloudtrail

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

func (r *CloudTrailRepository) ListEventsByInput(query *cloudtrail.LookupEventsInput) ([]Event, error) {
	source, errChan := r.ListEventsByInputAsync(query)

	// not service.ReadChannels: it returns on whichever channel closes first, and both are closed
	// together, so it would randomly miss the error already buffered in errChan. The producer
	// always closes source, and closes errChan before it, so the error is in place by then.
	events := make([]Event, 0)
	for event := range source {
		events = append(events, event)
	}

	if err, ok := <-errChan; ok && err != nil {
		return events, err
	}

	return events, nil
}

// ListEventsByInputAsync streams events until the lookup is exhausted, fails, or the repository
// context is done; both channels are closed then. The producer never blocks on the reader: events
// are buffered up to EventsBufferSize, and a reader that falls further behind ends the stream with
// an error rather than silently losing events. The first error ends the stream and is the only
// value errChan ever carries, so a reader that ignores errChan cannot block the producer.
func (r *CloudTrailRepository) ListEventsByInputAsync(query *cloudtrail.LookupEventsInput) (<-chan Event, <-chan *errors.Error) {
	source := make(chan Event, EventsBufferSize)
	errChan := make(chan *errors.Error, 1)

	go r.fetchEventsByInput(query, source, errChan)

	return source, errChan
}

func (r *CloudTrailRepository) fetchEventsByInput(
	query *cloudtrail.LookupEventsInput,
	source chan<- Event,
	errChan chan<- *errors.Error,
) {
	defer close(source)
	defer close(errChan)

	DebugQuery("[CloudTrailRepository.FindBy] debug query", query)

	start := time.Now()

	p := cloudtrail.NewLookupEventsPaginator(r.lookupEventsClient(), query)

	// reach end of pages or max results
	eventsFetchedCount := 0

	defer func() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiResourcesFetched.
				With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).
				Add(float64(eventsFetchedCount))

			// metrics
			metrics.AwsRepoCallDuration.
				With(r.promLabels("ListEventsByInput", ccfg.ResourceTypeTrailEvent)).
				Observe(time.Since(start).Seconds())
		}
	}()

	for p.HasMorePages() && (query.MaxResults == nil || eventsFetchedCount < int(*query.MaxResults)) {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).Inc()
		}

		log.Trace().Int("count", eventsFetchedCount).Msg("[CloudTrailRepository.FindBy] fetching events...")

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).Inc()
			}

			// the paginator does not advance past a failed page, so carrying on would request
			// the same page again forever
			errChan <- errors.New(err)
			return
		}

		eventsFetchedCount += len(resp.Events)

		for _, event := range resp.Events {
			var cloudTrailEvent CloudTrailEvent
			if err := json.Unmarshal([]byte(aws.ToString(event.CloudTrailEvent)), &cloudTrailEvent); err != nil {
				errChan <- errors.New(err)
				return
			}

			if err := service.WriteToChan(source, NewEvent(r.client, event, cloudTrailEvent)); err != nil {
				// the reader fell a whole buffer behind; this event is lost, so end the stream with
				// an error rather than hand back a list that only looks complete
				errChan <- errors.Errorf("reader fell %d events behind: %w", cap(source), err)
				return
			}
		}
	}
}

func (r *CloudTrailRepository) ListEventsByLookupAsync(lookup *LookupMiddleware) (<-chan Event, <-chan *errors.Error) {
	if errs, ok := lookup.Errors(); !ok {
		errChan := make(chan *errors.Error, 2)
		defer close(errChan)

		errChan <- errors.New(errs[0])

		// closed rather than nil: ranging over a nil channel blocks forever
		source := make(chan Event)
		close(source)

		return source, errChan
	}

	return r.ListEventsByInputAsync(lookup.Get())
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
