// Package cloudwatch reads CloudWatch metrics: discovery with ListMetrics and
// values with GetMetricData. It is a thin, service-agnostic layer; callers that
// know a namespace's dimension scheme (service/wafv2 for AWS/WAFV2) build on it.
//
// Neither method is a Get*/List* method on purpose: metric values are a moving
// window, and the generated cached wrapper would answer "how many requests in
// the last hour" from a cache filled hours ago.
package cloudwatch

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	cwclient "github.com/imunhatep/awslib/provider/v3/clients/cloudwatch"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// MaxQueriesPerRequest is GetMetricData's limit on queries per call.
const MaxQueriesPerRequest = 500

type CloudWatchRepository struct {
	ctx    context.Context
	client *v3.Client
	region ptypes.AwsRegion
}

func NewCloudWatchRepository(ctx context.Context, client *v3.Client) *CloudWatchRepository {
	return &CloudWatchRepository{ctx: ctx, client: client}
}

// WithRegion returns a repository that sends every call to the given region
// with this client's credentials. Metrics live in the region that published
// them — CloudFront-scoped WAF metrics in us-east-1 — whatever region the
// caller's client was built for.
func (r *CloudWatchRepository) WithRegion(region ptypes.AwsRegion) *CloudWatchRepository {
	return &CloudWatchRepository{ctx: r.ctx, client: r.client, region: region}
}

// Region is the region calls go to.
func (r *CloudWatchRepository) Region() ptypes.AwsRegion {
	if r.region != "" {
		return r.region
	}

	return r.client.GetRegion()
}

func (r *CloudWatchRepository) opts() []func(*awscw.Options) {
	if r.region == "" || r.region == r.client.GetRegion() {
		return nil
	}

	region := r.region.String()

	return []func(*awscw.Options){func(o *awscw.Options) { o.Region = region }}
}

func (r *CloudWatchRepository) promLabels(method string) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.Region().String(),
		"resource_type": ccfg.ResourceTypeToString(ccfg.ResourceTypeCloudWatchMetric),
		"method":        method,
	}
}

// FindMetrics lists the metrics matching a ListMetrics query, following every
// page. ListMetrics reports metrics that received data in the last two weeks
// only, so a metric that has been silent longer is not found.
func (r *CloudWatchRepository) FindMetrics(query *awscw.ListMetricsInput) ([]cwtypes.Metric, error) {
	var out []cwtypes.Metric

	p := awscw.NewListMetricsPaginator(cwclient.GetClient(r.client), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListMetrics")).Inc()
		}

		output, err := p.NextPage(r.ctx, r.opts()...)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListMetrics")).Inc()
			}

			return nil, errors.New(err)
		}

		out = append(out, output.Metrics...)
	}

	return out, nil
}

// MetricData is the merged answer to a set of GetMetricData queries.
type MetricData struct {
	// Results are keyed by query Id. A query whose metric had no data in the
	// window has an entry with no values.
	Results map[string]cwtypes.MetricDataResult
	// Messages are the request-level messages CloudWatch attached (e.g. a
	// partial answer because of throttling).
	Messages []string
}

// FetchMetricData runs GetMetricData for any number of queries, in batches of
// MaxQueriesPerRequest, following NextToken, and merges the values and
// timestamps each query receives across pages.
func (r *CloudWatchRepository) FetchMetricData(queries []cwtypes.MetricDataQuery, start, end time.Time) (*MetricData, error) {
	out := &MetricData{Results: make(map[string]cwtypes.MetricDataResult, len(queries))}

	for offset := 0; offset < len(queries); offset += MaxQueriesPerRequest {
		batch := queries[offset:min(offset+MaxQueriesPerRequest, len(queries))]

		p := awscw.NewGetMetricDataPaginator(cwclient.GetClient(r.client), &awscw.GetMetricDataInput{
			MetricDataQueries: batch,
			StartTime:         aws.Time(start),
			EndTime:           aws.Time(end),
			ScanBy:            cwtypes.ScanByTimestampAscending,
		})

		for p.HasMorePages() {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequests.With(r.promLabels("GetMetricData")).Inc()
			}

			output, err := p.NextPage(r.ctx, r.opts()...)
			if err != nil {
				if metrics.AwsMetricsEnabled {
					metrics.AwsApiRequestErrors.With(r.promLabels("GetMetricData")).Inc()
				}

				return nil, errors.New(err)
			}

			for _, result := range output.MetricDataResults {
				id := aws.ToString(result.Id)

				merged, seen := out.Results[id]
				if !seen {
					out.Results[id] = result

					continue
				}

				merged.Values = append(merged.Values, result.Values...)
				merged.Timestamps = append(merged.Timestamps, result.Timestamps...)
				merged.Messages = append(merged.Messages, result.Messages...)
				// A later page completing the series supersedes an earlier
				// PartialData; an error anywhere sticks.
				if merged.StatusCode != cwtypes.StatusCodeInternalError {
					merged.StatusCode = result.StatusCode
				}

				out.Results[id] = merged
			}

			for _, message := range output.Messages {
				out.Messages = append(out.Messages, fmt.Sprintf("%s: %s", aws.ToString(message.Code), aws.ToString(message.Value)))
			}
		}
	}

	return out, nil
}

// SingleWindow snaps [start, end) to the granularity CloudWatch keeps for data
// of that age and returns a period spanning the whole window, so that each
// query returns at most one datapoint: its total over the window.
//
// CloudWatch rounds a start time down to the minute when it is under 15 days
// old, to 5 minutes under 63 days and to the hour beyond, and requires the
// period to be a multiple of the same unit. Both ends are snapped to that unit,
// so the period is an exact multiple and the returned window is the one actually
// measured — callers should report it rather than the window they asked for.
func SingleWindow(start, end, now time.Time) (time.Time, time.Time, int32, error) {
	if !end.After(start) {
		return time.Time{}, time.Time{}, 0, errors.Errorf("end %s must be after start %s", end.Format(time.RFC3339), start.Format(time.RFC3339))
	}

	unit := time.Minute
	switch age := now.Sub(start); {
	case age > 63*24*time.Hour:
		unit = time.Hour
	case age > 15*24*time.Hour:
		unit = 5 * time.Minute
	}

	start = start.UTC().Truncate(unit)
	end = end.UTC().Truncate(unit)

	if !end.After(start) {
		end = start.Add(unit)
	}

	seconds := end.Sub(start) / time.Second
	if seconds > 1<<31-1 {
		return time.Time{}, time.Time{}, 0, errors.Errorf("window %s is too long", end.Sub(start))
	}

	return start, end, int32(seconds), nil
}
