// Package secretsmanager provides SecretsManager service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package secretsmanager

import (
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "secretsmanager"

// GetClient returns a cached or new SecretsManager client
func GetClient(client *v3.Client, optFns ...func(*secretsmanager.Options)) *secretsmanager.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*secretsmanager.Client)
	}

	// Create new client
	svc := secretsmanager.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
