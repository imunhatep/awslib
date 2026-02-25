// Package waf provides WAF service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package waf

import (
	"github.com/aws/aws-sdk-go-v2/service/waf"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "waf"

// GetClient returns a cached or new WAF client
func GetClient(client *v3.Client, optFns ...func(*waf.Options)) *waf.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*waf.Client)
	}

	// Create new client
	svc := waf.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
