package route53

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsr53 "github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests drive the real SDK clients through a stub transport, so they pin
// what goes over the wire. Response bodies follow the AWS API reference for
// each operation (Route 53: REST-XML; Route 53 Domains: JSON 1.1; STS: query).

const testAccountID = "123456789012"

type stubResponse struct {
	status int
	body   string
}

// stubAWS answers the requests the repositories make, keyed by
// "<host prefix> <path or X-Amz-Target>". Anything unexpected fails the test
// and is answered with 500, so no request can leave the process.
type stubAWS struct {
	t         *testing.T
	mu        sync.Mutex
	responses map[string]stubResponse
	requests  []string
}

func (s *stubAWS) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.SplitN(req.URL.Host, ".", 2)[0] + " " + req.URL.EscapedPath()
	if target := req.Header.Get("X-Amz-Target"); target != "" {
		key = strings.SplitN(req.URL.Host, ".", 2)[0] + " " + target
	}
	if strings.HasPrefix(req.URL.Host, "sts.") {
		key = "sts GetCallerIdentity"
	}
	s.requests = append(s.requests, key)

	resp, ok := s.responses[key]
	if !ok {
		s.t.Errorf("unexpected request %q", key)
		resp = stubResponse{status: http.StatusInternalServerError, body: ""}
	}

	contentType := "text/xml"
	if strings.HasPrefix(key, "route53domains ") {
		contentType = "application/x-amz-json-1.1"
	}

	return &http.Response{
		StatusCode: resp.status,
		Header:     http.Header{"Content-Type": {contentType}, "X-Amzn-Requestid": {"req-1"}},
		Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
		Request:    req,
	}, nil
}

func (s *stubAWS) seen(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.requests {
		if r == key {
			return true
		}
	}

	return false
}

const stsIdentityXML = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123456789012:user/test</Arn>
    <UserId>AIDATEST</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`

func zoneTagsXML(zoneID string, tags map[string]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<ListTagsForResourceResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ResourceTagSet>
    <ResourceType>hostedzone</ResourceType>
    <ResourceId>` + zoneID + `</ResourceId>
    <Tags>`)
	for k, v := range tags {
		b.WriteString(`<Tag><Key>` + k + `</Key><Value>` + v + `</Value></Tag>`)
	}
	b.WriteString(`</Tags>
  </ResourceTagSet>
</ListTagsForResourceResponse>`)

	return b.String()
}

func route53ErrorXML(code, message string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<ErrorResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <Error><Type>Sender</Type><Code>` + code + `</Code><Message>` + message + `</Message></Error>
  <RequestId>req-1</RequestId>
</ErrorResponse>`
}

func getHostedZoneXML(zoneID, name string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<GetHostedZoneResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZone>
    <Id>/hostedzone/` + zoneID + `</Id>
    <Name>` + name + `</Name>
    <CallerReference>ref-1</CallerReference>
    <Config><PrivateZone>false</PrivateZone></Config>
    <ResourceRecordSetCount>2</ResourceRecordSetCount>
  </HostedZone>
  <DelegationSet>
    <NameServers><NameServer>ns-1.awsdns-00.com</NameServer></NameServers>
  </DelegationSet>
</GetHostedZoneResponse>`
}

func newTestRepository(t *testing.T, responses map[string]stubResponse) (*Route53Repository, *stubAWS) {
	t.Helper()

	// Keep the developer's own AWS configuration out of the test.
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/credentials")
	t.Setenv("AWS_PROFILE", "")

	stub := &stubAWS{t: t, responses: responses}
	stub.responses["sts GetCallerIdentity"] = stubResponse{status: http.StatusOK, body: stsIdentityXML}

	client, err := v3.NewClient(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithHTTPClient(&http.Client{Transport: stub}),
		config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }),
	)
	require.NoError(t, err)

	return NewRoute53Repository(context.Background(), client), stub
}

