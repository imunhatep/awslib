// Package glue provides Glue service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package glue

import (
	"github.com/aws/aws-sdk-go-v2/service/glue"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "glue"

// GetClient returns a cached or new Glue client
func GetClient(client *v3.Client, optFns ...func(*glue.Options)) *glue.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*glue.Client)
	}

	// Create new client
	svc := glue.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
