package secretmanager

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct{}

func (fakeClient) GetRegion() ptypes.AwsRegion       { return "eu-central-1" }
func (fakeClient) GetAccountID() ptypes.AwsAccountID { return "111111111111" }

func TestNewSecretEntryFromListCarriesVersionStages(t *testing.T) {
	stages := map[string][]string{
		"3f1c-current":  {"AWSCURRENT"},
		"a72b-previous": {"AWSPREVIOUS"},
	}

	entry := NewSecretEntryFromList(fakeClient{}, types.SecretListEntry{
		ARN:                    aws.String(testSecretArn),
		Name:                   aws.String(testSecretName),
		SecretVersionsToStages: stages,
	})

	assert.Equal(t, stages, entry.VersionIdsToStages)
}

func TestNewSecretEntryFromListCopiesListFields(t *testing.T) {
	changed := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	entry := NewSecretEntryFromList(fakeClient{}, types.SecretListEntry{
		ARN:             aws.String(testSecretArn),
		Name:            aws.String(testSecretName),
		LastChangedDate: &changed,
		PrimaryRegion:   aws.String("eu-west-1"),
	})

	assert.Equal(t, testSecretName, entry.GetName())
	assert.Equal(t, &changed, entry.LastChangedDate)
	assert.Equal(t, "eu-west-1", aws.ToString(entry.PrimaryRegion))
	assert.Nil(t, entry.ReplicationStatus, "the list call carries no replication status")
}

// The cached repository stores []SecretEntry with gob, which rejects the
// middleware.Metadata inside DescribeSecretOutput unless SecretEntry encodes itself.
func TestSecretEntryGobRoundTrip(t *testing.T) {
	changed := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	in := []SecretEntry{NewSecretEntryFromList(fakeClient{}, types.SecretListEntry{
		ARN:                    aws.String(testSecretArn),
		Name:                   aws.String(testSecretName),
		LastChangedDate:        &changed,
		SecretVersionsToStages: map[string][]string{"3f1c-current": {"AWSCURRENT"}},
		Tags:                   []types.Tag{{Key: aws.String("team"), Value: aws.String("sre")}},
	})}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))

	var out []SecretEntry
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))

	require.Len(t, out, 1)
	assert.Equal(t, in[0].AbstractResource, out[0].AbstractResource)
	assert.Equal(t, in[0].DescribeSecretOutput, out[0].DescribeSecretOutput)
	assert.Equal(t, map[string]string{"team": "sre"}, out[0].GetTags())
}

func TestSecretEntryGobRoundTripAsInterface(t *testing.T) {
	in := []service.ResourceInterface{NewSecretEntryFromList(fakeClient{}, types.SecretListEntry{
		ARN:  aws.String(testSecretArn),
		Name: aws.String(testSecretName),
	})}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))

	var out []service.ResourceInterface
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))

	require.Len(t, out, 1)
	assert.Equal(t, in[0], out[0])
}
