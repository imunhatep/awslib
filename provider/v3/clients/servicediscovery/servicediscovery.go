// Package servicediscovery provides ServiceDiscovery service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package servicediscovery

import (
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "servicediscovery"

// GetClient returns a cached or new ServiceDiscovery client
func GetClient(client *v3.Client, optFns ...func(*servicediscovery.Options)) *servicediscovery.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*servicediscovery.Client)
	}

	// Create new client
	svc := servicediscovery.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
