package cloudfront

import (
	"bytes"
	"context"
	"encoding/gob"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubClient struct{}

func (stubClient) GetRegion() ptypes.AwsRegion       { return ptypes.AwsRegion("us-east-1") }
func (stubClient) GetAccountID() ptypes.AwsAccountID { return ptypes.AwsAccountID("123456789012") }

func distributionSummary() cftypes.DistributionSummary {
	return cftypes.DistributionSummary{
		Id:         aws.String("E131KALIESDPGZ"),
		ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E131KALIESDPGZ"),
		DomainName: aws.String("d1234.cloudfront.net"),
		Status:     aws.String("Deployed"),
		Enabled:    aws.Bool(true),
		Aliases:    &cftypes.Aliases{Quantity: aws.Int32(2), Items: []string{"www.example.com", "example.com"}},
		Origins: &cftypes.Origins{Quantity: aws.Int32(2), Items: []cftypes.Origin{
			{Id: aws.String("default_origin"), DomainName: aws.String("origin-a.example.com")},
			{Id: aws.String("s3"), DomainName: aws.String("assets.s3.amazonaws.com")},
		}},
	}
}

func TestDistributionEntity(t *testing.T) {
	d := NewDistribution(stubClient{}, distributionSummary(), []cftypes.Tag{
		{Key: aws.String("service"), Value: aws.String("web")},
	})

	assert.Equal(t, "E131KALIESDPGZ", d.GetId())
	assert.Equal(t, "arn:aws:cloudfront::123456789012:distribution/E131KALIESDPGZ", d.GetArn())
	assert.Equal(t, cfg.ResourceTypeDistribution, d.GetType())
	assert.Equal(t, "www.example.com", d.GetName(), "a distribution is known by its first alias")
	assert.Equal(t, []string{"origin-a.example.com", "assets.s3.amazonaws.com"}, d.GetOriginDomainNames())
	assert.Equal(t, map[string]string{"service": "web"}, d.GetTags())

	noAlias := distributionSummary()
	noAlias.Aliases = nil
	assert.Equal(t, "d1234.cloudfront.net", NewDistribution(stubClient{}, noAlias, nil).GetName())
}

func TestDistributionGetTagsReturnsCopy(t *testing.T) {
	d := NewDistribution(stubClient{}, distributionSummary(), []cftypes.Tag{{Key: aws.String("k"), Value: aws.String("v")}})
	d.GetTags()["k"] = "mutated"
	assert.Equal(t, "v", d.GetTags()["k"])

	d.GetAliases()[0] = "mutated"
	assert.Equal(t, "www.example.com", d.GetAliases()[0])
}

func TestDistributionGobRoundTrip(t *testing.T) {
	original := []service.ResourceInterface{NewDistribution(stubClient{}, distributionSummary(), []cftypes.Tag{
		{Key: aws.String("service"), Value: aws.String("web")},
	})}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded []service.ResourceInterface
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))
	require.Len(t, decoded, 1)

	d, ok := decoded[0].(Distribution)
	require.True(t, ok)
	assert.Equal(t, []string{"origin-a.example.com", "assets.s3.amazonaws.com"}, d.GetOriginDomainNames())
	assert.Equal(t, "web", d.GetTagValue("service"))
}

const stsIdentity = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123456789012:user/test</Arn>
    <UserId>AIDATEST</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
</GetCallerIdentityResponse>`

const distributionListXML = `<?xml version="1.0" encoding="UTF-8"?>
<DistributionList xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/">
  <Marker></Marker><MaxItems>100</MaxItems><IsTruncated>false</IsTruncated><Quantity>2</Quantity>
  <Items>
    <DistributionSummary>
      <Id>E1</Id><ARN>arn:aws:cloudfront::123456789012:distribution/E1</ARN>
      <Status>Deployed</Status><DomainName>d1.cloudfront.net</DomainName><Enabled>true</Enabled>
      <Aliases><Quantity>1</Quantity><Items><CNAME>www.example.com</CNAME></Items></Aliases>
      <Origins><Quantity>1</Quantity><Items><Origin><Id>o1</Id><DomainName>origin-1.example.com</DomainName></Origin></Items></Origins>
    </DistributionSummary>
    <DistributionSummary>
      <Id>E2</Id><ARN>arn:aws:cloudfront::123456789012:distribution/E2</ARN>
      <Status>Deployed</Status><DomainName>d2.cloudfront.net</DomainName><Enabled>true</Enabled>
      <Origins><Quantity>1</Quantity><Items><Origin><Id>o2</Id><DomainName>origin-2.example.com</DomainName></Origin></Items></Origins>
    </DistributionSummary>
  </Items>
