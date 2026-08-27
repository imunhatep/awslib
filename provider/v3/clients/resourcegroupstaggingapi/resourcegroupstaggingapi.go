// Package resourcegroupstaggingapi provides ResourceGroupsTaggingAPI service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package resourcegroupstaggingapi

import (
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "resourcegroupstaggingapi"

// GetClient returns a cached or new ResourceGroupsTaggingAPI client
func GetClient(client *v3.Client, optFns ...func(*resourcegroupstaggingapi.Options)) *resourcegroupstaggingapi.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*resourcegroupstaggingapi.Client)
	}

	// Create new client
	svc := resourcegroupstaggingapi.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
