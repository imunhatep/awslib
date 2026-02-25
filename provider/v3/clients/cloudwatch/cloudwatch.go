// Package cloudwatch provides CloudWatch service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "cloudwatch"

// GetClient returns a cached or new CloudWatch client
func GetClient(client *v3.Client, optFns ...func(*cloudwatch.Options)) *cloudwatch.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*cloudwatch.Client)
	}

	// Create new client
	svc := cloudwatch.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
