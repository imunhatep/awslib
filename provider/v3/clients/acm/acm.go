// Package acm provides ACM service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package acm

import (
	"github.com/aws/aws-sdk-go-v2/service/acm"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "acm"

// GetClient returns a cached or new ACM client
func GetClient(client *v3.Client, optFns ...func(*acm.Options)) *acm.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*acm.Client)
	}

	// Create new client
	svc := acm.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
