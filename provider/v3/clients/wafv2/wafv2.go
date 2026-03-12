// Package wafv2 provides WAFv2 service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package wafv2

import (
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "wafv2"

// GetClient returns a cached or new WAFv2 client
func GetClient(client *v3.Client, optFns ...func(*wafv2.Options)) *wafv2.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*wafv2.Client)
	}

	// Create new client
	svc := wafv2.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
