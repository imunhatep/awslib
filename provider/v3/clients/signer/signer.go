// Package signer provides Signer service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package signer

import (
	"github.com/aws/aws-sdk-go-v2/service/signer"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "signer"

// GetClient returns a cached or new Signer client
func GetClient(client *v3.Client, optFns ...func(*signer.Options)) *signer.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*signer.Client)
	}

	// Create new client
	svc := signer.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
