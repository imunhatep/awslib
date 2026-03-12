// Package ses provides SES service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package ses

import (
	"github.com/aws/aws-sdk-go-v2/service/ses"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "ses"

// GetClient returns a cached or new SES client
func GetClient(client *v3.Client, optFns ...func(*ses.Options)) *ses.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*ses.Client)
	}

	// Create new client
	svc := ses.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
