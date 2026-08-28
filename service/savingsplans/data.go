package savingsplans

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	awssp "github.com/aws/aws-sdk-go-v2/service/savingsplans"
	"github.com/aws/aws-sdk-go-v2/service/savingsplans/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
)

// The two commitment lengths Savings Plans are sold at, in seconds. A rate is only
// meaningful together with its term — the same instance type has a different rate for
// each — and the term is not a request filter, so callers match it on the results.
const (
	Term1yrSeconds = int64(31536000)
	Term3yrSeconds = int64(94608000)
)

// Offering-rate property names. AWS returns what a rate applies to as untyped
// name/value pairs rather than typed fields, so these names are the only way in.
const (
	PropertyRegion             = "region"
	PropertyInstanceType       = "instanceType"
	PropertyInstanceFamily     = "instanceFamily"
	PropertyProductDescription = "productDescription"
	PropertyTenancy            = "tenancy"
)

// DefaultOfferingRatePageSize is the largest page DescribeSavingsPlansOfferingRates
// accepts. A region holds thousands of rates, so anything smaller only multiplies the
// number of round trips.
const DefaultOfferingRatePageSize = int32(1000)

// OfferingRatesQuery describes a savings plan rate lookup in higher-level terms than
// the raw SDK input.
//
// Region is a filter, never an endpoint: the Savings Plans API is partition-global
// (savingsplans.amazonaws.com), so every client reaches the same endpoint and the
// region of the client that happens to make the call says nothing about the rates
// being asked for. Leaving Region empty asks for every region.
type OfferingRatesQuery struct {
	Region ptypes.AwsRegion

	// PlanTypes selects Compute vs EC2Instance (etc.) plans; PaymentOptions selects
	// No/Partial/All Upfront. Both are applied by the API.
	PlanTypes      []types.SavingsPlanType
	PaymentOptions []types.SavingsPlanPaymentOption

	// Products and ServiceCodes scope the rates to one AWS service — EC2 rates come
	// from Products{EC2} and ServiceCodes{AmazonEC2}.
	Products     []types.SavingsPlanProductType
	ServiceCodes []types.SavingsPlanRateServiceCode

	// Operations and UsageTypes are the billing-report identifiers of a line item,
	// e.g. operation "RunInstances" for a Linux EC2 instance.
	Operations []string
	UsageTypes []string

	// The remaining fields map onto the API's filter elements.
	InstanceTypes       []string
	InstanceFamilies    []string
	ProductDescriptions []string
	Tenancies           []string

	// DurationSeconds keeps only rates committed for that many seconds (Term1yrSeconds,
	// Term3yrSeconds). Unlike every other field this one is applied client-side: the
	// term is carried by the parent offering of each result and the API offers no
	// filter for it. Zero keeps every term.
	DurationSeconds int64

	// PageSize overrides DefaultOfferingRatePageSize. Non-positive values are ignored.
	PageSize int32
}

// ToInput renders the query as the SDK input, omitting every filter the caller left
// empty — an empty filter element is rejected by the API rather than ignored.
func (q OfferingRatesQuery) ToInput() *awssp.DescribeSavingsPlansOfferingRatesInput {
	query := &awssp.DescribeSavingsPlansOfferingRatesInput{
		Products:                  q.Products,
		ServiceCodes:              q.ServiceCodes,
		SavingsPlanTypes:          q.PlanTypes,
		SavingsPlanPaymentOptions: q.PaymentOptions,
		Operations:                q.Operations,
		UsageTypes:                q.UsageTypes,
		MaxResults:                q.pageSize(),
	}

	filters := []types.SavingsPlanOfferingRateFilterElement{}

	if q.Region != "" {
		filters = append(filters, filterElement(types.SavingsPlanRateFilterAttributeRegion, []string{q.Region.String()}))
	}

	filters = append(filters,
		optionalFilter(types.SavingsPlanRateFilterAttributeInstanceType, q.InstanceTypes)...,
	)
	filters = append(filters,
		optionalFilter(types.SavingsPlanRateFilterAttributeInstanceFamily, q.InstanceFamilies)...,
	)
	filters = append(filters,
		optionalFilter(types.SavingsPlanRateFilterAttributeProductDescription, q.ProductDescriptions)...,
	)
	filters = append(filters,
		optionalFilter(types.SavingsPlanRateFilterAttributeTenancy, q.Tenancies)...,
	)

	if len(filters) > 0 {
		query.Filters = filters
	}

	return query
}

