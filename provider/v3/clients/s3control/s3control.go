// Package s3control provides S3Control service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package s3control

import (
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "s3control"

// GetClient returns a cached or new S3Control client
func GetClient(client *v3.Client, optFns ...func(*s3control.Options)) *s3control.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*s3control.Client)
	}

	// Create new client
	svc := s3control.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
