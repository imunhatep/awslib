package costexplorer

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validQuery() CostQuery {
	return CostQuery{
		Start:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Granularity: types.GranularityMonthly,
		Metrics:     []string{MetricUnblendedCost},
	}
}

func TestCostQueryBuildFilter(t *testing.T) {
	// service + one tag key -> AND of the two
	q := CostQuery{
		Services: []string{"AmazonS3"},
		Tags:     map[string][]string{"Env": {"prod"}},
	}
	assert.Len(t, q.buildFilter().And, 2)

	// no criteria -> nil filter
	assert.Nil(t, CostQuery{}.buildFilter())

	// only a service -> unwrapped dimension expression
	only := CostQuery{Services: []string{"AmazonS3"}}.buildFilter()
	assert.NotNil(t, only.Dimensions)
	assert.Empty(t, only.And)
}

func TestCostQueryBuildFilter_RowsAreAnded(t *testing.T) {
	q := CostQuery{
		Services: []string{"AmazonS3"},
		Filters: []Filter{
			ByDimension(types.DimensionRegion, "eu-west-1"),
			ByCostCategory("Team", "platform"),
			ByTag("Owner").Absent(),
			ByDimension(types.DimensionRecordType, "Credit").Excluded(),
		},
		Filter: FilterByPurchaseType("Spot Instances"),
	}

	expr := q.buildFilter()
	require.Len(t, expr.And, 6)
}

func TestCostQueryBuildFilter_EmptyRowsDropOut(t *testing.T) {
	q := CostQuery{
		Filters: []Filter{
			ByDimension(types.DimensionRegion), // no values
			ByTag("Env"),                       // no values, no operator
			ByDimension(types.DimensionService, "AmazonS3"),
		},
	}

	expr := q.buildFilter()
	assert.Empty(t, expr.And)
	assert.Equal(t, types.DimensionService, expr.Dimensions.Key)
}

func TestCostQueryTimePeriod(t *testing.T) {
	q := validQuery()
	assert.Equal(t, "2026-07-01", *q.timePeriod().Start)
	assert.Equal(t, "2026-08-01", *q.timePeriod().End)

	// HOURLY is the one granularity the API wants a timestamp for
	hourly := q
	hourly.Granularity = types.GranularityHourly
	hourly.Start = time.Date(2026, 7, 1, 13, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-07-01T13:30:00Z", *hourly.timePeriod().Start)
}

func TestCostQueryTimePeriod_NormalisesToUTC(t *testing.T) {
	zone := time.FixedZone("EEST", 3*60*60)

	q := validQuery()
	q.Granularity = types.GranularityHourly
	q.Start = time.Date(2026, 7, 1, 3, 0, 0, 0, zone)

	assert.Equal(t, "2026-07-01T00:00:00Z", *q.timePeriod().Start)
}

func TestCostQueryValidate_Accepts(t *testing.T) {
	q := validQuery()
	q.GroupBy = []types.GroupDefinition{GroupByService(), GroupByTag("Team")}
	q.Filters = []Filter{
		ByDimension(types.DimensionRegion, "eu-west-1"),
		ByTag("Owner").Absent(),
		ByCostCategory("Team", "platform").Matching(types.MatchOptionCaseSensitive),
	}

	assert.NoError(t, q.Validate())
}

func TestCostQueryValidate_TimePeriod(t *testing.T) {
	assert.ErrorContains(t, CostQuery{Metrics: []string{MetricUnblendedCost}}.Validate(), "Start and End")

	backwards := validQuery()
	backwards.Start, backwards.End = backwards.End, backwards.Start
	assert.ErrorContains(t, backwards.Validate(), "must be after")
}

func TestCostQueryValidate_MetricsRequired(t *testing.T) {
	q := validQuery()
	q.Metrics = nil

	assert.ErrorContains(t, q.Validate(), "at least one metric")
}

func TestCostQueryValidate_GroupByLimit(t *testing.T) {
	q := validQuery()
	q.GroupBy = GroupByTags("Team", "Env", "Stage")

	assert.ErrorContains(t, q.Validate(), "at most 2")

	empty := validQuery()
	empty.GroupBy = []types.GroupDefinition{GroupByTag("")}
	assert.ErrorContains(t, empty.Validate(), "empty key")
}

// GetCostAndUsage takes a narrower set of match options than Expression allows;
// catching that here saves a round trip that would fail server-side.
func TestCostQueryValidate_MatchOptions(t *testing.T) {
	dimension := validQuery()
	dimension.Filters = []Filter{
		ByDimension(types.DimensionLinkedAccountName, "a").Matching(types.MatchOptionStartsWith),
	}
	assert.ErrorContains(t, dimension.Validate(), "STARTS_WITH")

	// ABSENT is valid on tags but not on dimensions
	dimensionAbsent := validQuery()
	dimensionAbsent.Filters = []Filter{
		ByDimension(types.DimensionService).Matching(types.MatchOptionAbsent),
	}
	assert.ErrorContains(t, dimensionAbsent.Validate(), "ABSENT")

	tag := validQuery()
	tag.Filters = []Filter{ByTag("Env", "prod").Matching(types.MatchOptionContains)}
	assert.ErrorContains(t, tag.Validate(), "CONTAINS")

	category := validQuery()
	category.Filters = []Filter{ByCostCategory("Team", "x").Matching(types.MatchOptionEndsWith)}
	assert.ErrorContains(t, category.Validate(), "ENDS_WITH")
}
