package cloudfront

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tenantArn = "arn:aws:cloudfront::123456789012:distribution-tenant/dt_1"

// tenantXML is a GetDistributionTenant / GetDistributionTenantByDomain body
// without Tags, as CloudFront returns it although the API reference lists them.
const tenantXML = `<?xml version="1.0" encoding="UTF-8"?>
<DistributionTenant xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/">
  <Id>dt_1</Id>
  <Arn>` + tenantArn + `</Arn>
  <Name>example.com</Name>
  <DistributionId>E1</DistributionId>
  <Domains><member><Domain>*.example.com</Domain><Status>active</Status></member></Domains>
  <Enabled>true</Enabled>
  <Status>Deployed</Status>
</DistributionTenant>`

type tenantStub struct {
	mu     sync.Mutex
	tagged []string // ARNs passed to ListTagsForResource
}

// stubTenantCloudFront serves STS GetCallerIdentity, GetDistributionTenant,
// GetDistributionTenantByDomain and ListTagsForResource. tagStatus decides the
// status of the tag read.
func stubTenantCloudFront(t *testing.T, tagStatus int, tagErrorCode string) (*v3.Client, *tenantStub) {
	t.Helper()

	stub := &tenantStub{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(stsIdentity))
		case r.URL.Path == "/2020-05-31/distribution-tenant/dt_1",
			r.URL.Path == "/2020-05-31/distribution-tenant" && r.URL.Query().Get("domain") == "*.example.com":
			w.Header().Set("Content-Type", "text/xml")
			w.Header().Set("ETag", "E2TAG")
			_, _ = w.Write([]byte(tenantXML))
		case strings.HasSuffix(r.URL.Path, "/tagging"):
			stub.mu.Lock()
			stub.tagged = append(stub.tagged, r.URL.Query().Get("Resource"))
			stub.mu.Unlock()

			w.Header().Set("Content-Type", "text/xml")
			if tagStatus != http.StatusOK {
				w.WriteHeader(tagStatus)
				_, _ = w.Write([]byte(`<ErrorResponse><Error><Type>Sender</Type><Code>` + tagErrorCode + `</Code><Message>no</Message></Error></ErrorResponse>`))
				return
			}
			_, _ = w.Write([]byte(`<Tags><Items><Tag><Key>hnr.io/managed-by</Key><Value>hostname-rotation</Value></Tag></Items></Tags>`))
		default:
			t.Errorf("unexpected request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := v3.NewClient(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return client, stub
}

// CloudFront leaves the tags out of a tenant Get; the repository reads them
// with ListTagsForResource, so GetTagValue answers from the tenant's real tags.
func TestGetDistributionTenantCarriesTags(t *testing.T) {
	reads := map[string]func(*CloudFrontRepository) (*DistributionTenant, error){
		"by identifier": func(r *CloudFrontRepository) (*DistributionTenant, error) { return r.GetDistributionTenant("dt_1") },
		"by domain": func(r *CloudFrontRepository) (*DistributionTenant, error) {
			return r.GetDistributionTenantByDomain("*.example.com")
		},
	}

	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			client, stub := stubTenantCloudFront(t, http.StatusOK, "")

			tenant, err := read(NewCloudFrontRepository(context.Background(), client))
			require.NoError(t, err)
			require.NotNil(t, tenant)

			assert.Equal(t, []string{tenantArn}, stub.tagged, "tags read by the tenant's ARN")
			assert.Equal(t, "hostname-rotation", tenant.GetTagValue("hnr.io/managed-by"))
			assert.Equal(t, "E2TAG", tenant.ETag)
			assert.Equal(t, []string{"*.example.com"}, tenant.DomainNames())
		})
	}
}

// Unreadable tags must not turn into an untagged tenant: that answer is
// indistinguishable from the truth, and ownership checks decide on it.
func TestGetDistributionTenantFailsWhenTagsCannotBeRead(t *testing.T) {
	client, _ := stubTenantCloudFront(t, http.StatusForbidden, "AccessDenied")

	tenant, err := NewCloudFrontRepository(context.Background(), client).GetDistributionTenant("dt_1")
	require.Error(t, err)
	assert.Nil(t, tenant)
}

// A tenant deleted between the read and the tag read is gone, like a read
// that finds nothing.
func TestGetDistributionTenantDeletedBeforeTagRead(t *testing.T) {
	client, _ := stubTenantCloudFront(t, http.StatusNotFound, "NoSuchResource")

	tenant, err := NewCloudFrontRepository(context.Background(), client).GetDistributionTenant("dt_1")
	require.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, tenant)
}
