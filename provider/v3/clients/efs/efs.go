// Package efs provides EFS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package efs

import (
	"github.com/aws/aws-sdk-go-v2/service/efs"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "efs"

// GetClient returns a cached or new EFS client
func GetClient(client *v3.Client, optFns ...func(*efs.Options)) *efs.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*efs.Client)
	}

	// Create new client
	svc := efs.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
