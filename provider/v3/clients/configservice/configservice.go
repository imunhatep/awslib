// Package configservice provides ConfigService service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package configservice

import (
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "configservice"

// GetClient returns a cached or new ConfigService client
func GetClient(client *v3.Client, optFns ...func(*configservice.Options)) *configservice.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*configservice.Client)
	}

	// Create new client
	svc := configservice.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
