package resources

import (
	"context"
	"testing"

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

// fakeProxy stands in for a RepoProxy. err makes it fail outright.
type fakeProxy struct {
	region    ptypes.AwsRegion
	accountID ptypes.AwsAccountID
	resources []service.ResourceInterface
	err       error
}

func (f *fakeProxy) GetAccountID() ptypes.AwsAccountID { return f.accountID }
func (f *fakeProxy) GetRegion() ptypes.AwsRegion       { return f.region }
func (f *fakeProxy) GetClient() *v3.Client             { return nil }
func (f *fakeProxy) GetContext() context.Context       { return context.Background() }

func (f *fakeProxy) FindAll(_ cfgtypes.ResourceType) ([]service.ResourceInterface, error) {
	return f.resources, f.err
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
