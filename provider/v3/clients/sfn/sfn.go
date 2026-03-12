// Package sfn provides StepFunctions service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package sfn

import (
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "sfn"

// GetClient returns a cached or new StepFunctions client
func GetClient(client *v3.Client, optFns ...func(*sfn.Options)) *sfn.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*sfn.Client)
	}

	// Create new client
	svc := sfn.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
