package cloudfront

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/stretchr/testify/assert"
)

func TestDomainNames(t *testing.T) {
	domains := []cftypes.DomainResult{
		{Domain: aws.String("a.example.com")},
		{Domain: aws.String("b.example.com")},
		{Domain: nil},
	}

	assert.Equal(t, []string{"a.example.com", "b.example.com", ""}, domainNames(domains))
}

// A tenant is only usable once every attached domain is active, and a tenant
// with no domains has nothing to serve — so it must not read as active.
func TestDomainsActive(t *testing.T) {
	tests := []struct {
		name     string
		domains  []cftypes.DomainResult
		expected bool
	}{
		{"no domains", nil, false},
		{
			"all active",
			[]cftypes.DomainResult{
				{Domain: aws.String("a.example.com"), Status: cftypes.DomainStatusActive},
				{Domain: aws.String("b.example.com"), Status: cftypes.DomainStatusActive},
			},
			true,
		},
		{
			"one inactive",
			[]cftypes.DomainResult{
				{Domain: aws.String("a.example.com"), Status: cftypes.DomainStatusActive},
				{Domain: aws.String("b.example.com"), Status: cftypes.DomainStatusInactive},
			},
			false,
		},
		{
			"status unset",
			[]cftypes.DomainResult{{Domain: aws.String("a.example.com")}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, domainsActive(tt.domains))
		})
	}
}

func TestTagsToMap(t *testing.T) {
	assert.Equal(t, map[string]string{}, tagsToMap(nil))

	tags := &cftypes.Tags{Items: []cftypes.Tag{
		{Key: aws.String("env"), Value: aws.String("prod")},
		{Key: aws.String("owner"), Value: nil},
	}}

	assert.Equal(t, map[string]string{"env": "prod", "owner": ""}, tagsToMap(tags))
}

func TestTenantCertificateStatusHelpers(t *testing.T) {
	none := TenantCertificate{}
	assert.False(t, none.CertificateIssued())
	assert.False(t, none.PendingValidation())
	assert.Nil(t, none.ValidationRedirects())

	issued := TenantCertificate{Certificate: &cftypes.ManagedCertificateDetails{
		CertificateStatus: cftypes.ManagedCertificateStatusIssued,
	}}
	assert.True(t, issued.CertificateIssued())
	assert.False(t, issued.PendingValidation())

	pending := TenantCertificate{Certificate: &cftypes.ManagedCertificateDetails{
		CertificateStatus:      cftypes.ManagedCertificateStatusPendingValidation,
		ValidationTokenDetails: []cftypes.ValidationTokenDetail{{Domain: aws.String("a.example.com")}},
	}}
	assert.False(t, pending.CertificateIssued())
	assert.True(t, pending.PendingValidation())
	assert.Len(t, pending.ValidationRedirects(), 1)
}

func TestConnectionGroupFromSummaryCarriesRoutingFields(t *testing.T) {
	summary := cftypes.ConnectionGroupSummary{
		Arn:             aws.String("arn:aws:cloudfront::123456789012:connection-group/cg-1"),
		Id:              aws.String("cg-1"),
		Name:            aws.String("pool-a"),
		RoutingEndpoint: aws.String("d123abc.cloudfront.net"),
		Enabled:         aws.Bool(true),
		IsDefault:       aws.Bool(false),
	}

	group := connectionGroupFromSummary(summary)

	assert.Equal(t, "cg-1", aws.ToString(group.Id))
	assert.Equal(t, "d123abc.cloudfront.net", aws.ToString(group.RoutingEndpoint))
	assert.True(t, aws.ToBool(group.Enabled))
	assert.False(t, aws.ToBool(group.IsDefault))
}

func TestParseArn(t *testing.T) {
	assert.Nil(t, parseArn(nil))
	assert.Nil(t, parseArn(aws.String("")))
	assert.Nil(t, parseArn(aws.String("not-an-arn")))

	parsed := parseArn(aws.String("arn:aws:cloudfront::123456789012:distribution-tenant/dt-1"))
	if assert.NotNil(t, parsed) {
		assert.Equal(t, "cloudfront", parsed.Service)
		assert.Equal(t, "123456789012", parsed.AccountID)
		assert.Equal(t, "distribution-tenant/dt-1", parsed.Resource)
	}
}
