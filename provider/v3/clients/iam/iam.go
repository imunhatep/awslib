// Package iam provides IAM service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package iam

import (
	"github.com/aws/aws-sdk-go-v2/service/iam"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "iam"

// GetClient returns a cached or new IAM client
func GetClient(client *v3.Client, optFns ...func(*iam.Options)) *iam.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*iam.Client)
	}

	// Create new client
	svc := iam.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
