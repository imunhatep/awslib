package cloudfront

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenant2Arn = "arn:aws:cloudfront::123456789012:distribution-tenant/dt_2"
	group1Arn  = "arn:aws:cloudfront::123456789012:connection-group/cg_1"
	group2Arn  = "arn:aws:cloudfront::123456789012:connection-group/cg_2"
)

const tenantListXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListDistributionTenantsResult xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/">
  <DistributionTenantList>
    <DistributionTenantSummary><Id>dt_1</Id><Arn>` + tenantArn + `</Arn><Name>one</Name><DistributionId>E1</DistributionId><ETag>ET1</ETag><Enabled>true</Enabled></DistributionTenantSummary>
    <DistributionTenantSummary><Id>dt_2</Id><Arn>` + tenant2Arn + `</Arn><Name>two</Name><DistributionId>E1</DistributionId><ETag>ET2</ETag><Enabled>true</Enabled></DistributionTenantSummary>
  </DistributionTenantList>
</ListDistributionTenantsResult>`

const groupListXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListConnectionGroupsResult xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/">
  <ConnectionGroups>
    <ConnectionGroupSummary><Id>cg_1</Id><Arn>` + group1Arn + `</Arn><Name>one</Name><ETag>EG1</ETag><RoutingEndpoint>one.example.net</RoutingEndpoint></ConnectionGroupSummary>
    <ConnectionGroupSummary><Id>cg_2</Id><Arn>` + group2Arn + `</Arn><Name>two</Name><ETag>EG2</ETag><RoutingEndpoint>two.example.net</RoutingEndpoint></ConnectionGroupSummary>
  </ConnectionGroups>
</ListConnectionGroupsResult>`

const groupXML = `<?xml version="1.0" encoding="UTF-8"?>
<ConnectionGroup xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/">
  <Id>cg_1</Id><Arn>` + group1Arn + `</Arn><Name>one</Name><Enabled>true</Enabled><RoutingEndpoint>one.example.net</RoutingEndpoint>
</ConnectionGroup>`

// sweepOf renders a GetResources page tagging the given ARNs, each with service=<its ARN>.
func sweepOf(arns ...string) string {
	mappings := make([]map[string]any, 0, len(arns))
	for _, a := range arns {
		mappings = append(mappings, map[string]any{
			"ResourceARN": a,
			"Tags":        []map[string]string{{"Key": "service", "Value": a}},
		})
	}

	out, _ := json.Marshal(map[string]any{"ResourceTagMappingList": mappings})

	return string(out)
}

