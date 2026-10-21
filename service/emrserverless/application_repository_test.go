package emrserverless

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsemr "github.com/aws/aws-sdk-go-v2/service/emrserverless"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Response bodies follow the EMR Serverless API reference (REST-JSON).

type stubResponse struct {
	status    int
	errorType string
	body      string
}

type stubAWS struct {
	t         *testing.T
	responses map[string]stubResponse
}

func (s *stubAWS) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.Path
	if strings.HasPrefix(req.URL.Host, "sts.") {
		key = "sts GetCallerIdentity"
	}

	resp, ok := s.responses[key]
	if !ok {
		s.t.Errorf("unexpected request %q", key)
		resp = stubResponse{status: http.StatusInternalServerError}
	}

	header := http.Header{"Content-Type": {"application/json"}, "X-Amzn-Requestid": {"req-1"}}
	if key == "sts GetCallerIdentity" {
		header.Set("Content-Type", "text/xml")
	}
	if resp.errorType != "" {
		header.Set("X-Amzn-Errortype", resp.errorType)
	}

	return &http.Response{
		StatusCode: resp.status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
		Request:    req,
	}, nil
}

func newTestRepository(t *testing.T, responses map[string]stubResponse) *EMRServerlessRepository {
	t.Helper()

	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/credentials")
	t.Setenv("AWS_PROFILE", "")

	responses["sts GetCallerIdentity"] = stubResponse{status: http.StatusOK, body: `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult><Arn>arn:aws:iam::123456789012:user/test</Arn><UserId>AIDATEST</UserId><Account>123456789012</Account></GetCallerIdentityResult>
</GetCallerIdentityResponse>`}

	client, err := v3.NewClient(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithHTTPClient(&http.Client{Transport: &stubAWS{t: t, responses: responses}}),
		config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }),
	)
	require.NoError(t, err)

	return NewEMRServerlessRepository(context.Background(), client)
}

func appSummaryJSON(id string) string {
	return `{"id":"` + id + `","arn":"arn:aws:emr-serverless:us-east-1:123456789012:/applications/` + id + `","name":"` + id + `",` +
		`"releaseLabel":"emr-7.0.0","type":"Spark","state":"STARTED","createdAt":1700000000,"updatedAt":1700000000}`
}

func appJSON(id string) string {
	return `{"application":{"applicationId":"` + id + `","arn":"arn:aws:emr-serverless:us-east-1:123456789012:/applications/` + id + `","name":"` + id + `",` +
		`"releaseLabel":"emr-7.0.0","type":"Spark","state":"STARTED","createdAt":1700000000,"updatedAt":1700000000}}`
}

func TestListApplicationsByInput_ReturnsDescribedApplications(t *testing.T) {
	repo := newTestRepository(t, map[string]stubResponse{
		"GET /applications":      {status: http.StatusOK, body: `{"applications":[` + appSummaryJSON("app1") + `,` + appSummaryJSON("app2") + `]}`},
		"GET /applications/app1": {status: http.StatusOK, body: appJSON("app1")},
		"GET /applications/app2": {status: http.StatusOK, body: appJSON("app2")},
	})

	apps, err := repo.ListApplicationsByInput(&awsemr.ListApplicationsInput{})

	require.NoError(t, err)
	require.Len(t, apps, 2, "every described application is returned")
	assert.Equal(t, "app1", aws.ToString(apps[0].ApplicationId))
	assert.Equal(t, "app2", aws.ToString(apps[1].ApplicationId))
}

// The describe-error check was inverted: a failed GetApplication dereferenced
// its nil result (panic) and every successfully described application was
// dropped. A failure is now reported.
func TestListApplicationsByInput_ReportsDescribeFailure(t *testing.T) {
	repo := newTestRepository(t, map[string]stubResponse{
		"GET /applications":      {status: http.StatusOK, body: `{"applications":[` + appSummaryJSON("app1") + `,` + appSummaryJSON("app2") + `]}`},
		"GET /applications/app1": {status: http.StatusOK, body: appJSON("app1")},
		"GET /applications/app2": {status: http.StatusNotFound, errorType: "ResourceNotFoundException", body: `{"message":"Application app2 does not exist"}`},
	})

	var (
		apps []Application
		err  error
	)
	require.NotPanics(t, func() { apps, err = repo.ListApplicationsByInput(&awsemr.ListApplicationsInput{}) })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ResourceNotFoundException")
	require.Len(t, apps, 1, "applications described before the failure are kept")
	assert.Equal(t, "app1", aws.ToString(apps[0].ApplicationId))
}
