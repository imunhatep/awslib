// Package elasticloadbalancingv2 provides ElasticLoadBalancingV2 service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package elasticloadbalancingv2

import (
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "elasticloadbalancingv2"

// GetClient returns a cached or new ElasticLoadBalancingV2 client
func GetClient(client *v3.Client, optFns ...func(*elasticloadbalancingv2.Options)) *elasticloadbalancingv2.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*elasticloadbalancingv2.Client)
	}

	// Create new client
	svc := elasticloadbalancingv2.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
