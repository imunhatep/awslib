package resourcegroupstagging

import (
	"strings"
	"testing"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/stretchr/testify/assert"
)

// unsupportedResourceTypes are the types this API cannot answer for, listed so
// TestEveryResourceTypeIsDecided can tell a deliberate omission from a
// forgotten one.
//
// Route53 records and Route53Domains domains are not ARN-addressable through the
// tagging API at all. The CloudFront SaaS Manager types have no established
// filter spelling. Nothing here is a mistake to be fixed by adding a guess.
var unsupportedResourceTypes = map[cfg.ResourceType]string{
	ccfg.ResourceTypeRoute53ResourceRecord:               "records are not taggable resources",
	ccfg.ResourceTypeRoute53Domain:                       "not ARN-addressable through the tagging API",
	ccfg.ResourceTypeRoute53DomainSummary:                "not ARN-addressable through the tagging API",
	ccfg.ResourceTypeCloudFrontDistributionTenantSummary: "no established filter spelling",
	ccfg.ResourceTypeCloudFrontConnectionGroup:           "no established filter spelling",
}

// TestEveryResourceTypeIsDecided fails when a resource type is added to
// service/cfg without anyone deciding whether the tagging API can answer for it.
//
// The failure mode this guards against is silent: an unmapped type falls back to
// an unfiltered region sweep, which still returns correct tags, so nothing
// breaks loudly — the cost is just paid on every call, forever, unnoticed.
func TestEveryResourceTypeIsDecided(t *testing.T) {
	for _, resourceType := range ccfg.ResourceTypeList() {
		_, mapped := ResourceTypeFilter(resourceType)
		_, known := unsupportedResourceTypes[resourceType]

		assert.True(t, mapped || known,
			"resource type %s has neither a tagging filter nor an entry in unsupportedResourceTypes: "+
				"add the filter, or record why the API cannot answer for it", resourceType)
	}
}

// TestUnsupportedResourceTypesHaveNoFilter keeps the two lists from disagreeing:
// a type documented as unanswerable must not also carry a filter.
func TestUnsupportedResourceTypesHaveNoFilter(t *testing.T) {
	for resourceType, reason := range unsupportedResourceTypes {
		filter, mapped := ResourceTypeFilter(resourceType)

		assert.False(t, mapped,
			"resource type %s is documented as unsupported (%s) but maps to filter %q",
			resourceType, reason, filter)
	}
}

// TestResourceTypeFilterFormat pins the shape GetResources accepts:
// "service" or "service:resourceType", lower-cased service, no AWS:: prefix.
// A CloudFormation type name leaking into the table would be accepted by the
// SDK and rejected by AWS.
func TestResourceTypeFilterFormat(t *testing.T) {
	for resourceType, filter := range resourceTypeFilters {
		assert.NotContains(t, filter, "::",
			"filter for %s looks like a CloudFormation type, not a tagging filter", resourceType)
		assert.LessOrEqual(t, strings.Count(filter, ":"), 1,
			"filter for %s has more than one separator", resourceType)
		assert.NotEmpty(t, filter, "filter for %s is empty", resourceType)

		service := strings.SplitN(filter, ":", 2)[0]
		assert.Equal(t, strings.ToLower(service), service,
			"service segment of filter for %s must be lower case", resourceType)
	}
}

// TestResourceTypeFilterSpotChecks pins the entries that a mechanical
// lower-casing of the CloudFormation type would get wrong. These are the reason
// the table exists rather than a strings.Replace.
func TestResourceTypeFilterSpotChecks(t *testing.T) {
	cases := map[cfg.ResourceType]string{
		cfg.ResourceTypeInstance:            "ec2:instance",
		cfg.ResourceTypeDBInstance:          "rds:db",
		cfg.ResourceTypeDBSnapshot:          "rds:snapshot",
		ccfg.ResourceTypeCloudWatchLogGroup: "logs:log-group",
		cfg.ResourceTypeEFSFileSystem:       "elasticfilesystem:file-system",
		ccfg.ResourceTypeEmrCluster:         "elasticmapreduce:cluster",
		cfg.ResourceTypeLoadBalancerV2:      "elasticloadbalancing:loadbalancer",
		cfg.ResourceTypeAutoScalingGroup:    "autoscaling:autoScalingGroup",
		cfg.ResourceTypeEip:                 "ec2:elastic-ip",
		// Service-only filters: these ARNs carry no resource-type segment, so
		// "s3:bucket" or "sns:topic" would be an invented spelling.
		cfg.ResourceTypeBucket: "s3",
		cfg.ResourceTypeTopic:  "sns",
		cfg.ResourceTypeQueue:  "sqs",
	}

	for resourceType, expected := range cases {
		filter, ok := ResourceTypeFilter(resourceType)

		assert.True(t, ok, "expected a filter for %s", resourceType)
		assert.Equal(t, expected, filter, "filter for %s", resourceType)
	}
}

// TestResourceTypeFiltersReportsUnmapped pins the two-return contract: unmapped
// types are handed back, never quietly dropped from the filter list.
func TestResourceTypeFiltersReportsUnmapped(t *testing.T) {
	filters, unmapped := ResourceTypeFilters([]cfg.ResourceType{
		cfg.ResourceTypeInstance,
		ccfg.ResourceTypeRoute53ResourceRecord,
		cfg.ResourceTypeBucket,
	})

	assert.ElementsMatch(t, []string{"ec2:instance", "s3"}, filters)
	assert.Equal(t, []cfg.ResourceType{ccfg.ResourceTypeRoute53ResourceRecord}, unmapped)
}

// TestResourceTypeFiltersCollapsesDuplicates covers the EMR Serverless case,
// where two resource types share one filter. Sending it twice is not an error,
// but it is noise in the request.
func TestResourceTypeFiltersCollapsesDuplicates(t *testing.T) {
	filters, unmapped := ResourceTypeFilters([]cfg.ResourceType{
		ccfg.ResourceTypeEmrServerlessApplication,
		ccfg.ResourceTypeEmrServerlessJobRun,
	})

	assert.Equal(t, []string{"emr-serverless"}, filters)
	assert.Empty(t, unmapped)
}

// TestResourceTypeFiltersEmptyInput documents that no input means no filters,
// which callers turn into an unfiltered region sweep rather than an error.
func TestResourceTypeFiltersEmptyInput(t *testing.T) {
	filters, unmapped := ResourceTypeFilters(nil)

	assert.Empty(t, filters)
	assert.Empty(t, unmapped)
}
