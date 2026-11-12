package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// DistributionTenantList holds a list of DistributionTenant items.
type DistributionTenantList struct {
	Items []DistributionTenant
}

// DistributionTenant is a full tenant as returned by a Get or a write. It carries
// the tenant's tags: read separately after a Get, because CloudFront leaves them
// out of the response, and carried over from what the caller already had after a
// write. ETag is carried alongside because every mutating call needs it for
// optimistic locking.
type DistributionTenant struct {
	service.AbstractResource
	cftypes.DistributionTenant
	ETag string
}

// DistributionTenantSummaryList holds a list of DistributionTenantSummary items.
type DistributionTenantSummaryList struct {
	Items []DistributionTenantSummary
}

// DistributionTenantSummary is the lighter entity returned by
// ListDistributionTenants. It carries almost everything the full tenant does —
// including the ETag — but not Parameters.
//
// Tags are not part of the list response either; the listing reads them for all
// tenants in one tagging sweep and stores them here.
type DistributionTenantSummary struct {
	service.AbstractResource
	cftypes.DistributionTenantSummary
	Tags []cftypes.Tag
}

// TenantCertificate pairs a tenant summary with its CloudFront-managed
// certificate details. Certificate state is not part of the tenant list
// response, so it has to be fetched per tenant; Certificate is nil when the
// tenant has no managed certificate (it inherits the distribution's, or brings
// its own ACM certificate).
type TenantCertificate struct {
	Tenant      DistributionTenantSummary
	Certificate *cftypes.ManagedCertificateDetails
}

func NewDistributionTenant(client AwsClient, tenant cftypes.DistributionTenant, etag string) DistributionTenant {
	return DistributionTenant{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(tenant.Id),
			ARN:       parseArn(tenant.Arn),
			CreatedAt: aws.ToTime(tenant.CreatedTime),
			Type:      ccfg.ResourceTypeCloudFrontDistributionTenant,
		},
		DistributionTenant: tenant,
		ETag:               etag,
	}
}

func (e DistributionTenant) GetName() string {
	return aws.ToString(e.Name)
}

func (e DistributionTenant) GetTags() map[string]string {
	return tagsToMap(e.DistributionTenant.Tags)
}

func (e DistributionTenant) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

// IsEnabled reports whether the tenant is serving. A disabled tenant is the
// precondition for deletion.
func (e DistributionTenant) IsEnabled() bool {
	return aws.ToBool(e.Enabled)
}

// DomainNames returns the hostnames attached to the tenant.
func (e DistributionTenant) DomainNames() []string {
	return domainNames(e.Domains)
}

// DomainsActive reports whether every attached domain has reached active
// status. A tenant with no domains is not considered active.
func (e DistributionTenant) DomainsActive() bool {
	return domainsActive(e.Domains)
}

func NewDistributionTenantSummary(client AwsClient, summary cftypes.DistributionTenantSummary) DistributionTenantSummary {
	return DistributionTenantSummary{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Id),
			ARN:       parseArn(summary.Arn),
			CreatedAt: aws.ToTime(summary.CreatedTime),
			Type:      ccfg.ResourceTypeCloudFrontDistributionTenantSummary,
		},
		DistributionTenantSummary: summary,
	}
}

func (e DistributionTenantSummary) GetName() string {
	return aws.ToString(e.Name)
}

func (e DistributionTenantSummary) GetTags() map[string]string {
	return tagsToMap(&cftypes.Tags{Items: e.Tags})
}

func (e DistributionTenantSummary) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

func (e DistributionTenantSummary) IsEnabled() bool {
	return aws.ToBool(e.Enabled)
}

func (e DistributionTenantSummary) DomainNames() []string {
	return domainNames(e.Domains)
}

func (e DistributionTenantSummary) DomainsActive() bool {
	return domainsActive(e.Domains)
}

// GetETag returns the version token needed by the mutating calls.
func (e DistributionTenantSummary) GetETag() string {
	return aws.ToString(e.ETag)
}

func domainNames(domains []cftypes.DomainResult) []string {
	names := make([]string, 0, len(domains))
	for _, d := range domains {
		names = append(names, aws.ToString(d.Domain))
	}

	return names
}

func domainsActive(domains []cftypes.DomainResult) bool {
	if len(domains) == 0 {
		return false
	}

	for _, d := range domains {
		if d.Status != cftypes.DomainStatusActive {
			return false
		}
	}

	return true
}

func tagsToMap(tags *cftypes.Tags) map[string]string {
	out := make(map[string]string)
	if tags == nil {
		return out
	}

	for _, tag := range tags.Items {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}

// CertificateIssued reports whether the managed certificate has been issued and
// the tenant can serve TLS. Returns false when there is no managed certificate.
func (e TenantCertificate) CertificateIssued() bool {
	return e.Certificate != nil &&
		e.Certificate.CertificateStatus == cftypes.ManagedCertificateStatusIssued
}

// PendingValidation reports whether the managed certificate is still waiting on
// domain validation.
func (e TenantCertificate) PendingValidation() bool {
	return e.Certificate != nil &&
		e.Certificate.CertificateStatus == cftypes.ManagedCertificateStatusPendingValidation
}

// ValidationRedirects returns the per-domain redirect targets a self-hosted
// validation flow must serve. Empty for the CloudFront-hosted flow.
func (e TenantCertificate) ValidationRedirects() []cftypes.ValidationTokenDetail {
	if e.Certificate == nil {
		return nil
	}

	return e.Certificate.ValidationTokenDetails
}
