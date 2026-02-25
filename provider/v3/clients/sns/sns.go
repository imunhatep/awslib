// Package sns provides SNS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package sns

import (
	"github.com/aws/aws-sdk-go-v2/service/sns"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "sns"

// GetClient returns a cached or new SNS client
func GetClient(client *v3.Client, optFns ...func(*sns.Options)) *sns.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*sns.Client)
	}

	// Create new client
	svc := sns.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
