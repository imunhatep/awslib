package pricing

import "github.com/aws/aws-sdk-go-v2/service/ec2/types"

type PricePerUnit struct {
	USD string `json:"USD"`
}

type PriceDimension struct {
	Unit         string       `json:"unit"`
	EndRange     string       `json:"endRange"`
	Description  string       `json:"description"`
	AppliesTo    []string     `json:"appliesTo"`
	RateCode     string       `json:"rateCode"`
	BeginRange   string       `json:"beginRange"`
	PricePerUnit PricePerUnit `json:"pricePerUnit"`
}

type TermAttributes struct {
	LeaseContractLength string                   `json:"LeaseContractLength,omitempty"`
	OfferingClass       types.OfferingClassType  `json:"OfferingClass,omitempty"`
	PurchaseOption      types.OfferingTypeValues `json:"PurchaseOption,omitempty"`
}

type Term struct {
	PriceDimensions map[string]PriceDimension `json:"priceDimensions"`
	SKU             string                    `json:"sku"`
	EffectiveDate   string                    `json:"effectiveDate"`
	OfferTermCode   string                    `json:"offerTermCode"`
	TermAttributes  TermAttributes            `json:"termAttributes"`
}

type Terms struct {
	OnDemand map[string]Term `json:"OnDemand"`
	Reserved map[string]Term `json:"Reserved"`
}

type Attributes struct {
	EnhancedNetworkingSupported string `json:"enhancedNetworkingSupported"`
	IntelTurboAvailable         string `json:"intelTurboAvailable"`
	Memory                      string `json:"memory"`
	DedicatedEbsThroughput      string `json:"dedicatedEbsThroughput"`
	Vcpu                        string `json:"vcpu"`
	ClassicNetworkingSupport    string `json:"classicnetworkingsupport"`
	CapacityStatus              string `json:"capacitystatus"`
	LocationType                string `json:"locationType"`
	Storage                     string `json:"storage"`
	InstanceFamily              string `json:"instanceFamily"`
	OperatingSystem             string `json:"operatingSystem"`
	IntelAvx2Available          string `json:"intelAvx2Available"`
	RegionCode                  string `json:"regionCode"`
	PhysicalProcessor           string `json:"physicalProcessor"`
	ClockSpeed                  string `json:"clockSpeed"`
	Ecu                         string `json:"ecu"`
	NetworkPerformance          string `json:"networkPerformance"`
	ServiceName                 string `json:"servicename"`
	GpuMemory                   string `json:"gpuMemory"`
	VpcNetworkingSupport        string `json:"vpcnetworkingsupport"`
	InstanceType                string `json:"instanceType"`
	Tenancy                     string `json:"tenancy"`
	UsageType                   string `json:"usagetype"`
	NormalizationSizeFactor     string `json:"normalizationSizeFactor"`
	IntelAvxAvailable           string `json:"intelAvxAvailable"`
	ServiceCode                 string `json:"servicecode"`
	LicenseModel                string `json:"licenseModel"`
	CurrentGeneration           string `json:"currentGeneration"`
	PreInstalledSw              string `json:"preInstalledSw"`
	Location                    string `json:"location"`
	ProcessorArchitecture       string `json:"processorArchitecture"`
	MarketOption                string `json:"marketoption"`
	Operation                   string `json:"operation"`
	AvailabilityZone            string `json:"availabilityzone"`
}

type Product struct {
	ProductFamily string     `json:"productFamily"`
	Attributes    Attributes `json:"attributes"`
	SKU           string     `json:"sku"`
}
