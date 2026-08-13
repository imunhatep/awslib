package costexplorer

import (
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

// The metrics GetCostAndUsage accepts, matching the console's "Cost type"
// selector plus the two usage metrics.
const (
	MetricAmortizedCost    = "AmortizedCost"
	MetricBlendedCost      = "BlendedCost"
	MetricNetAmortizedCost = "NetAmortizedCost"
	MetricNetUnblendedCost = "NetUnblendedCost"
	MetricUnblendedCost    = "UnblendedCost"
	MetricUsageQuantity    = "UsageQuantity"
	MetricNormalizedUsage  = "NormalizedUsageAmount"
)

// CostAndUsage is the accumulated result of a (paginated) GetCostAndUsage call.
// It flattens the per-page ResultsByTime / DimensionValueAttributes into single
// slices while keeping the GroupDefinitions echoed back by the API.
type CostAndUsage struct {
	GroupDefinitions         []types.GroupDefinition
	DimensionValueAttributes []types.DimensionValuesWithAttributes
	ResultsByTime            []types.ResultByTime
}

// GetResultsByTime returns the cost/usage results grouped by time period.
func (c CostAndUsage) GetResultsByTime() []types.ResultByTime {
	return c.ResultsByTime
}

// GetGroupDefinitions returns the group definitions echoed back by the API.
func (c CostAndUsage) GetGroupDefinitions() []types.GroupDefinition {
	return c.GroupDefinitions
}

// GetTotalByMetric sums the given metric (e.g. "UnblendedCost", "AmortizedCost")
// across every returned time period, using the period-level Total map (the
// aggregate emitted when no GroupBy is set). It returns the summed amount and the
// unit reported by AWS. The unit is empty and the amount is 0 when the metric is
// not present in any period.
func (c CostAndUsage) GetTotalByMetric(metric string) (amount float64, unit string) {
	for _, r := range c.ResultsByTime {
		v, ok := r.Total[metric]
		if !ok {
			continue
		}

		if v.Unit != nil {
			unit = *v.Unit
		}

		if v.Amount != nil {
			if parsed, err := strconv.ParseFloat(*v.Amount, 64); err == nil {
				amount += parsed
			}
		}
	}

	return amount, unit
}
