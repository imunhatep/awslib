// Package accessanalyzer provides AccessAnalyzer service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package accessanalyzer

import (
	"github.com/aws/aws-sdk-go-v2/service/accessanalyzer"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "accessanalyzer"

// GetClient returns a cached or new AccessAnalyzer client
func GetClient(client *v3.Client, optFns ...func(*accessanalyzer.Options)) *accessanalyzer.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*accessanalyzer.Client)
	}

	// Create new client
	svc := accessanalyzer.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
