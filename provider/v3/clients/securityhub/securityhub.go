// Package securityhub provides SecurityHub service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package securityhub

import (
	"github.com/aws/aws-sdk-go-v2/service/securityhub"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "securityhub"

// GetClient returns a cached or new SecurityHub client
func GetClient(client *v3.Client, optFns ...func(*securityhub.Options)) *securityhub.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*securityhub.Client)
	}

	// Create new client
	svc := securityhub.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
