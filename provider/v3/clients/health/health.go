// Package health provides Health service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package health

import (
	"github.com/aws/aws-sdk-go-v2/service/health"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "health"

// GetClient returns a cached or new Health client
func GetClient(client *v3.Client, optFns ...func(*health.Options)) *health.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*health.Client)
	}

	// Create new client
	svc := health.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
