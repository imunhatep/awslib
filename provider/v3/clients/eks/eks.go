// Package eks provides EKS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package eks

import (
	"github.com/aws/aws-sdk-go-v2/service/eks"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "eks"

// GetClient returns a cached or new EKS client
func GetClient(client *v3.Client, optFns ...func(*eks.Options)) *eks.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*eks.Client)
	}

	// Create new client
	svc := eks.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
