// Package savingsplans provides SavingsPlans service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package savingsplans

import (
	"github.com/aws/aws-sdk-go-v2/service/savingsplans"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "savingsplans"

// GetClient returns a cached or new SavingsPlans client
func GetClient(client *v3.Client, optFns ...func(*savingsplans.Options)) *savingsplans.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*savingsplans.Client)
	}

	// Create new client
	svc := savingsplans.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
