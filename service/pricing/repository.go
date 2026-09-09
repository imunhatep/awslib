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
// Later entries win on a duplicate instance type. The filters above should leave one
// row per type, and a duplicate means they no longer do — logged, because the price
// that survives is then arbitrary.
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

		if _, ok := products[instanceType]; ok {
			log.Warn().
				Str("region", region.String()).
				Str("instanceType", instanceType).
				Msg("[PricingRepository.GetInstancePricingByRegion] duplicate pricing item, filters no longer select a single row")
		}

		products[instanceType] = *product
	}

	log.Debug().
		Str("region", region.String()).
		Int("instanceTypes", len(products)).
		Msg("[PricingRepository.GetInstancePricingByRegion] region priced")

	return products, nil
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
