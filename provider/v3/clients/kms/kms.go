// Package kms provides KMS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package kms

import (
	"github.com/aws/aws-sdk-go-v2/service/kms"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "kms"

// GetClient returns a cached or new KMS client
func GetClient(client *v3.Client, optFns ...func(*kms.Options)) *kms.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*kms.Client)
	}

	// Create new client
	svc := kms.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
