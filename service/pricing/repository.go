package pricing

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awspricing "github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/pricing"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type PricingRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewPricingRepository(ctx context.Context, client *v3.Client) *PricingRepository {
	repo := &PricingRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *PricingRepository) pricingClient() *awspricing.Client {
	return pricing.GetClient(r.client)
}

func (r *PricingRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *PricingRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

// GetProducts filters shared by the pricing lookups.
//
// The set narrows an EC2 product to one comparable configuration: Linux, no
// pre-installed software, shared tenancy, and capacitystatus Used, which is the
// ordinary on-demand row rather than a capacity reservation.
func ec2ProductFilters(region ptypes.AwsRegion) []types.Filter {
	return []types.Filter{
		{
			Type:  types.FilterType("TERM_MATCH"),
			Field: aws.String("regionCode"),
			Value: aws.String(region.String()),
		},
		{
			Type:  types.FilterType("TERM_MATCH"),
			Field: aws.String("operatingSystem"),
			Value: aws.String("Linux"),
		},
		{
			Type:  types.FilterType("TERM_MATCH"),
			Field: aws.String("preInstalledSw"),
			Value: aws.String("NA"),
		},
		{
			Type:  types.FilterType("TERM_MATCH"),
			Field: aws.String("tenancy"),
			Value: aws.String("Shared"),
		},
		{
			Type:  types.FilterType("TERM_MATCH"),
			Field: aws.String("capacitystatus"),
			Value: aws.String("Used"),
		},
	}
}

// GetInstancePricingByRegion returns the on-demand products of every instance type in
// one region, keyed by instance type.
//
// This is the call to reach for when more than one instance type is wanted, and it is
// the reason GetInstancePricing should not be used in a loop. GetProducts filtered by
// region alone returns the whole region in a handful of pages, so pricing a region
// costs single-digit requests; asking per instance type costs one request each, which
// against a region's ~850 types is ~850 requests to learn the same thing. The Pricing
// API rate-limits per account and answers the difference with ThrottlingException.
//
// A product whose JSON does not parse, or that carries no instance type, is skipped
// with a warning rather than failing the region: one malformed row must not cost the
// caller every other price.
//
// A duplicate instance type is resolved by pickEc2Product rather than by arrival
// order: the filters above should leave one row per type, and where they do not, which
// row wins must not depend on the order the pages came back in.
func (r *PricingRepository) GetInstancePricingByRegion(region ptypes.AwsRegion) (map[string]Ec2Product, error) {
	query := &awspricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters:     ec2ProductFilters(region),
	}

	priceList, err := r.GetInstancePricingByInput(query)
	if err != nil {
		return nil, errors.New(err)
	}

	products := make(map[string]Ec2Product, len(priceList))
	for _, priceItem := range priceList {
		product, err := NewEc2Product(priceItem)
		if err != nil {
			log.Warn().Err(err).
				Str("region", region.String()).
				Msg("[PricingRepository.GetInstancePricingByRegion] skipping unparseable pricing item")

			continue
		}

		instanceType := product.GetInstanceType()
		if instanceType == "" {
			log.Warn().
				Str("region", region.String()).
				Msg("[PricingRepository.GetInstancePricingByRegion] skipping pricing item with no instance type")

			continue
		}

		if existing, ok := products[instanceType]; ok {
			products[instanceType] = pickEc2Product(region, instanceType, existing, *product)

			continue
		}

		products[instanceType] = *product
	}

	log.Debug().
		Str("region", region.String()).
		Int("instanceTypes", len(products)).
		Msg("[PricingRepository.GetInstancePricingByRegion] region priced")

	return products, nil
}

// ec2ProductDiscriminators are the product attributes ec2ProductFilters does *not*
// pin down, which is where the difference between two rows for the same instance type
// has to live.
//
// Filtering on regionCode, operatingSystem, preInstalledSw, tenancy and capacitystatus
// is meant to select exactly one row per instance type. When it does not, one of these
// is why, and there is no way to tell which from outside a live response — so the
// warning prints the ones that actually differ instead of naming a suspect. Adding a
// filter for whichever attribute the logs finger is then a one-line change backed by
// evidence rather than a guess about how AWS models an instance family.
func ec2ProductDiscriminators(product Ec2Product) map[string]string {
	attributes := product.Product.Attributes

	return map[string]string{
		"sku":              product.Product.SKU,
		"productFamily":    product.GetProductFamily(),
		"marketoption":     attributes.MarketOption,
		"usagetype":        attributes.UsageType,
		"operation":        attributes.Operation,
		"availabilityzone": attributes.AvailabilityZone,
		"licenseModel":     attributes.LicenseModel,
		"location":         attributes.Location,
		"locationType":     attributes.LocationType,
		"instanceFamily":   attributes.InstanceFamily,
	}
}

