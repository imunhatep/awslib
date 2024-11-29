package pricing

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/go-errors/errors"
)

type Ec2Product struct {
	Product         Product `json:"product"`
	ServiceCode     string  `json:"serviceCode"`
	Terms           Terms   `json:"terms"`
	Version         string  `json:"version"`
	PublicationDate string  `json:"publicationDate"`
}

func NewEc2Product(priceList string) (*Ec2Product, error) {
	ec2pricing := &Ec2Product{}
	err := json.Unmarshal([]byte(priceList), &ec2pricing)
	if err != nil {
		return &Ec2Product{}, errors.New(err)
	}

	return ec2pricing, nil
}

func (e Ec2Product) GetProductFamily() string {
	return e.Product.ProductFamily
}

func (e Ec2Product) GetInstanceType() string {
	return e.Product.Attributes.InstanceType
}

func (e Ec2Product) GetInstanceVcpu() string {
	return e.Product.Attributes.Vcpu
}

func (e Ec2Product) GetInstanceMemory() string {
	return e.Product.Attributes.Memory
}

func (e Ec2Product) GetTerms() Terms {
	return e.Terms
}

func (e Ec2Product) GetConvertible1yrNoUpfrontPrice() string {
	return e.GetReservedPrice(types.OfferingClassTypeConvertible, types.OfferingTypeValuesNoUpfront, "1yr")
}

func (e Ec2Product) GetStandard1yrNoUpfrontPrice() string {
	return e.GetReservedPrice(types.OfferingClassTypeStandard, types.OfferingTypeValuesNoUpfront, "1yr")
}

func (e Ec2Product) GetOnDemandPrice() string {
	onDemand := e.GetTerms().OnDemand
	for _, term := range onDemand {
		for _, priceDimension := range term.PriceDimensions {
			return priceDimension.PricePerUnit.USD
		}
	}

	return "N/A"
}

func (e Ec2Product) GetReservedPrice(
	offeringClass types.OfferingClassType,
	offeringType types.OfferingTypeValues,
	leaseContractLength string,
) string {
	reserved := e.GetTerms().Reserved
	for _, term := range reserved {
		if term.TermAttributes.OfferingClass != offeringClass {
			continue
		}

		if term.TermAttributes.PurchaseOption != offeringType {
			continue
		}

		if term.TermAttributes.LeaseContractLength != leaseContractLength {
			continue
		}

		for _, priceDimension := range term.PriceDimensions {
			return priceDimension.PricePerUnit.USD
		}
	}

	return "N/A"
}
