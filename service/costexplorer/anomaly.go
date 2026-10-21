package costexplorer

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/gocollection/slice"
)

// AnomalyQuery describes a GetAnomalies request in higher-level terms than the
// raw SDK input: a window of days, and the optional monitor, feedback and impact
// filters the Cost Anomaly Detection console offers.
//
// The window matches an anomaly's **end date**, not its start: the API returns
// the anomalies whose AnomalyEndDate falls between Start and End. An anomaly that
// began before Start but is still running is included; one that began inside the
// window and ended after End is not. Anomalies are retained for 90 days.
type AnomalyQuery struct {
	// Start is the first day of the window and is required. End is the last day,
	// inclusive — unlike CostQuery, whose End is exclusive. A zero End leaves the
	// window open up to today.
	Start time.Time
	End   time.Time

	// MonitorArn optionally restricts results to one anomaly monitor; see
	// GetAnomalyMonitors for the ARNs.
	MonitorArn string

	// Feedback optionally restricts results to anomalies that were given this
	// feedback (YES, NO or PLANNED_ACTIVITY).
	Feedback types.AnomalyFeedbackType

	// MinTotalImpact, when positive, keeps only anomalies whose total impact —
	// actual minus expected spend, in USD — is at least this amount. It is
	// shorthand for a GREATER_THAN_OR_EQUAL TotalImpact filter.
	MinTotalImpact float64

	// TotalImpact holds an arbitrary impact filter (BETWEEN, LESS_THAN, ...). It
	// is mutually exclusive with MinTotalImpact.
	TotalImpact *types.TotalImpactFilter
}

// dateInterval renders the window as the plain dates AnomalyDateInterval expects.
func (q AnomalyQuery) dateInterval() *types.AnomalyDateInterval {
	interval := &types.AnomalyDateInterval{
		StartDate: aws.String(q.Start.UTC().Format(dateLayout)),
	}

	if !q.End.IsZero() {
		interval.EndDate = aws.String(q.End.UTC().Format(dateLayout))
	}

	return interval
}

// totalImpact returns the impact filter the query asks for, or nil for none.
func (q AnomalyQuery) totalImpact() *types.TotalImpactFilter {
	if q.TotalImpact != nil {
		return q.TotalImpact
	}

	if q.MinTotalImpact > 0 {
		return &types.TotalImpactFilter{
			NumericOperator: types.NumericOperatorGreaterThanOrEqual,
			StartValue:      q.MinTotalImpact,
		}
	}

	return nil
}

// Validate reports the constraints GetAnomalies enforces that are cheaper to
// catch here than after a billed round trip: a start day, an end day that is not
// before it, a known feedback value and a single impact filter.
func (q AnomalyQuery) Validate() error {
	if q.Start.IsZero() {
		return errors.Errorf("anomaly query needs a Start")
	}

	if !q.End.IsZero() && q.End.UTC().Format(dateLayout) < q.Start.UTC().Format(dateLayout) {
		return errors.Errorf("anomaly query End (%s) must not be before Start (%s)", q.End, q.Start)
	}

	if q.Feedback != "" && !slice.Contains(q.Feedback.Values(), q.Feedback) {
		return errors.Errorf("anomaly query has unknown Feedback %q, want one of %v", q.Feedback, q.Feedback.Values())
	}

	if q.MinTotalImpact < 0 {
		return errors.Errorf("anomaly query MinTotalImpact (%v) must not be negative", q.MinTotalImpact)
	}

	if q.MinTotalImpact > 0 && q.TotalImpact != nil {
		return errors.Errorf("anomaly query sets both MinTotalImpact and TotalImpact, use one")
	}

	return nil
}

// input builds the GetAnomaliesInput for the query.
func (q AnomalyQuery) input() *awsce.GetAnomaliesInput {
	input := &awsce.GetAnomaliesInput{
		DateInterval: q.dateInterval(),
		Feedback:     q.Feedback,
		TotalImpact:  q.totalImpact(),
	}

	if q.MonitorArn != "" {
		input.MonitorArn = aws.String(q.MonitorArn)
	}

	return input
}
