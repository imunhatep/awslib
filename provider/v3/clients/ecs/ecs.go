// Package ecs provides ECS service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package ecs

import (
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "ecs"

// GetClient returns a cached or new ECS client
func GetClient(client *v3.Client, optFns ...func(*ecs.Options)) *ecs.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*ecs.Client)
	}

	// Create new client
	svc := ecs.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