func (q OfferingRatesQuery) pageSize() int32 {
	if q.PageSize > 0 {
		return q.PageSize
	}

	return DefaultOfferingRatePageSize
}

// keepsTerm reports whether a rate committed for durationSeconds is wanted. A query
// that names no term keeps every one.
func (q OfferingRatesQuery) keepsTerm(durationSeconds int64) bool {
	return q.DurationSeconds == 0 || q.DurationSeconds == durationSeconds
}

func optionalFilter(name types.SavingsPlanRateFilterAttribute, values []string) []types.SavingsPlanOfferingRateFilterElement {
	if len(values) == 0 {
		return nil
	}

	return []types.SavingsPlanOfferingRateFilterElement{filterElement(name, values)}
}

func filterElement(name types.SavingsPlanRateFilterAttribute, values []string) types.SavingsPlanOfferingRateFilterElement {
	return types.SavingsPlanOfferingRateFilterElement{Name: name, Values: values}
}

// OfferingRate is a savings plan rate on offer for one product in one region.
//
// The SDK type is embedded rather than copied so nothing is lost, and kept exported
// so the value survives the gob round trip the cache performs. The accessors exist
// because everything worth reading off a rate — which instance type it applies to,
// how long the commitment runs — is either an untyped property or lives on the parent
// offering behind a pointer.
type OfferingRate struct {
	types.SavingsPlanOfferingRate
}

func NewOfferingRate(rate types.SavingsPlanOfferingRate) OfferingRate {
	return OfferingRate{SavingsPlanOfferingRate: rate}
}

// HourlyRate is the discounted rate in the offering's currency, "" when absent.
func (r OfferingRate) HourlyRate() string {
	return aws.ToString(r.SavingsPlanOfferingRate.Rate)
}

// DurationSeconds is the commitment length the rate is offered at, 0 when the parent
// offering is absent.
func (r OfferingRate) DurationSeconds() int64 {
	if r.SavingsPlanOffering == nil {
		return 0
	}

	return r.SavingsPlanOffering.DurationSeconds
}

func (r OfferingRate) PlanType() types.SavingsPlanType {
	if r.SavingsPlanOffering == nil {
		return ""
	}

	return r.SavingsPlanOffering.PlanType
}

func (r OfferingRate) PaymentOption() types.SavingsPlanPaymentOption {
	if r.SavingsPlanOffering == nil {
		return ""
	}

	return r.SavingsPlanOffering.PaymentOption
}

func (r OfferingRate) Currency() types.CurrencyCode {
	if r.SavingsPlanOffering == nil {
		return ""
	}

	return r.SavingsPlanOffering.Currency
}

func (r OfferingRate) InstanceType() string {
	return r.Property(PropertyInstanceType)
}

func (r OfferingRate) InstanceFamily() string {
	return r.Property(PropertyInstanceFamily)
}

func (r OfferingRate) Region() string {
	return r.Property(PropertyRegion)
}

func (r OfferingRate) ProductDescription() string {
	return r.Property(PropertyProductDescription)
}

func (r OfferingRate) Tenancy() string {
	return r.Property(PropertyTenancy)
}

// Property returns the named offering-rate property, "" when the rate does not carry
// it. A rate only carries the properties that apply to its product type, so an absent
// property is ordinary, not an error.
func (r OfferingRate) Property(name string) string {
	for _, property := range r.Properties {
		if aws.ToString(property.Name) == name {
			return aws.ToString(property.Value)
		}
	}

	return ""
}
