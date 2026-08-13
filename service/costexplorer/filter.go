package costexplorer

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/gocollection/slice"
)

// -----------------------------------------------------------------------------
// Filter rows
// -----------------------------------------------------------------------------

// Filter is one row of the Cost Explorer console filter panel: a dimension, tag
// or cost category, the values it should match, the operator used to compare
// them and whether the row includes or excludes what it matches.
//
// Rows collected on a CostQuery are AND-ed together, while the values inside a
// row are OR-ed by the API — the same semantics the console applies.
type Filter interface {
	// Expression renders the row as a Cost Explorer Expression, or nil when the
	// row carries nothing to filter on.
	Expression() *types.Expression

	// validateForCostAndUsage reports whether the row is accepted by
	// GetCostAndUsage, which supports fewer match options than Expression allows.
	validateForCostAndUsage() error
}

// DimensionFilter filters on one of the Cost Explorer dimensions (SERVICE,
// LINKED_ACCOUNT, REGION, ...).
type DimensionFilter struct {
	Dimension    types.Dimension
	Values       []string
	MatchOptions []types.MatchOption
	Exclude      bool
}

// TagFilter filters on a cost allocation tag key.
type TagFilter struct {
	Key          string
	Values       []string
	MatchOptions []types.MatchOption
	Exclude      bool
}

// CostCategoryFilter filters on a cost category.
type CostCategoryFilter struct {
	Key          string
	Values       []string
	MatchOptions []types.MatchOption
	Exclude      bool
}

// ByDimension starts a dimension filter row. Values are OR-ed by the API.
func ByDimension(dimension types.Dimension, values ...string) DimensionFilter {
	return DimensionFilter{Dimension: dimension, Values: values}
}

// ByTag starts a cost allocation tag filter row. Values are OR-ed by the API.
func ByTag(key string, values ...string) TagFilter {
	return TagFilter{Key: key, Values: values}
}

// ByCostCategory starts a cost category filter row. Values are OR-ed by the API.
func ByCostCategory(key string, values ...string) CostCategoryFilter {
	return CostCategoryFilter{Key: key, Values: values}
}

// Matching sets the operator for the row. The API defaults to EQUALS plus
// CASE_SENSITIVE when none is given.
//
// GetCostAndUsage accepts only EQUALS and CASE_SENSITIVE on dimensions; the
// remaining options (CONTAINS, STARTS_WITH, ENDS_WITH, ...) apply to cost
// category rules, anomaly subscriptions and GetDimensionValues.
func (f DimensionFilter) Matching(options ...types.MatchOption) DimensionFilter {
	f.MatchOptions = options
	return f
}

// Excluded flips the row from include to exclude, wrapping it in a Not — the
// console's "Exclude" toggle.
func (f DimensionFilter) Excluded() DimensionFilter {
	f.Exclude = true
	return f
}

// Expression renders the row, returning nil when no values were supplied.
func (f DimensionFilter) Expression() *types.Expression {
	if len(f.Values) == 0 {
		return nil
	}

	return negate(f.Exclude, &types.Expression{
		Dimensions: &types.DimensionValues{
			Key:          f.Dimension,
			Values:       f.Values,
			MatchOptions: f.MatchOptions,
		},
	})
}

func (f DimensionFilter) validateForCostAndUsage() error {
	return validateMatchOptions(string(f.Dimension), f.MatchOptions, costAndUsageDimensionMatchOptions)
}

// Matching sets the operator for the row. GetCostAndUsage accepts EQUALS,
// ABSENT and CASE_SENSITIVE on tags.
func (f TagFilter) Matching(options ...types.MatchOption) TagFilter {
	f.MatchOptions = options
	return f
}

// Absent turns the row into the console's "no tag key" filter: resources that do
// not carry the tag key at all. Any values already set are dropped, since ABSENT
// matches on the key alone.
func (f TagFilter) Absent() TagFilter {
	f.Values = nil
	f.MatchOptions = []types.MatchOption{types.MatchOptionAbsent}

	return f
}

// Present matches resources carrying the tag key with any value — the inverse of
// Absent, since the API has no positive "key exists" operator.
func (f TagFilter) Present() TagFilter {
	f = f.Absent()
	f.Exclude = true

	return f
}

