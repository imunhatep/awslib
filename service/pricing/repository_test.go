package pricing

import (
	"testing"

	ptypes "github.com/imunhatep/awslib/provider/types"
)

func ec2ProductWith(sku, price string, attributes Attributes) Ec2Product {
	attributes.InstanceType = "p5.48xlarge"

	product := Ec2Product{
		Product: Product{
			ProductFamily: "Compute Instance",
			SKU:           sku,
			Attributes:    attributes,
		},
	}

	if price != "" {
		product.Terms = Terms{
			OnDemand: map[string]Term{
				"term": {
					PriceDimensions: map[string]PriceDimension{
						"dim": {PricePerUnit: PricePerUnit{USD: price}},
					},
				},
			},
		}
	}

	return product
}

// A row with a price beats one without, whichever order the pages arrive in: the
// caller wants the on-demand rate, and a row with no OnDemand term cannot supply one.
func TestPickEc2ProductPrefersPricedRow(t *testing.T) {
	priced := ec2ProductWith("ZZZZZZZZ", "98.32", Attributes{MarketOption: "OnDemand"})
	unpriced := ec2ProductWith("AAAAAAAA", "", Attributes{MarketOption: "CapacityBlock"})

	region := ptypes.AwsRegion("us-east-2")

	// the unpriced row sorts first by SKU, so this also proves price beats the tie-break
	if got := pickEc2Product(region, "p5.48xlarge", unpriced, priced); got.Product.SKU != "ZZZZZZZZ" {
		t.Errorf("candidate priced: kept SKU %q, want ZZZZZZZZ", got.Product.SKU)
	}

	if got := pickEc2Product(region, "p5.48xlarge", priced, unpriced); got.Product.SKU != "ZZZZZZZZ" {
		t.Errorf("existing priced: kept SKU %q, want ZZZZZZZZ", got.Product.SKU)
	}
}

// awslib answers a missing on-demand term with the string "N/A", so a row carrying
// that is unpriced and must not beat a real rate.
func TestPickEc2ProductTreatsNAAsUnpriced(t *testing.T) {
	withRate := ec2ProductWith("BBBBBBBB", "98.32", Attributes{})
	notAvailable := ec2ProductWith("AAAAAAAA", "N/A", Attributes{})

	got := pickEc2Product(ptypes.AwsRegion("us-east-2"), "p5.48xlarge", notAvailable, withRate)
	if got.Product.SKU != "BBBBBBBB" {
		t.Errorf("kept SKU %q, want BBBBBBBB", got.Product.SKU)
	}
}

// Two priced rows are decided by SKU, not arrival order — the reported price must not
// change between runs because the pages came back differently.
func TestPickEc2ProductIsOrderIndependent(t *testing.T) {
	first := ec2ProductWith("AAAAAAAA", "98.32", Attributes{UsageType: "BoxUsage:p5.48xlarge"})
	second := ec2ProductWith("BBBBBBBB", "55.00", Attributes{UsageType: "UnusedDed:p5.48xlarge"})

	region := ptypes.AwsRegion("us-east-2")

	forward := pickEc2Product(region, "p5.48xlarge", first, second)
	backward := pickEc2Product(region, "p5.48xlarge", second, first)

	if forward.Product.SKU != backward.Product.SKU {
		t.Fatalf("order changed the winner: %q vs %q", forward.Product.SKU, backward.Product.SKU)
	}

	if forward.Product.SKU != "AAAAAAAA" {
		t.Errorf("kept SKU %q, want AAAAAAAA (lowest)", forward.Product.SKU)
	}
}

// The discriminators are the attributes ec2ProductFilters leaves free. If a filtered
// attribute crept into the set the warning would report a difference that cannot
// happen, and a free one missing from it is a cause the log can never name.
func TestEc2ProductDiscriminatorsExcludeFilteredAttributes(t *testing.T) {
	discriminators := ec2ProductDiscriminators(ec2ProductWith("AAAAAAAA", "98.32", Attributes{}))

	filtered := []string{"regionCode", "operatingSystem", "preInstalledSw", "tenancy", "capacitystatus"}
	for _, attribute := range filtered {
		if _, ok := discriminators[attribute]; ok {
			t.Errorf("%q is pinned by ec2ProductFilters and must not be reported as a difference", attribute)
		}
	}

	for _, attribute := range []string{"sku", "marketoption", "usagetype", "operation", "availabilityzone"} {
		if _, ok := discriminators[attribute]; !ok {
			t.Errorf("%q is left free by ec2ProductFilters and must be reportable", attribute)
		}
	}
}
