package iam

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Response bodies follow the IAM and STS API references (query protocol, XML).

// stubIAM answers by the query Action; an unknown action fails the test.
type stubIAM struct {
	t         *testing.T
	responses map[string]stubIAMResponse
}

type stubIAMResponse struct {
	status int
	body   string
}

func (s *stubIAM) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, _ := io.ReadAll(req.Body)
	form, _ := url.ParseQuery(string(raw))
	action := form.Get("Action")

	resp, ok := s.responses[action]
	if !ok {
		s.t.Errorf("unexpected action %q", action)
		resp = stubIAMResponse{status: http.StatusInternalServerError}
	}

	return &http.Response{
		StatusCode: resp.status,
		Header:     http.Header{"Content-Type": {"text/xml"}, "X-Amzn-Requestid": {"req-1"}},
		Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
		Request:    req,
	}, nil
}

const (
	stsIdentityXML = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult><Arn>arn:aws:iam::123456789012:user/test</Arn><UserId>AIDATEST</UserId><Account>123456789012</Account></GetCallerIdentityResult>
</GetCallerIdentityResponse>`

	listRolesXML = `<ListRolesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolesResult>
    <IsTruncated>false</IsTruncated>
    <Roles>
      <member>
        <Path>/</Path>
        <RoleName>app</RoleName>
        <RoleId>AROATEST</RoleId>
        <Arn>arn:aws:iam::123456789012:role/app</Arn>
        <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      </member>
    </Roles>
  </ListRolesResult>
  <ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata>
</ListRolesResponse>`

	accessDeniedXML = `<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <Error><Type>Sender</Type><Code>AccessDenied</Code><Message>User is not authorized to perform: iam:ListRoleTags</Message></Error>
  <RequestId>req-1</RequestId>
</ErrorResponse>`
)

// A listing still returns a resource whose tags could not be read — the
// resource exists either way — but the failure is logged as a warning: an
// empty tag set would otherwise be indistinguishable from an untagged
// resource.
func TestListRolesByInput_WarnsWhenTagsCannotBeRead(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/credentials")
	t.Setenv("AWS_PROFILE", "")

	var logs bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&logs).Level(zerolog.WarnLevel)
	t.Cleanup(func() { log.Logger = previous })

	client, err := v3.NewClient(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithHTTPClient(&http.Client{Transport: &stubIAM{t: t, responses: map[string]stubIAMResponse{
			"GetCallerIdentity": {http.StatusOK, stsIdentityXML},
			"ListRoles":         {http.StatusOK, listRolesXML},
			"ListRoleTags":      {http.StatusForbidden, accessDeniedXML},
		}}}),
		config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }),
	)
	require.NoError(t, err)

	roles, err := NewIamRepository(context.Background(), client).ListRolesByInput(&awsiam.ListRolesInput{})

	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, "app", roles[0].GetName())
	assert.Empty(t, roles[0].GetTags())

	out := logs.String()
	assert.Contains(t, out, `"level":"warn"`)
	assert.Contains(t, out, `"role":"app"`)
	assert.Contains(t, out, "AccessDenied")
	assert.True(t, strings.Contains(out, "[IamRepository.ListRolesByInput] failed to read role tags"), out)
}
