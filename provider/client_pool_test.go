package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// provider.ClientPool needs the default client's account, which costs one STS
// GetCallerIdentity. A local httptest server answers it; every other request
// fails the test, which is what proves region clients are built with no AWS
// call of their own.

const localAccount types.AwsAccountID = "111111111111"

var (
	regionErr     = errors.New("operation error STS: GetCallerIdentity, https response error StatusCode: 403, RequestID: 1, api error InvalidClientTokenId: The security token included in the request is invalid.")
	credentialErr = errors.New("operation error EC2: DescribeInstances, failed to refresh cached credentials, api error ExpiredToken: token expired")
)

type fakeSTS struct {
	server  *httptest.Server
	callerN atomic.Int32
}

func newFakeSTS(t *testing.T, account types.AwsAccountID) *fakeSTS {
	t.Helper()

	f := &fakeSTS{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("Action") != "GetCallerIdentity" {
			t.Errorf("unexpected AWS request: %s %s %v", r.Method, r.URL.Path, r.Form)
			http.Error(w, "unexpected request", http.StatusInternalServerError)

			return
		}

		f.callerN.Add(1)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = fmt.Fprintf(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult><Arn>arn:aws:iam::%[1]s:user/test</Arn><UserId>AIDATEST</UserId><Account>%[1]s</Account></GetCallerIdentityResult>
  <ResponseMetadata><RequestId>1</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`, account)
	}))
	t.Cleanup(f.server.Close)

	return f
}

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
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
}

func testPool(t *testing.T) (*ClientPool, *fakeSTS) {
	t.Helper()
	isolateAwsEnv(t)

	sts := newFakeSTS(t, localAccount)
	builder := v3.NewClientBuilder(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIDTEST", "secret", "")),
		config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }),
		config.WithBaseEndpoint(sts.server.URL),
	)

	return NewClientPool(context.Background(), builder), sts
}

func cached(p *ClientPool, accountID types.AwsAccountID, region types.AwsRegion) bool {
	_, ok := p.cachedClient(accountID, region)
	return ok
}

func TestLocalPoolBuildsRegionClientsWithoutSts(t *testing.T) {
	pool, sts := testPool(t)

	clients, err := pool.GetClients("eu-west-1", "us-east-1", "me-south-1", "ap-south-1")
	require.NoError(t, err)
	require.Len(t, clients, 4)

	for _, client := range clients {
		assert.Equal(t, localAccount, client.GetAccountID())
	}

	assert.Same(t, clients[0].Config().Credentials, clients[1].Config().Credentials, "regions share one credentials cache")
	assert.Equal(t, int32(1), sts.callerN.Load(), "one GetCallerIdentity for the default client, none per region")
}

func TestLocalPoolRejectsForeignAccount(t *testing.T) {
	pool, _ := testPool(t)

	client, err := pool.GetClient("222222222222", "eu-west-1")
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "222222222222")
	assert.Contains(t, err.Error(), string(localAccount))
	assert.False(t, cached(pool, "222222222222", "eu-west-1"))

	_, err = pool.GetAccountClients("222222222222", "eu-west-1")
	require.Error(t, err)
}

func TestLocalPoolRemembersRegionFailure(t *testing.T) {
	pool, _ := testPool(t)

	failing, err := pool.GetClient(localAccount, "me-south-1")
	require.NoError(t, err)
	healthy, err := pool.GetClient(localAccount, "eu-west-1")
	require.NoError(t, err)

	require.True(t, failing.ReportFailure(fmt.Errorf("fetch: %w", regionErr)))
	assert.False(t, cached(pool, localAccount, "me-south-1"), "the failed pair's client is evicted")

	client, err := pool.GetClient(localAccount, "me-south-1")
	assert.Nil(t, client, "not rebuilt while the failure is fresh")
	require.Error(t, err)
	assert.ErrorIs(t, err, regionErr, "the remembered failure is replayed")
	assert.False(t, cached(pool, localAccount, "me-south-1"))

	clients, err := pool.GetClients("me-south-1", "eu-west-1")
	require.NoError(t, err)
	require.Len(t, clients, 1, "a sweep skips the remembered pair")
	assert.Same(t, healthy, clients[0])
}

func TestLocalPoolDoesNotRememberCredentialFailure(t *testing.T) {
	pool, _ := testPool(t)

	client, err := pool.GetClient(localAccount, "eu-west-1")
	require.NoError(t, err)

	assert.False(t, client.ReportFailure(credentialErr))

	again, err := pool.GetClient(localAccount, "eu-west-1")
	require.NoError(t, err)
	assert.Same(t, client, again)
}

func TestLocalPoolKeepsClientOnNonRegionError(t *testing.T) {
	pool, _ := testPool(t)

	client, err := pool.GetClient(localAccount, "eu-west-1")
	require.NoError(t, err)

	assert.False(t, client.ReportFailure(errors.New("api error AccessDenied: not authorized")))
	assert.False(t, client.ReportFailure(errors.New("dial tcp: lookup x.eu-west-1.amazonaws.com: no such host")))

	again, err := pool.GetClient(localAccount, "eu-west-1")
	require.NoError(t, err)
	assert.Same(t, client, again)
}

func TestLocalPoolConcurrentReportsAndBuilds(t *testing.T) {
	pool, sts := testPool(t)
	regions := []types.AwsRegion{"eu-west-1", "eu-central-1", "us-east-1", "me-south-1", "ap-east-1"}

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			client, err := pool.GetClient(localAccount, regions[i%len(regions)])
			if err != nil {
				return
			}

			if i%3 == 0 {
				client.ReportFailure(regionErr)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(1), sts.callerN.Load())
}
