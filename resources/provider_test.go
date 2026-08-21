package resources

import (
	"context"
	"testing"
	"time"

	cfgtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResource is the smallest thing satisfying service.ResourceInterface.
type fakeResource struct {
	service.AbstractResource
}

func (fakeResource) GetName() string            { return "fake" }
func (fakeResource) GetTags() map[string]string { return map[string]string{} }

// fakeProxy stands in for a RepoProxy. block holds FindAll open to model a
// region whose endpoint does not route; err makes it fail outright.
type fakeProxy struct {
	region    ptypes.AwsRegion
	accountID ptypes.AwsAccountID
	resources []service.ResourceInterface
	err       error
	block     chan struct{}
}

func (f *fakeProxy) GetAccountID() ptypes.AwsAccountID { return f.accountID }
func (f *fakeProxy) GetRegion() ptypes.AwsRegion       { return f.region }
func (f *fakeProxy) GetClient() *v3.Client             { return nil }
func (f *fakeProxy) GetContext() context.Context       { return context.Background() }

func (f *fakeProxy) FindAll(_ cfgtypes.ResourceType) ([]service.ResourceInterface, error) {
	if f.block != nil {
		<-f.block
	}

	return f.resources, f.err
}

// TestProviderTimeoutDoesNotBlockOtherProxies is the guarantee the timeout
// exists for: before it, Read() waited on every proxy, so one unroutable region
// meant the caller got nothing at all instead of every other region's resources.
func TestProviderTimeoutDoesNotBlockOtherProxies(t *testing.T) {
	blocked := make(chan struct{})
	// Release the hung proxy when the test ends so its goroutine exits.
	t.Cleanup(func() { close(blocked) })

	hung := &fakeProxy{region: "me-south-1", accountID: "111111111111", block: blocked}
	healthy := &fakeProxy{
		region:    "eu-central-1",
		accountID: "111111111111",
		resources: []service.ResourceInterface{fakeResource{}, fakeResource{}},
	}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, hung, healthy).
		WithTimeout(100 * time.Millisecond).
		Run()

	start := time.Now()
	found := reader.Read()
	elapsed := time.Since(start)

	assert.Len(t, found, 2, "the healthy region's resources must still be returned")
	assert.Less(t, elapsed, 5*time.Second, "a hung proxy must not hold the result set")

	failures := reader.Failures()
	require.Len(t, failures, 1, "the hung proxy must be reported, not silently dropped")
	assert.Equal(t, ptypes.AwsRegion("me-south-1"), failures[0].Region)
	assert.Equal(t, ptypes.AwsAccountID("111111111111"), failures[0].AccountID)
	assert.ErrorContains(t, failures[0].Err, "timed out")
}

// TestProviderReportsProxyErrors: a proxy that fails fast is reported too, so a
// short list is distinguishable from a complete one.
func TestProviderReportsProxyErrors(t *testing.T) {
	broken := &fakeProxy{region: "ap-east-1", accountID: "222222222222", err: assert.AnError}
	healthy := &fakeProxy{
		region:    "eu-west-1",
		accountID: "222222222222",
		resources: []service.ResourceInterface{fakeResource{}},
	}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, broken, healthy).Run()

	assert.Len(t, reader.Read(), 1)

	failures := reader.Failures()
	require.Len(t, failures, 1)
	assert.Equal(t, ptypes.AwsRegion("ap-east-1"), failures[0].Region)
}

// TestProviderNoFailuresWhenAllAnswer: an empty Failures() is what lets a caller
// treat a short list as genuinely complete.
func TestProviderNoFailuresWhenAllAnswer(t *testing.T) {
	empty := &fakeProxy{region: "eu-north-1", accountID: "333333333333"}

	reader := NewProvider(cfgtypes.ResourceTypeInstance, empty).Run()

	assert.Empty(t, reader.Read())
	assert.Empty(t, reader.Failures(), "a region that answered zero resources is not a failure")
}

// TestProviderTimeoutCannotBeDisabled guards the deliberate asymmetry in
// WithTimeout: a non-positive value keeps the default rather than meaning
// "wait forever", which is the behaviour being removed.
func TestProviderTimeoutCannotBeDisabled(t *testing.T) {
	p := NewProvider(cfgtypes.ResourceTypeInstance).WithTimeout(0)
	assert.Equal(t, DefaultRegionTimeout, p.timeout)

	p = NewProvider(cfgtypes.ResourceTypeInstance).WithTimeout(-time.Second)
	assert.Equal(t, DefaultRegionTimeout, p.timeout)

	p = NewProvider(cfgtypes.ResourceTypeInstance).WithTimeout(5 * time.Second)
	assert.Equal(t, 5*time.Second, p.timeout)
}
