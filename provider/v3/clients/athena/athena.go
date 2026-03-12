// Package athena provides Athena service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package athena

import (
	"github.com/aws/aws-sdk-go-v2/service/athena"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "athena"

// GetClient returns a cached or new Athena client
func GetClient(client *v3.Client, optFns ...func(*athena.Options)) *athena.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*athena.Client)
	}

	// Create new client
	svc := athena.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
