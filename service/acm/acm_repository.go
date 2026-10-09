// Package acm wraps AWS Certificate Manager: requesting, reading, tagging and
// deleting public certificates.
//
// ACM is regional, and the region is not a detail: a certificate can only be
// attached to resources in its own region, and CloudFront (including
// distribution tenants) only accepts certificates from us-east-1. Build the
// repository from a v3 client for the region the certificate must live in —
// the provider caches one ACM client per v3 client, so a region override on a
// client built for another region does not take effect.
package acm

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsacm "github.com/aws/aws-sdk-go-v2/service/acm"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	acmclient "github.com/imunhatep/awslib/provider/v3/clients/acm"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// tagFetchConcurrency bounds the fan-out used when reading tags for a page of
// certificates. ListCertificates does not return tags, so a tagged listing is
// unavoidably 1+N calls.
const tagFetchConcurrency = 8

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type AcmRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewAcmRepository(ctx context.Context, client *v3.Client) *AcmRepository {
	repo := &AcmRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *AcmRepository) acmClient() *awsacm.Client {
	return acmclient.GetClient(r.client)
}

func (r *AcmRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *AcmRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

// parseArn turns an ARN returned by the API into the parsed form the resource
// abstraction expects. ACM hands back a complete ARN, so there is nothing to
// assemble by hand.
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

// certificateID extracts the certificate UUID from an ACM ARN
// (arn:aws:acm:<region>:<account>:certificate/<id>).
func certificateID(parsed *arn.ARN) string {
	if parsed == nil {
		return ""
	}

	return strings.TrimPrefix(parsed.Resource, "certificate/")
}
