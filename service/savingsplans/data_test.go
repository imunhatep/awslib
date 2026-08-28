package savingsplans

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/savingsplans/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
)

func filterValues(t *testing.T, filters []types.SavingsPlanOfferingRateFilterElement, name types.SavingsPlanRateFilterAttribute) []string {
	t.Helper()

	for _, filter := range filters {
		if filter.Name == name {
			return filter.Values
		}
	}

	return nil
}

func TestOfferingRatesQueryToInput(t *testing.T) {
	query := OfferingRatesQuery{
		Region:              ptypes.AwsRegion("eu-central-1"),
		PlanTypes:           []types.SavingsPlanType{types.SavingsPlanTypeCompute},
		PaymentOptions:      []types.SavingsPlanPaymentOption{types.SavingsPlanPaymentOptionNoUpfront},
		Products:            []types.SavingsPlanProductType{types.SavingsPlanProductTypeEc2},
		ServiceCodes:        []types.SavingsPlanRateServiceCode{types.SavingsPlanRateServiceCodeEc2},
		ProductDescriptions: []string{"Linux/UNIX"},
		Tenancies:           []string{"shared"},
		InstanceTypes:       []string{"m5.large", "c5.xlarge"},
		DurationSeconds:     Term1yrSeconds,
	}

	input := query.ToInput()

	if got := filterValues(t, input.Filters, types.SavingsPlanRateFilterAttributeRegion); len(got) != 1 || got[0] != "eu-central-1" {
		t.Errorf("region filter = %v, want [eu-central-1]", got)
	}

	if got := filterValues(t, input.Filters, types.SavingsPlanRateFilterAttributeInstanceType); len(got) != 2 {
		t.Errorf("instance type filter = %v, want two values", got)
	}

	if got := filterValues(t, input.Filters, types.SavingsPlanRateFilterAttributeTenancy); len(got) != 1 || got[0] != "shared" {
		t.Errorf("tenancy filter = %v, want [shared]", got)
	}

	if len(input.SavingsPlanTypes) != 1 || input.SavingsPlanTypes[0] != types.SavingsPlanTypeCompute {
		t.Errorf("plan types = %v, want [Compute]", input.SavingsPlanTypes)
	}

	if len(input.SavingsPlanPaymentOptions) != 1 || input.SavingsPlanPaymentOptions[0] != types.SavingsPlanPaymentOptionNoUpfront {
		t.Errorf("payment options = %v, want [No Upfront]", input.SavingsPlanPaymentOptions)
	}

	if input.MaxResults != DefaultOfferingRatePageSize {
		t.Errorf("page size = %d, want the default %d", input.MaxResults, DefaultOfferingRatePageSize)
	}

	// The commitment length has no API filter — it must not leak into the request.
	if got := filterValues(t, input.Filters, types.SavingsPlanRateFilterAttribute("duration")); got != nil {
		t.Errorf("duration filter = %v, want none (filtered client-side)", got)
	}
}

// An empty filter element is rejected by the API rather than ignored, so a query that
// names nothing must send no filters at all.
func TestOfferingRatesQueryOmitsEmptyFilters(t *testing.T) {
	input := OfferingRatesQuery{}.ToInput()

	if len(input.Filters) != 0 {
		t.Errorf("filters = %v, want none", input.Filters)
	}
}

func TestOfferingRatesQueryPageSizeOverride(t *testing.T) {
	if got := (OfferingRatesQuery{PageSize: 250}).ToInput().MaxResults; got != 250 {
		t.Errorf("page size = %d, want 250", got)
	}

	if got := (OfferingRatesQuery{PageSize: -1}).ToInput().MaxResults; got != DefaultOfferingRatePageSize {
		t.Errorf("page size for a negative override = %d, want the default %d", got, DefaultOfferingRatePageSize)
	}
}

func TestOfferingRatesQueryKeepsTerm(t *testing.T) {
	oneYear := OfferingRatesQuery{DurationSeconds: Term1yrSeconds}
	if !oneYear.keepsTerm(Term1yrSeconds) {
		t.Error("a 1yr query must keep 1yr rates")
	}

	if oneYear.keepsTerm(Term3yrSeconds) {
		t.Error("a 1yr query must drop 3yr rates")
	}

	// a query naming no term keeps every one, including a rate with no parent offering
	if !(OfferingRatesQuery{}).keepsTerm(0) || !(OfferingRatesQuery{}).keepsTerm(Term3yrSeconds) {
		t.Error("a query with no term must keep every rate")
	}
}

func TestOfferingRateAccessors(t *testing.T) {
	rate := NewOfferingRate(types.SavingsPlanOfferingRate{
		Rate: aws.String("0.0680"),
		Properties: []types.SavingsPlanOfferingRateProperty{
			{Name: aws.String(PropertyRegion), Value: aws.String("eu-central-1")},
			{Name: aws.String(PropertyInstanceType), Value: aws.String("m5.large")},
			{Name: aws.String(PropertyInstanceFamily), Value: aws.String("m5")},
			{Name: aws.String(PropertyTenancy), Value: aws.String("shared")},
		},
		SavingsPlanOffering: &types.ParentSavingsPlanOffering{
			DurationSeconds: Term1yrSeconds,
			PlanType:        types.SavingsPlanTypeCompute,
			PaymentOption:   types.SavingsPlanPaymentOptionNoUpfront,
			Currency:        types.CurrencyCodeUsd,
		},
	})

	if rate.HourlyRate() != "0.0680" {
		t.Errorf("hourly rate = %q, want 0.0680", rate.HourlyRate())
	}

	if rate.InstanceType() != "m5.large" || rate.InstanceFamily() != "m5" {
		t.Errorf("instance type/family = %q/%q, want m5.large/m5", rate.InstanceType(), rate.InstanceFamily())
	}

	if rate.Region() != "eu-central-1" || rate.Tenancy() != "shared" {
		t.Errorf("region/tenancy = %q/%q, want eu-central-1/shared", rate.Region(), rate.Tenancy())
	}

	if rate.DurationSeconds() != Term1yrSeconds {
		t.Errorf("duration = %d, want %d", rate.DurationSeconds(), Term1yrSeconds)
	}

	if rate.PlanType() != types.SavingsPlanTypeCompute || rate.PaymentOption() != types.SavingsPlanPaymentOptionNoUpfront {
		t.Errorf("plan type/payment = %q/%q", rate.PlanType(), rate.PaymentOption())
	}

	if rate.Currency() != types.CurrencyCodeUsd {
		t.Errorf("currency = %q, want USD", rate.Currency())
	}

	// a property the rate does not carry is ordinary, not an error
	if rate.ProductDescription() != "" {
		t.Errorf("missing property = %q, want empty", rate.ProductDescription())
	}
}

// Every accessor reads through a pointer AWS may leave nil, so a bare rate must be
// readable rather than panic.
func TestOfferingRateWithoutParentOffering(t *testing.T) {
	rate := OfferingRate{}

	if rate.DurationSeconds() != 0 || rate.PlanType() != "" || rate.PaymentOption() != "" || rate.Currency() != "" {
		t.Errorf("empty rate = %+v, want zero values", rate)
	}

	if rate.HourlyRate() != "" || rate.InstanceType() != "" {
		t.Errorf("empty rate rate/type = %q/%q, want empty", rate.HourlyRate(), rate.InstanceType())
	}
}
