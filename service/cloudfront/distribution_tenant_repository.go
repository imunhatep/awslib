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

// ListDistributionTenantsByInput lists tenants together with their tags.
//
// The list response carries no tags, so they come from one tagging sweep for the
// whole account (see bulkTags) rather than a ListTagsForResource per tenant —
// tenants exist to be provisioned in large numbers, and CloudFront throttles the
// per-resource call hard. A tenant whose tags cannot be read fails the listing
// instead of being returned untagged.
func (r *CloudFrontRepository) ListDistributionTenantsByInput(query *awscf.ListDistributionTenantsInput) ([]DistributionTenantSummary, error) {
	start := time.Now()

	summaries, err := r.listDistributionTenantSummaries(query)
	if err != nil {
		return nil, err
	}

	arns := make([]string, len(summaries))
	for i, summary := range summaries {
		arns[i] = aws.ToString(summary.Arn)
	}

	tags, err := r.bulkTags(arns, distributionTenantTagSource)
	if err != nil {
		return nil, err
	}

	tenants := make([]DistributionTenantSummary, 0, len(summaries))
	for i, summary := range summaries {
		tenant := NewDistributionTenantSummary(r.client, summary)
		tenant.Tags = tags[i]

		tenants = append(tenants, tenant)
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

func (r *CloudFrontRepository) listDistributionTenantSummaries(query *awscf.ListDistributionTenantsInput) ([]cftypes.DistributionTenantSummary, error) {
	var summaries []cftypes.DistributionTenantSummary

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

			return nil, classifyErr(err)
		}

		summaries = append(summaries, output.DistributionTenantList...)
	}

	return summaries, nil
}

// GetDistributionTenantByInput reads a tenant together with its tags.
func (r *CloudFrontRepository) GetDistributionTenantByInput(query *awscf.GetDistributionTenantInput) (*DistributionTenant, error) {
	return r.getDistributionTenant(query, true)
}

// getDistributionTenant reads a tenant, with its tags when withTags is set. The
// delete sequence reads without them: it needs only the current ETag, and the tag
// read would cost a call and require cloudfront:ListTagsForResource for nothing.
func (r *CloudFrontRepository) getDistributionTenant(query *awscf.GetDistributionTenantInput, withTags bool) (*DistributionTenant, error) {
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

	read := *output.DistributionTenant
	if withTags {
		if read, err = r.withTenantTags(read); err != nil {
			return nil, err
		}
	}

	tenant := NewDistributionTenant(r.client, read, aws.ToString(output.ETag))

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

	tagged, err := r.withTenantTags(*output.DistributionTenant)
	if err != nil {
		return nil, err
	}

	tenant := NewDistributionTenant(r.client, tagged, aws.ToString(output.ETag))

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

// withTenantTags fills in the tags of a tenant read with GetDistributionTenant
// or GetDistributionTenantByDomain. The API reference lists Tags in both
// responses, but CloudFront leaves them out, so they are read with
// ListTagsForResource whenever the response carries none.
//
// A tenant whose tags cannot be read fails the read instead of coming back
// untagged: an empty tag set is indistinguishable from "this tenant is
// untagged", and ownership checks decide on it. A tenant deleted between the
// two calls is ErrNotFound, like the read itself.
func (r *CloudFrontRepository) withTenantTags(tenant cftypes.DistributionTenant) (cftypes.DistributionTenant, error) {
	tags, err := r.completeTags(
		"distribution tenant "+aws.ToString(tenant.Id),
		aws.ToString(tenant.Arn),
		tenant.Tags,
		ccfg.ResourceTypeCloudFrontDistributionTenant,
	)
	if err != nil {
		return tenant, err
	}

	tenant.Tags = tags

	return tenant, nil
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
	}, func(o *awscf.Options) { o.RetryMaxAttempts = certificateFetchMaxAttempts })
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
// certificate state, and there is no bulk read for it. The per-tenant reads run
// concurrently, bounded by certificateFetchConcurrency, each retried up to
// certificateFetchMaxAttempts times because CloudFront throttles per-resource
// reads hard. Tags are read in bulk by the listing, not per tenant.
//
// A tenant without a managed certificate (CloudFront answers EntityNotFound: it
// inherits the distribution's certificate or brings its own) is returned with a
// nil Certificate. Any other failure — throttling past the retries, a denied
// read — fails the listing: a nil Certificate is what "no managed certificate"
// looks like, and reporting a throttled read that way would be a wrong answer
// rather than a missing one.
func (r *CloudFrontRepository) ListDistributionTenantsWithCertificatesByInput(query *awscf.ListDistributionTenantsInput) ([]TenantCertificate, error) {
	tenants, err := r.ListDistributionTenantsByInput(query)
	if err != nil {
		return nil, err
	}

	results := make([]TenantCertificate, len(tenants))
	errs := make([]error, len(tenants))
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
			if errors.Is(err, ErrNotFound) {
				log.Debug().
					Str("tenant", t.GetId()).
					Msg("[CloudFrontRepository.ListDistributionTenantsWithCertificates] no managed certificate")

				return
			}

			results[idx].Certificate, errs[idx] = details, err
		}(i, tenant)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, errors.Errorf("reading managed certificate of distribution tenant %s: %w", tenants[i].GetId(), err)
		}
	}

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

	// A create applies exactly the tags it was sent, so those stand in for a
	// response that leaves them out, without reading them back.
	created := *output.DistributionTenant
	created.Tags = carryTags(created.Tags, input.Tags)

	tenant := NewDistributionTenant(r.client, created, aws.ToString(output.ETag))

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
//
// The caller holds no tags to carry over, so when the response leaves them out
// they are read back with one ListTagsForResource. If that read fails the update
// has still been applied: the tenant is returned with its new ETag but without
// tags, alongside the error. SetDistributionTenantEnabled and
// UpdateDistributionTenantDomains avoid the read-back by carrying over the tags
// of the tenant they read first.
func (r *CloudFrontRepository) UpdateDistributionTenant(input *awscf.UpdateDistributionTenantInput) (*DistributionTenant, error) {
	tenant, err := r.updateDistributionTenant(input, nil)
	if err != nil || tenant == nil {
		return tenant, err
	}

	tagged, err := r.withTenantTags(tenant.DistributionTenant)
	if err != nil {
		return tenant, errors.Errorf("distribution tenant %s was updated (ETag %s), but its tags could not be read: %w", tenant.GetId(), tenant.ETag, err)
	}

	tenant.DistributionTenant = tagged

	return tenant, nil
}

