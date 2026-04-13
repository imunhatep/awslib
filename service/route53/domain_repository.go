package route53

import (
	"strings"
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

// ListOperations returns all route53domains operations matching the given type
// and status filters, iterating through all result pages.  Pass nil slices to
// fetch operations of any type / status.
func (r *Route53Repository) ListOperations(opTypes []domtypes.OperationType, statuses []domtypes.OperationStatus) ([]OperationInfo, error) {
	input := &route53domains.ListOperationsInput{}
	if len(opTypes) > 0 {
		input.Type = opTypes
	}
	if len(statuses) > 0 {
		input.Status = statuses
	}

	var results []OperationInfo
	for {
		out, err := r.domainsClient().ListOperations(r.ctx, input)
		if err != nil {
			return nil, errors.New(err)
		}

		for _, op := range out.Operations {
			results = append(results, OperationInfo{
				OperationID: aws.ToString(op.OperationId),
				Status:      op.Status,
				Type:        op.Type,
				DomainName:  aws.ToString(op.DomainName),
			})
		}

		if out.NextPageMarker == nil || aws.ToString(out.NextPageMarker) == "" {
			break
		}
		input.Marker = out.NextPageMarker
	}

	return results, nil
}

// FindInProgressRegistration returns the operationID of an in-progress
// REGISTER_DOMAIN operation for domainName, or ("", nil) when none is found.
func (r *Route53Repository) FindInProgressRegistration(domainName string) (string, error) {
	ops, err := r.ListOperations(
		[]domtypes.OperationType{domtypes.OperationTypeRegisterDomain},
		[]domtypes.OperationStatus{domtypes.OperationStatusInProgress},
	)

	if err != nil {
		return "", err
	}

	for _, op := range ops {
		if strings.EqualFold(op.DomainName, domainName) {
			return op.OperationID, nil
		}
	}

	return "", nil
}

// RegisterDomain registers a new domain and returns its full details.
func (r *Route53Repository) RegisterDomain(input *route53domains.RegisterDomainInput) (*Domain, *string, error) {
	if input.DomainName == nil || aws.ToString(input.DomainName) == "" {
		return nil, nil, errors.New("DomainName cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("RegisterDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	registrationOp, err := r.domainsClient().RegisterDomain(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("RegisterDomain", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, nil, errors.New(err)
	}

	domain, err := r.GetDomainByInput(&route53domains.GetDomainDetailInput{
		DomainName: input.DomainName,
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetDomainDetail", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, registrationOp.OperationId, err
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("RegisterDomain", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return domain, registrationOp.OperationId, nil
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

// CheckDomainAvailability checks the availability of a domain name.
func (r *Route53Repository) CheckDomainAvailability(input *route53domains.CheckDomainAvailabilityInput) (*route53domains.CheckDomainAvailabilityOutput, error) {
	if input.DomainName == nil || aws.ToString(input.DomainName) == "" {
		return nil, errors.New("DomainName cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CheckDomainAvailability", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	output, err := r.domainsClient().CheckDomainAvailability(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CheckDomainAvailability", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CheckDomainAvailability", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// GetOperationDetail returns the current status of an operation that is not completed.
func (r *Route53Repository) GetOperationDetail(input *route53domains.GetOperationDetailInput) (*route53domains.GetOperationDetailOutput, error) {
	// OperationId is required
	if input.OperationId == nil || aws.ToString(input.OperationId) == "" {
		return nil, errors.New("OperationId cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetOperationDetail", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	output, err := r.domainsClient().GetOperationDetail(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetOperationDetail", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetOperationDetail", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// UpdateDomainNameservers replaces the nameservers for the domain with the specified nameservers.
func (r *Route53Repository) UpdateDomainNameservers(input *route53domains.UpdateDomainNameserversInput) (*route53domains.UpdateDomainNameserversOutput, error) {
	if input.DomainName == nil || aws.ToString(input.DomainName) == "" {
		return nil, errors.New("DomainName cannot be empty")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("UpdateDomainNameservers", ccfg.ResourceTypeRoute53Domain)).Inc()
	}

	output, err := r.domainsClient().UpdateDomainNameservers(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateDomainNameservers", ccfg.ResourceTypeRoute53Domain)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("UpdateDomainNameservers", ccfg.ResourceTypeRoute53Domain)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}