// Excluded flips the row from include to exclude, wrapping it in a Not.
func (f TagFilter) Excluded() TagFilter {
	f.Exclude = true
	return f
}

// Expression renders the row, returning nil when it has neither values nor an
// operator that matches on the key alone.
func (f TagFilter) Expression() *types.Expression {
	if len(f.Values) == 0 && len(f.MatchOptions) == 0 {
		return nil
	}

	return negate(f.Exclude, &types.Expression{
		Tags: &types.TagValues{
			Key:          aws.String(f.Key),
			Values:       f.Values,
			MatchOptions: f.MatchOptions,
		},
	})
}

func (f TagFilter) validateForCostAndUsage() error {
	return validateMatchOptions("tag "+f.Key, f.MatchOptions, costAndUsageTagMatchOptions)
}

// Matching sets the operator for the row. GetCostAndUsage accepts EQUALS,
// ABSENT and CASE_SENSITIVE on cost categories.
func (f CostCategoryFilter) Matching(options ...types.MatchOption) CostCategoryFilter {
	f.MatchOptions = options
	return f
}

// Absent matches everything that falls outside the cost category.
func (f CostCategoryFilter) Absent() CostCategoryFilter {
	f.Values = nil
	f.MatchOptions = []types.MatchOption{types.MatchOptionAbsent}

	return f
}

// Present matches everything the cost category classifies, whatever the value.
func (f CostCategoryFilter) Present() CostCategoryFilter {
	f = f.Absent()
	f.Exclude = true

	return f
}

// Excluded flips the row from include to exclude, wrapping it in a Not.
func (f CostCategoryFilter) Excluded() CostCategoryFilter {
	f.Exclude = true
	return f
}

// Expression renders the row, returning nil when it has neither values nor an
// operator that matches on the key alone.
func (f CostCategoryFilter) Expression() *types.Expression {
	if len(f.Values) == 0 && len(f.MatchOptions) == 0 {
		return nil
	}

	return negate(f.Exclude, &types.Expression{
		CostCategories: &types.CostCategoryValues{
			Key:          aws.String(f.Key),
			Values:       f.Values,
			MatchOptions: f.MatchOptions,
		},
	})
}

func (f CostCategoryFilter) validateForCostAndUsage() error {
	return validateMatchOptions("cost category "+f.Key, f.MatchOptions, costAndUsageTagMatchOptions)
}

// FiltersExpression AND-s the given rows into a single Expression, skipping rows
// that render to nothing. Returns nil when no row contributes anything.
func FiltersExpression(filters ...Filter) *types.Expression {
	exprs := make([]*types.Expression, 0, len(filters))
	for _, f := range filters {
		exprs = append(exprs, f.Expression())
	}

	return And(exprs...)
}

// GetCostAndUsage supports a narrower set of match options than the Expression
// type as a whole; the remainder belong to cost category rules and anomaly
// subscriptions.
var (
	costAndUsageDimensionMatchOptions = []types.MatchOption{
		types.MatchOptionEquals,
		types.MatchOptionCaseSensitive,
	}

	costAndUsageTagMatchOptions = []types.MatchOption{
		types.MatchOptionEquals,
		types.MatchOptionAbsent,
		types.MatchOptionCaseSensitive,
	}
)

