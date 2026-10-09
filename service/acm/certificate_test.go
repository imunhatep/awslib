package acm

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/stretchr/testify/assert"
)

const testCertificateArn = "arn:aws:acm:us-east-1:123456789012:certificate/0f1e2d3c-4b5a-6978-8a9b-0c1d2e3f4a5b"

type fakeClient struct{}

func (fakeClient) GetRegion() ptypes.AwsRegion       { return "us-east-1" }
func (fakeClient) GetAccountID() ptypes.AwsAccountID { return "123456789012" }

func TestNewCertificate(t *testing.T) {
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	cert := NewCertificate(fakeClient{}, acmtypes.CertificateDetail{
		CertificateArn: aws.String(testCertificateArn),
		DomainName:     aws.String("*.example.com"),
		CreatedAt:      aws.Time(created),
	})

	assert.Equal(t, "0f1e2d3c-4b5a-6978-8a9b-0c1d2e3f4a5b", cert.GetId())
	assert.Equal(t, testCertificateArn, cert.GetArn())
	assert.Equal(t, "*.example.com", cert.GetName())
	assert.Equal(t, ccfg.ResourceTypeAcmCertificate, cert.GetType())
	assert.Equal(t, ptypes.AwsRegion("us-east-1"), cert.GetRegion())
	assert.Equal(t, created, cert.GetCreatedAt())
}

func TestNewCertificateSummary(t *testing.T) {
	summary := NewCertificateSummary(fakeClient{}, acmtypes.CertificateSummary{
		CertificateArn: aws.String(testCertificateArn),
		DomainName:     aws.String("*.example.com"),
		Status:         acmtypes.CertificateStatusIssued,
		InUse:          aws.Bool(true),
	})

	assert.Equal(t, "0f1e2d3c-4b5a-6978-8a9b-0c1d2e3f4a5b", summary.GetId())
	assert.Equal(t, ccfg.ResourceTypeAcmCertificateSummary, summary.GetType())
	assert.True(t, summary.IsIssued())
	assert.True(t, summary.IsInUse())
}

func TestNewCertificateWithoutArn(t *testing.T) {
	cert := NewCertificate(fakeClient{}, acmtypes.CertificateDetail{})

	assert.Empty(t, cert.GetId())
	assert.Empty(t, cert.GetArn())
}

func TestCertificateStatus(t *testing.T) {
	tests := []struct {
		status                  acmtypes.CertificateStatus
		issued, pending, failed bool
	}{
		{acmtypes.CertificateStatusPendingValidation, false, true, false},
		{acmtypes.CertificateStatusIssued, true, false, false},
		{acmtypes.CertificateStatusFailed, false, false, true},
		{acmtypes.CertificateStatusValidationTimedOut, false, false, true},
		{acmtypes.CertificateStatusExpired, false, false, false},
		{acmtypes.CertificateStatusRevoked, false, false, false},
		{acmtypes.CertificateStatusInactive, false, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			cert := Certificate{CertificateDetail: acmtypes.CertificateDetail{Status: tt.status}}

			assert.Equal(t, tt.issued, cert.IsIssued())
			assert.Equal(t, tt.pending, cert.IsPendingValidation())
			assert.Equal(t, tt.failed, cert.IsFailed())
		})
	}
}

func TestCertificateIsInUse(t *testing.T) {
	assert.False(t, Certificate{}.IsInUse())

	inUse := Certificate{CertificateDetail: acmtypes.CertificateDetail{
		InUseBy: []string{"arn:aws:cloudfront::123456789012:distribution-tenant/dt_1"},
	}}
	assert.True(t, inUse.IsInUse())
}

// A wildcard and its apex validate through the same CNAME, so ACM repeats it
// per domain; callers must get it once. Options without a record yet (the
// records appear asynchronously after RequestCertificate) are skipped.
func TestValidationRecords(t *testing.T) {
	shared := &acmtypes.ResourceRecord{
		Name:  aws.String("_x1.example.com."),
		Type:  acmtypes.RecordTypeCname,
		Value: aws.String("_y1.acm-validations.aws."),
	}
	other := &acmtypes.ResourceRecord{
		Name:  aws.String("_x2.other.example.com."),
		Type:  acmtypes.RecordTypeCname,
		Value: aws.String("_y2.acm-validations.aws."),
	}

	cert := Certificate{CertificateDetail: acmtypes.CertificateDetail{
		DomainValidationOptions: []acmtypes.DomainValidation{
			{DomainName: aws.String("*.example.com"), ResourceRecord: shared},
			{DomainName: aws.String("example.com"), ResourceRecord: shared},
			{DomainName: aws.String("other.example.com"), ResourceRecord: other},
			{DomainName: aws.String("pending.example.com")},
			{DomainName: aws.String("unnamed.example.com"), ResourceRecord: &acmtypes.ResourceRecord{}},
		},
	}}

	assert.Equal(t, []acmtypes.ResourceRecord{*shared, *other}, cert.ValidationRecords())
	assert.Empty(t, Certificate{}.ValidationRecords())
}

func TestTags(t *testing.T) {
	assert.Equal(t, map[string]string{}, Certificate{}.GetTags())
	assert.Equal(t, map[string]string{}, CertificateSummary{}.GetTags())

	cert := Certificate{Tags: map[string]string{"managed-by": "hostname-rotation"}}
	assert.Equal(t, "hostname-rotation", cert.GetTagValue("managed-by"))

	// GetTags must hand out a copy, so callers cannot mutate the entity.
	cert.GetTags()["managed-by"] = "changed"
	assert.Equal(t, "hostname-rotation", cert.GetTagValue("managed-by"))
}

func TestTagConversion(t *testing.T) {
	tags := map[string]string{"a": "1", "b": ""}

	assert.Equal(t, tags, tagsToMap(tagsFromMap(tags)))
	assert.Empty(t, tagsFromMap(nil))
	assert.Equal(t, map[string]string{}, tagsToMap(nil))
}
