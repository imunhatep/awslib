package cloudfront

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListDistributionTenantsAll lists every distribution tenant in the account.
func (r *CloudFrontRepository) ListDistributionTenantsAll() ([]DistributionTenantSummary, error) {
	return r.ListDistributionTenantsByInput(&awscf.ListDistributionTenantsInput{})
}

// ListDistributionTenantsByDistribution lists the tenants attached to one
// multi-tenant distribution.
func (r *CloudFrontRepository) ListDistributionTenantsByDistribution(distributionID string) ([]DistributionTenantSummary, error) {
	if distributionID == "" {
		return nil, errors.New("DistributionId cannot be empty")
	}

	return r.ListDistributionTenantsByInput(&awscf.ListDistributionTenantsInput{
		AssociationFilter: &cftypes.DistributionTenantAssociationFilter{
			DistributionId: aws.String(distributionID),
		},
	})
}

func (r *CloudFrontRepository) ListDistributionTenantsByInput(query *awscf.ListDistributionTenantsInput) ([]DistributionTenantSummary, error) {
	start := time.Now()

	var tenants []DistributionTenantSummary

	p := awscf.NewListDistributionTenantsPaginator(r.cloudFrontClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListDistributionTenants", ccfg.ResourceTypeCloudFrontDistributionTenantSummary)).Inc()
		}

		output, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListDistributionTenants", ccfg.ResourceTypeCloudFrontDistributionTenantSummary)).Inc()
			}

			return tenants, classifyErr(err)
		}

		for _, summary := range output.DistributionTenantList {
			tenants = append(tenants, NewDistributionTenantSummary(r.client, summary))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListDistributionTenants", ccfg.ResourceTypeCloudFrontDistributionTenantSummary)).
			Add(float64(len(tenants)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDistributionTenantsByInput", ccfg.ResourceTypeCloudFrontDistributionTenantSummary)).
			Observe(time.Since(start).Seconds())
	}

	return tenants, nil
}

func (r *CloudFrontRepository) GetDistributionTenantByInput(query *awscf.GetDistributionTenantInput) (*DistributionTenant, error) {
	start := time.Now()

	if query.Identifier == nil || aws.ToString(query.Identifier) == "" {
		return nil, errors.New("Identifier cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
	}

	output, err := r.cloudFrontClient().GetDistributionTenant(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.DistributionTenant == nil {
		return nil, nil
	}

	tenant := NewDistributionTenant(r.client, *output.DistributionTenant, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Inc()

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetDistributionTenantByInput", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Observe(time.Since(start).Seconds())
	}

	return &tenant, nil
}

// GetDistributionTenant reads a tenant by ARN, ID or name.
func (r *CloudFrontRepository) GetDistributionTenant(identifier string) (*DistributionTenant, error) {
	return r.GetDistributionTenantByInput(&awscf.GetDistributionTenantInput{
		Identifier: aws.String(identifier),
	})
}

// GetDistributionTenantByDomain resolves the tenant currently serving a
// hostname. This is the reverse lookup a rotation pool needs to answer
// "who owns this hostname".
func (r *CloudFrontRepository) GetDistributionTenantByDomain(domain string) (*DistributionTenant, error) {
	start := time.Now()

	if domain == "" {
		return nil, errors.New("Domain cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetDistributionTenantByDomain", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
	}

	output, err := r.cloudFrontClient().GetDistributionTenantByDomain(r.ctx, &awscf.GetDistributionTenantByDomainInput{
		Domain: aws.String(domain),
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetDistributionTenantByDomain", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.DistributionTenant == nil {
		return nil, nil
	}

	tenant := NewDistributionTenant(r.client, *output.DistributionTenant, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDistributionTenantByDomain", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Inc()

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetDistributionTenantByDomain", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Observe(time.Since(start).Seconds())
	}

	return &tenant, nil
}

// GetManagedCertificateDetails reads the CloudFront-managed ACM certificate for
// a tenant. The identifier accepts a tenant ARN, ID or name.
//
// Certificate state is deliberately not part of the tenant list response, so
// this is the only way to learn whether a tenant can serve TLS yet.
func (r *CloudFrontRepository) GetManagedCertificateDetails(identifier string) (*cftypes.ManagedCertificateDetails, error) {
	start := time.Now()

	if identifier == "" {
		return nil, errors.New("Identifier cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetManagedCertificateDetails", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
	}

	output, err := r.cloudFrontClient().GetManagedCertificateDetails(r.ctx, &awscf.GetManagedCertificateDetailsInput{
		Identifier: aws.String(identifier),
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetManagedCertificateDetails", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
		}

		return nil, classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetManagedCertificateDetails", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Observe(time.Since(start).Seconds())
	}

	return output.ManagedCertificateDetails, nil
}

// ListDistributionTenantsWithCertificatesAll lists every tenant together with
// its managed certificate details.
func (r *CloudFrontRepository) ListDistributionTenantsWithCertificatesAll() ([]TenantCertificate, error) {
	return r.ListDistributionTenantsWithCertificatesByInput(&awscf.ListDistributionTenantsInput{})
}

// ListDistributionTenantsWithCertificatesByInput lists tenants and fills in the
// certificate details each one is missing.
//
// This is 1+N API calls by necessity — ListDistributionTenants does not return
// certificate state. The per-tenant reads run concurrently, bounded by
// certificateFetchConcurrency. A tenant whose certificate cannot be read (most
// commonly because it has no managed certificate) is returned with a nil
// Certificate rather than failing the whole listing.
func (r *CloudFrontRepository) ListDistributionTenantsWithCertificatesByInput(query *awscf.ListDistributionTenantsInput) ([]TenantCertificate, error) {
	tenants, err := r.ListDistributionTenantsByInput(query)
	if err != nil {
		return nil, err
	}

	results := make([]TenantCertificate, len(tenants))
	sem := make(chan struct{}, certificateFetchConcurrency)

	var wg sync.WaitGroup
	for i, tenant := range tenants {
		wg.Add(1)

		go func(idx int, t DistributionTenantSummary) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = TenantCertificate{Tenant: t}

			details, err := r.GetManagedCertificateDetails(t.GetId())
			if err != nil {
				log.Debug().
					Err(err).
					Str("tenant", t.GetId()).
					Msg("[CloudFrontRepository.ListDistributionTenantsWithCertificates] no managed certificate details")

				return
			}

			results[idx].Certificate = details
		}(i, tenant)
	}

	wg.Wait()

	return results, nil
}

// CreateDistributionTenant provisions a tenant on a multi-tenant distribution.
func (r *CloudFrontRepository) CreateDistributionTenant(input *awscf.CreateDistributionTenantInput) (*DistributionTenant, error) {
	start := time.Now()

	if aws.ToString(input.DistributionId) == "" {
		return nil, errors.New("DistributionId cannot be empty")
	}

	if aws.ToString(input.Name) == "" {
		return nil, errors.New("Name cannot be empty")
	}

	if len(input.Domains) == 0 {
		return nil, errors.New("Domains cannot be empty: a tenant must serve at least one domain")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CreateDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
	}

	output, err := r.cloudFrontClient().CreateDistributionTenant(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CreateDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.DistributionTenant == nil {
		return nil, nil
	}

	tenant := NewDistributionTenant(r.client, *output.DistributionTenant, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Observe(time.Since(start).Seconds())
	}

	return &tenant, nil
}

// UpdateDistributionTenant applies a caller-built update. IfMatch must carry
// the ETag of the version being replaced; a stale one yields
// ErrPreconditionFailed.
func (r *CloudFrontRepository) UpdateDistributionTenant(input *awscf.UpdateDistributionTenantInput) (*DistributionTenant, error) {
	start := time.Now()

	if aws.ToString(input.Id) == "" {
		return nil, errors.New("Id cannot be empty")
	}

	if aws.ToString(input.IfMatch) == "" {
		return nil, errors.New("IfMatch cannot be empty: updates require the current ETag")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("UpdateDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
	}

	output, err := r.cloudFrontClient().UpdateDistributionTenant(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.DistributionTenant == nil {
		return nil, nil
	}

	tenant := NewDistributionTenant(r.client, *output.DistributionTenant, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("UpdateDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Observe(time.Since(start).Seconds())
	}

	return &tenant, nil
}

// SetDistributionTenantEnabled flips a tenant between serving and disabled,
// reading the current ETag first so callers do not have to thread it through.
// A no-op when the tenant is already in the requested state.
func (r *CloudFrontRepository) SetDistributionTenantEnabled(identifier string, enabled bool) (*DistributionTenant, error) {
	tenant, err := r.GetDistributionTenant(identifier)
	if err != nil {
		return nil, err
	}

	if tenant == nil {
		return nil, errors.New(ErrNotFound)
	}

	if tenant.IsEnabled() == enabled {
		return tenant, nil
	}

	return r.UpdateDistributionTenant(&awscf.UpdateDistributionTenantInput{
		Id:      tenant.Id,
		IfMatch: aws.String(tenant.ETag),
		Enabled: aws.Bool(enabled),
	})
}

// UpdateDistributionTenantDomains replaces the set of hostnames a tenant
// serves, reading the current ETag first.
func (r *CloudFrontRepository) UpdateDistributionTenantDomains(identifier string, domains []string) (*DistributionTenant, error) {
	if len(domains) == 0 {
		return nil, errors.New("Domains cannot be empty: a tenant must serve at least one domain")
	}

	tenant, err := r.GetDistributionTenant(identifier)
	if err != nil {
		return nil, err
	}

	if tenant == nil {
		return nil, errors.New(ErrNotFound)
	}

	items := make([]cftypes.DomainItem, 0, len(domains))
	for _, domain := range domains {
		items = append(items, cftypes.DomainItem{Domain: aws.String(domain)})
	}

	return r.UpdateDistributionTenant(&awscf.UpdateDistributionTenantInput{
		Id:      tenant.Id,
		IfMatch: aws.String(tenant.ETag),
		Domains: items,
	})
}

// DeleteDistributionTenantByInput deletes a tenant with a caller-supplied
// ETag. CloudFront rejects the call with ErrNotDisabled unless the tenant is
// already disabled — use DeleteDistributionTenant for the full sequence.
func (r *CloudFrontRepository) DeleteDistributionTenantByInput(input *awscf.DeleteDistributionTenantInput) error {
	start := time.Now()

	if aws.ToString(input.Id) == "" {
		return errors.New("Id cannot be empty")
	}

	if aws.ToString(input.IfMatch) == "" {
		return errors.New("IfMatch cannot be empty: deletes require the current ETag")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DeleteDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
	}

	_, err := r.cloudFrontClient().DeleteDistributionTenant(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).Inc()
		}

		return classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteDistributionTenant", ccfg.ResourceTypeCloudFrontDistributionTenant)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}

// DeleteDistributionTenant runs the disable-then-delete sequence CloudFront
// requires. Deleting an already-absent tenant is not an error.
//
// The disable and the delete are issued back to back. If CloudFront has not
// finished applying the disable, the delete comes back as ErrNotDisabled and
// the caller should retry — this mirrors how every other CloudFront config
// change settles asynchronously.
func (r *CloudFrontRepository) DeleteDistributionTenant(identifier string) error {
	tenant, err := r.GetDistributionTenant(identifier)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}

		return err
	}

	if tenant == nil {
		return nil
	}

	if tenant.IsEnabled() {
		log.Debug().
			Str("tenant", tenant.GetId()).
			Msg("[CloudFrontRepository.DeleteDistributionTenant] disabling tenant before deletion")

		// The update response carries the new version, so no re-read is needed
		// to get a usable ETag for the delete.
		if tenant, err = r.SetDistributionTenantEnabled(tenant.GetId(), false); err != nil {
			return err
		}

		if tenant == nil {
			return nil
		}
	}

	err = r.DeleteDistributionTenantByInput(&awscf.DeleteDistributionTenantInput{
		Id:      tenant.Id,
		IfMatch: aws.String(tenant.ETag),
	})
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	return err
}
