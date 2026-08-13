package costexplorer

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterByService(t *testing.T) {
	expr := FilterByService("Amazon Elastic Compute Cloud - Compute", "AmazonS3")

	assert.Nil(t, FilterByService())
	assert.NotNil(t, expr.Dimensions)
	assert.Equal(t, types.DimensionService, expr.Dimensions.Key)
	assert.ElementsMatch(t, []string{"Amazon Elastic Compute Cloud - Compute", "AmazonS3"}, expr.Dimensions.Values)
}

func TestFilterByTag(t *testing.T) {
	expr := FilterByTag("Team", "platform", "data")

	assert.NotNil(t, expr.Tags)
	assert.Equal(t, "Team", *expr.Tags.Key)
	assert.ElementsMatch(t, []string{"platform", "data"}, expr.Tags.Values)
}

func TestFilterByTags_MultipleKeysAreAnded(t *testing.T) {
	expr := FilterByTags(map[string][]string{
		"Team": {"platform"},
		"Env":  {"prod"},
	})

	// two keys -> AND node with two children
	assert.Len(t, expr.And, 2)
	assert.Empty(t, expr.Or)
}

func TestFilterByTags_SingleKeyIsUnwrapped(t *testing.T) {
	expr := FilterByTags(map[string][]string{"Team": {"platform"}})

	// a single child must NOT be wrapped in an And node (CE rejects that)
	assert.Empty(t, expr.And)
	assert.NotNil(t, expr.Tags)
	assert.Equal(t, "Team", *expr.Tags.Key)
}

func TestFilterByTags_EmptyIsNil(t *testing.T) {
	assert.Nil(t, FilterByTags(nil))
	assert.Nil(t, FilterByTags(map[string][]string{}))
}

func TestAnd_SkipsNilAndUnwrapsSingle(t *testing.T) {
	svc := FilterByService("AmazonS3")

	// nils are dropped; the single remaining expr is returned unwrapped
	combined := And(nil, svc, nil)
	assert.Empty(t, combined.And)
	assert.NotNil(t, combined.Dimensions)

	// nothing left -> nil
	assert.Nil(t, And(nil, nil))

	// two real exprs -> AND node
	both := And(svc, FilterByTag("Env", "prod"))
	assert.Len(t, both.And, 2)
}

func TestNot(t *testing.T) {
	assert.Nil(t, Not(nil))

	expr := Not(FilterByService("AmazonS3"))
	require.NotNil(t, expr.Not)
	assert.Equal(t, types.DimensionService, expr.Not.Dimensions.Key)
}

// -----------------------------------------------------------------------------
// Filter rows
// -----------------------------------------------------------------------------

func TestByDimension_MatchOptionsAndExclusion(t *testing.T) {
	plain := ByDimension(types.DimensionRegion, "eu-west-1").Expression()
	require.NotNil(t, plain.Dimensions)
	assert.Empty(t, plain.Dimensions.MatchOptions)
	assert.Nil(t, plain.Not)

	matched := ByDimension(types.DimensionLinkedAccountName, "a").
		Matching(types.MatchOptionStartsWith).
		Expression()
	assert.Equal(t, []types.MatchOption{types.MatchOptionStartsWith}, matched.Dimensions.MatchOptions)

	excluded := ByDimension(types.DimensionRecordType, "Credit", "Refund").Excluded().Expression()
	require.NotNil(t, excluded.Not)
	assert.ElementsMatch(t, []string{"Credit", "Refund"}, excluded.Not.Dimensions.Values)

	// nothing to match on -> nil, so it drops out of an And
	assert.Nil(t, ByDimension(types.DimensionRegion).Expression())
}

func TestByTag_Absent(t *testing.T) {
	expr := ByTag("Team", "ignored").Absent().Expression()

	require.NotNil(t, expr.Tags)
	assert.Equal(t, "Team", *expr.Tags.Key)
	assert.Equal(t, []types.MatchOption{types.MatchOptionAbsent}, expr.Tags.MatchOptions)
	// ABSENT matches on the key alone; any values set earlier are dropped
	assert.Empty(t, expr.Tags.Values)

	assert.Equal(t, expr, FilterByTagAbsent("Team"))
}

