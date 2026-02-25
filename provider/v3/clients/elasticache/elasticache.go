// Package elasticache provides ElastiCache service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package elasticache

import (
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "elasticache"

// GetClient returns a cached or new ElastiCache client
func GetClient(client *v3.Client, optFns ...func(*elasticache.Options)) *elasticache.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*elasticache.Client)
	}

	// Create new client
	svc := elasticache.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
