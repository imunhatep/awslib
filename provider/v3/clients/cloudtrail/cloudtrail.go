// Package cloudtrail provides CloudTrail service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package cloudtrail

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "cloudtrail"

// GetClient returns a cached or new CloudTrail client
func GetClient(client *v3.Client, optFns ...func(*cloudtrail.Options)) *cloudtrail.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*cloudtrail.Client)
	}

	// Create new client
	svc := cloudtrail.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