func TestByTag_Present(t *testing.T) {
	// the API has no positive "key exists" operator — it is NOT(ABSENT)
	expr := FilterByTagPresent("Team")

	require.NotNil(t, expr.Not)
	assert.Equal(t, "Team", *expr.Not.Tags.Key)
	assert.Equal(t, []types.MatchOption{types.MatchOptionAbsent}, expr.Not.Tags.MatchOptions)

	category := FilterByCostCategoryPresent("Team")
	require.NotNil(t, category.Not)
	assert.Equal(t, []types.MatchOption{types.MatchOptionAbsent}, category.Not.CostCategories.MatchOptions)
}

func TestByTag_NoValuesAndNoOperatorIsNil(t *testing.T) {
	// a bare key is ambiguous — Absent/Present state the intent instead
	assert.Nil(t, ByTag("Team").Expression())
	assert.Nil(t, FilterByTag("Team"))
}

func TestByCostCategory(t *testing.T) {
	expr := ByCostCategory("Team", "platform").Expression()

	require.NotNil(t, expr.CostCategories)
	assert.Equal(t, "Team", *expr.CostCategories.Key)
	assert.ElementsMatch(t, []string{"platform"}, expr.CostCategories.Values)

	absent := FilterByCostCategoryAbsent("Team")
	assert.Equal(t, []types.MatchOption{types.MatchOptionAbsent}, absent.CostCategories.MatchOptions)

	excluded := ByCostCategory("Team", "platform").Excluded().Expression()
	require.NotNil(t, excluded.Not)
	assert.NotNil(t, excluded.Not.CostCategories)
}

func TestFilterByCostCategories_MultipleKeysAreAnded(t *testing.T) {
	expr := FilterByCostCategories(map[string][]string{
		"Team":  {"platform"},
		"Stage": {"prod"},
	})

	assert.Len(t, expr.And, 2)
	assert.Nil(t, FilterByCostCategories(nil))
}

func TestFiltersExpression_AndsRowsAndDropsEmpty(t *testing.T) {
	expr := FiltersExpression(
		ByDimension(types.DimensionService, "AmazonS3"),
		ByTag("Env", "prod"),
		ByDimension(types.DimensionRegion), // no values -> dropped
	)

	assert.Len(t, expr.And, 2)
	assert.Nil(t, FiltersExpression())
}

// Every builder returning nil when empty is what lets rows compose without the
// caller checking each one.
func TestFilterRows_ComposeIntoNestedExpression(t *testing.T) {
	// ((REGION == eu-west-1 OR REGION == eu-north-1) AND NOT USAGE_TYPE contains DataTransfer)
	expr := And(
		FilterByRegion("eu-west-1", "eu-north-1"),
		ByDimension(types.DimensionUsageType, "DataTransfer").Excluded().Expression(),
	)

	require.Len(t, expr.And, 2)
	assert.Equal(t, types.DimensionRegion, expr.And[0].Dimensions.Key)
	require.NotNil(t, expr.And[1].Not)
	assert.Equal(t, types.DimensionUsageType, expr.And[1].Not.Dimensions.Key)
}

