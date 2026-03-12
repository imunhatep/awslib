// Package transfer provides Transfer service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package transfer

import (
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "transfer"

// GetClient returns a cached or new Transfer client
func GetClient(client *v3.Client, optFns ...func(*transfer.Options)) *transfer.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*transfer.Client)
	}

	// Create new client
	svc := transfer.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