func validateMatchOptions(subject string, options, allowed []types.MatchOption) error {
	for _, opt := range options {
		if !slice.Contains(allowed, opt) {
			return errors.Errorf(
				"match option %q is not supported on %s by GetCostAndUsage (allowed: %v)",
				opt, subject, allowed,
			)
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Expression builders
// -----------------------------------------------------------------------------

// FilterByDimension is the generic dimension filter (SERVICE, USAGE_TYPE, REGION,
// LINKED_ACCOUNT, ...). Returns nil when no values are supplied. Use ByDimension
// when the row also needs an operator or exclusion.
func FilterByDimension(dimension types.Dimension, values ...string) *types.Expression {
	return ByDimension(dimension, values...).Expression()
}

// FilterByService matches the given AWS service names via the SERVICE dimension
// (e.g. "Amazon Elastic Compute Cloud - Compute").
func FilterByService(services ...string) *types.Expression {
	return FilterByDimension(types.DimensionService, services...)
}

// FilterByLinkedAccount matches the given member account IDs.
func FilterByLinkedAccount(accountIDs ...string) *types.Expression {
	return FilterByDimension(types.DimensionLinkedAccount, accountIDs...)
}

// FilterByRegion matches the given regions (e.g. "eu-west-1").
func FilterByRegion(regions ...string) *types.Expression {
	return FilterByDimension(types.DimensionRegion, regions...)
}

// FilterByAvailabilityZone matches the given availability zones.
func FilterByAvailabilityZone(zones ...string) *types.Expression {
	return FilterByDimension(types.DimensionAz, zones...)
}

// FilterByInstanceType matches the given instance types (e.g. "t3.micro").
func FilterByInstanceType(instanceTypes ...string) *types.Expression {
	return FilterByDimension(types.DimensionInstanceType, instanceTypes...)
}

// FilterByInstanceTypeFamily matches the given instance families (e.g. "t3").
func FilterByInstanceTypeFamily(families ...string) *types.Expression {
	return FilterByDimension(types.DimensionInstanceTypeFamily, families...)
}

// FilterByUsageType matches the given USAGE_TYPE values (e.g. "BoxUsage:t3.micro").
func FilterByUsageType(usageTypes ...string) *types.Expression {
	return FilterByDimension(types.DimensionUsageType, usageTypes...)
}

// FilterByUsageTypeGroup matches the given usage type groups (e.g. "EC2: Running Hours").
func FilterByUsageTypeGroup(groups ...string) *types.Expression {
	return FilterByDimension(types.DimensionUsageTypeGroup, groups...)
}

// FilterByChargeType matches the console's "Charge type" (RECORD_TYPE) — e.g.
// "Usage", "Credit", "Refund", "Tax", "Support".
func FilterByChargeType(recordTypes ...string) *types.Expression {
	return FilterByDimension(types.DimensionRecordType, recordTypes...)
}

// FilterByPurchaseType matches the console's "Purchase option" (PURCHASE_TYPE) —
// e.g. "On Demand Instances", "Standard Reserved Instances", "Spot Instances".
func FilterByPurchaseType(purchaseTypes ...string) *types.Expression {
	return FilterByDimension(types.DimensionPurchaseType, purchaseTypes...)
}

// FilterByOperation matches the console's "API operation" (OPERATION).
func FilterByOperation(operations ...string) *types.Expression {
	return FilterByDimension(types.DimensionOperation, operations...)
}

// FilterByResourceID matches individual resource IDs. Only meaningful once
// resource-level data is enabled, and only for GetCostAndUsageWithResources.
func FilterByResourceID(resourceIDs ...string) *types.Expression {
	return FilterByDimension(types.DimensionResourceId, resourceIDs...)
}

// FilterByPlatform matches the given platforms (EC2 operating system, e.g. "Linux/UNIX").
func FilterByPlatform(platforms ...string) *types.Expression {
	return FilterByDimension(types.DimensionPlatform, platforms...)
}

// FilterByOperatingSystem matches the given operating systems.
func FilterByOperatingSystem(operatingSystems ...string) *types.Expression {
	return FilterByDimension(types.DimensionOperatingSystem, operatingSystems...)
}

// FilterByTenancy matches the given tenancies (e.g. "Shared", "Dedicated").
func FilterByTenancy(tenancies ...string) *types.Expression {
	return FilterByDimension(types.DimensionTenancy, tenancies...)
}

// FilterByDatabaseEngine matches the given RDS engines (e.g. "PostgreSQL").
func FilterByDatabaseEngine(engines ...string) *types.Expression {
	return FilterByDimension(types.DimensionDatabaseEngine, engines...)
}

// FilterByDeploymentOption matches the given RDS deployment options
// (e.g. "Single-AZ", "Multi-AZ").
func FilterByDeploymentOption(options ...string) *types.Expression {
	return FilterByDimension(types.DimensionDeploymentOption, options...)
}

// FilterByCacheEngine matches the given ElastiCache engines (e.g. "Redis").
func FilterByCacheEngine(engines ...string) *types.Expression {
	return FilterByDimension(types.DimensionCacheEngine, engines...)
}

// FilterByBillingEntity matches the given billing entities (e.g. "AWS",
// "AWS Marketplace").
func FilterByBillingEntity(entities ...string) *types.Expression {
	return FilterByDimension(types.DimensionBillingEntity, entities...)
}

// FilterByLegalEntity matches the given legal entity names (the AWS seller of record).
func FilterByLegalEntity(entities ...string) *types.Expression {
	return FilterByDimension(types.DimensionLegalEntityName, entities...)
}

// FilterByInvoicingEntity matches the given invoicing entities.
func FilterByInvoicingEntity(entities ...string) *types.Expression {
	return FilterByDimension(types.DimensionInvoicingEntity, entities...)
}

// FilterByScope matches the given reservation scopes (e.g. "Region", "Availability Zone").
func FilterByScope(scopes ...string) *types.Expression {
	return FilterByDimension(types.DimensionScope, scopes...)
}

// FilterByPaymentOption matches the given commitment payment options
// (e.g. "All Upfront", "No Upfront").
func FilterByPaymentOption(paymentOptions ...string) *types.Expression {
	return FilterByDimension(types.DimensionPaymentOption, paymentOptions...)
}

// FilterBySavingsPlansType matches the given Savings Plans types
// (e.g. "Compute Savings Plans").
func FilterBySavingsPlansType(planTypes ...string) *types.Expression {
	return FilterByDimension(types.DimensionSavingsPlansType, planTypes...)
}

// FilterBySavingsPlanArn matches the given Savings Plan ARNs.
func FilterBySavingsPlanArn(arns ...string) *types.Expression {
	return FilterByDimension(types.DimensionSavingsPlanArn, arns...)
}

// FilterByReservationID matches the given reservation IDs.
func FilterByReservationID(reservationIDs ...string) *types.Expression {
	return FilterByDimension(types.DimensionReservationId, reservationIDs...)
}

// FilterBySubscriptionID matches the given subscription IDs.
func FilterBySubscriptionID(subscriptionIDs ...string) *types.Expression {
	return FilterByDimension(types.DimensionSubscriptionId, subscriptionIDs...)
}

// FilterByTag matches resources carrying the given tag key with any of the
// provided values (values are OR-ed by the API). Supplying no values yields nil —
// use FilterByTagAbsent for the console's "no tag key" filter, or list the values
// explicitly to match a key regardless of value.
func FilterByTag(key string, values ...string) *types.Expression {
	return ByTag(key, values...).Expression()
}

// FilterByTagAbsent matches resources that do not carry the given tag key at all
// — the console's "no tag key" filter.
func FilterByTagAbsent(key string) *types.Expression {
	return ByTag(key).Absent().Expression()
}

// FilterByTagPresent matches resources carrying the given tag key with any value.
func FilterByTagPresent(key string) *types.Expression {
	return ByTag(key).Present().Expression()
}

// FilterByTags builds a filter over multiple tag keys. Values within one key are
// OR-ed by the API; the keys are AND-ed together, so a resource must match every
// listed tag key. Returns nil when the map is empty.
func FilterByTags(tags map[string][]string) *types.Expression {
	exprs := make([]*types.Expression, 0, len(tags))
	for key, values := range tags {
		exprs = append(exprs, FilterByTag(key, values...))
	}

	return And(exprs...)
}

// FilterByCostCategory matches the given values of a cost category.
func FilterByCostCategory(key string, values ...string) *types.Expression {
	return ByCostCategory(key, values...).Expression()
}

// FilterByCostCategoryAbsent matches everything that falls outside the given
// cost category.
func FilterByCostCategoryAbsent(key string) *types.Expression {
	return ByCostCategory(key).Absent().Expression()
}

// FilterByCostCategoryPresent matches everything the given cost category
// classifies, whatever the value.
func FilterByCostCategoryPresent(key string) *types.Expression {
	return ByCostCategory(key).Present().Expression()
}

// FilterByCostCategories builds a filter over multiple cost categories. Values
// within one category are OR-ed by the API; the categories are AND-ed together.
// Returns nil when the map is empty.
func FilterByCostCategories(categories map[string][]string) *types.Expression {
	exprs := make([]*types.Expression, 0, len(categories))
	for key, values := range categories {
		exprs = append(exprs, FilterByCostCategory(key, values...))
	}

	return And(exprs...)
}

// -----------------------------------------------------------------------------
// Logical operators
// -----------------------------------------------------------------------------

// And combines expressions with a logical AND, skipping nil entries. It returns
// nil when nothing remains, and the sole expression unwrapped when only one
// remains — the Cost Explorer API rejects And/Or nodes with a single child.
func And(exprs ...*types.Expression) *types.Expression {
	return combine(true, exprs)
}

// Or combines expressions with a logical OR, with the same nil/single-child
// handling as And.
func Or(exprs ...*types.Expression) *types.Expression {
	return combine(false, exprs)
}

// Not negates an expression — the console's "Exclude" toggle. Returns nil for a
// nil input, so it composes with the builders that return nil when empty.
func Not(expr *types.Expression) *types.Expression {
	if expr == nil {
		return nil
	}

	return &types.Expression{Not: expr}
}

func negate(exclude bool, expr *types.Expression) *types.Expression {
	if !exclude {
		return expr
	}

	return Not(expr)
}

func combine(and bool, exprs []*types.Expression) *types.Expression {
	children := make([]types.Expression, 0, len(exprs))
	for _, e := range exprs {
		if e != nil {
			children = append(children, *e)
		}
	}

	switch len(children) {
	case 0:
		return nil
	case 1:
		return &children[0]
	default:
		if and {
			return &types.Expression{And: children}
		}

		return &types.Expression{Or: children}
	}
}

// -----------------------------------------------------------------------------
// GroupBy builders
// -----------------------------------------------------------------------------

// MaxGroupBy is the number of groupings GetCostAndUsage accepts per request.
const MaxGroupBy = 2

// GroupByDimension groups results by an arbitrary dimension. Together with
// GroupByTag and GroupByCostCategory this covers every entry in the console's
// "Group by" selector.
func GroupByDimension(dimension types.Dimension) types.GroupDefinition {
	return types.GroupDefinition{
		Type: types.GroupDefinitionTypeDimension,
		Key:  aws.String(string(dimension)),
	}
}

// GroupByService groups results by the SERVICE dimension.
func GroupByService() types.GroupDefinition {
	return GroupByDimension(types.DimensionService)
}

// GroupByLinkedAccount groups results by member account.
func GroupByLinkedAccount() types.GroupDefinition {
	return GroupByDimension(types.DimensionLinkedAccount)
}

// GroupByRegion groups results by region.
func GroupByRegion() types.GroupDefinition {
	return GroupByDimension(types.DimensionRegion)
}

// GroupByInstanceType groups results by instance type.
func GroupByInstanceType() types.GroupDefinition {
	return GroupByDimension(types.DimensionInstanceType)
}

// GroupByUsageType groups results by the USAGE_TYPE dimension (service usage type).
func GroupByUsageType() types.GroupDefinition {
	return GroupByDimension(types.DimensionUsageType)
}

// GroupByChargeType groups results by the console's "Charge type" (RECORD_TYPE).
func GroupByChargeType() types.GroupDefinition {
	return GroupByDimension(types.DimensionRecordType)
}

// GroupByPurchaseType groups results by the console's "Purchase option".
func GroupByPurchaseType() types.GroupDefinition {
	return GroupByDimension(types.DimensionPurchaseType)
}

// GroupByOperation groups results by API operation.
func GroupByOperation() types.GroupDefinition {
	return GroupByDimension(types.DimensionOperation)
}

// GroupByTag groups results by the given cost allocation tag key.
func GroupByTag(key string) types.GroupDefinition {
	return types.GroupDefinition{
		Type: types.GroupDefinitionTypeTag,
		Key:  aws.String(key),
	}
}

// GroupByTags builds a GroupBy list from several tag keys. Note that
// GetCostAndUsage accepts at most MaxGroupBy groupings per request across all
// group types combined.
func GroupByTags(keys ...string) []types.GroupDefinition {
	groups := make([]types.GroupDefinition, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, GroupByTag(k))
	}

	return groups
}

// GroupByCostCategory groups results by the given cost category.
func GroupByCostCategory(key string) types.GroupDefinition {
	return types.GroupDefinition{
		Type: types.GroupDefinitionTypeCostCategory,
		Key:  aws.String(key),
	}
}

// GroupByCostCategories builds a GroupBy list from several cost categories,
// subject to the same MaxGroupBy limit as GroupByTags.
func GroupByCostCategories(keys ...string) []types.GroupDefinition {
	groups := make([]types.GroupDefinition, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, GroupByCostCategory(k))
	}

	return groups
}
