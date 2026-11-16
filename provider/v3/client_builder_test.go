package v3

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Everything in this file runs offline. Configs carry static credentials and a
// BaseEndpoint pointing at a local httptest server: fakeSTS answers the two STS
// calls the builder may legitimately make, and fails the test on anything else.

const testAccount types.AwsAccountID = "111111111111"

// isolateAwsEnv keeps the developer's ~/.aws and AWS_* environment out of the
// config loader, so a test sees only the providers it passes.
func isolateAwsEnv(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	for _, key := range []string{
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL_STS",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
	} {
		t.Setenv(key, "") // registers the restore
		require.NoError(t, os.Unsetenv(key))
	}
}

// fakeSTS answers GetCallerIdentity with a fixed account and AssumeRole with
// static session credentials, counts every action, and fails the test on any
// other request. With account "" it is a pure tripwire: every request fails.
type fakeSTS struct {
	t       *testing.T
	account string
	server  *httptest.Server

	mu    sync.Mutex
	calls map[string]int
}

func newFakeSTS(t *testing.T, account string) *fakeSTS {
	t.Helper()

	f := &fakeSTS{t: t, account: account, calls: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)

	return f
}

func (f *fakeSTS) serve(w http.ResponseWriter, r *http.Request) {
	action := ""
	if err := r.ParseForm(); err == nil {
		action = r.Form.Get("Action")
	}

	f.mu.Lock()
	f.calls[action]++
	f.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")

	switch {
	case f.account != "" && action == "GetCallerIdentity":
		_, _ = fmt.Fprintf(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::%[1]s:user/test</Arn>
    <UserId>AIDATEST</UserId>
    <Account>%[1]s</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>1</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`, f.account)
	case f.account != "" && action == "AssumeRole":
		_, _ = fmt.Fprint(w, `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>ASIATEST</AccessKeyId>
      <SecretAccessKey>secret</SecretAccessKey>
      <SessionToken>token</SessionToken>
      <Expiration>2099-01-01T00:00:00Z</Expiration>
    </Credentials>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::222222222222:assumed-role/reader/test</Arn>
      <AssumedRoleId>AROATEST:test</AssumedRoleId>
    </AssumedRoleUser>
  </AssumeRoleResult>
  <ResponseMetadata><RequestId>2</RequestId></ResponseMetadata>
</AssumeRoleResponse>`)
	default:
		f.t.Errorf("unexpected AWS request: %s %s action=%q", r.Method, r.URL.Path, action)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}
}

func (f *fakeSTS) count(action string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls[action]
}

func (f *fakeSTS) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0
	for _, c := range f.calls {
		n += c
	}

	return n
}

// testProviders are static credentials, no retries and every endpoint on the
// fake server.
func testProviders(endpoint string, extra ...func(*config.LoadOptions) error) []func(*config.LoadOptions) error {
	providers := []func(*config.LoadOptions) error{
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIDTEST", "secret", "")),
		config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }),
		config.WithBaseEndpoint(endpoint),
	}

	return append(providers, extra...)
}

func testBuilder(t *testing.T, sts *fakeSTS, extra ...func(*config.LoadOptions) error) *ClientBuilder {
	t.Helper()
	isolateAwsEnv(t)

	return NewClientBuilder(context.Background(), testProviders(sts.server.URL, extra...)...)
}

func TestNewClientForAccountMakesNoNetworkCall(t *testing.T) {
	isolateAwsEnv(t)
	tripwire := newFakeSTS(t, "")

	client, err := NewClientForAccount(context.Background(), testAccount,
		testProviders(tripwire.server.URL, config.WithRegion("eu-west-1"))...)
	require.NoError(t, err)

	assert.Equal(t, testAccount, client.GetAccountID())
	assert.Equal(t, types.AwsRegion("eu-west-1"), client.GetRegion())
	assert.Equal(t, "eu-west-1", client.Config().Region)
	assert.Zero(t, tripwire.total(), "building a client for a known account must not call AWS")
}

// TestNewClientForAccountWithoutAccountResolvesIt: an empty account is the
// NewClient path, which is the one place an STS call is expected.
func TestNewClientForAccountWithoutAccountResolvesIt(t *testing.T) {
	isolateAwsEnv(t)
	sts := newFakeSTS(t, "210987654321")

	client, err := NewClientForAccount(context.Background(), "",
		testProviders(sts.server.URL, config.WithRegion("eu-west-1"))...)
	require.NoError(t, err)

	assert.Equal(t, types.AwsAccountID("210987654321"), client.GetAccountID())
	assert.Equal(t, 1, sts.count("GetCallerIdentity"))
}

func TestLocalClientForAccountMakesNoNetworkCall(t *testing.T) {
	tripwire := newFakeSTS(t, "")
	builder := testBuilder(t, tripwire)

	for _, region := range []types.AwsRegion{"eu-west-1", "us-east-1", "me-south-1"} {
		client, err := builder.LocalClientForAccount(testAccount, region)
		require.NoError(t, err)

		assert.Equal(t, testAccount, client.GetAccountID())
		assert.Equal(t, region, client.GetRegion())
		assert.Equal(t, region.String(), client.Config().Region)
	}

	assert.Zero(t, tripwire.total())
	assert.Nil(t, builder.client, "the local path must not build the default client")
}

func TestAssumeClientForAccountNeedsARole(t *testing.T) {
	tripwire := newFakeSTS(t, "")
	builder := testBuilder(t, tripwire)

	_, err := builder.AssumeClientForAccount(testAccount, "", "eu-west-1")

	require.Error(t, err)
	assert.Zero(t, tripwire.total())
}

// TestAssumeClientForAccountAssumesOnFirstUseOnly: building role clients costs
// the default client's one GetCallerIdentity and no AssumeRole; the role is
// assumed when credentials are first needed, once for every region.
func TestAssumeClientForAccountAssumesOnFirstUseOnly(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	builder := testBuilder(t, sts)
	role := types.RoleArn("arn:aws:iam::222222222222:role/reader")

	regions := []types.AwsRegion{"eu-west-1", "us-east-1", "ap-south-1"}
	clients := make([]*Client, 0, len(regions))

	for _, region := range regions {
		client, err := builder.AssumeClientForAccount("222222222222", role, region)
		require.NoError(t, err)

		assert.Equal(t, types.AwsAccountID("222222222222"), client.GetAccountID())
		assert.Equal(t, region, client.GetRegion())

		clients = append(clients, client)
	}

	assert.Equal(t, 1, sts.count("GetCallerIdentity"), "only the default client resolves its identity")
	assert.Zero(t, sts.count("AssumeRole"), "the role is assumed lazily")

	for _, client := range clients {
		creds, err := client.Config().Credentials.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "ASIATEST", creds.AccessKeyID)
	}

	assert.Equal(t, 1, sts.count("AssumeRole"), "every region shares one assumed-role credential cache")
}

// TestRegionClientsShareBaseConfig: region clients are copies of one base, so
// credentials, retryer and HTTP client are the same instances and only the
// region differs.
func TestRegionClientsShareBaseConfig(t *testing.T) {
	tripwire := newFakeSTS(t, "")
	builder := testBuilder(t, tripwire)

	eu, err := builder.LocalClientForAccount(testAccount, "eu-west-1")
	require.NoError(t, err)
	us, err := builder.LocalClientForAccount(testAccount, "us-east-1")
	require.NoError(t, err)

	euCfg, usCfg := eu.Config(), us.Config()

	require.NotNil(t, euCfg.Credentials)
	assert.Same(t, euCfg.Credentials, usCfg.Credentials, "one credentials cache per credential source")
	assert.Same(t, euCfg.HTTPClient, usCfg.HTTPClient, "one HTTP client, so one connection pool")
	assert.Equal(t, reflect.ValueOf(euCfg.Retryer).Pointer(), reflect.ValueOf(usCfg.Retryer).Pointer())
	assert.Equal(t, euCfg.BaseEndpoint, usCfg.BaseEndpoint)

	assert.Equal(t, "eu-west-1", euCfg.Region)
	assert.Equal(t, "us-east-1", usCfg.Region)
	assert.NotSame(t, eu, us)
}

// TestBaseConfigLoadedOncePerSourceConcurrently drives the local and two role
// sources from 50 goroutines. Each source must load exactly one base config and
// hand every region the same credentials, and the default client must be built
// once. Run with -race: this is also the test for loadMu/defaultMu.
func TestBaseConfigLoadedOncePerSourceConcurrently(t *testing.T) {
	var loads atomic.Int32

	sts := newFakeSTS(t, "999999999999")
	builder := testBuilder(t, sts, func(*config.LoadOptions) error {
		loads.Add(1)
		return nil
	})

	roles := []types.RoleArn{"", "arn:aws:iam::222222222222:role/reader", "arn:aws:iam::333333333333:role/reader"}
	regions := []types.AwsRegion{"eu-west-1", "eu-central-1", "us-east-1", "us-west-2", "ap-south-1"}

	type built struct {
		role   types.RoleArn
		client *Client
	}

	results := make(chan built, 50)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			role := roles[i%len(roles)]
			region := regions[i%len(regions)]

			var (
				client *Client
				err    error
			)
			if role == "" {
				client, err = builder.LocalClientForAccount(testAccount, region)
			} else {
				client, err = builder.AssumeClientForAccount(types.AwsAccountID(roleArnAccount(role)), role, region)
			}

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, region, client.GetRegion())
			results <- built{role: role, client: client}
		}(i)
	}

	wg.Wait()
	close(results)

	credsBySource := map[types.RoleArn]aws.CredentialsProvider{}
	count := 0

	for r := range results {
		count++

		creds := r.client.Config().Credentials
		if first, ok := credsBySource[r.role]; ok {
			assert.Same(t, first, creds, "source %q", r.role)
		} else {
			credsBySource[r.role] = creds
		}
	}

	assert.Equal(t, 50, count)
	require.Len(t, credsBySource, 3)
	assert.NotSame(t, credsBySource[roles[0]], credsBySource[roles[1]])
	assert.NotSame(t, credsBySource[roles[1]], credsBySource[roles[2]])

	// Three bases plus the default client the roles need.
	assert.Equal(t, int32(4), loads.Load(), "one config load per credential source, plus DefaultClient")
	assert.Len(t, builder.bases, 3)
	assert.Equal(t, 1, sts.count("GetCallerIdentity"))
	assert.Zero(t, sts.count("AssumeRole"))
}

func TestDefaultHTTPClientTimeouts(t *testing.T) {
	client := DefaultHTTPClient()

	assert.Equal(t, AwsDialTimeout, client.GetDialer().Timeout)
	assert.Equal(t, AwsTLSHandshakeTimeout, client.GetTransport().TLSHandshakeTimeout)
	assert.Equal(t, 5*time.Second, AwsDialTimeout)
	assert.Equal(t, 5*time.Second, AwsTLSHandshakeTimeout)
}

func TestBuilderConfigCarriesDefaultHTTPClient(t *testing.T) {
	tripwire := newFakeSTS(t, "")
	builder := testBuilder(t, tripwire)

	client, err := builder.LocalClientForAccount(testAccount, "eu-west-1")
	require.NoError(t, err)

	httpClient, ok := client.Config().HTTPClient.(*awshttp.BuildableClient)
	require.True(t, ok, "got %T", client.Config().HTTPClient)
	assert.Equal(t, AwsDialTimeout, httpClient.GetDialer().Timeout)
	assert.Equal(t, AwsTLSHandshakeTimeout, httpClient.GetTransport().TLSHandshakeTimeout)
}

// countingTransport proves a caller's HTTP client is the one actually used, not
// merely the one stored on the config.
type countingTransport struct{ requests atomic.Int32 }

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.requests.Add(1)
	return http.DefaultTransport.RoundTrip(r)
}

func TestCallerHTTPClientOverridesDefault(t *testing.T) {
	transport := &countingTransport{}
	custom := &http.Client{Transport: transport}

	sts := newFakeSTS(t, "999999999999")
	builder := testBuilder(t, sts, config.WithHTTPClient(custom))

	local, err := builder.LocalClientForAccount(testAccount, "eu-west-1")
	require.NoError(t, err)
	assert.Same(t, custom, local.Config().HTTPClient)

	defaultClient, err := builder.DefaultClient()
	require.NoError(t, err)
	assert.Same(t, custom, defaultClient.Config().HTTPClient)
	assert.Equal(t, int32(1), transport.requests.Load(), "DefaultClient's GetCallerIdentity went through the caller's client")
}

func TestDefaultClientBuiltOnceConcurrently(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	builder := testBuilder(t, sts)

	clients := make([]*Client, 20)

	var wg sync.WaitGroup
	for i := range clients {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			client, err := builder.DefaultClient()
			assert.NoError(t, err)
			clients[i] = client
		}(i)
	}

	wg.Wait()

	for _, client := range clients {
		assert.Same(t, clients[0], client)
	}

	assert.Equal(t, 1, sts.count("GetCallerIdentity"))
	assert.Equal(t, types.AwsAccountID("999999999999"), clients[0].GetAccountID())
	assert.Equal(t, types.DefaultAwsRegion, clients[0].GetRegion())
}
