package wafv2

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/service/cloudwatch"
)

// MetricNamespace is where WAF publishes request counts.
const MetricNamespace = "AWS/WAFV2"

// Dimension names WAF uses in AWS/WAFV2.
const (
	DimWebACL               = "WebACL"
	DimRegion               = "Region"
	DimRule                 = "Rule"
	DimRuleGroup            = "RuleGroup"
	DimManagedRuleGroup     = "ManagedRuleGroup"
	DimManagedRuleGroupRule = "ManagedRuleGroupRule"
	DimLabel                = "Label"
	DimLabelNamespace       = "LabelNamespace"

	// RuleAll and RuleDefaultAction are the reserved values of the Rule
	// dimension: the web ACL's totals and the requests no rule terminated.
	RuleAll           = "ALL"
	RuleDefaultAction = "Default_Action"
)

// maxMetricsPerQuery bounds how many series one stats call reads (four
// GetMetricData calls). A web ACL with every managed group and label metric
// enabled reaches the hundreds, not thousands.
const maxMetricsPerQuery = 2000

// ruleDimensions are the dimensions that locate a count within a web ACL's
// rules; a metric carrying only these (plus WebACL and Region) is a rule count.
var ruleDimensions = map[string]bool{
	DimRule:                 true,
	DimRuleGroup:            true,
	DimManagedRuleGroup:     true,
	DimManagedRuleGroupRule: true,
}

var labelDimensions = map[string]bool{
	DimLabel:          true,
	DimLabelNamespace: true,
}

// MetricsQuery selects the window and the breadth of a FetchWebACLMetrics call.
type MetricsQuery struct {
	Start time.Time
	End   time.Time
	// IncludeLabels adds the per-label counts (Label/LabelNamespace dimensions),
	// which managed rule groups publish in quantity.
	IncludeLabels bool
	// Now is the reference time for CloudWatch's retention rounding; zero means
	// time.Now().
	Now time.Time
}

// MetricSum is one CloudWatch series summed over the window.
type MetricSum struct {
	// MetricName is the count: AllowedRequests, BlockedRequests,
	// CountedRequests, CaptchaRequests, ChallengeRequests, …
	MetricName string
	// Dimensions are the series' dimensions without WebACL and Region, which
	// every series of one call shares — e.g. {Rule: ALL}, {Rule: <metric>},
	// {RuleGroup: <group>, Rule: <rule>}.
	Dimensions map[string]string
	Sum        float64
	// Partial is set when CloudWatch did not return the whole window.
	Partial bool
}

// WebACLMetrics is every request count a web ACL published in a window.
type WebACLMetrics struct {
	// MetricName is the ACL's visibility metric name, the WebACL dimension value.
	MetricName string
	// Region is where the metrics were read: the ACL's region, us-east-1 for
	// CLOUDFRONT.
	Region string
	// Start, End and PeriodSeconds are the window actually measured, after
	// CloudWatch's rounding (see cloudwatch.SingleWindow).
	Start         time.Time
	End           time.Time
	PeriodSeconds int32
	Sums          []MetricSum
	// Discovered is how many series ListMetrics returned for the ACL's metric
	// name before filtering.
	Discovered int
	// SkippedDimensions counts series left out because they carry dimensions
	// beyond rule ones (geo, bot category, …, and labels unless IncludeLabels),
	// keyed by the extra dimension's name.
	SkippedDimensions map[string]int
	// Truncated is set when more than maxMetricsPerQuery series matched.
	Truncated bool
	Warnings  []string
}

