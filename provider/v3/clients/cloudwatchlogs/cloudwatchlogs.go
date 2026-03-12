// Package cloudwatchlogs provides CloudWatchLogs service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package cloudwatchlogs

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "cloudwatchlogs"

// GetClient returns a cached or new CloudWatchLogs client
func GetClient(client *v3.Client, optFns ...func(*cloudwatchlogs.Options)) *cloudwatchlogs.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*cloudwatchlogs.Client)
	}

	// Create new client
	svc := cloudwatchlogs.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