func cfError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<ErrorResponse><Error><Type>Sender</Type><Code>` + code + `</Code><Message>no</Message></Error></ErrorResponse>`))
}

func cfXML(w http.ResponseWriter, etag, body string) {
	w.Header().Set("Content-Type", "text/xml")
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	_, _ = w.Write([]byte(body))
}

// apiStub records every CloudFront and tagging call and answers them from routes, keyed
// "METHOD path". STS and the tagging sweep are answered by default: the sweep with the page in
// sweep, or with an AccessDeniedException when sweep is empty. ListTagsForResource answers
// service=<ARN> unless a route overrides "GET /2020-05-31/tagging".
type apiStub struct {
	mu           sync.Mutex
	calls        []string
	sweepFilters [][]string
	sweepRegions []string
}

func (s *apiStub) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, call)
}

func (s *apiStub) count(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0
	for _, call := range s.calls {
		if strings.HasPrefix(call, prefix) {
			n++
		}
	}

	return n
}

func newAPIStub(t *testing.T, sweep string, routes map[string]http.HandlerFunc) (*v3.Client, *apiStub) {
	t.Helper()

	stub := &apiStub{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".GetResources") {
			var input struct{ ResourceTypeFilters []string }
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &input)

			auth := r.Header.Get("Authorization")
			scope := strings.Split(auth[strings.Index(auth, "Credential=")+len("Credential="):], "/")

			stub.mu.Lock()
			stub.sweepFilters = append(stub.sweepFilters, input.ResourceTypeFilters)
			stub.sweepRegions = append(stub.sweepRegions, scope[2])
			stub.mu.Unlock()
			stub.record("GetResources")

			w.Header().Set("Content-Type", "application/x-amz-json-1.1")
			if sweep == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
				return
			}
			_, _ = w.Write([]byte(sweep))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/" {
			cfXML(w, "", stsIdentity)
			return
		}

		key := r.Method + " " + r.URL.Path
		if strings.HasSuffix(r.URL.Path, "/tagging") {
			key = "GET /2020-05-31/tagging"
			stub.record(key + " " + r.URL.Query().Get("Resource"))
		} else {
			stub.record(key)
		}

		if route, ok := routes[key]; ok {
			route(w, r)
			return
		}

		if strings.HasPrefix(key, "GET /2020-05-31/tagging") {
			cfXML(w, "", `<Tags><Items><Tag><Key>service</Key><Value>`+r.URL.Query().Get("Resource")+`</Value></Tag></Items></Tags>`)
			return
		}

		t.Errorf("unexpected request %s?%s", key, r.URL.RawQuery)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	// Deliberately not us-east-1: the sweep has to pin its own region.
	client, err := v3.NewClient(context.Background(),
		config.WithRegion("eu-central-1"),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return client, stub
}

func listTenants(w http.ResponseWriter, _ *http.Request) { cfXML(w, "", tenantListXML) }
func listGroups(w http.ResponseWriter, _ *http.Request)  { cfXML(w, "", groupListXML) }
func getTenant(w http.ResponseWriter, _ *http.Request)   { cfXML(w, "E2TAG", tenantXML) }

// Tenants are provisioned in large numbers and the list response carries no tags; one sweep
// answers for all of them, where ListTagsForResource per tenant is what CloudFront throttles.
func TestListDistributionTenantsTagsFromOneSweep(t *testing.T) {
	client, stub := newAPIStub(t, sweepOf(tenantArn, "arn:aws:cloudfront::123456789012:distribution/E1"),
		map[string]http.HandlerFunc{"POST /2020-05-31/distribution-tenants": listTenants})

	tenants, err := NewCloudFrontRepository(context.Background(), client).ListDistributionTenantsAll()
	require.NoError(t, err)
	require.Len(t, tenants, 2)

	assert.Zero(t, stub.count("GET /2020-05-31/tagging"), "a successful sweep needs no per-tenant calls")
	assert.Equal(t, []string{"us-east-1"}, stub.sweepRegions)
	assert.Equal(t, [][]string{{"cloudfront"}}, stub.sweepFilters,
		"the bare service filter is the one observed to report tenant ARNs")

	assert.Equal(t, tenantArn, tenants[0].GetTagValue("service"))
	assert.Empty(t, tenants[1].GetTags(), "a tenant absent from the sweep is untagged")
}

func TestListDistributionTenantsFallsBackWhenSweepFails(t *testing.T) {
	client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
		"POST /2020-05-31/distribution-tenants": listTenants,
	})

	tenants, err := NewCloudFrontRepository(context.Background(), client).ListDistributionTenantsAll()
	require.NoError(t, err)
	require.Len(t, tenants, 2)

	assert.Equal(t, 2, stub.count("GET /2020-05-31/tagging"), "one ListTagsForResource per tenant")
	assert.Equal(t, tenantArn, tenants[0].GetTagValue("service"), "tags must land on their own tenant")
	assert.Equal(t, tenant2Arn, tenants[1].GetTagValue("service"))
}

func TestListDistributionTenantsFailsWhenTagsCannotBeRead(t *testing.T) {
	client, _ := newAPIStub(t, "", map[string]http.HandlerFunc{
		"POST /2020-05-31/distribution-tenants": listTenants,
		"GET /2020-05-31/tagging": func(w http.ResponseWriter, _ *http.Request) {
			cfError(w, http.StatusForbidden, "AccessDenied")
		},
	})

	tenants, err := NewCloudFrontRepository(context.Background(), client).ListDistributionTenantsAll()
	require.Error(t, err)
	assert.Nil(t, tenants)
}

// Connection groups have not been observed in a tagging sweep, so a sweep that reports none
// of them proves nothing: their tags are read per group. Once the sweep reports one, the type
// is indexed and an absent group is untagged.
func TestListConnectionGroupsTrustsSweepOnlyWithEvidence(t *testing.T) {
	routes := map[string]http.HandlerFunc{"POST /2020-05-31/connection-groups": listGroups}

	t.Run("no group in the sweep", func(t *testing.T) {
		client, stub := newAPIStub(t, sweepOf(tenantArn), routes)

		groups, err := NewCloudFrontRepository(context.Background(), client).ListConnectionGroupsAll()
		require.NoError(t, err)
		require.Len(t, groups, 2)

		assert.Equal(t, 2, stub.count("GET /2020-05-31/tagging"))
		assert.Equal(t, group1Arn, groups[0].GetTagValue("service"))
		assert.Equal(t, group2Arn, groups[1].GetTagValue("service"))
		assert.Equal(t, "one.example.net", groups[0].GetRoutingEndpoint())
	})

	t.Run("a group in the sweep", func(t *testing.T) {
		client, stub := newAPIStub(t, sweepOf(group1Arn), routes)

		groups, err := NewCloudFrontRepository(context.Background(), client).ListConnectionGroupsAll()
		require.NoError(t, err)
		require.Len(t, groups, 2)

		assert.Zero(t, stub.count("GET /2020-05-31/tagging"))
		assert.Equal(t, group1Arn, groups[0].GetTagValue("service"))
		assert.Empty(t, groups[1].GetTags())
	})
}

func TestGetConnectionGroupCarriesTags(t *testing.T) {
	client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
		"GET /2020-05-31/connection-group/cg_1": func(w http.ResponseWriter, _ *http.Request) { cfXML(w, "EG1", groupXML) },
	})

	group, err := NewCloudFrontRepository(context.Background(), client).GetConnectionGroup("cg_1")
	require.NoError(t, err)
	require.NotNil(t, group)

	assert.Equal(t, 1, stub.count("GET /2020-05-31/tagging"))
	assert.Equal(t, group1Arn, group.GetTagValue("service"))
}

// A delete needs the ETag and nothing else: no tag read, and no second read of the tenant
// to disable it.
func TestDeleteDistributionTenantReadsNoTags(t *testing.T) {
	client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
		"GET /2020-05-31/distribution-tenant/dt_1": getTenant,
		"PUT /2020-05-31/distribution-tenant/dt_1": getTenant,
		"DELETE /2020-05-31/distribution-tenant/dt_1": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})

	require.NoError(t, NewCloudFrontRepository(context.Background(), client).DeleteDistributionTenant("dt_1"))

	assert.Equal(t, []string{
		"GET /2020-05-31/distribution-tenant/dt_1",
		"PUT /2020-05-31/distribution-tenant/dt_1",
		"DELETE /2020-05-31/distribution-tenant/dt_1",
	}, stub.calls)
}

func TestDeleteConnectionGroupReadsNoTags(t *testing.T) {
	client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
		"GET /2020-05-31/connection-group/cg_1": func(w http.ResponseWriter, _ *http.Request) { cfXML(w, "EG1", groupXML) },
		"PUT /2020-05-31/connection-group/cg_1": func(w http.ResponseWriter, _ *http.Request) { cfXML(w, "EG2", groupXML) },
		"DELETE /2020-05-31/connection-group/cg_1": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})

	require.NoError(t, NewCloudFrontRepository(context.Background(), client).DeleteConnectionGroup("cg_1"))
	assert.Zero(t, stub.count("GET /2020-05-31/tagging"))
	assert.Equal(t, 1, stub.count("GET /2020-05-31/connection-group/cg_1"))
}

// The tags read before the update are carried into the result instead of being read again,
// and the read happens before anything is changed.
func TestSetDistributionTenantEnabledCarriesTags(t *testing.T) {
	t.Run("carried over", func(t *testing.T) {
		client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
			"GET /2020-05-31/distribution-tenant/dt_1": getTenant,
			"PUT /2020-05-31/distribution-tenant/dt_1": getTenant,
		})

		tenant, err := NewCloudFrontRepository(context.Background(), client).SetDistributionTenantEnabled("dt_1", false)
		require.NoError(t, err)
		require.NotNil(t, tenant)

		assert.Equal(t, 1, stub.count("GET /2020-05-31/tagging"))
		assert.Equal(t, tenantArn, tenant.GetTagValue("service"))
	})

	t.Run("unreadable tags stop the write", func(t *testing.T) {
		client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
			"GET /2020-05-31/distribution-tenant/dt_1": getTenant,
			"GET /2020-05-31/tagging": func(w http.ResponseWriter, _ *http.Request) {
				cfError(w, http.StatusForbidden, "AccessDenied")
			},
		})

		_, err := NewCloudFrontRepository(context.Background(), client).SetDistributionTenantEnabled("dt_1", false)
		require.Error(t, err)
		assert.Zero(t, stub.count("PUT "))
	})
}

func TestCreateDistributionTenantCarriesInputTags(t *testing.T) {
	client, stub := newAPIStub(t, "", map[string]http.HandlerFunc{
		"POST /2020-05-31/distribution-tenant": getTenant,
	})

	tenant, err := NewCloudFrontRepository(context.Background(), client).CreateDistributionTenant(&awscf.CreateDistributionTenantInput{
		DistributionId: aws.String("E1"),
		Name:           aws.String("example.com"),
		Domains:        []cftypes.DomainItem{{Domain: aws.String("*.example.com")}},
		Tags:           &cftypes.Tags{Items: []cftypes.Tag{{Key: aws.String("owner"), Value: aws.String("sre")}}},
	})
	require.NoError(t, err)
	require.NotNil(t, tenant)

	assert.Zero(t, stub.count("GET /2020-05-31/tagging"))
	assert.Equal(t, "sre", tenant.GetTagValue("owner"))
}

// A caller-built update has no tags to carry over, so they are read back. A failed read-back
// must not hide that the update was applied: the new ETag comes back with the error.
func TestUpdateDistributionTenantReadsTagsBack(t *testing.T) {
	update := &awscf.UpdateDistributionTenantInput{Id: aws.String("dt_1"), IfMatch: aws.String("E1TAG"), Enabled: aws.Bool(false)}

	t.Run("read back", func(t *testing.T) {
		client, _ := newAPIStub(t, "", map[string]http.HandlerFunc{
			"PUT /2020-05-31/distribution-tenant/dt_1": getTenant,
		})

		tenant, err := NewCloudFrontRepository(context.Background(), client).UpdateDistributionTenant(update)
		require.NoError(t, err)
		assert.Equal(t, tenantArn, tenant.GetTagValue("service"))
	})

	t.Run("read-back fails", func(t *testing.T) {
		client, _ := newAPIStub(t, "", map[string]http.HandlerFunc{
			"PUT /2020-05-31/distribution-tenant/dt_1": getTenant,
			"GET /2020-05-31/tagging": func(w http.ResponseWriter, _ *http.Request) {
				cfError(w, http.StatusForbidden, "AccessDenied")
			},
		})

		tenant, err := NewCloudFrontRepository(context.Background(), client).UpdateDistributionTenant(update)
		require.Error(t, err)
		require.NotNil(t, tenant)
		assert.Equal(t, "E2TAG", tenant.ETag)
	})
}

// No managed certificate is a valid answer; a certificate read that failed for any other
// reason is not, and must not be reported as one.
func TestListDistributionTenantsWithCertificates(t *testing.T) {
	routes := func(dt2 http.HandlerFunc) map[string]http.HandlerFunc {
		return map[string]http.HandlerFunc{
			"POST /2020-05-31/distribution-tenants": listTenants,
			"GET /2020-05-31/managed-certificate/dt_1": func(w http.ResponseWriter, _ *http.Request) {
				cfXML(w, "", `<ManagedCertificateDetails><CertificateStatus>issued</CertificateStatus></ManagedCertificateDetails>`)
			},
			"GET /2020-05-31/managed-certificate/dt_2": dt2,
		}
	}

	t.Run("no managed certificate", func(t *testing.T) {
		client, _ := newAPIStub(t, sweepOf(tenantArn), routes(func(w http.ResponseWriter, _ *http.Request) {
			cfError(w, http.StatusNotFound, "EntityNotFound")
		}))

		results, err := NewCloudFrontRepository(context.Background(), client).ListDistributionTenantsWithCertificatesAll()
		require.NoError(t, err)
		require.Len(t, results, 2)

		assert.True(t, results[0].CertificateIssued())
		assert.Nil(t, results[1].Certificate)
		assert.Equal(t, tenantArn, results[0].Tenant.GetTagValue("service"), "tenants keep the tags of the listing")
	})

	t.Run("certificate read fails", func(t *testing.T) {
		client, _ := newAPIStub(t, sweepOf(tenantArn), routes(func(w http.ResponseWriter, _ *http.Request) {
			cfError(w, http.StatusForbidden, "AccessDenied")
		}))

		results, err := NewCloudFrontRepository(context.Background(), client).ListDistributionTenantsWithCertificatesAll()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dt_2")
		assert.Nil(t, results)
	})
}
