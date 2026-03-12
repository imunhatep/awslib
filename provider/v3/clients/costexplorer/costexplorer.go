// Package costexplorer provides CostExplorer service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package costexplorer

import (
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "costexplorer"

// GetClient returns a cached or new CostExplorer client
func GetClient(client *v3.Client, optFns ...func(*costexplorer.Options)) *costexplorer.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*costexplorer.Client)
	}

	// Create new client
	svc := costexplorer.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
