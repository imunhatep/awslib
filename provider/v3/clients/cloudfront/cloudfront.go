// Package cloudfront provides CloudFront service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "cloudfront"

// GetClient returns a cached or new CloudFront client
func GetClient(client *v3.Client, optFns ...func(*cloudfront.Options)) *cloudfront.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*cloudfront.Client)
	}

	// Create new client
	svc := cloudfront.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
