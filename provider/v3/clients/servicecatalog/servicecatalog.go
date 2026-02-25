// Package servicecatalog provides ServiceCatalog service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package servicecatalog

import (
	"github.com/aws/aws-sdk-go-v2/service/servicecatalog"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "servicecatalog"

// GetClient returns a cached or new ServiceCatalog client
func GetClient(client *v3.Client, optFns ...func(*servicecatalog.Options)) *servicecatalog.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*servicecatalog.Client)
	}

	// Create new client
	svc := servicecatalog.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