// pickEc2Product decides which of two rows for the same instance type to keep, and
// reports what set them apart.
//
// A row carrying an on-demand price beats one that does not: the caller wants the
// ordinary on-demand rate, and a row with no OnDemand term cannot supply it whatever
// else it describes. Where both (or neither) have a price, the lower SKU wins — an
// arbitrary rule, but a stable one, so the reported price does not change between runs
// with the page order. Two identically-priced rows are not worth a warning.
func pickEc2Product(region ptypes.AwsRegion, instanceType string, existing, candidate Ec2Product) Ec2Product {
	existingPrice, candidatePrice := existing.GetOnDemandPrice(), candidate.GetOnDemandPrice()

	winner, loser := existing, candidate

	switch {
	case hasOnDemandPrice(candidate) && !hasOnDemandPrice(existing):
		winner, loser = candidate, existing
	case hasOnDemandPrice(existing) == hasOnDemandPrice(candidate) &&
		candidate.Product.SKU < existing.Product.SKU:
		winner, loser = candidate, existing
	}

	if existingPrice == candidatePrice {
		return winner
	}

	event := log.Warn().
		Str("region", region.String()).
		Str("instanceType", instanceType).
		Str("keptPrice", winner.GetOnDemandPrice()).
		Str("droppedPrice", loser.GetOnDemandPrice())

	// Only the attributes that actually differ, so the log line names the cause
	// instead of restating the whole row twice.
	keptAttributes, droppedAttributes := ec2ProductDiscriminators(winner), ec2ProductDiscriminators(loser)
	for attribute, keptValue := range keptAttributes {
		droppedValue := droppedAttributes[attribute]
		if keptValue == droppedValue {
			continue
		}

		event = event.Str("kept."+attribute, keptValue).Str("dropped."+attribute, droppedValue)
	}

	event.Msg("[PricingRepository.GetInstancePricingByRegion] duplicate pricing item, filters no longer select a single row")

	return winner
}

func hasOnDemandPrice(product Ec2Product) bool {
	price := product.GetOnDemandPrice()

	return price != "" && price != "N/A"
}

// GetInstancePricing fetches the pricing for one instance type in one region.
//
// One request per call. Use GetInstancePricingByRegion for more than a couple of
// types — see the note there.
func (r *PricingRepository) GetInstancePricing(region ptypes.AwsRegion, instanceType ec2types.InstanceType) (*Ec2Product, error) {
	query := &awspricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: append(ec2ProductFilters(region), types.Filter{
			Type:  types.FilterType("TERM_MATCH"),
			Field: aws.String("instanceType"),
			Value: aws.String(string(instanceType)),
		}),
	}

	priceList, err := r.GetInstancePricingByInput(query)
	if err != nil {
		return nil, errors.New(err)
	}

	if len(priceList) > 1 {
		log.Warn().
			Str("instanceType", string(instanceType)).
			Msgf("[PricingRepository.GetInstancePricing] multiple pricing items found")
	}

	for _, priceItem := range priceList {
		ec2instance, err := NewEc2Product(priceItem)
		if err != nil {
			return nil, errors.New(err)
		}

		return ec2instance, nil
	}

	return nil, nil
}

// GetInstancePricingByInput runs a GetProducts query to completion and returns every
// price-list item it yields.
//
// It paginates. It previously made a single GetProducts call and dropped NextToken,
// which silently truncated any query whose result did not fit one page — invisible
// while every query was filtered down to one instance type, and wrong the moment a
// query covers a whole region.
func (r *PricingRepository) GetInstancePricingByInput(query *awspricing.GetProductsInput) ([]string, error) {
	start := time.Now()
	var priceList []string

	p := awspricing.NewGetProductsPaginator(r.pricingClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetProducts", cfg.ResourceTypeInstance)).Inc()
		}

		output, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetProducts", cfg.ResourceTypeInstance)).Inc()
			}

			return priceList, errors.New(err)
		}

		priceList = append(priceList, output.PriceList...)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetProducts", cfg.ResourceTypeInstance)).
			Add(float64(len(priceList)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetProducts", cfg.ResourceTypeInstance)).
			Observe(time.Since(start).Seconds())
	}

	return priceList, nil
}
