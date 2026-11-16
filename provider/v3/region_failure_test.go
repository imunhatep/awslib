package v3

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Error texts as the SDK renders them. The classifier matches on text, so these
// are the shapes it has to recognise, not paraphrases.
const (
	errInvalidClientTokenID = "operation error STS: GetCallerIdentity, https response error StatusCode: 403, RequestID: 1, api error InvalidClientTokenId: The security token included in the request is invalid."
	errUnrecognizedClient   = "operation error Lambda: ListFunctions, https response error StatusCode: 403, RequestID: 1, api error UnrecognizedClientException: The security token included in the request is invalid."
	errEc2AuthFailure       = "operation error EC2: DescribeInstances, https response error StatusCode: 401, RequestID: 1, api error AuthFailure: AWS was not able to validate the provided access credentials"
	errDialTimeout          = `operation error EC2: DescribeInstances, exceeded maximum number of attempts, 5, https response error StatusCode: 0, RequestID: , request send failed, Post "https://ec2.me-south-1.amazonaws.com/": dial tcp 157.175.0.1:443: i/o timeout`
	errDialRefused          = `operation error EC2: DescribeInstances, https response error StatusCode: 0, RequestID: , request send failed, Post "https://ec2.me-south-1.amazonaws.com/": dial tcp 157.175.0.1:443: connect: connection refused`
	errDialUnreachable      = `request send failed, Post "https://ec2.me-south-1.amazonaws.com/": dial tcp 157.175.0.1:443: connect: network is unreachable`
	errDialNoRoute          = `request send failed, Post "https://ec2.me-south-1.amazonaws.com/": dial tcp 157.175.0.1:443: connect: no route to host`
	errTLSHandshakeTimeout  = `operation error EC2: DescribeInstances, https response error StatusCode: 0, RequestID: , request send failed, Post "https://ec2.me-south-1.amazonaws.com/": net/http: TLS handshake timeout`
)

func TestRegionFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		// The region is not enabled for the account.
		{"sts InvalidClientTokenId", errors.New(errInvalidClientTokenID), true},
		{"UnrecognizedClientException", errors.New(errUnrecognizedClient), true},
		{"ec2 AuthFailure", errors.New(errEc2AuthFailure), true},

		// The endpoint cannot be reached.
		{"dial i/o timeout", errors.New(errDialTimeout), true},
		{"dial connection refused", errors.New(errDialRefused), true},
		{"dial network is unreachable", errors.New(errDialUnreachable), true},
		{"dial no route to host", errors.New(errDialNoRoute), true},
		{"tls handshake timeout", errors.New(errTLSHandshakeTimeout), true},

		// Wrapping must not hide the cause.
		{"fmt %w wrapped dial timeout", fmt.Errorf("fetch ec2 instances: %w", errors.New(errDialTimeout)), true},
		{"go-errors wrapped InvalidClientTokenId", goerrors.New(errors.New(errInvalidClientTokenID)), true},
		{"go-errors Wrap of fmt-wrapped AuthFailure", goerrors.Wrap(fmt.Errorf("proxy: %w", errors.New(errEc2AuthFailure)), 1), true},
		{"go-errors Errorf around tls timeout", goerrors.Errorf("list: %w", errors.New(errTLSHandshakeTimeout)), true},

		// Not a region failure.
		{"nil", nil, false},
		{"dns no such host", errors.New(`request send failed, Get "https://emr-serverless.ap-east-2.amazonaws.com/": dial tcp: lookup emr-serverless.ap-east-2.amazonaws.com: no such host`), false},
		{"wrapped dns no such host", fmt.Errorf("proxy: %w", goerrors.New(`dial tcp: lookup x: no such host`)), false},
		{"access denied", errors.New("operation error IAM: ListRoles, https response error StatusCode: 403, RequestID: 1, api error AccessDenied: User is not authorized to perform: iam:ListRoles"), false},
		{"access denied exception", errors.New("operation error Lambda: ListFunctions, https response error StatusCode: 403, api error AccessDeniedException: not authorized"), false},
		{"ec2 UnauthorizedOperation", errors.New("operation error EC2: DescribeVpcs, https response error StatusCode: 403, api error UnauthorizedOperation: You are not authorized to perform this operation."), false},
		{"throttling", errors.New("operation error CloudWatch: ListMetrics, exceeded maximum number of attempts, 5, https response error StatusCode: 400, api error Throttling: Rate exceeded"), false},
		{"throttling exception", errors.New("api error ThrottlingException: Rate exceeded"), false},
		{"read timeout mid-response", errors.New(`operation error S3: ListBuckets, https response error StatusCode: 200, RequestID: 1, deserialization failed, read tcp 10.0.0.2:51234->52.95.0.1:443: i/o timeout`), false},
		{"read connection reset", errors.New(`read tcp 10.0.0.2:51234->52.95.0.1:443: read: connection reset by peer`), false},
		{"client timeout awaiting headers", errors.New(`net/http: request canceled (Client.Timeout exceeded while awaiting headers)`), false},
		{"context canceled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
		{"sdk canceled", errors.New("operation error EC2: DescribeInstances, canceled, context canceled"), false},
		{"dial canceled by caller", errors.New(`request send failed, Post "https://ec2.me-south-1.amazonaws.com/": dial tcp 157.175.0.1:443: operation was canceled`), false},
		{"wrapped context canceled", fmt.Errorf("fetch: %w", context.Canceled), false},

		// Credential failures win over everything: the pool must never remember
		// them, even when the credential refresh itself failed to connect.
		{"expired token", errors.New("operation error STS: GetCallerIdentity, https response error StatusCode: 403, api error ExpiredToken: The security token included in the request is expired"), false},
		{"sso refresh failed to dial", errors.New(`failed to refresh cached credentials, operation error SSO: GetRoleCredentials, request send failed, Get "https://portal.sso.eu-west-1.amazonaws.com/federation/credentials": dial tcp 1.2.3.4:443: i/o timeout`), false},
		{"assume role failed", errors.New("operation error EC2: DescribeInstances, failed to retrieve credentials: operation error STS: AssumeRole, https response error StatusCode: 403, api error AccessDenied"), false},
		{"no credential providers", errors.New("NoCredentialProviders: no valid providers in chain"), false},
		{"wrapped expired token", goerrors.New(fmt.Errorf("proxy: %w", errors.New("api error ExpiredTokenException: token expired, InvalidClientTokenId"))), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RegionFailure(tc.err))
		})
	}
}

