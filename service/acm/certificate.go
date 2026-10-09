package acm

import (
	"maps"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// CertificateList holds a list of Certificate items.
type CertificateList struct {
	Items []Certificate
}

// Certificate is a full certificate as returned by DescribeCertificate,
// including the DNS validation records and the resources using it.
//
// DescribeCertificate does not return tags. Tags is nil unless filled in by
// the caller from GetCertificateTags.
type Certificate struct {
	service.AbstractResource
	acmtypes.CertificateDetail
	Tags map[string]string
}

// CertificateSummaryList holds a list of CertificateSummary items.
type CertificateSummaryList struct {
	Items []CertificateSummary
}

// CertificateSummary is the lighter entity returned by ListCertificates. It
// carries status, key algorithm and whether the certificate is in use, but no
// validation records.
//
// ListCertificates does not return tags either: Tags is only filled in by
// ListCertificatesWithTagsByInput.
type CertificateSummary struct {
	service.AbstractResource
	acmtypes.CertificateSummary
	Tags map[string]string
}

func NewCertificate(client AwsClient, detail acmtypes.CertificateDetail) Certificate {
	parsed := parseArn(detail.CertificateArn)

	return Certificate{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        certificateID(parsed),
			ARN:       parsed,
			CreatedAt: aws.ToTime(detail.CreatedAt),
			Type:      ccfg.ResourceTypeAcmCertificate,
		},
		CertificateDetail: detail,
	}
}

func (e Certificate) GetName() string {
	return aws.ToString(e.DomainName)
}

func (e Certificate) GetTags() map[string]string {
	return copyTags(e.Tags)
}

func (e Certificate) GetTagValue(tag string) string {
	return e.Tags[tag]
}

// IsIssued reports whether the certificate can be attached to a resource.
func (e Certificate) IsIssued() bool {
	return e.Status == acmtypes.CertificateStatusIssued
}

// IsPendingValidation reports whether ACM is still waiting for the domain
// validation records to resolve.
func (e Certificate) IsPendingValidation() bool {
	return e.Status == acmtypes.CertificateStatusPendingValidation
}

// IsFailed reports whether the request can no longer be issued: validation
// failed or was not completed within 72 hours. A new certificate must be
// requested.
func (e Certificate) IsFailed() bool {
	return e.Status == acmtypes.CertificateStatusFailed ||
		e.Status == acmtypes.CertificateStatusValidationTimedOut
}

// IsInUse reports whether any resource still references the certificate. ACM
// refuses to delete a certificate in use.
func (e Certificate) IsInUse() bool {
	return len(e.InUseBy) > 0
}

// ValidationRecords returns the DNS records that prove domain ownership, one
// per distinct record name.
//
// A wildcard and its apex validate through the same record, so ACM lists it
// once per domain; it is collapsed here so callers write each record once. The
// records appear asynchronously — for a few seconds after RequestCertificate
// the list can be empty, so callers must poll rather than treat an empty
// result as final.
func (e Certificate) ValidationRecords() []acmtypes.ResourceRecord {
	return validationRecords(e.DomainValidationOptions)
}

func NewCertificateSummary(client AwsClient, summary acmtypes.CertificateSummary) CertificateSummary {
	parsed := parseArn(summary.CertificateArn)

	return CertificateSummary{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        certificateID(parsed),
			ARN:       parsed,
			CreatedAt: aws.ToTime(summary.CreatedAt),
			Type:      ccfg.ResourceTypeAcmCertificateSummary,
		},
		CertificateSummary: summary,
	}
}

func (e CertificateSummary) GetName() string {
	return aws.ToString(e.DomainName)
}

// GetTags returns an empty map unless the summary came from
// ListCertificatesWithTagsByInput: ListCertificates does not return tags.
func (e CertificateSummary) GetTags() map[string]string {
	return copyTags(e.Tags)
}

func (e CertificateSummary) GetTagValue(tag string) string {
	return e.Tags[tag]
}

func (e CertificateSummary) IsIssued() bool {
	return e.Status == acmtypes.CertificateStatusIssued
}

func (e CertificateSummary) IsInUse() bool {
	return aws.ToBool(e.InUse)
}

func validationRecords(options []acmtypes.DomainValidation) []acmtypes.ResourceRecord {
	seen := map[string]bool{}
	records := make([]acmtypes.ResourceRecord, 0, len(options))

	for _, option := range options {
		if option.ResourceRecord == nil {
			continue
		}

		name := aws.ToString(option.ResourceRecord.Name)
		if name == "" || seen[name] {
			continue
		}

		seen[name] = true
		records = append(records, *option.ResourceRecord)
	}

	return records
}

func tagsToMap(tags []acmtypes.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}

func tagsFromMap(tags map[string]string) []acmtypes.Tag {
	out := make([]acmtypes.Tag, 0, len(tags))
	for key, value := range tags {
		out = append(out, acmtypes.Tag{Key: aws.String(key), Value: aws.String(value)})
	}

	return out
}

func copyTags(tags map[string]string) map[string]string {
	if tags == nil {
		return map[string]string{}
	}

	return maps.Clone(tags)
}
