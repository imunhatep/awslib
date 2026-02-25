// Package shield provides Shield service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package shield

import (
	"github.com/aws/aws-sdk-go-v2/service/shield"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "shield"

// GetClient returns a cached or new Shield client
func GetClient(client *v3.Client, optFns ...func(*shield.Options)) *shield.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*shield.Client)
	}

	// Create new client
	svc := shield.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
