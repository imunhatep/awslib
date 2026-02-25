// Package route53 provides Route53 service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package route53

import (
	"github.com/aws/aws-sdk-go-v2/service/route53"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "route53"

// GetClient returns a cached or new Route53 client
func GetClient(client *v3.Client, optFns ...func(*route53.Options)) *route53.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*route53.Client)
	}

	// Create new client
	svc := route53.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
