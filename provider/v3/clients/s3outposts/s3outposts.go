// Package s3outposts provides S3Outposts service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package s3outposts

import (
	"github.com/aws/aws-sdk-go-v2/service/s3outposts"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "s3outposts"

// GetClient returns a cached or new S3Outposts client
func GetClient(client *v3.Client, optFns ...func(*s3outposts.Options)) *s3outposts.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*s3outposts.Client)
	}

	// Create new client
	svc := s3outposts.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