// updateDistributionTenant sends an update. An update cannot change tags, so
// known — the tags of the tenant as read before the update — stand in for a
// response that leaves them out. nil known leaves them out, for the delete
// sequence, which needs only the new ETag.
func (r *CloudFrontRepository) updateDistributionTenant(input *awscf.UpdateDistributionTenantInput, known *cftypes.Tags) (*DistributionTenant, error) {
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

	updated := *output.DistributionTenant
	updated.Tags = carryTags(updated.Tags, known)

	tenant := NewDistributionTenant(r.client, updated, aws.ToString(output.ETag))

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
//
// The first read includes the tags, and the returned tenant carries them over
// rather than reading them again after the update. Reading them up front means a
// tag read that fails stops the call before anything is changed.
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

	return r.updateDistributionTenant(&awscf.UpdateDistributionTenantInput{
		Id:      tenant.Id,
		IfMatch: aws.String(tenant.ETag),
		Enabled: aws.Bool(enabled),
	}, tenant.DistributionTenant.Tags)
}

// UpdateDistributionTenantDomains replaces the set of hostnames a tenant
// serves, reading the current ETag first. Tags are handled as in
// SetDistributionTenantEnabled.
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

	return r.updateDistributionTenant(&awscf.UpdateDistributionTenantInput{
		Id:      tenant.Id,
		IfMatch: aws.String(tenant.ETag),
		Domains: items,
	}, tenant.DistributionTenant.Tags)
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
//
// Tags play no part in a delete, so none are read: the sequence is one read,
// at most one update and the delete, and it needs no
// cloudfront:ListTagsForResource permission.
func (r *CloudFrontRepository) DeleteDistributionTenant(identifier string) error {
	tenant, err := r.getDistributionTenant(&awscf.GetDistributionTenantInput{
		Identifier: aws.String(identifier),
	}, false)
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
		if tenant, err = r.updateDistributionTenant(&awscf.UpdateDistributionTenantInput{
			Id:      tenant.Id,
			IfMatch: aws.String(tenant.ETag),
			Enabled: aws.Bool(false),
		}, nil); err != nil {
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
