package resources

import (
	"context"
	"errors"
	"testing"

	cfgtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cancelledProxy is a clientProxy whose context has already ended.
type cancelledProxy struct {
	clientProxy
	ctx context.Context
}

func (c *cancelledProxy) GetClient() *v3.Client       { return c.client }
func (c *cancelledProxy) GetContext() context.Context { return c.ctx }

// TestProviderDoesNotReportFailureAfterCallerCancelled: a dial cut short by the
// caller's deadline reads exactly like a dead endpoint, so a failure seen after
// the caller's context ended must not mark the region unusable.
func TestProviderDoesNotReportFailureAfterCallerCancelled(t *testing.T) {
	dialErr := errors.New("operation error EC2: DescribeInstances, https response error: dial tcp 3.5.0.1:443: i/o timeout")

	client, hooked := hookedClient(t, "222222222222", "eu-west-1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	proxy := &cancelledProxy{
		clientProxy: clientProxy{
			fakeProxy: fakeProxy{region: "eu-west-1", accountID: "222222222222", err: dialErr},
			client:    client,
		},
		ctx: ctx,
	}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, proxy).Run()
	reader.Read()

	require.Len(t, reader.Failures(), 1, "the failure is still reported to the caller")
	assert.Empty(t, hooked(), "but the region is not remembered as unusable")
}
