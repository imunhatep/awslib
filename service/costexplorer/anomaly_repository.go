package costexplorer

import (
	"time"

	awsce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// GetAnomalies retrieves the cost anomalies Cost Anomaly Detection found for the
// query, following NextPageToken pagination and accumulating every page. Each
// anomaly carries all of its root causes (service, region, usage type, linked
// account and their contribution), not only the top ones an alert email shows.
func (r *CostExplorerRepository) GetAnomalies(query *awsce.GetAnomaliesInput) ([]types.Anomaly, error) {
	start := time.Now()

	result := []types.Anomaly{}
	nextToken := query.NextPageToken

	for {
		select {
		case <-r.ctx.Done():
			return result, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetAnomalies", ccfg.ResourceTypeCostAnomaly)).Inc()
		}

		page := *query
		page.NextPageToken = nextToken

		output, err := r.costExplorerClient().GetAnomalies(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetAnomalies", ccfg.ResourceTypeCostAnomaly)).Inc()
			}

			return nil, errors.New(err)
		}

		result = append(result, output.Anomalies...)

		if output.NextPageToken == nil {
			break
		}

		nextToken = output.NextPageToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetAnomalies", ccfg.ResourceTypeCostAnomaly)).
			Add(float64(len(result)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetAnomalies", ccfg.ResourceTypeCostAnomaly)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}

// GetAnomaliesByQuery validates the high-level AnomalyQuery, builds a
// GetAnomaliesInput from it and delegates to GetAnomalies.
func (r *CostExplorerRepository) GetAnomaliesByQuery(q AnomalyQuery) ([]types.Anomaly, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	return r.GetAnomalies(q.input())
}

// GetAnomalyMonitors retrieves the anomaly monitors — what each one watches and
// how it slices spend — following NextPageToken pagination. An empty
// MonitorArnList returns every monitor in the account.
func (r *CostExplorerRepository) GetAnomalyMonitors(query *awsce.GetAnomalyMonitorsInput) ([]types.AnomalyMonitor, error) {
	start := time.Now()

	result := []types.AnomalyMonitor{}
	nextToken := query.NextPageToken

	for {
		select {
		case <-r.ctx.Done():
			return result, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetAnomalyMonitors", ccfg.ResourceTypeCostAnomalyMonitor)).Inc()
		}

		page := *query
		page.NextPageToken = nextToken

		output, err := r.costExplorerClient().GetAnomalyMonitors(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetAnomalyMonitors", ccfg.ResourceTypeCostAnomalyMonitor)).Inc()
			}

			return nil, errors.New(err)
		}

		result = append(result, output.AnomalyMonitors...)

		if output.NextPageToken == nil {
			break
		}

		nextToken = output.NextPageToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetAnomalyMonitors", ccfg.ResourceTypeCostAnomalyMonitor)).
			Add(float64(len(result)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetAnomalyMonitors", ccfg.ResourceTypeCostAnomalyMonitor)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}

// GetAnomalySubscriptions retrieves the alert subscriptions — who is notified,
// how often and above which threshold — following NextPageToken pagination.
// Set MonitorArn to list only the subscriptions of one monitor.
func (r *CostExplorerRepository) GetAnomalySubscriptions(query *awsce.GetAnomalySubscriptionsInput) ([]types.AnomalySubscription, error) {
	start := time.Now()

	result := []types.AnomalySubscription{}
	nextToken := query.NextPageToken

	for {
		select {
		case <-r.ctx.Done():
			return result, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetAnomalySubscriptions", ccfg.ResourceTypeCostAnomalySubscription)).Inc()
		}

		page := *query
		page.NextPageToken = nextToken

		output, err := r.costExplorerClient().GetAnomalySubscriptions(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetAnomalySubscriptions", ccfg.ResourceTypeCostAnomalySubscription)).Inc()
			}

			return nil, errors.New(err)
		}

		result = append(result, output.AnomalySubscriptions...)

		if output.NextPageToken == nil {
			break
		}

		nextToken = output.NextPageToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetAnomalySubscriptions", ccfg.ResourceTypeCostAnomalySubscription)).
			Add(float64(len(result)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetAnomalySubscriptions", ccfg.ResourceTypeCostAnomalySubscription)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}
