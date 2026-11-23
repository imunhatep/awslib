// Package ecr provides ECR service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package ecr

import (
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "ecr"

// GetClient returns a cached or new ECR client
func GetClient(client *v3.Client, optFns ...func(*ecr.Options)) *ecr.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*ecr.Client)
	}

	// Create new client
	svc := ecr.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
