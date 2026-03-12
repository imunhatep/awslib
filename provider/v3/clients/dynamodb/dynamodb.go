// Package dynamodb provides DynamoDB service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "dynamodb"

// GetClient returns a cached or new DynamoDB client
func GetClient(client *v3.Client, optFns ...func(*dynamodb.Options)) *dynamodb.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*dynamodb.Client)
	}

	// Create new client
	svc := dynamodb.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
