// Package lambda provides Lambda service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package lambda

import (
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "lambda"

// GetClient returns a cached or new Lambda client
func GetClient(client *v3.Client, optFns ...func(*lambda.Options)) *lambda.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*lambda.Client)
	}

	// Create new client
	svc := lambda.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
