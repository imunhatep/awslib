package savingsplans

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssp "github.com/aws/aws-sdk-go-v2/service/savingsplans"
	"github.com/aws/aws-sdk-go-v2/service/savingsplans/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// SavingsPlanStateActive is the state of a plan currently covering usage. A plan is
// also returned once it is queued, retired or payment-failed, so filtering by state
// is nearly always what a caller wants.
const SavingsPlanStateActive = types.SavingsPlanStateActive

// ListSavingsPlansAll returns every savings plan the account holds, in any state.
//
// Plans are account-wide, not regional: the same list comes back whichever region the
// repository's client was built for, so calling this once per region multiplies the
// requests without adding a plan.
func (r *SavingsPlansRepository) ListSavingsPlansAll() ([]types.SavingsPlan, error) {
	return r.ListSavingsPlansByInput(&awssp.DescribeSavingsPlansInput{})
}

// ListSavingsPlansByStates returns the account's savings plans in the given states.
func (r *SavingsPlansRepository) ListSavingsPlansByStates(states []types.SavingsPlanState) ([]types.SavingsPlan, error) {
	return r.ListSavingsPlansByInput(&awssp.DescribeSavingsPlansInput{States: states})
}

// ListSavingsPlansByInput returns every savings plan matching the raw SDK input. The
// API exposes no paginator, so the NextToken loop is explicit.
func (r *SavingsPlansRepository) ListSavingsPlansByInput(
	query *awssp.DescribeSavingsPlansInput,
) ([]types.SavingsPlan, error) {
	start := time.Now()
	plans := []types.SavingsPlan{}

	nextToken := query.NextToken

	for {
		select {
		case <-r.ctx.Done():
			return plans, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeSavingsPlans", ccfg.ResourceTypeSavingsPlan)).
				Inc()
		}

		// copy the query per page so the caller's input is not mutated
		page := *query
		page.NextToken = nextToken

		resp, err := r.savingsPlansClient().DescribeSavingsPlans(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeSavingsPlans", ccfg.ResourceTypeSavingsPlan)).
					Inc()
			}

			return plans, errors.New(err)
		}

		plans = append(plans, resp.SavingsPlans...)

		if aws.ToString(resp.NextToken) == "" {
			break
		}

		nextToken = resp.NextToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeSavingsPlans", ccfg.ResourceTypeSavingsPlan)).
			Add(float64(len(plans)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListSavingsPlansByInput", ccfg.ResourceTypeSavingsPlan)).
			Observe(time.Since(start).Seconds())
	}

	return plans, nil
}
