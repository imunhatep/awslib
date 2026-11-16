package v3

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/imunhatep/awslib/provider/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These drive v3.ClientPool against fakeSTS (client_builder_test.go): the only
// network a pool may touch while building clients is the default client's one
// GetCallerIdentity, and that is answered locally.

var (
	regionErr     = errors.New(errInvalidClientTokenID)
	credentialErr = errors.New("operation error EC2: DescribeInstances, failed to refresh cached credentials, api error ExpiredToken: token expired")
	accessDenied  = errors.New("operation error EC2: DescribeInstances, https response error StatusCode: 403, api error UnauthorizedOperation: You are not authorized to perform this operation.")
)

func testRolePool(t *testing.T, sts *fakeSTS) *ClientPool {
	t.Helper()

	return NewClientPool(t.Context(), testBuilder(t, sts), testRoles())
}

func hasCachedClient(p *ClientPool, accountID types.AwsAccountID, region types.AwsRegion) bool {
	_, ok := p.cachedClient(accountID, region)
	return ok
}

// TestPoolBuildsRoleClientsWithoutAssuming: a sweep over two accounts and three
// regions makes one GetCallerIdentity (the default client the roles are assumed
// from) and no AssumeRole; each client carries its account from the role map.
func TestPoolBuildsRoleClientsWithoutAssuming(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts)

	clients, err := pool.GetClients("eu-west-1", "us-east-1", "me-south-1")
	require.NoError(t, err)
	require.Len(t, clients, 6)

	seen := map[types.AwsAccountID]int{}
	for _, client := range clients {
		seen[client.GetAccountID()]++
	}

	assert.Equal(t, map[types.AwsAccountID]int{"111111111111": 3, "222222222222": 3}, seen)
	assert.Equal(t, 1, sts.count("GetCallerIdentity"))
	assert.Zero(t, sts.count("AssumeRole"))
}

func TestPoolRemembersRegionFailureReportedByClient(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts)

	failing, err := pool.GetClient("111111111111", "me-south-1")
	require.NoError(t, err)
	healthy, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)

	require.True(t, failing.ReportFailure(regionErr))

	assert.False(t, hasCachedClient(pool, "111111111111", "me-south-1"), "the failed pair's client is evicted")

	client, err := pool.GetClient("111111111111", "me-south-1")
	assert.Nil(t, client, "the pair is not rebuilt while the failure is fresh")
	assert.Same(t, regionErr, err, "the remembered failure is replayed")
	assert.False(t, hasCachedClient(pool, "111111111111", "me-south-1"))

	again, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)
	assert.Same(t, healthy, again, "other regions of the account are untouched")

	other, err := pool.GetClient("222222222222", "me-south-1")
	require.NoError(t, err, "the same region in another account is untouched")
	assert.Equal(t, types.AwsAccountID("222222222222"), other.GetAccountID())

	clients, err := pool.GetAccountClients("111111111111", "me-south-1", "eu-west-1")
	require.NoError(t, err)
	require.Len(t, clients, 1, "a sweep skips the remembered pair")
	assert.Same(t, healthy, clients[0])
}

func TestPoolRegionFailureLapsesAndRebuilds(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts).WithFailureTTL(20 * time.Millisecond)

	first, err := pool.GetClient("111111111111", "me-south-1")
	require.NoError(t, err)
	require.True(t, first.ReportFailure(regionErr))

	_, err = pool.GetClient("111111111111", "me-south-1")
	require.Error(t, err)

	time.Sleep(40 * time.Millisecond)

	second, err := pool.GetClient("111111111111", "me-south-1")
	require.NoError(t, err, "a lapsed failure lets the pair be tried again")
	assert.NotSame(t, first, second)
}

func TestPoolDoesNotRememberCredentialFailure(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts)

	client, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)

	assert.False(t, client.ReportFailure(credentialErr))

	again, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err, "re-authenticating must take effect immediately")
	assert.Same(t, client, again)
}

func TestPoolKeepsClientOnNonRegionError(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts)

	client, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)

	for _, err := range []error{
		accessDenied,
		errors.New(`dial tcp: lookup emr-serverless.eu-west-1.amazonaws.com: no such host`),
		errors.New("api error ThrottlingException: Rate exceeded"),
	} {
		assert.False(t, client.ReportFailure(err), "%v", err)
	}

	again, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)
	assert.Same(t, client, again)
}

// TestPoolLocalModeRejectsForeignAccount: with no roles the pool serves the
// default credentials' account only, and builds those clients with no STS call
// beyond the default client's own.
func TestPoolLocalModeRejectsForeignAccount(t *testing.T) {
	sts := newFakeSTS(t, string(testAccount))
	pool := NewClientPool(t.Context(), testBuilder(t, sts), nil)

	client, err := pool.GetClient("222222222222", "eu-west-1")
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "222222222222")
	assert.Contains(t, err.Error(), string(testAccount))

	clients, err := pool.GetClients("eu-west-1", "us-east-1")
	require.NoError(t, err)
	require.Len(t, clients, 2)

	for _, c := range clients {
		assert.Equal(t, testAccount, c.GetAccountID())
	}

	assert.Equal(t, 1, sts.count("GetCallerIdentity"))

	require.True(t, clients[0].ReportFailure(regionErr))
	_, err = pool.GetClient(testAccount, clients[0].GetRegion())
	assert.Same(t, regionErr, err, "local mode remembers region failures too")
}

// TestPoolRoleAccountMustMatchRoleArn: the role map's key labels the client,
// with no GetCallerIdentity to correct it, so a map entry whose key and ARN
// disagree (a typo in accountID=roleArn) must be refused rather than attribute
// every resource fetched with it to the wrong account.
func TestPoolRoleAccountMustMatchRoleArn(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := NewClientPool(t.Context(), testBuilder(t, sts), map[types.AwsAccountID]types.RoleArn{
		"111111111111": "arn:aws:iam::222222222222:role/reader",
	})

	client, err := pool.GetClient("111111111111", "eu-west-1")
	require.Error(t, err, "a client must not claim an account its credentials do not reach")
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "222222222222")
	assert.Equal(t, 0, sts.count("AssumeRole"), "refused before any credentials are assumed")
}

// TestPoolConcurrentReportsAndBuilds exercises the hook, eviction and failure
// cache from many goroutines; its value is under -race.
func TestPoolConcurrentReportsAndBuilds(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts)

	regions := []types.AwsRegion{"eu-west-1", "eu-central-1", "us-east-1", "me-south-1", "ap-east-1"}

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			account := types.AwsAccountID("111111111111")
			if i%2 == 1 {
				account = "222222222222"
			}

			client, err := pool.GetClient(account, regions[i%len(regions)])
			if err != nil {
				return // already remembered by another goroutine
			}

			if i%3 == 0 {
				client.ReportFailure(regionErr)
			}
		}(i)
	}

	wg.Wait()

	assert.Zero(t, sts.count("AssumeRole"))
}
