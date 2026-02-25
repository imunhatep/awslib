// Package emrserverless provides EMRServerless service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package emrserverless

import (
	"github.com/aws/aws-sdk-go-v2/service/emrserverless"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "emrserverless"

// GetClient returns a cached or new EMRServerless client
func GetClient(client *v3.Client, optFns ...func(*emrserverless.Options)) *emrserverless.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*emrserverless.Client)
	}

	// Create new client
	svc := emrserverless.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
