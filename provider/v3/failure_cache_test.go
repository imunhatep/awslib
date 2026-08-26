package v3

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailureCacheRemembersWithinTTL(t *testing.T) {
	c := NewFailureCache(time.Minute)
	boom := errors.New("InvalidClientTokenId")

	assert.NoError(t, c.Err("111111111111", "me-south-1"), "nothing remembered yet")

	c.Add("111111111111", "me-south-1", boom)

	require.Error(t, c.Err("111111111111", "me-south-1"))
	assert.Equal(t, boom, c.Err("111111111111", "me-south-1"), "the original error is replayed")
}

// TestFailureCacheExpires: even a region-scoped failure eventually lapses, so an
// account that opts into a region is picked up without restarting the server.
func TestFailureCacheExpires(t *testing.T) {
	c := NewFailureCache(20 * time.Millisecond)
	// A region-scoped error on purpose: a credential error is not cached at all.
	c.Add("111111111111", "eu-central-1", errors.New("InvalidClientTokenId"))

	require.Error(t, c.Err("111111111111", "eu-central-1"))

	time.Sleep(40 * time.Millisecond)

	assert.NoError(t, c.Err("111111111111", "eu-central-1"), "the entry must lapse")
}

// TestFailureCacheSkipsCredentialFailures is what licenses the long TTL. A
// credential failure hits every region at once and is fixed by re-authenticating,
// so caching it would turn `aws sso login` into a TTL-long outage.
func TestFailureCacheSkipsCredentialFailures(t *testing.T) {
	credentialErrors := []string{
		"operation error STS: GetCallerIdentity, api error ExpiredToken: token has expired",
		"failed to refresh cached SSO token",
		"operation error SSO: GetRoleCredentials, api error UnauthorizedException",
		"InvalidGrantException: refresh token is invalid",
		"NoCredentialProviders: no valid providers in chain",
		"failed to retrieve credentials",
	}

	for _, msg := range credentialErrors {
		t.Run(msg, func(t *testing.T) {
			c := NewFailureCache(time.Hour)
			c.Add("111111111111", "eu-central-1", errors.New(msg))

			assert.NoError(t, c.Err("111111111111", "eu-central-1"),
				"a credential failure must not be cached")
		})
	}
}

// TestFailureCacheCachesRegionFailures is the other half: region-scoped failures
// are what the cache exists for and must survive the full TTL. Notably
// InvalidClientTokenId, which is what STS returns for a region the account has
// not opted into.
func TestFailureCacheCachesRegionFailures(t *testing.T) {
	regionErrors := []string{
		"operation error STS: GetCallerIdentity, https response error StatusCode: 403, api error InvalidClientTokenId: The security token included in the request is invalid",
		`request send failed, Post "https://sts.me-south-1.amazonaws.com/": dial tcp 99.82.136.65:443: i/o timeout`,
		"could not connect to the endpoint URL",
	}

	for _, msg := range regionErrors {
		t.Run(msg, func(t *testing.T) {
			c := NewFailureCache(time.Hour)
			c.Add("111111111111", "me-south-1", errors.New(msg))

			assert.Error(t, c.Err("111111111111", "me-south-1"),
				"a region-scoped failure is the whole point of the cache")
		})
	}
}

// TestFailureCacheIsPerAccountAndRegion: one dead region must not suppress the
// rest of the matrix.
func TestFailureCacheIsPerAccountAndRegion(t *testing.T) {
	c := NewFailureCache(time.Minute)
	c.Add("111111111111", "me-south-1", errors.New("no route"))

	assert.Error(t, c.Err("111111111111", "me-south-1"))
	assert.NoError(t, c.Err("111111111111", "eu-central-1"), "another region is unaffected")
	assert.NoError(t, c.Err("222222222222", "me-south-1"), "another account is unaffected")
}

func TestFailureCacheForget(t *testing.T) {
	c := NewFailureCache(time.Minute)
	c.Add("111111111111", "eu-west-1", errors.New("transient"))
	require.Error(t, c.Err("111111111111", "eu-west-1"))

	c.Forget("111111111111", "eu-west-1")

	assert.NoError(t, c.Err("111111111111", "eu-west-1"))
}

func TestFailureCacheIgnoresNilError(t *testing.T) {
	c := NewFailureCache(time.Minute)
	c.Add("111111111111", "eu-west-1", nil)

	assert.NoError(t, c.Err("111111111111", "eu-west-1"), "a nil error must not be remembered as a failure")
}

// TestFailureCacheNilReceiver keeps a pool built without a cache working
// unchanged rather than panicking.
func TestFailureCacheNilReceiver(t *testing.T) {
	var c *FailureCache

	assert.NoError(t, c.Err("111111111111", "eu-west-1"))
	assert.NotPanics(t, func() {
		c.Add("111111111111", "eu-west-1", errors.New("x"))
		c.Forget("111111111111", "eu-west-1")
	})
}

func TestFailureCacheDefaultTTL(t *testing.T) {
	// Deliberately not asserting the constant's literal value: that duplicates
	// the declaration and turns a tuning decision into a test failure. What
	// matters is that a caller who passes nothing gets the default rather than a
	// zero TTL, which would disable the cache silently.
	assert.Equal(t, DefaultClientFailureTTL, NewFailureCache(0).ttl)
	assert.Equal(t, DefaultClientFailureTTL, NewFailureCache(-time.Second).ttl)
	assert.Positive(t, DefaultClientFailureTTL, "a non-positive default would disable the cache")
	assert.Equal(t, time.Minute, NewFailureCache(time.Minute).ttl)
}

// TestFailureCacheConcurrentUse mirrors collectClients, which calls into the
// pool from one goroutine per region.
func TestFailureCacheConcurrentUse(t *testing.T) {
	c := NewFailureCache(time.Minute)
	done := make(chan struct{})

	for i := 0; i < 16; i++ {
		go func() {
			defer func() { done <- struct{}{} }()

			c.Add("111111111111", "eu-west-1", errors.New("x"))
			_ = c.Err("111111111111", "eu-west-1")
			c.Forget("111111111111", "eu-west-1")
		}()
	}

	for i := 0; i < 16; i++ {
		<-done
	}
}
