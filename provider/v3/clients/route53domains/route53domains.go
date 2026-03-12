// Package route53domains provides Route53Domains service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package route53domains

import (
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "route53domains"

// GetClient returns a cached or new Route53Domains client
func GetClient(client *v3.Client, optFns ...func(*route53domains.Options)) *route53domains.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*route53domains.Client)
	}

	// Create new client
	svc := route53domains.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
