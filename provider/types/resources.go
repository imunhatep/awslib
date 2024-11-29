package types

import "github.com/imunhatep/gocollection/dict"

type AwsAccountID string

func (a AwsAccountID) String() string { return string(a) }

const DefaultAwsRegion AwsRegion = "us-east-1"

type AwsRegion string

func (r AwsRegion) String() string {
	return string(r)
}

func GetAwsRegionList() []AwsRegion {
	return dict.Keys(GetAwsRegionData())
}

func GetAwsRegionStringList() []string {
	regions := []string{}
	for r, _ := range GetAwsRegionData() {
		regions = append(regions, r.String())
	}

	return regions
}

func GetAwsRegionDescription(region AwsRegion) string {
	regions := GetAwsRegionData()
	if data, ok := regions[region]; ok {
		return data["description"]
	}

	return "undefined"
}

func GetAwsRegionData() map[AwsRegion]map[string]string {
	return map[AwsRegion]map[string]string{
		"af-south-1":     {"description": "Africa (Cape Town)"},
		"ap-east-1":      {"description": "Asia Pacific (Hong Kong)"},
		"ap-northeast-1": {"description": "Asia Pacific (Tokyo)"},
		"ap-northeast-2": {"description": "Asia Pacific (Seoul)"},
		"ap-northeast-3": {"description": "Asia Pacific (Osaka)"},
		"ap-south-1":     {"description": "Asia Pacific (Mumbai)"},
		"ap-south-2":     {"description": "Asia Pacific (Hyderabad)"},
		"ap-southeast-1": {"description": "Asia Pacific (Singapore)"},
		"ap-southeast-2": {"description": "Asia Pacific (Sydney)"},
		"ap-southeast-3": {"description": "Asia Pacific (Jakarta)"},
		"ap-southeast-4": {"description": "Asia Pacific (Melbourne)"},
		"ap-southeast-5": {"description": "Asia Pacific (Malaysia)"},
		// "aws-global":     {"description": "AWS Standard global region"},
		"ca-central-1": {"description": "Canada (Central)"},
		"ca-west-1":    {"description": "Canada West (Calgary)"},
		"eu-central-1": {"description": "Europe (Frankfurt)"},
		"eu-central-2": {"description": "Europe (Zurich)"},
		"eu-north-1":   {"description": "Europe (Stockholm)"},
		"eu-south-1":   {"description": "Europe (Milan)"},
		"eu-south-2":   {"description": "Europe (Spain)"},
		"eu-west-1":    {"description": "Europe (Ireland)"},
		"eu-west-2":    {"description": "Europe (London)"},
		"eu-west-3":    {"description": "Europe (Paris)"},
		"il-central-1": {"description": "Israel (Tel Aviv)"},
		"me-central-1": {"description": "Middle East (UAE)"},
		"me-south-1":   {"description": "Middle East (Bahrain)"},
		"sa-east-1":    {"description": "South America (Sao Paulo)"},
		"us-east-1":    {"description": "US East (N. Virginia)"},
		"us-east-2":    {"description": "US East (Ohio)"},
		"us-west-1":    {"description": "US West (N. California)"},
		"us-west-2":    {"description": "US West (Oregon)"},
	}
}
