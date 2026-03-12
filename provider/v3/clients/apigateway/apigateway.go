// Package apigateway provides APIGateway service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package apigateway

import (
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "apigateway"

// GetClient returns a cached or new APIGateway client
func GetClient(client *v3.Client, optFns ...func(*apigateway.Options)) *apigateway.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*apigateway.Client)
	}

	// Create new client
	svc := apigateway.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
