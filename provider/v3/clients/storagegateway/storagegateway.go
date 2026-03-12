// Package storagegateway provides StorageGateway service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package storagegateway

import (
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "storagegateway"

// GetClient returns a cached or new StorageGateway client
func GetClient(client *v3.Client, optFns ...func(*storagegateway.Options)) *storagegateway.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*storagegateway.Client)
	}

	// Create new client
	svc := storagegateway.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
