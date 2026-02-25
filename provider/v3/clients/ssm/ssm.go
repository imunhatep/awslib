// Package ssm provides SSM service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package ssm

import (
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "ssm"

// GetClient returns a cached or new SSM client
func GetClient(client *v3.Client, optFns ...func(*ssm.Options)) *ssm.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*ssm.Client)
	}

	// Create new client
	svc := ssm.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
