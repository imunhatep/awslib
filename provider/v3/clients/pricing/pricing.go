// Package pricing provides Pricing service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package pricing

import (
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "pricing"

// GetClient returns a cached or new Pricing client
func GetClient(client *v3.Client, optFns ...func(*pricing.Options)) *pricing.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*pricing.Client)
	}

	// Create new client
	svc := pricing.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
