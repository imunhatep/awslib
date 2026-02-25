// Package ec2 provides EC2 service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package ec2

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "ec2"

// GetClient returns a cached or new EC2 client
func GetClient(client *v3.Client, optFns ...func(*ec2.Options)) *ec2.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*ec2.Client)
	}

	// Create new client
	svc := ec2.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