// TestRegionFailureOnRealDialErrors runs the classifier over errors produced by
// the net package itself, so a change in Go's wording would show here rather
// than in production. Neither dial sends a packet: the refused one targets a
// closed loopback port, the timed-out one has a deadline already in the past.
func TestRegionFailureOnRealDialErrors(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	_, refused := net.Dial("tcp", addr)
	require.Error(t, refused)
	assert.True(t, RegionFailure(fmt.Errorf("request send failed: %w", refused)), "real refusal: %v", refused)

	_, timedOut := (&net.Dialer{Timeout: time.Nanosecond}).Dial("tcp", "192.0.2.1:443")
	require.Error(t, timedOut)
	assert.True(t, RegionFailure(fmt.Errorf("request send failed: %w", timedOut)), "real dial timeout: %v", timedOut)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, canceled := (&net.Dialer{}).DialContext(ctx, "tcp", "192.0.2.1:443")
	require.Error(t, canceled)
	assert.False(t, RegionFailure(canceled), "a caller's cancellation is not the region's fault: %v", canceled)
}

func TestReportFailure(t *testing.T) {
	regionErr := errors.New(errInvalidClientTokenID)

	t.Run("nil client", func(t *testing.T) {
		var client *Client

		assert.NotPanics(t, func() {
			assert.False(t, client.ReportFailure(regionErr))
		})
	})

	t.Run("no hook", func(t *testing.T) {
		client := &Client{}

		assert.False(t, client.ReportFailure(regionErr))
	})

	t.Run("non-region error leaves the hook alone", func(t *testing.T) {
		called := 0
		client := (&Client{}).OnRegionFailure(func(error) { called++ })

		for _, err := range []error{
			nil,
			errors.New("api error AccessDenied: not authorized"),
			errors.New("dial tcp: lookup x: no such host"),
			errors.New("api error ExpiredToken: expired"),
			context.Canceled,
		} {
			assert.False(t, client.ReportFailure(err), "%v", err)
		}

		assert.Zero(t, called)
	})

	t.Run("region error calls the hook with it", func(t *testing.T) {
		var got []error
		client := &Client{}

		assert.Same(t, client, client.OnRegionFailure(func(err error) { got = append(got, err) }), "OnRegionFailure chains")

		wrapped := fmt.Errorf("fetch: %w", regionErr)
		assert.True(t, client.ReportFailure(wrapped))

		require.Len(t, got, 1)
		assert.Same(t, wrapped, got[0], "the hook receives the reported error, not a copy")
	})
}
