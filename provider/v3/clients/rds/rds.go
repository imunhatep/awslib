// Package rds provides RDS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package rds

import (
	"github.com/aws/aws-sdk-go-v2/service/rds"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "rds"

// GetClient returns a cached or new RDS client
func GetClient(client *v3.Client, optFns ...func(*rds.Options)) *rds.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*rds.Client)
	}

	// Create new client
	svc := rds.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
