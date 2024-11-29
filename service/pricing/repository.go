package pricing

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/go-errors/errors"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	Pricing() *pricing.Client
}

type PricingRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewPricingRepository(ctx context.Context, client AwsClient) *PricingRepository {
	repo := &PricingRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
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

// GetInstancePricing fetches the pricing for a given instance type using the AWS Pricing API.
func (r *PricingRepository) GetInstancePricing(region ptypes.AwsRegion, instanceType ec2types.InstanceType) (*Ec2Product, error) {

	// Define the pricing filter for EC2 On-Demand instances in a specific region
	query := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []types.Filter{
			{
				Type:  types.FilterType("TERM_MATCH"),
				Field: aws.String("instanceType"),
				Value: aws.String(string(instanceType)),
			},
			{
				Type:  types.FilterType("TERM_MATCH"),
				Field: aws.String("regionCode"),
				Value: aws.String(region.String()),
			},
			{
				Type:  types.FilterType("TERM_MATCH"),
				Field: aws.String("operatingSystem"),
				Value: aws.String("Linux"), // Assuming Linux instances, change as needed
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
		},
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

// GetInstancePricingByInput fetches the pricing for a given instance type using the AWS Pricing API.
func (r *PricingRepository) GetInstancePricingByInput(query *pricing.GetProductsInput) ([]string, error) {
	// Fetch the products (pricing details)
	output, err := r.client.Pricing().GetProducts(r.ctx, query)
	if err != nil {
		return []string{}, errors.New(err)
	}

	// Extract pricing details from the JSON response
	return output.PriceList, nil
}