func tagMap(tags []types.Tag) map[string]string {
	out := map[string]string{}
	for _, tag := range tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Hosted zone id
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeHostedZoneID(t *testing.T) {
	for in, want := range map[string]string{
		"/hostedzone/Z123": "Z123",
		"hostedzone/Z123":  "Z123",
		"Z123":             "Z123",
		"":                 "",
	} {
		assert.Equal(t, want, NormalizeHostedZoneID(in), in)
	}
}

type fakeAwsClient struct{}

func (fakeAwsClient) GetRegion() ptypes.AwsRegion       { return "us-east-1" }
func (fakeAwsClient) GetAccountID() ptypes.AwsAccountID { return testAccountID }

// The entity's identity is the bare id; the embedded SDK struct keeps the id
// AWS returned. The ARN used to be built from the prefixed id, which produced
// "hostedzone//hostedzone/Z123".
func TestNewHostedZone_IdentityIsBareID(t *testing.T) {
	zone := NewHostedZone(fakeAwsClient{}, types.HostedZone{Id: aws.String("/hostedzone/Z123"), Name: aws.String("example.com.")}, nil, nil, nil)

	assert.Equal(t, "Z123", zone.GetId())
	require.NotNil(t, zone.ARN)
	assert.Equal(t, "hostedzone/Z123", zone.ARN.Resource)
	assert.Equal(t, "/hostedzone/Z123", aws.ToString(zone.HostedZone.Id), "the AWS payload is left as returned")
}

// ─────────────────────────────────────────────────────────────────────────────
// Hosted zone tags
// ─────────────────────────────────────────────────────────────────────────────

// ListTagsForResource takes the bare id; the SDK does not strip "/hostedzone/"
// for the tag operations as it does for the hosted-zone ones.
func TestListHostedZoneTags_SendsBareID(t *testing.T) {
	for _, id := range []string{"/hostedzone/Z123", "Z123"} {
		t.Run(id, func(t *testing.T) {
			repo, stub := newTestRepository(t, map[string]stubResponse{
				"route53 /2013-04-01/tags/hostedzone/Z123": {http.StatusOK, zoneTagsXML("Z123", map[string]string{"hnr.io/managed-by": "hostname-rotation"})},
			})

			tags, err := repo.ListHostedZoneTags(id)

			require.NoError(t, err)
			assert.True(t, stub.seen("route53 /2013-04-01/tags/hostedzone/Z123"))
			assert.Equal(t, map[string]string{"hnr.io/managed-by": "hostname-rotation"}, tagMap(tags))
		})
	}
}

func TestListHostedZoneTags_UntaggedZoneIsEmptyNotNil(t *testing.T) {
	repo, _ := newTestRepository(t, map[string]stubResponse{
		"route53 /2013-04-01/tags/hostedzone/Z123": {http.StatusOK, zoneTagsXML("Z123", nil)},
	})

	tags, err := repo.ListHostedZoneTags("Z123")

	require.NoError(t, err)
	assert.NotNil(t, tags)
	assert.Empty(t, tags)
}

// A failed read is reported instead of being answered with "no tags".
func TestListHostedZoneTags_ReportsFailure(t *testing.T) {
	repo, _ := newTestRepository(t, map[string]stubResponse{
		"route53 /2013-04-01/tags/hostedzone/Z404": {http.StatusNotFound, route53ErrorXML("NoSuchHostedZone", "No hosted zone found with ID: Z404")},
	})

	tags, err := repo.ListHostedZoneTags("/hostedzone/Z404")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchHostedZone")
	assert.Nil(t, tags)
}

func TestListHostedZoneTags_RejectsEmptyID(t *testing.T) {
	repo, stub := newTestRepository(t, map[string]stubResponse{})

	_, err := repo.ListHostedZoneTags("/hostedzone/")

	require.Error(t, err)
	assert.Equal(t, []string{"sts GetCallerIdentity"}, stub.requests, "no tag request is sent")
}

// The deprecated reader keeps its contract — nil on failure — but now sends the
// bare id, so a readable zone's tags actually arrive.
func TestGetHostedZoneTags_Deprecated(t *testing.T) {
	repo, _ := newTestRepository(t, map[string]stubResponse{
		"route53 /2013-04-01/tags/hostedzone/Z123": {http.StatusOK, zoneTagsXML("Z123", map[string]string{"owner": "dev"})},
		"route53 /2013-04-01/tags/hostedzone/Z404": {http.StatusNotFound, route53ErrorXML("NoSuchHostedZone", "missing")},
	})

	assert.Equal(t, map[string]string{"owner": "dev"}, tagMap(repo.GetHostedZoneTags(types.HostedZone{Id: aws.String("/hostedzone/Z123")})))
	assert.Nil(t, repo.GetHostedZoneTags(types.HostedZone{Id: aws.String("/hostedzone/Z404")}))
}

// GetHostedZoneByInput still answers when only the tag read fails, with the
// zone's identity normalised.
func TestGetHostedZoneByInput_Tags(t *testing.T) {
	t.Run("tagged", func(t *testing.T) {
		repo, _ := newTestRepository(t, map[string]stubResponse{
			"route53 /2013-04-01/hostedzone/Z123":      {http.StatusOK, getHostedZoneXML("Z123", "example.com.")},
			"route53 /2013-04-01/tags/hostedzone/Z123": {http.StatusOK, zoneTagsXML("Z123", map[string]string{"owner": "dev"})},
		})

		zone, err := repo.GetHostedZoneByInput(&awsr53.GetHostedZoneInput{Id: aws.String("/hostedzone/Z123")})

		require.NoError(t, err)
		assert.Equal(t, "Z123", zone.GetId())
		assert.Equal(t, "dev", zone.GetTagValue("owner"))
	})

	t.Run("tag read fails", func(t *testing.T) {
		repo, _ := newTestRepository(t, map[string]stubResponse{
			"route53 /2013-04-01/hostedzone/Z123":      {http.StatusOK, getHostedZoneXML("Z123", "example.com.")},
			"route53 /2013-04-01/tags/hostedzone/Z123": {http.StatusBadRequest, route53ErrorXML("Throttling", "Rate exceeded")},
		})

		zone, err := repo.GetHostedZoneByInput(&awsr53.GetHostedZoneInput{Id: aws.String("Z123")})

		require.NoError(t, err)
		assert.Equal(t, "Z123", zone.GetId())
		assert.Empty(t, zone.GetTags())
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain tags
// ─────────────────────────────────────────────────────────────────────────────

const domainsListTags = "route53domains Route53Domains_v20140515.ListTagsForDomain"

func TestListDomainTags(t *testing.T) {
	t.Run("tagged", func(t *testing.T) {
		repo, _ := newTestRepository(t, map[string]stubResponse{
			domainsListTags: {http.StatusOK, `{"TagList":[{"Key":"hnr.io/managed-by","Value":"hostname-rotation"}]}`},
		})

		tags, err := repo.ListDomainTags("example.com")

		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "hnr.io/managed-by", aws.ToString(tags[0].Key))
		assert.Equal(t, "hostname-rotation", aws.ToString(tags[0].Value))
	})

	t.Run("untagged is empty, not nil", func(t *testing.T) {
		repo, _ := newTestRepository(t, map[string]stubResponse{
			domainsListTags: {http.StatusOK, `{"TagList":[]}`},
		})

		tags, err := repo.ListDomainTags("example.com")

		require.NoError(t, err)
		assert.NotNil(t, tags)
		assert.Empty(t, tags)
	})

	t.Run("failure is reported", func(t *testing.T) {
		repo, _ := newTestRepository(t, map[string]stubResponse{
			domainsListTags: {http.StatusBadRequest, `{"__type":"InvalidInput","message":"Domain example.com not found in account"}`},
		})

		tags, err := repo.ListDomainTags("example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "InvalidInput")
		assert.Nil(t, tags)
		assert.Nil(t, repo.GetDomainTags("example.com"), "the deprecated reader still answers nil")
	})

	t.Run("empty name", func(t *testing.T) {
		repo, stub := newTestRepository(t, map[string]stubResponse{})

		_, err := repo.ListDomainTags("")

		require.Error(t, err)
		assert.Equal(t, []string{"sts GetCallerIdentity"}, stub.requests)
	})
}
