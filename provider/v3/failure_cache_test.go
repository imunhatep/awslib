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

// TestFailureCacheExpires is the safety valve: an expired SSO session fails
// client creation the same way a disabled region does, so a remembered failure
// must not outlive the operator logging back in.
func TestFailureCacheExpires(t *testing.T) {
	c := NewFailureCache(20 * time.Millisecond)
	c.Add("111111111111", "eu-central-1", errors.New("expired token"))

	require.Error(t, c.Err("111111111111", "eu-central-1"))

	time.Sleep(40 * time.Millisecond)

	assert.NoError(t, c.Err("111111111111", "eu-central-1"), "the entry must lapse so a recovered credential is retried")
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
	assert.Equal(t, DefaultClientFailureTTL, NewFailureCache(0).ttl)
	assert.Equal(t, DefaultClientFailureTTL, NewFailureCache(-time.Second).ttl)
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