</DistributionList>`

// tagSweepJSON is a GetResources page tagging only E1: E2 is untagged, and the sweep reports
// it by leaving it out.
const tagSweepJSON = `{"ResourceTagMappingList":[{"ResourceARN":"arn:aws:cloudfront::123456789012:distribution/E1","Tags":[{"Key":"service","Value":"E1"}]}]}`

type cloudFrontStub struct {
	sweepRegions []string // signing region of every GetResources call
	tagged       []string // ARNs passed to ListTagsForResource
}

// stubCloudFront serves STS GetCallerIdentity, ListDistributions, the tagging API's
// GetResources and ListTagsForResource. sweepStatus and tagStatus decide the status of the
// last two; tagStatus is asked per distribution ARN.
func stubCloudFront(t *testing.T, sweepStatus int, tagStatus func(arn string) int) (*v3.Client, *cloudFrontStub) {
	t.Helper()

	var mu sync.Mutex
	stub := &cloudFrontStub{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".GetResources"):
			// Authorization: ... Credential=<key>/<date>/<region>/tagging/aws4_request
			auth := r.Header.Get("Authorization")
			scope := strings.Split(auth[strings.Index(auth, "Credential=")+len("Credential="):], "/")
			mu.Lock()
			stub.sweepRegions = append(stub.sweepRegions, scope[2])
			mu.Unlock()

			w.Header().Set("Content-Type", "application/x-amz-json-1.1")
			if sweepStatus != http.StatusOK {
				w.WriteHeader(sweepStatus)
				_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
				return
			}
			_, _ = w.Write([]byte(tagSweepJSON))
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(stsIdentity))
		case strings.HasSuffix(r.URL.Path, "/distribution"):
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(distributionListXML))
		case strings.HasSuffix(r.URL.Path, "/tagging"):
			arn := r.URL.Query().Get("Resource")
			mu.Lock()
			stub.tagged = append(stub.tagged, arn)
			mu.Unlock()

			w.Header().Set("Content-Type", "text/xml")
			if status := tagStatus(arn); status != http.StatusOK {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`<ErrorResponse><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>denied</Message></Error></ErrorResponse>`))
				return
			}

			id := arn[strings.LastIndex(arn, "/")+1:]
			_, _ = w.Write([]byte(`<Tags><Items><Tag><Key>service</Key><Value>` + id + `</Value></Tag></Items></Tags>`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
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

func allowed(string) int { return http.StatusOK }

// The tagging sweep answers for every distribution at once. Per-distribution
// ListTagsForResource is what CloudFront throttled into failing a 65-distribution listing.
func TestListDistributionsTagsFromOneSweep(t *testing.T) {
	client, stub := stubCloudFront(t, http.StatusOK, allowed)

	items, err := NewCloudFrontRepository(context.Background(), client).ListDistributionsAll()
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Empty(t, stub.tagged, "a successful sweep needs no per-distribution calls")
	assert.Equal(t, []string{"us-east-1"}, stub.sweepRegions,
		"CloudFront tags are only visible to the tagging API in us-east-1, whatever the client region")

	assert.Equal(t, "E1", items[0].GetId())
	assert.Equal(t, "www.example.com", items[0].GetName())
	assert.Equal(t, []string{"origin-1.example.com"}, items[0].GetOriginDomainNames())
	assert.Equal(t, "E1", items[0].GetTagValue("service"))

	assert.Equal(t, "E2", items[1].GetId())
	assert.Empty(t, items[1].GetTags(), "a distribution absent from the sweep is untagged")
}

func TestListDistributionsFallsBackWhenSweepFails(t *testing.T) {
	client, stub := stubCloudFront(t, http.StatusBadRequest, allowed)

	items, err := NewCloudFrontRepository(context.Background(), client).ListDistributionsAll()
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Len(t, stub.tagged, 2, "one ListTagsForResource per distribution")
	assert.Equal(t, "E1", items[0].GetTagValue("service"), "tags must land on their own distribution")
	assert.Equal(t, "E2", items[1].GetTagValue("service"))
}

// An unreadable tag set must not turn into an untagged distribution: that answer is
// indistinguishable from the truth and is what cost attribution reads.
func TestListDistributionsFailsWhenTagsCannotBeRead(t *testing.T) {
	client, _ := stubCloudFront(t, http.StatusBadRequest, func(arn string) int {
		if strings.HasSuffix(arn, "/E2") {
			return http.StatusForbidden
		}

		return http.StatusOK
	})

	items, err := NewCloudFrontRepository(context.Background(), client).ListDistributionsAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "E2")
	assert.Nil(t, items)
}
