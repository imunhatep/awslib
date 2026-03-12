// Package synthetics provides Synthetics service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package synthetics

import (
	"github.com/aws/aws-sdk-go-v2/service/synthetics"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "synthetics"

// GetClient returns a cached or new Synthetics client
func GetClient(client *v3.Client, optFns ...func(*synthetics.Options)) *synthetics.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*synthetics.Client)
	}

	// Create new client
	svc := synthetics.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
