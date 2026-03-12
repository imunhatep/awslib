// Package s3 provides S3 service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package s3

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "s3"

// GetClient returns a cached or new S3 client
func GetClient(client *v3.Client, optFns ...func(*s3.Options)) *s3.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*s3.Client)
	}

	// Create new client
	svc := s3.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
