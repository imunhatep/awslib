// Package servicequotas provides ServiceQuotas service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package servicequotas

import (
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "servicequotas"

// GetClient returns a cached or new ServiceQuotas client
func GetClient(client *v3.Client, optFns ...func(*servicequotas.Options)) *servicequotas.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*servicequotas.Client)
	}

	// Create new client
	svc := servicequotas.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
