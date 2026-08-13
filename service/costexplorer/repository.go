package costexplorer

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/costexplorer"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// Layouts the Cost Explorer API expects for DateInterval. HOURLY granularity is
// the only one that takes a timestamp; the rest take a plain date.
const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02T15:04:05Z"
)

type CostExplorerRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewCostExplorerRepository(ctx context.Context, client *v3.Client) *CostExplorerRepository {
	repo := &CostExplorerRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *CostExplorerRepository) costExplorerClient() *awsce.Client {
	return costexplorer.GetClient(r.client)
}

func (r *CostExplorerRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *CostExplorerRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

// GetCostAndUsage retrieves cost and usage metrics for the given query, following
// NextPageToken pagination and accumulating every page into a single CostAndUsage.
func (r *CostExplorerRepository) GetCostAndUsage(query *awsce.GetCostAndUsageInput) (*CostAndUsage, error) {
	start := time.Now()

	result := &CostAndUsage{}
	nextToken := query.NextPageToken

	for {
		select {
		case <-r.ctx.Done():
			return result, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetCostAndUsage", ccfg.ResourceTypeCostAndUsage)).Inc()
		}

		// copy the query per page so the caller's input is not mutated
		page := *query
		page.NextPageToken = nextToken

		output, err := r.costExplorerClient().GetCostAndUsage(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetCostAndUsage", ccfg.ResourceTypeCostAndUsage)).Inc()
			}

			return nil, errors.New(err)
		}

		result.GroupDefinitions = output.GroupDefinitions
		result.DimensionValueAttributes = append(result.DimensionValueAttributes, output.DimensionValueAttributes...)
		result.ResultsByTime = append(result.ResultsByTime, output.ResultsByTime...)

		if output.NextPageToken == nil {
			break
		}

		nextToken = output.NextPageToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetCostAndUsage", ccfg.ResourceTypeCostAndUsage)).
			Add(float64(len(result.ResultsByTime)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetCostAndUsage", ccfg.ResourceTypeCostAndUsage)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}

// CostQuery describes a cost-and-usage request in higher-level terms than the raw
// SDK input, mirroring how the Cost Explorer console is driven: a time period, a
// granularity, the cost metrics to report, a list of filter rows and up to
// MaxGroupBy groupings.
//
// Every filter row is AND-ed with the others; the values inside a row are OR-ed
// by the API.
type CostQuery struct {
	// Start is inclusive, End is exclusive, as required by the Cost Explorer API.
	Start       time.Time
	End         time.Time
	Granularity types.Granularity
	Metrics     []string

	// Services optionally restricts results to the given AWS service names
	// (SERVICE dimension). Shorthand for a ByDimension(DimensionService, ...) row.
	Services []string

	// Tags optionally restricts results to resources carrying the given tags:
	// key -> allowed values. Shorthand for one ByTag row per key.
	Tags map[string][]string

	// Filters holds arbitrary filter rows — dimensions, tags or cost categories,
	// each with its own operator and include/exclude flag. Build them with
	// ByDimension, ByTag and ByCostCategory.
	Filters []Filter

	// Filter, when set, is AND-ed with everything above, allowing a hand-built
	// Expression to be supplied directly.
	Filter *types.Expression

	// GroupBy optionally groups the results — e.g. GroupByService(),
	// GroupByTag("Team"), GroupByCostCategory("Team"). The API accepts at most
	// MaxGroupBy groupings across all group types combined.
	GroupBy []types.GroupDefinition

	// BillingViewArn optionally scopes the query to a billing view.
	BillingViewArn string
}

// buildFilter combines the Services, Tags, Filters rows and explicit Filter of
// the query into a single Expression, returning nil when no criteria are set.
func (q CostQuery) buildFilter() *types.Expression {
	exprs := []*types.Expression{
		FilterByService(q.Services...),
		FilterByTags(q.Tags),
	}

	for _, f := range q.Filters {
		exprs = append(exprs, f.Expression())
	}

	return And(append(exprs, q.Filter)...)
}

// timePeriod renders the query window in the layout the requested granularity
// expects: HOURLY takes a timestamp, DAILY and MONTHLY take a plain date.
func (q CostQuery) timePeriod() *types.DateInterval {
	layout := dateLayout
	if q.Granularity == types.GranularityHourly {
		layout = dateTimeLayout
	}

	return &types.DateInterval{
		Start: aws.String(q.Start.UTC().Format(layout)),
		End:   aws.String(q.End.UTC().Format(layout)),
	}
}

// Validate reports the constraints GetCostAndUsage enforces that are cheaper to
// catch here than after a round trip: a well-formed time period, at least one
// metric, at most MaxGroupBy groupings and match options the API accepts.
func (q CostQuery) Validate() error {
	if q.Start.IsZero() || q.End.IsZero() {
		return errors.Errorf("cost query needs both Start and End")
	}

	if !q.End.After(q.Start) {
		return errors.Errorf("cost query End (%s) must be after Start (%s)", q.End, q.Start)
	}

	if len(q.Metrics) == 0 {
		return errors.Errorf("cost query needs at least one metric, e.g. %q", MetricUnblendedCost)
	}

	if len(q.GroupBy) > MaxGroupBy {
		return errors.Errorf("cost query has %d groupings, GetCostAndUsage accepts at most %d", len(q.GroupBy), MaxGroupBy)
	}

	for _, g := range q.GroupBy {
		if aws.ToString(g.Key) == "" {
			return errors.Errorf("cost query has a %s grouping with an empty key", g.Type)
		}
	}

	for _, f := range q.Filters {
		if err := f.validateForCostAndUsage(); err != nil {
			return err
		}
	}

	return nil
}

// GetCostAndUsageByQuery validates the high-level CostQuery, builds a
// GetCostAndUsageInput from it and delegates to GetCostAndUsage.
func (r *CostExplorerRepository) GetCostAndUsageByQuery(q CostQuery) (*CostAndUsage, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	query := &awsce.GetCostAndUsageInput{
		Granularity: q.Granularity,
		Metrics:     q.Metrics,
		TimePeriod:  q.timePeriod(),
		GroupBy:     q.GroupBy,
		Filter:      q.buildFilter(),
	}

	if q.BillingViewArn != "" {
		query.BillingViewArn = aws.String(q.BillingViewArn)
	}

	return r.GetCostAndUsage(query)
}

// GetCostAndUsageByPeriod is a thin convenience wrapper around
// GetCostAndUsageByQuery that takes just a time window, granularity, the requested
// metrics and an optional GroupBy (no filtering).
func (r *CostExplorerRepository) GetCostAndUsageByPeriod(
	start, end time.Time,
	granularity types.Granularity,
	costMetrics []string,
	groupBy []types.GroupDefinition,
) (*CostAndUsage, error) {
	return r.GetCostAndUsageByQuery(CostQuery{
		Start:       start,
		End:         end,
		Granularity: granularity,
		Metrics:     costMetrics,
		GroupBy:     groupBy,
	})
}

// GetCostForecast retrieves a forecast of how much the account is expected to
// spend over the requested time period.
func (r *CostExplorerRepository) GetCostForecast(query *awsce.GetCostForecastInput) (*awsce.GetCostForecastOutput, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetCostForecast", ccfg.ResourceTypeCostForecast)).Inc()
	}

	output, err := r.costExplorerClient().GetCostForecast(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetCostForecast", ccfg.ResourceTypeCostForecast)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetCostForecast", ccfg.ResourceTypeCostForecast)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// GetDimensionValues retrieves the available values for a dimension (e.g. SERVICE,
// LINKED_ACCOUNT) over the requested time period, following NextPageToken
// pagination and accumulating every page.
func (r *CostExplorerRepository) GetDimensionValues(query *awsce.GetDimensionValuesInput) ([]types.DimensionValuesWithAttributes, error) {
	start := time.Now()

	result := []types.DimensionValuesWithAttributes{}
	nextToken := query.NextPageToken

	for {
		select {
		case <-r.ctx.Done():
			return result, errors.New(r.ctx.Err())
		default:
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetDimensionValues", ccfg.ResourceTypeCostDimensionValue)).Inc()
		}

		page := *query
		page.NextPageToken = nextToken

		output, err := r.costExplorerClient().GetDimensionValues(r.ctx, &page)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetDimensionValues", ccfg.ResourceTypeCostDimensionValue)).Inc()
			}

			return nil, errors.New(err)
		}

		result = append(result, output.DimensionValues...)

		if output.NextPageToken == nil {
			break
		}

		nextToken = output.NextPageToken
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDimensionValues", ccfg.ResourceTypeCostDimensionValue)).
			Add(float64(len(result)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetDimensionValues", ccfg.ResourceTypeCostDimensionValue)).
			Observe(time.Since(start).Seconds())
	}

	return result, nil
}
