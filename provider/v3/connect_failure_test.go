package v3

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectFailure(t *testing.T) {
	cases := []struct {
		err  string
		want bool
	}{
		{"dial tcp 15.185.0.1:443: i/o timeout", true},
		{"dial tcp 15.185.0.1:443: connect: connection refused", true},
		{"dial tcp 15.185.0.1:443: connect: network is unreachable", true},
		{"dial tcp 15.185.0.1:443: connect: no route to host", true},
		{"net/http: TLS handshake timeout", true},
		// The local resolver, not the region.
		{"dial tcp: lookup ec2.me-south-1.amazonaws.com on 10.0.0.2:53: read udp 10.0.0.5:5353->10.0.0.2:53: i/o timeout", false},
		{"dial tcp: lookup sts.me-south-1.amazonaws.com: no such host", false},
		// A slow answer from a working endpoint.
		{"read tcp 10.0.0.5:5353->52.0.0.1:443: i/o timeout", false},
		{"api error ThrottlingException: Rate exceeded", false},
	}

	for _, c := range cases {
		wrapped := fmt.Errorf("operation error EC2: DescribeInstances, https response error: %w", errors.New(c.err))
		assert.Equal(t, c.want, ConnectFailure(wrapped), c.err)
	}

	assert.False(t, ConnectFailure(nil))
}

// TestNewRetryerDoesNotRetryConnectFailures: a dead endpoint is given up on
// after one dial, while everything the SDK retries otherwise still is.
func TestNewRetryerDoesNotRetryConnectFailures(t *testing.T) {
	r := NewRetryer()

	assert.Equal(t, AwsRetryAttempts, r.MaxAttempts())

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	_, dialErr := net.DialTimeout("tcp", addr, time.Second)
	require.Error(t, dialErr)
	require.True(t, ConnectFailure(dialErr), "a refused dial is a connect failure: %v", dialErr)

	assert.False(t, r.IsErrorRetryable(dialErr), "a refused dial is not retried")
	assert.False(t, r.IsErrorRetryable(errors.New("net/http: TLS handshake timeout")))

	// Still retried: a connection reset mid-request, and throttling.
	assert.True(t, r.IsErrorRetryable(&net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}))
	assert.True(t, retry.IsErrorRetryables(retry.DefaultRetryables).IsErrorRetryable(&net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}) == aws.TrueTernary)
}

// TestFailureCacheNetworkFailureTTL: a connection failure is remembered for
// NetworkFailureTTL, a region the account has not enabled for the full TTL.
func TestFailureCacheNetworkFailureTTL(t *testing.T) {
	cache := NewFailureCache(6 * time.Hour)

	network := errors.New("dial tcp 15.185.0.1:443: i/o timeout")
	disabled := errors.New(errInvalidClientTokenID)

	cache.Add("111111111111", "me-south-1", network)
	cache.Add("111111111111", "ap-east-1", disabled)

	require.Equal(t, network, cache.Err("111111111111", "me-south-1"))
	require.Equal(t, disabled, cache.Err("111111111111", "ap-east-1"))

	age := func(region types.AwsRegion, by time.Duration) {
		cache.mx.Lock()
		defer cache.mx.Unlock()

		e := cache.entries["111111111111"][region]
		e.at = e.at.Add(-by)
		cache.entries["111111111111"][region] = e
	}

	age("me-south-1", NetworkFailureTTL+time.Second)
	age("ap-east-1", NetworkFailureTTL+time.Second)

	assert.NoError(t, cache.Err("111111111111", "me-south-1"), "a connection failure lapses after NetworkFailureTTL")
	assert.Equal(t, disabled, cache.Err("111111111111", "ap-east-1"), "a disabled region is kept for the cache's TTL")

	// A cache TTL shorter than NetworkFailureTTL applies to both.
	short := NewFailureCache(time.Minute)
	short.Add("111111111111", "me-south-1", network)
	assert.Equal(t, time.Minute, short.entries["111111111111"]["me-south-1"].ttl)
}

// TestPoolLateReportKeepsNewerClient: a slow request from a client that was
// replaced after an earlier failure lapsed must not evict its successor.
func TestPoolLateReportKeepsNewerClient(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	pool := testRolePool(t, sts)

	old, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)

	// The old client is replaced, as it is once a failure's TTL has lapsed.
	pool.Lock()
	delete(pool.clients["111111111111"], "eu-west-1")
	pool.Unlock()

	fresh, err := pool.GetClient("111111111111", "eu-west-1")
	require.NoError(t, err)
	require.NotSame(t, old, fresh)

	require.True(t, old.ReportFailure(regionErr))

	cached, ok := pool.cachedClient("111111111111", "eu-west-1")
	assert.True(t, ok)
	assert.Same(t, fresh, cached, "the late report does not evict the newer client")
}

func TestAssumeClientForAccountChecksRoleAccount(t *testing.T) {
	sts := newFakeSTS(t, "999999999999")
	builder := testBuilder(t, sts)

	_, err := builder.AssumeClientForAccount("111111111111", "arn:aws:iam::222222222222:role/reader", "eu-west-1")
	require.Error(t, err)

	_, err = builder.AssumeClientForAccount("111111111111", "not-an-arn", "eu-west-1")
	require.Error(t, err)

	client, err := builder.AssumeClientForAccount("111111111111", "arn:aws:iam::111111111111:role/reader", "eu-west-1")
	require.NoError(t, err)
	assert.Equal(t, types.AwsAccountID("111111111111"), client.GetAccountID())
	assert.Equal(t, 0, sts.count("AssumeRole"), "the role is not assumed to build it")
}
