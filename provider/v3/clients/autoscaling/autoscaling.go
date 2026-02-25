// Package autoscaling provides AutoScaling service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package autoscaling

import (
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "autoscaling"

// GetClient returns a cached or new AutoScaling client
func GetClient(client *v3.Client, optFns ...func(*autoscaling.Options)) *autoscaling.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*autoscaling.Client)
	}

	// Create new client
	svc := autoscaling.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
