// Package sqs provides SQS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package sqs

import (
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "sqs"

// GetClient returns a cached or new SQS client
func GetClient(client *v3.Client, optFns ...func(*sqs.Options)) *sqs.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*sqs.Client)
	}

	// Create new client
	svc := sqs.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