func TestNamedDimensionFilters(t *testing.T) {
	cases := map[types.Dimension]*types.Expression{
		types.DimensionLinkedAccount:      FilterByLinkedAccount("123456789012"),
		types.DimensionRegion:             FilterByRegion("eu-west-1"),
		types.DimensionAz:                 FilterByAvailabilityZone("eu-west-1a"),
		types.DimensionInstanceType:       FilterByInstanceType("t3.micro"),
		types.DimensionInstanceTypeFamily: FilterByInstanceTypeFamily("t3"),
		types.DimensionUsageType:          FilterByUsageType("BoxUsage:t3.micro"),
		types.DimensionUsageTypeGroup:     FilterByUsageTypeGroup("EC2: Running Hours"),
		types.DimensionRecordType:         FilterByChargeType("Usage"),
		types.DimensionPurchaseType:       FilterByPurchaseType("Spot Instances"),
		types.DimensionOperation:          FilterByOperation("RunInstances"),
		types.DimensionResourceId:         FilterByResourceID("i-0123"),
		types.DimensionPlatform:           FilterByPlatform("Linux/UNIX"),
		types.DimensionOperatingSystem:    FilterByOperatingSystem("Linux"),
		types.DimensionTenancy:            FilterByTenancy("Shared"),
		types.DimensionDatabaseEngine:     FilterByDatabaseEngine("PostgreSQL"),
		types.DimensionDeploymentOption:   FilterByDeploymentOption("Multi-AZ"),
		types.DimensionCacheEngine:        FilterByCacheEngine("Redis"),
		types.DimensionBillingEntity:      FilterByBillingEntity("AWS Marketplace"),
		types.DimensionLegalEntityName:    FilterByLegalEntity("Amazon Web Services EMEA SARL"),
		types.DimensionInvoicingEntity:    FilterByInvoicingEntity("Amazon Web Services EMEA SARL"),
		types.DimensionScope:              FilterByScope("Region"),
		types.DimensionPaymentOption:      FilterByPaymentOption("All Upfront"),
		types.DimensionSavingsPlansType:   FilterBySavingsPlansType("Compute Savings Plans"),
		types.DimensionSavingsPlanArn:     FilterBySavingsPlanArn("arn:aws:savingsplans::1:savingsplan/x"),
		types.DimensionReservationId:      FilterByReservationID("ri-0123"),
		types.DimensionSubscriptionId:     FilterBySubscriptionID("1234"),
	}

	for dimension, expr := range cases {
		require.NotNilf(t, expr, "filter for %s", dimension)
		assert.Equalf(t, dimension, expr.Dimensions.Key, "filter for %s", dimension)
		assert.Lenf(t, expr.Dimensions.Values, 1, "filter for %s", dimension)
	}
}

// -----------------------------------------------------------------------------
// GroupBy
// -----------------------------------------------------------------------------

func TestGroupByBuilders(t *testing.T) {
	svc := GroupByService()
	assert.Equal(t, types.GroupDefinitionTypeDimension, svc.Type)
	assert.Equal(t, string(types.DimensionService), *svc.Key)

	usage := GroupByUsageType()
	assert.Equal(t, string(types.DimensionUsageType), *usage.Key)

	tag := GroupByTag("Team")
	assert.Equal(t, types.GroupDefinitionTypeTag, tag.Type)
	assert.Equal(t, "Team", *tag.Key)

	tags := GroupByTags("Team", "Env")
	assert.Len(t, tags, 2)
	assert.Equal(t, types.GroupDefinitionTypeTag, tags[0].Type)
}

func TestGroupByNamedDimensions(t *testing.T) {
	cases := map[types.Dimension]types.GroupDefinition{
		types.DimensionLinkedAccount: GroupByLinkedAccount(),
		types.DimensionRegion:        GroupByRegion(),
		types.DimensionInstanceType:  GroupByInstanceType(),
		types.DimensionRecordType:    GroupByChargeType(),
		types.DimensionPurchaseType:  GroupByPurchaseType(),
		types.DimensionOperation:     GroupByOperation(),
	}

	for dimension, group := range cases {
		assert.Equal(t, types.GroupDefinitionTypeDimension, group.Type)
		assert.Equal(t, string(dimension), *group.Key)
	}
}

func TestGroupByCostCategory(t *testing.T) {
	group := GroupByCostCategory("Team")
	assert.Equal(t, types.GroupDefinitionTypeCostCategory, group.Type)
	assert.Equal(t, "Team", *group.Key)

	groups := GroupByCostCategories("Team", "Stage")
	assert.Len(t, groups, 2)
	assert.Equal(t, types.GroupDefinitionTypeCostCategory, groups[1].Type)
}
