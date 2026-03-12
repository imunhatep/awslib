// Package emr provides EMR service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package emr

import (
	"github.com/aws/aws-sdk-go-v2/service/emr"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "emr"

// GetClient returns a cached or new EMR client
func GetClient(client *v3.Client, optFns ...func(*emr.Options)) *emr.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*emr.Client)
	}

	// Create new client
	svc := emr.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
