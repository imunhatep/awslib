package cloudtrail

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *CloudTrailRepository) ListEventsByInput(query *cloudtrail.LookupEventsInput) ([]Event, error) {
	DebugQuery("[CloudTrailRepository.FindBy] debug query", query)

	start := time.Now()
	var events []Event

	p := cloudtrail.NewLookupEventsPaginator(r.client.CloudTrail(), query)

	// reach end of pages or max results
	for p.HasMorePages() && (query.MaxResults == nil || len(events) < int(*query.MaxResults)) {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).Inc()
		}

		log.Trace().Msgf("[CloudTrailRepository.FindBy] fetching events %d ...", len(events))

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).Inc()
			}

			return events, errors.New(err)
		}

		for _, event := range resp.Events {
			events = append(events, NewEvent(r.client, event))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("LookupEvents", ccfg.ResourceTypeTrailEvent)).
			Add(float64(len(events)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListEventsByInput", ccfg.ResourceTypeTrailEvent)).
			Observe(time.Since(start).Seconds())
	}

	return events, nil
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

	log.
		Trace().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", ccfg.ResourceTypeToString(ccfg.ResourceTypeTrailEvent)).
		Msgf("[CloudTrailRepository.ListEventsByLookupCached] events found: %d", len(items))

	return items, err
}
