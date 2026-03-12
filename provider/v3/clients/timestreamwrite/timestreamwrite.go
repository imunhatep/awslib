// Package timestreamwrite provides TimestreamWrite service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package timestreamwrite

import (
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "timestreamwrite"

// GetClient returns a cached or new TimestreamWrite client
func GetClient(client *v3.Client, optFns ...func(*timestreamwrite.Options)) *timestreamwrite.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*timestreamwrite.Client)
	}

	// Create new client
	svc := timestreamwrite.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
