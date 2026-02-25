// Package cloudcontrol provides CloudControl service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package cloudcontrol

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "cloudcontrol"

// GetClient returns a cached or new CloudControl client
func GetClient(client *v3.Client, optFns ...func(*cloudcontrol.Options)) *cloudcontrol.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*cloudcontrol.Client)
	}

	// Create new client
	svc := cloudcontrol.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
