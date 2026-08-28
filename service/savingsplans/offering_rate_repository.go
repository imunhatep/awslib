package savingsplans

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssp "github.com/aws/aws-sdk-go-v2/service/savingsplans"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListOfferingRatesByQuery returns every offering rate matching the query, following
// pagination to the end.
//
// There is deliberately no unfiltered ListOfferingRatesAll: rates are offered per
// region, product, instance type, term and payment option, so an unfiltered call is a
// multi-thousand-page sweep of the whole price list that no caller wants by accident.
func (r *SavingsPlansRepository) ListOfferingRatesByQuery(query OfferingRatesQuery) ([]OfferingRate, error) {
	rates, err := r.ListOfferingRatesByInput(query.ToInput())
	if err != nil {
		return rates, errors.New(err)
	}

	if query.DurationSeconds == 0 {
		return rates, nil
	}

	// The commitment length is carried by each result's parent offering and cannot be
	// filtered server-side, so a query that names a term drops the others here.
	kept := make([]OfferingRate, 0, len(rates))
	for _, rate := range rates {
		if query.keepsTerm(rate.DurationSeconds()) {
			kept = append(kept, rate)
		}
	}

	log.Debug().
		Str("region", query.Region.String()).
		Int64("durationSeconds", query.DurationSeconds).
		Int("rates", len(kept)).
		Int("dropped", len(rates)-len(kept)).
		Msg("[SavingsPlansRepository.ListOfferingRatesByQuery] rates filtered by term")

	return kept, nil
}

// ListOfferingRatesByInput returns every offering rate matching the raw SDK input.
// The API exposes no paginator, so the NextToken loop is explicit.
func (r *SavingsPlansRepository) ListOfferingRatesByInput(
	query *awssp.DescribeSavingsPlansOfferingRatesInput,
) ([]OfferingRate, error) {
	start := time.Now()
	rates := []OfferingRate{}

	nextToken := query.NextToken

	for {
		select {
		case <-r.ctx.Done():
			return rates, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeSavingsPlansOfferingRates", ccfg.ResourceTypeSavingsPlanOfferingRate)).
				Inc()
		}

		// copy the query per page so the caller's input is not mutated
		page := *query
		page.NextToken = nextToken

		resp, err := r.savingsPlansClient().DescribeSavingsPlansOfferingRates(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeSavingsPlansOfferingRates", ccfg.ResourceTypeSavingsPlanOfferingRate)).
					Inc()
			}

			return rates, errors.New(err)
		}

		for _, rate := range resp.SearchResults {
			rates = append(rates, NewOfferingRate(rate))
		}

		if aws.ToString(resp.NextToken) == "" {
			break
		}

		nextToken = resp.NextToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeSavingsPlansOfferingRates", ccfg.ResourceTypeSavingsPlanOfferingRate)).
			Add(float64(len(rates)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListOfferingRatesByInput", ccfg.ResourceTypeSavingsPlanOfferingRate)).
			Observe(time.Since(start).Seconds())
	}

	return rates, nil
}
