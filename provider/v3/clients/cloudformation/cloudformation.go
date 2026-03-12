// Package cloudformation provides CloudFormation service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package cloudformation

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "cloudformation"

// GetClient returns a cached or new CloudFormation client
func GetClient(client *v3.Client, optFns ...func(*cloudformation.Options)) *cloudformation.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*cloudformation.Client)
	}

	// Create new client
	svc := cloudformation.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