// FetchWebACLMetrics reads the request counts a web ACL published in a window:
// totals (Rule=ALL), the default action, every rule, and the rules inside rule
// groups and managed rule groups, each summed over the window.
//
// Series are discovered with ListMetrics rather than constructed from the ACL's
// rules, for two reasons: WAF publishes a count only when it is non-zero, so most
// constructed series would be empty, and the dimension combinations below the
// rule level (rule groups, managed rule groups) are WAF's to choose. The cost is
// ListMetrics' two-week horizon: a series silent for longer is not discovered, so
// a window reaching further back can under-report rules that have since gone
// quiet. The caller should say so.
//
// Two web ACLs with the same metric name in the same region and scope publish
// into the same series; CloudWatch cannot tell them apart and neither can this.
//
// Deliberately not a Get*/List* method — see the cloudwatch package.
func (r *Wafv2Repository) FetchWebACLMetrics(acl WebACL, query MetricsQuery) (*WebACLMetrics, error) {
	metricName := acl.GetMetricName()
	if metricName == "" {
		return nil, errors.Errorf("web ACL %s has no CloudWatch metric name", acl.GetName())
	}

	now := query.Now
	if now.IsZero() {
		now = time.Now()
	}

	start, end, period, err := cloudwatch.SingleWindow(query.Start, query.End, now)
	if err != nil {
		return nil, err
	}

	region := acl.GetRegion()
	repo := cloudwatch.NewCloudWatchRepository(r.ctx, r.client).WithRegion(region)

	found, err := repo.FindMetrics(&awscw.ListMetricsInput{
		Namespace:  aws.String(MetricNamespace),
		Dimensions: []cwtypes.DimensionFilter{{Name: aws.String(DimWebACL), Value: aws.String(metricName)}},
	})
	if err != nil {
		return nil, errors.Errorf("listing WAF metrics for %s: %w", metricName, err)
	}

	out := &WebACLMetrics{
		MetricName:        metricName,
		Region:            region.String(),
		Start:             start,
		End:               end,
		PeriodSeconds:     period,
		Discovered:        len(found),
		SkippedDimensions: map[string]int{},
	}

	selected, warning := selectRegion(found, acl.Scope, region.String())
	if warning != "" {
		out.Warnings = append(out.Warnings, warning)
	}

	selected = selectDimensions(selected, query.IncludeLabels, out.SkippedDimensions)

	if len(selected) > maxMetricsPerQuery {
		sortMetrics(selected)
		selected = selected[:maxMetricsPerQuery]
		out.Truncated = true
	}

	if len(selected) == 0 {
		return out, nil
	}

	queries := make([]cwtypes.MetricDataQuery, 0, len(selected))
	for i, metric := range selected {
		queries = append(queries, cwtypes.MetricDataQuery{
			Id: aws.String(fmt.Sprintf("m%d", i)),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  metric.Namespace,
					MetricName: metric.MetricName,
					Dimensions: metric.Dimensions,
				},
				Period: aws.Int32(period),
				Stat:   aws.String(string(cwtypes.StatisticSum)),
			},
			ReturnData: aws.Bool(true),
		})
	}

	data, err := repo.FetchMetricData(queries, start, end)
	if err != nil {
		return nil, errors.Errorf("reading WAF metrics for %s: %w", metricName, err)
	}

	out.Warnings = append(out.Warnings, data.Messages...)

	for i, metric := range selected {
		result := data.Results[fmt.Sprintf("m%d", i)]

		sum := 0.0
		for _, value := range result.Values {
			sum += value
		}

		out.Sums = append(out.Sums, MetricSum{
			MetricName: aws.ToString(metric.MetricName),
			Dimensions: ruleDims(metric.Dimensions),
			Sum:        sum,
			Partial:    result.StatusCode == cwtypes.StatusCodePartialData || result.StatusCode == cwtypes.StatusCodeInternalError,
		})
	}

	return out, nil
}

// selectRegion keeps the series that belong to this ACL's scope and region.
//
// A REGIONAL ACL's series carry Region=<its region>. CloudFront-scoped ACLs are
// documented to publish without a Region dimension; "Global" is accepted too.
// If a CloudFront ACL's series turn out to carry Region=us-east-1 instead, they
// are used — with a warning, since a REGIONAL us-east-1 ACL sharing the metric
// name would then be indistinguishable.
func selectRegion(found []cwtypes.Metric, scope waftypes.Scope, region string) ([]cwtypes.Metric, string) {
	var global, regional []cwtypes.Metric

	others := map[string]bool{}

	for _, metric := range found {
		value, has := dimension(metric.Dimensions, DimRegion)

		switch {
		case !has || strings.EqualFold(value, "global"):
			global = append(global, metric)
		case value == region:
			regional = append(regional, metric)
		default:
			others[value] = true
		}
	}

	if scope != waftypes.ScopeCloudfront {
		return regional, ""
	}

	if len(global) > 0 || len(regional) == 0 {
		return global, ""
	}

	return regional, fmt.Sprintf("CloudFront web ACL metrics carry Region=%s; if a regional web ACL in %s uses the same metric name, its requests are included", region, region)
}

// selectDimensions keeps the rule-level series (and label series when asked),
// counting what it leaves out by the dimension that excluded it.
func selectDimensions(found []cwtypes.Metric, includeLabels bool, skipped map[string]int) []cwtypes.Metric {
	out := make([]cwtypes.Metric, 0, len(found))

	for _, metric := range found {
		extra := ""

		for _, dim := range metric.Dimensions {
			name := aws.ToString(dim.Name)

			switch {
			case name == DimWebACL || name == DimRegion || ruleDimensions[name]:
			case includeLabels && labelDimensions[name]:
			default:
				extra = name
			}
		}

		if extra != "" {
			skipped[extra]++

			continue
		}

		out = append(out, metric)
	}

	return out
}

// sortMetrics orders series so that truncation keeps the coarse ones: fewer
// dimensions first (Rule=ALL and per-rule counts before per-label counts).
func sortMetrics(metrics []cwtypes.Metric) {
	sort.SliceStable(metrics, func(i, j int) bool {
		return len(metrics[i].Dimensions) < len(metrics[j].Dimensions)
	})
}

func dimension(dims []cwtypes.Dimension, name string) (string, bool) {
	for _, dim := range dims {
		if aws.ToString(dim.Name) == name {
			return aws.ToString(dim.Value), true
		}
	}

	return "", false
}

func ruleDims(dims []cwtypes.Dimension) map[string]string {
	out := make(map[string]string, len(dims))

	for _, dim := range dims {
		name := aws.ToString(dim.Name)
		if name == DimWebACL || name == DimRegion {
			continue
		}

		out[name] = aws.ToString(dim.Value)
	}

	return out
}
