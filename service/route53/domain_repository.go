package route53

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	domtypes "github.com/aws/aws-sdk-go-v2/service/route53domains/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

func (r *Route53Repository) ListDomainsAll() ([]DomainSummary, error) {
	return r.ListDomainsByInput(&route53domains.ListDomainsInput{})
}

func (r *Route53Repository) ListDomainsByInput(query *route53domains.ListDomainsInput) ([]DomainSummary, error) {
	start := time.Now()

	var domains []DomainSummary
	nextQuery := *query

	for {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListDomains", ccfg.ResourceTypeRoute53DomainSummary)).Inc()
		}

		output, err := r.domainsClient().ListDomains(r.ctx, &nextQuery)
		if err != nil {
			return nil, errors.New(err)
		}

		for _, summary := range output.Domains {
			domains = append(domains, NewDomainSummary(r.client, summary))
		}

		if output.NextPageMarker == nil || aws.ToString(output.NextPageMarker) == "" {
			break
		}

		nextQuery.Marker = output.NextPageMarker
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListDomains", ccfg.ResourceTypeRoute53DomainSummary)).
			Add(float64(len(domains)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDomainsByInput", ccfg.ResourceTypeRoute53DomainSummary)).
			Observe(time.Since(start).Seconds())
	}

	return domains, nil
}

// ListDomainsDetailsByInput lists all domains as full Domain objects. Note that
// this issues one GetDomainDetail + one ListTagsForDomain call per domain, so
// it can produce a large number of API requests on accounts with many domains.
func (r *Route53Repository) ListDomainsDetailsByInput(query *route53domains.ListDomainsInput) ([]Domain, error) {
	summaries, err := r.ListDomainsByInput(query)
	if err != nil {
		return nil, err
	}

	domains := make([]Domain, 0, len(summaries))
	for _, s := range summaries {
		domain, err := r.GetDomainByInput(&route53domains.GetDomainDetailInput{
			DomainName: aws.String(s.GetName()),
		})
		if err != nil {
			return nil, err
		}

		if domain != nil {
			domains = append(domains, *domain)
		}
	}

	return domains, nil
}

func (r *Route53Repository) GetDomainByInput(query *route53domains.GetDomainDetailInput) (*Domain, error) {
	start := time.Now()

	if query.DomainName == nil || aws.ToString(query.DomainName) == "" {
		return nil, errors.New("DomainName cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetDomainDetail", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	output, err := r.domainsClient().GetDomainDetail(r.ctx, query)
	if err != nil {
		return nil, errors.New(err)
	}

	if output.DomainName == nil {
		return nil, nil
	}

	tags := r.GetDomainTags(aws.ToString(output.DomainName))
	domain := NewDomain(r.client, *output, tags)

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetDomainDetail", ccfg.ResourceTypeRoute53Domain)).
			Inc()

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetDomainByInput", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return &domain, nil
}

func (r *Route53Repository) GetDomainTags(domainName string) []domtypes.Tag {
	start := time.Now()

	output, err := r.domainsClient().ListTagsForDomain(r.ctx, &route53domains.ListTagsForDomainInput{
		DomainName: aws.String(domainName),
	})
	if err != nil {
		return nil
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTagsForDomain", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return output.TagList
}

// RegisterDomain registers a new domain and returns its full details.
func (r *Route53Repository) RegisterDomain(input *route53domains.RegisterDomainInput) (*Domain, error) {
	if input.DomainName == nil || aws.ToString(input.DomainName) == "" {
		return nil, errors.New("DomainName cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("RegisterDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	_, err := r.domainsClient().RegisterDomain(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("RegisterDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, errors.New(err)
	}

	domain, err := r.GetDomainByInput(&route53domains.GetDomainDetailInput{
		DomainName: input.DomainName,
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetDomainDetail", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, err
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("RegisterDomain", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return domain, nil
}

// DeleteDomain deletes a registered domain.
func (r *Route53Repository) DeleteDomain(input *route53domains.DeleteDomainInput) error {
	if input.DomainName == nil || aws.ToString(input.DomainName) == "" {
		return errors.New("DomainName cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DeleteDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	_, err := r.domainsClient().DeleteDomain(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteDomain", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}

// ChangeTagsForDomain adds or removes tags for a domain.
func (r *Route53Repository) ChangeTagsForDomain(input *route53domains.UpdateTagsForDomainInput) error {
	if input.DomainName == nil || aws.ToString(input.DomainName) == "" {
		return errors.New("DomainName cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("UpdateTagsForDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	_, err := r.domainsClient().UpdateTagsForDomain(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateTagsForDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ChangeTagsForDomain", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}
