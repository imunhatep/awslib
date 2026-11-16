package resources

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cfgtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clientProxy is fakeProxy with a real client behind GetClient, so the
// provider's ReportFailure call reaches a hook.
type clientProxy struct {
	fakeProxy
	client *v3.Client
}

func (c *clientProxy) GetClient() *v3.Client { return c.client }

// hookedClient builds a client for a known account — no AWS call — and records
// what its region-failure hook receives.
func hookedClient(t *testing.T, accountID ptypes.AwsAccountID, region ptypes.AwsRegion) (*v3.Client, func() []error) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_PROFILE", "")
	require.NoError(t, os.Unsetenv("AWS_PROFILE"))

	client, err := v3.NewClientForAccount(context.Background(), accountID,
		config.WithRegion(region.String()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIDTEST", "secret", "")),
	)
	require.NoError(t, err)

	var (
		mu  sync.Mutex
		got []error
	)

	client.OnRegionFailure(func(err error) {
		mu.Lock()
		defer mu.Unlock()

		got = append(got, err)
	})

	return client, func() []error {
		mu.Lock()
		defer mu.Unlock()

		return append([]error(nil), got...)
	}
}

func TestProviderReportsRegionFailureToClient(t *testing.T) {
	regionErr := errors.New("operation error EC2: DescribeInstances, https response error StatusCode: 401, api error AuthFailure: AWS was not able to validate the provided access credentials")

	client, hooked := hookedClient(t, "222222222222", "me-south-1")
	proxy := &clientProxy{
		fakeProxy: fakeProxy{region: "me-south-1", accountID: "222222222222", err: regionErr},
		client:    client,
	}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, proxy).Run()

	assert.Empty(t, reader.Read())

	failures := reader.Failures()
	require.Len(t, failures, 1, "the failure is still reported to the caller")
	assert.Same(t, regionErr, failures[0].Err)
	assert.Equal(t, ptypes.AwsRegion("me-south-1"), failures[0].Region)

	got := hooked()
	require.Len(t, got, 1, "the client's pool is told")
	assert.Same(t, regionErr, got[0])
}

func TestProviderDoesNotReportOtherFailuresToClient(t *testing.T) {
	denied := errors.New("operation error EC2: DescribeInstances, https response error StatusCode: 403, api error UnauthorizedOperation: You are not authorized to perform this operation.")

	client, hooked := hookedClient(t, "222222222222", "eu-west-1")
	proxy := &clientProxy{
		fakeProxy: fakeProxy{region: "eu-west-1", accountID: "222222222222", err: denied},
		client:    client,
	}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, proxy).Run()

	assert.Empty(t, reader.Read())
	require.Len(t, reader.Failures(), 1)
	assert.Empty(t, hooked(), "access denied is one service's problem, not the region's")
}

// TestProviderDoesNotReportSuccessToClient: a proxy that answers never reaches
// the hook, whatever its client.
func TestProviderDoesNotReportSuccessToClient(t *testing.T) {
	client, hooked := hookedClient(t, "222222222222", "eu-west-1")
	proxy := &clientProxy{
		fakeProxy: fakeProxy{region: "eu-west-1", accountID: "222222222222", resources: []service.ResourceInterface{fakeResource{}}},
		client:    client,
	}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, proxy).Run()

	assert.Len(t, reader.Read(), 1)
	assert.Empty(t, reader.Failures())
	assert.Empty(t, hooked())
}

// TestProviderToleratesNilClient: a proxy with no client (fakes, or a proxy
// whose client was never built) must still report its failure, not panic. The
// fetch runs on the provider's goroutine, so a panic there would abort the test
// binary rather than trip NotPanics; either way the run fails.
func TestProviderToleratesNilClient(t *testing.T) {
	regionErr := errors.New(`request send failed, Post "https://ec2.me-south-1.amazonaws.com/": dial tcp 157.175.0.1:443: i/o timeout`)
	proxy := &fakeProxy{region: "me-south-1", accountID: "222222222222", err: regionErr}

	require.Nil(t, proxy.GetClient())

	var reader *ResourceReader

	require.NotPanics(t, func() {
		reader = NewProvider(cfgtypes.ResourceTypeInstance, proxy).Run()
		reader.Read()
	})

	failures := reader.Failures()
	require.Len(t, failures, 1)
	assert.Same(t, regionErr, failures[0].Err)
}
