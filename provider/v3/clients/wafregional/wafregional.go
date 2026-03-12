// Package wafregional provides WAFRegional service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package wafregional

import (
	"github.com/aws/aws-sdk-go-v2/service/wafregional"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "wafregional"

// GetClient returns a cached or new WAFRegional client
func GetClient(client *v3.Client, optFns ...func(*wafregional.Options)) *wafregional.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*wafregional.Client)
	}

	// Create new client
	svc := wafregional.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
