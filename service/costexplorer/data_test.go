package costexplorer

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/stretchr/testify/assert"
)

func resultByTime(amount, unit string) types.ResultByTime {
	return types.ResultByTime{
		Total: map[string]types.MetricValue{
			"UnblendedCost": {Amount: aws.String(amount), Unit: aws.String(unit)},
		},
	}
}

func TestCostAndUsageGetTotalByMetric(t *testing.T) {
	cu := CostAndUsage{
		ResultsByTime: []types.ResultByTime{
			resultByTime("10.50", "USD"),
			resultByTime("4.25", "USD"),
		},
	}

	amount, unit := cu.GetTotalByMetric("UnblendedCost")

	assert.InDelta(t, 14.75, amount, 0.0001)
	assert.Equal(t, "USD", unit)
}

func TestCostAndUsageGetTotalByMetric_MissingMetric(t *testing.T) {
	cu := CostAndUsage{
		ResultsByTime: []types.ResultByTime{resultByTime("10.50", "USD")},
	}

	amount, unit := cu.GetTotalByMetric("AmortizedCost")

	assert.Zero(t, amount)
	assert.Empty(t, unit)
}

func TestCostAndUsageGetTotalByMetric_Empty(t *testing.T) {
	cu := CostAndUsage{}

	amount, unit := cu.GetTotalByMetric("UnblendedCost")

	assert.Zero(t, amount)
	assert.Empty(t, unit)
}
