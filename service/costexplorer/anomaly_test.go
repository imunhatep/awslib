package costexplorer

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAnomalyQuery() AnomalyQuery {
	return AnomalyQuery{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC),
	}
}

func TestAnomalyQueryInput(t *testing.T) {
	q := validAnomalyQuery()
	q.MonitorArn = "arn:aws:ce::123456789012:anomalymonitor/abc"
	q.Feedback = types.AnomalyFeedbackTypeNo

	input := q.input()

	assert.Equal(t, "2026-10-01", *input.DateInterval.StartDate)
	assert.Equal(t, "2026-10-21", *input.DateInterval.EndDate)
	assert.Equal(t, "arn:aws:ce::123456789012:anomalymonitor/abc", *input.MonitorArn)
	assert.Equal(t, types.AnomalyFeedbackTypeNo, input.Feedback)
	assert.Nil(t, input.TotalImpact)
}

// A zero End leaves the window open: the API takes EndDate as optional, and
// sending today's date instead would pin the cache key to the day of the call.
func TestAnomalyQueryInput_OpenEnded(t *testing.T) {
	q := AnomalyQuery{Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}

	input := q.input()

	assert.Equal(t, "2026-10-01", *input.DateInterval.StartDate)
	assert.Nil(t, input.DateInterval.EndDate)
	assert.Nil(t, input.MonitorArn)
	assert.Empty(t, input.Feedback)
}

func TestAnomalyQueryInput_NormalisesToUTC(t *testing.T) {
	zone := time.FixedZone("EEST", 3*60*60)

	q := AnomalyQuery{Start: time.Date(2026, 10, 2, 1, 0, 0, 0, zone)}

	assert.Equal(t, "2026-10-01", *q.input().DateInterval.StartDate)
}

func TestAnomalyQueryInput_MinTotalImpact(t *testing.T) {
	q := validAnomalyQuery()
	q.MinTotalImpact = 100

	impact := q.input().TotalImpact

	require.NotNil(t, impact)
	assert.Equal(t, types.NumericOperatorGreaterThanOrEqual, impact.NumericOperator)
	assert.Equal(t, 100.0, impact.StartValue)
}

func TestAnomalyQueryInput_TotalImpactPassesThrough(t *testing.T) {
	filter := &types.TotalImpactFilter{
		NumericOperator: types.NumericOperatorBetween,
		StartValue:      50,
		EndValue:        500,
	}

	q := validAnomalyQuery()
	q.TotalImpact = filter

	assert.Same(t, filter, q.input().TotalImpact)
}

func TestAnomalyQueryValidate_Accepts(t *testing.T) {
	assert.NoError(t, validAnomalyQuery().Validate())

	// a single-day window: both bounds are inclusive days
	day := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	assert.NoError(t, AnomalyQuery{Start: day, End: day.Add(5 * time.Hour)}.Validate())

	withFeedback := validAnomalyQuery()
	withFeedback.Feedback = types.AnomalyFeedbackTypePlannedActivity
	withFeedback.MinTotalImpact = 10
	assert.NoError(t, withFeedback.Validate())
}

func TestAnomalyQueryValidate_Window(t *testing.T) {
	assert.ErrorContains(t, AnomalyQuery{}.Validate(), "needs a Start")

	backwards := validAnomalyQuery()
	backwards.Start, backwards.End = backwards.End, backwards.Start
	assert.ErrorContains(t, backwards.Validate(), "must not be before")
}

func TestAnomalyQueryValidate_Feedback(t *testing.T) {
	q := validAnomalyQuery()
	q.Feedback = "MAYBE"

	assert.ErrorContains(t, q.Validate(), `unknown Feedback "MAYBE"`)
}

func TestAnomalyQueryValidate_TotalImpact(t *testing.T) {
	negative := validAnomalyQuery()
	negative.MinTotalImpact = -1
	assert.ErrorContains(t, negative.Validate(), "must not be negative")

	both := validAnomalyQuery()
	both.MinTotalImpact = 10
	both.TotalImpact = &types.TotalImpactFilter{NumericOperator: types.NumericOperatorGreaterThan, StartValue: 5}
	assert.ErrorContains(t, both.Validate(), "both MinTotalImpact and TotalImpact")
}

// The cached repository stores GetAnomalies results with gob. Pin that an anomaly
// keeps its nested impact, score and every root cause across the round trip — the
// root causes are the reason to call the API at all.
func TestAnomaliesSurviveGobRoundTrip(t *testing.T) {
	original := []types.Anomaly{{
		AnomalyId:        aws.String("a-1"),
		MonitorArn:       aws.String("arn:aws:ce::123456789012:anomalymonitor/abc"),
		AnomalyStartDate: aws.String("2026-10-13"),
		AnomalyEndDate:   aws.String("2026-10-15"),
		DimensionValue:   aws.String("Amazon Simple Storage Service"),
		Feedback:         types.AnomalyFeedbackTypeYes,
		AnomalyScore:     &types.AnomalyScore{CurrentScore: 0.4, MaxScore: 0.9},
		Impact: &types.Impact{
			MaxImpact:          65,
			TotalImpact:        408.81,
			TotalActualSpend:   aws.Float64(900),
			TotalExpectedSpend: aws.Float64(491.19),
		},
		RootCauses: []types.RootCause{
			{Service: aws.String("Amazon Simple Storage Service"), Region: aws.String("eu-central-1"), UsageType: aws.String("EUC1-TimedStorage-ByteHrs"), LinkedAccount: aws.String("123456789012"), Impact: &types.RootCauseImpact{Contribution: 59}},
			{Service: aws.String("EC2 - Other"), Region: aws.String("eu-central-1"), UsageType: aws.String("EUC1-EBS:VolumeUsage.gp3"), Impact: &types.RootCauseImpact{Contribution: 200}},
		},
	}}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded []types.Anomaly
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))

	require.Len(t, decoded, 1)
	assert.Equal(t, "a-1", *decoded[0].AnomalyId)
	assert.Equal(t, types.AnomalyFeedbackTypeYes, decoded[0].Feedback)
	assert.Equal(t, 408.81, decoded[0].Impact.TotalImpact)
	assert.Equal(t, 491.19, *decoded[0].Impact.TotalExpectedSpend)
	assert.Equal(t, 0.9, decoded[0].AnomalyScore.MaxScore)
	require.Len(t, decoded[0].RootCauses, 2)
	assert.Equal(t, "EUC1-EBS:VolumeUsage.gp3", *decoded[0].RootCauses[1].UsageType)
	assert.Equal(t, 200.0, decoded[0].RootCauses[1].Impact.Contribution)
}
