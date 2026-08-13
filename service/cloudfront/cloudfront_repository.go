// Package cloudfront wraps the CloudFront SaaS Manager (multi-tenant
// distribution) API: distribution tenants, connection groups and the
// CloudFront-managed ACM certificates attached to a tenant.
//
// A tenant is one hostname's front door on a shared "tenant-only" distribution,
// which makes it the primitive for provisioning large numbers of hostnames
// without one distribution per hostname.
//
// Two things about CloudFront differ from the other services in this library:
//
//   - The control plane is global. The client is still built from the v3 client's
//     config, so the region carried by entities is the client's region, not a
//     property of the resource.
//   - ACM certificates referenced from a tenant (Customizations.Certificate.Arn)
//     must live in us-east-1 regardless of where anything else runs.
package cloudfront

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	cfclient "github.com/imunhatep/awslib/provider/v3/clients/cloudfront"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// certificateFetchConcurrency bounds the fan-out used when reading managed
// certificate details for a page of tenants. Certificate state is not part of
// the tenant list response, so a bulk read is unavoidably 1+N calls.
const certificateFetchConcurrency = 8

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type CloudFrontRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewCloudFrontRepository(ctx context.Context, client *v3.Client) *CloudFrontRepository {
	repo := &CloudFrontRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *CloudFrontRepository) cloudFrontClient() *awscf.Client {
	return cfclient.GetClient(r.client)
}

func (r *CloudFrontRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *CloudFrontRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

// parseArn turns an ARN returned by the API into the parsed form the resource
// abstraction expects. CloudFront hands back a complete ARN, so unlike most
// services here there is nothing to assemble by hand.
func parseArn(value *string) *arn.ARN {
	if value == nil || *value == "" {
		return nil
	}

	parsed, err := arn.Parse(*value)
	if err != nil {
		return nil
	}

	return &parsed
}
