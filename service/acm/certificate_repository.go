package acm

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsacm "github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListCertificatesAll lists every certificate in the region, whatever its
// status or key algorithm.
//
// The key algorithm filter is set explicitly because ListCertificates without
// one returns only RSA_2048 certificates — an ECDSA certificate would silently
// be missing from an "all" listing.
func (r *AcmRepository) ListCertificatesAll() ([]CertificateSummary, error) {
	return r.ListCertificatesByInput(&awsacm.ListCertificatesInput{
		Includes: &acmtypes.Filters{KeyTypes: acmtypes.KeyAlgorithm("").Values()},
	})
}

func (r *AcmRepository) ListCertificatesByInput(query *awsacm.ListCertificatesInput) ([]CertificateSummary, error) {
	start := time.Now()

	var certificates []CertificateSummary

	p := awsacm.NewListCertificatesPaginator(r.acmClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListCertificates", ccfg.ResourceTypeAcmCertificateSummary)).Inc()
		}

		output, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListCertificates", ccfg.ResourceTypeAcmCertificateSummary)).Inc()
			}

			return certificates, classifyErr(err)
		}

		for _, summary := range output.CertificateSummaryList {
			certificates = append(certificates, NewCertificateSummary(r.client, summary))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListCertificates", ccfg.ResourceTypeAcmCertificateSummary)).
			Add(float64(len(certificates)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListCertificatesByInput", ccfg.ResourceTypeAcmCertificateSummary)).
			Observe(time.Since(start).Seconds())
	}

	return certificates, nil
}

// ListCertificatesWithTagsAll lists every certificate together with its tags.
func (r *AcmRepository) ListCertificatesWithTagsAll() ([]CertificateSummary, error) {
	return r.ListCertificatesWithTagsByInput(&awsacm.ListCertificatesInput{
		Includes: &acmtypes.Filters{KeyTypes: acmtypes.KeyAlgorithm("").Values()},
	})
}

// ListCertificatesWithTagsByInput lists certificates and fills in the tags
// each summary is missing.
//
// This is 1+N API calls by necessity — ListCertificates does not return tags.
// The per-certificate reads run concurrently, bounded by tagFetchConcurrency.
// Unlike a missing optional detail, missing tags would make a certificate look
// unowned to a caller selecting by tag, so any failed tag read fails the whole
// listing.
func (r *AcmRepository) ListCertificatesWithTagsByInput(query *awsacm.ListCertificatesInput) ([]CertificateSummary, error) {
	certificates, err := r.ListCertificatesByInput(query)
	if err != nil {
		return nil, err
	}

	errs := make([]error, len(certificates))
	sem := make(chan struct{}, tagFetchConcurrency)

	var wg sync.WaitGroup
	for i := range certificates {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tags, err := r.GetCertificateTags(certificates[idx].GetArn())
			if err != nil {
				errs[idx] = err

				return
			}

			certificates[idx].Tags = tags
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err == nil {
			continue
		}

		// A certificate deleted between the listing and the tag read is gone,
		// not unreadable: drop it rather than fail the listing.
		if errors.Is(err, ErrNotFound) {
			log.Debug().
				Str("certificate", certificates[i].GetArn()).
				Msg("[AcmRepository.ListCertificatesWithTags] certificate deleted during listing")

			continue
		}

		return nil, err
	}

	result := make([]CertificateSummary, 0, len(certificates))
	for i, certificate := range certificates {
		if errs[i] == nil {
			result = append(result, certificate)
		}
	}

	return result, nil
}

func (r *AcmRepository) GetCertificateByInput(query *awsacm.DescribeCertificateInput) (*Certificate, error) {
	start := time.Now()

	if aws.ToString(query.CertificateArn) == "" {
		return nil, errors.New("CertificateArn cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
	}

	output, err := r.acmClient().DescribeCertificate(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DescribeCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.Certificate == nil {
		return nil, nil
	}

	certificate := NewCertificate(r.client, *output.Certificate)

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeCertificate", ccfg.ResourceTypeAcmCertificate)).
			Inc()

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetCertificateByInput", ccfg.ResourceTypeAcmCertificate)).
			Observe(time.Since(start).Seconds())
	}

	return &certificate, nil
}

// GetCertificate reads a certificate by ARN, including its validation records.
// Tags are not part of the response; read them with GetCertificateTags.
func (r *AcmRepository) GetCertificate(certificateArn string) (*Certificate, error) {
	return r.GetCertificateByInput(&awsacm.DescribeCertificateInput{
		CertificateArn: aws.String(certificateArn),
	})
}

// GetCertificateTags reads the tags of one certificate.
func (r *AcmRepository) GetCertificateTags(certificateArn string) (map[string]string, error) {
	start := time.Now()

	if certificateArn == "" {
		return nil, errors.New("CertificateArn cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
	}

	output, err := r.acmClient().ListTagsForCertificate(r.ctx, &awsacm.ListTagsForCertificateInput{
		CertificateArn: aws.String(certificateArn),
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListTagsForCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
		}

		return nil, classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetCertificateTags", ccfg.ResourceTypeAcmCertificate)).
			Observe(time.Since(start).Seconds())
	}

	return tagsToMap(output.Tags), nil
}

// RequestCertificate requests a public certificate and returns its ARN.
//
// The certificate starts in PENDING_VALIDATION. With DNS validation the
// records to write appear on GetCertificate a few seconds later, not in this
// response. Set IdempotencyToken to make a retried request return the same
// certificate instead of issuing a second one.
func (r *AcmRepository) RequestCertificate(input *awsacm.RequestCertificateInput) (string, error) {
	start := time.Now()

	if aws.ToString(input.DomainName) == "" {
		return "", errors.New("DomainName cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("RequestCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
	}

	output, err := r.acmClient().RequestCertificate(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("RequestCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
		}

		return "", classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("RequestCertificate", ccfg.ResourceTypeAcmCertificate)).
			Observe(time.Since(start).Seconds())
	}

	return aws.ToString(output.CertificateArn), nil
}

// AddTagsToCertificate adds or overwrites tags on a certificate.
func (r *AcmRepository) AddTagsToCertificate(certificateArn string, tags map[string]string) error {
	start := time.Now()

	if certificateArn == "" {
		return errors.New("CertificateArn cannot be empty")
	}

	if len(tags) == 0 {
		return nil
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("AddTagsToCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
	}

	_, err := r.acmClient().AddTagsToCertificate(r.ctx, &awsacm.AddTagsToCertificateInput{
		CertificateArn: aws.String(certificateArn),
		Tags:           tagsFromMap(tags),
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("AddTagsToCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
		}

		return classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("AddTagsToCertificate", ccfg.ResourceTypeAcmCertificate)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}

// DeleteCertificateByInput deletes a certificate. ACM rejects the call with
// ErrInUse while any resource still references it — for a CloudFront tenant
// that means until the tenant is deleted, not merely disabled.
func (r *AcmRepository) DeleteCertificateByInput(input *awsacm.DeleteCertificateInput) error {
	start := time.Now()

	if aws.ToString(input.CertificateArn) == "" {
		return errors.New("CertificateArn cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DeleteCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
	}

	_, err := r.acmClient().DeleteCertificate(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteCertificate", ccfg.ResourceTypeAcmCertificate)).Inc()
		}

		return classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteCertificate", ccfg.ResourceTypeAcmCertificate)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}

// DeleteCertificate deletes a certificate by ARN. Deleting an already-absent
// certificate is not an error; a certificate still in use returns ErrInUse.
func (r *AcmRepository) DeleteCertificate(certificateArn string) error {
	err := r.DeleteCertificateByInput(&awsacm.DeleteCertificateInput{
		CertificateArn: aws.String(certificateArn),
	})
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	return err
}
