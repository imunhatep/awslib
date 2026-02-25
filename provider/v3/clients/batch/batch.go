// Package batch provides Batch service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package batch

import (
	"github.com/aws/aws-sdk-go-v2/service/batch"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "batch"

// GetClient returns a cached or new Batch client
func GetClient(client *v3.Client, optFns ...func(*batch.Options)) *batch.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*batch.Client)
	}

	// Create new client
	svc := batch.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
