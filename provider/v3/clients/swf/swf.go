// Package swf provides SWF service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package swf

import (
	"github.com/aws/aws-sdk-go-v2/service/swf"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "swf"

// GetClient returns a cached or new SWF client
func GetClient(client *v3.Client, optFns ...func(*swf.Options)) *swf.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*swf.Client)
	}

	// Create new client
	svc := swf.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
