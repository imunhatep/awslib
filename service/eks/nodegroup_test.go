package eks

import (
	"bytes"
	"encoding/gob"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-errors/errors"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockClient struct{}

func (mockClient) GetRegion() ptypes.AwsRegion       { return ptypes.AwsRegion("eu-west-1") }
func (mockClient) GetAccountID() ptypes.AwsAccountID { return ptypes.AwsAccountID("123456789012") }

func spotNodegroup() types.Nodegroup {
	return types.Nodegroup{
		NodegroupName:  aws.String("workers-spot"),
		NodegroupArn:   aws.String("arn:aws:eks:eu-west-1:123456789012:nodegroup/prod/workers-spot/1ac9a1b2-0000-4c1d-9e8f-1234567890ab"),
		ClusterName:    aws.String("prod"),
		CreatedAt:      aws.Time(time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)),
		Status:         types.NodegroupStatusDegraded,
		CapacityType:   types.CapacityTypesSpot,
		AmiType:        types.AMITypesAl2023X8664Standard,
		InstanceTypes:  []string{"m6i.large", "m5.large"},
		ReleaseVersion: aws.String("1.31.0-20241024"),
		Version:        aws.String("1.31"),
		NodeRole:       aws.String("arn:aws:iam::123456789012:role/eks-node"),
		Subnets:        []string{"subnet-a", "subnet-b"},
		ScalingConfig: &types.NodegroupScalingConfig{
			MinSize:     aws.Int32(2),
			MaxSize:     aws.Int32(10),
			DesiredSize: aws.Int32(3),
		},
		LaunchTemplate: &types.LaunchTemplateSpecification{
			Id:      aws.String("lt-0abc"),
			Name:    aws.String("workers"),
			Version: aws.String("7"),
		},
		Resources: &types.NodegroupResources{
			AutoScalingGroups: []types.AutoScalingGroup{{Name: aws.String("eks-workers-spot-1ac9")}},
		},
		Health: &types.NodegroupHealth{Issues: []types.Issue{
			{Code: types.NodegroupIssueCodeAsgInstanceLaunchFailures},
			{Code: types.NodegroupIssueCodeAccessDenied},
			{Code: types.NodegroupIssueCodeAsgInstanceLaunchFailures},
		}},
		Tags: map[string]string{"Team": "platform"},
	}
}

func TestNodegroupIdentity(t *testing.T) {
	ng := NewNodegroup(mockClient{}, spotNodegroup())

	var _ service.ResourceInterface = ng

	assert.Equal(t, "prod/workers-spot", ng.GetId(), "a node group name is unique per cluster only")
	assert.Equal(t, "workers-spot", ng.GetName())
	assert.Equal(t, "arn:aws:eks:eu-west-1:123456789012:nodegroup/prod/workers-spot/1ac9a1b2-0000-4c1d-9e8f-1234567890ab", ng.GetArn())
	assert.Equal(t, ccfg.ResourceTypeEKSNodegroup, ng.GetType())
	assert.Equal(t, ptypes.AwsAccountID("123456789012"), ng.GetAccountID())
	assert.Equal(t, ptypes.AwsRegion("eu-west-1"), ng.GetRegion())
	assert.Equal(t, time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC), ng.GetCreatedAt())
}

func TestNodegroupHelpers(t *testing.T) {
	ng := NewNodegroup(mockClient{}, spotNodegroup())

	assert.Equal(t, "prod", ng.GetClusterName())
	assert.Equal(t, "DEGRADED", ng.GetStatus())
	assert.Equal(t, "SPOT", ng.GetCapacityType())
	assert.Equal(t, "AL2023_x86_64_STANDARD", ng.GetAmiType())
	assert.Equal(t, []string{"m6i.large", "m5.large"}, ng.GetInstanceTypes())
	assert.Equal(t, "1.31.0-20241024", ng.GetReleaseVersion())
	assert.Equal(t, "1.31", ng.GetKubernetesVersion())
	assert.Equal(t, int32(2), ng.GetMinSize())
	assert.Equal(t, int32(10), ng.GetMaxSize())
	assert.Equal(t, int32(3), ng.GetDesiredSize())
	assert.Equal(t, []string{"subnet-a", "subnet-b"}, ng.GetSubnets())
	assert.Equal(t, "arn:aws:iam::123456789012:role/eks-node", ng.GetNodeRole())
	assert.Equal(t, "lt-0abc", ng.GetLaunchTemplateId())
	assert.Equal(t, "workers", ng.GetLaunchTemplateName())
	assert.Equal(t, "7", ng.GetLaunchTemplateVersion())
	assert.Equal(t, []string{"eks-workers-spot-1ac9"}, ng.GetAutoScalingGroupNames())
	assert.Equal(t, 3, ng.GetHealthIssueCount())
	assert.Equal(t, []string{"AccessDenied", "AsgInstanceLaunchFailures"}, ng.GetHealthIssueCodes())

	// Returned slices are copies.
	ng.GetInstanceTypes()[0] = "x"
	ng.GetSubnets()[0] = "x"
	assert.Equal(t, "m6i.large", ng.InstanceTypes[0])
	assert.Equal(t, "subnet-a", ng.Subnets[0])
}

func TestNodegroupHelpersOnSparseNodegroup(t *testing.T) {
	ng := NewNodegroup(mockClient{}, types.Nodegroup{
		NodegroupName: aws.String("ng"),
		ClusterName:   aws.String("c"),
	})

	assert.Empty(t, ng.GetArn(), "an unparseable ARN is left unset, not rendered as \":::\"")
	assert.Zero(t, ng.GetMinSize())
	assert.Zero(t, ng.GetMaxSize())
	assert.Zero(t, ng.GetDesiredSize())
	assert.Empty(t, ng.GetLaunchTemplateId())
	assert.Empty(t, ng.GetLaunchTemplateVersion())
	assert.Nil(t, ng.GetAutoScalingGroupNames())
	assert.Zero(t, ng.GetHealthIssueCount())
	assert.Nil(t, ng.GetHealthIssueCodes())
	assert.Empty(t, ng.GetTags())
}

func TestNodegroupGetTagsReturnsCopy(t *testing.T) {
	ng := NewNodegroup(mockClient{}, spotNodegroup())

	tags := ng.GetTags()
	tags["Team"] = "changed"
	tags["Injected"] = "value"

	assert.Equal(t, "platform", ng.GetTagValue("Team"))
	assert.NotContains(t, ng.GetTags(), "Injected")
}

func TestNodegroupGobRoundTrip(t *testing.T) {
	in := []Nodegroup{NewNodegroup(mockClient{}, spotNodegroup())}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))

	var out []Nodegroup
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))
	require.Len(t, out, 1)
	assert.Equal(t, in[0].GetArn(), out[0].GetArn())
	assert.Equal(t, in[0].GetId(), out[0].GetId())
	assert.Equal(t, int32(3), out[0].GetDesiredSize())
	assert.Equal(t, "platform", out[0].GetTagValue("Team"))
}

func TestIsNotFound(t *testing.T) {
	notFound := &types.ResourceNotFoundException{Message: aws.String("gone")}

	assert.True(t, isNotFound(notFound))
	assert.True(t, isNotFound(errors.Errorf("describing node group a/b: %w", notFound)))
	assert.False(t, isNotFound(&types.ServerException{}))
	assert.False(t, isNotFound(nil))
}

func TestForEachBounded(t *testing.T) {
	var inFlight, peak atomic.Int32

	err := forEachBounded(20, func(i int) error {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}

		time.Sleep(time.Millisecond)
		inFlight.Add(-1)

		return nil
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, peak.Load(), int32(nodegroupFetchConcurrency))

	// Every index runs even after a failure, and the first error by index wins, so a
	// listing never comes back short without saying so.
	var ran atomic.Int32
	err = forEachBounded(5, func(i int) error {
		ran.Add(1)
		if i == 1 || i == 3 {
			return errors.Errorf("failed %d", i)
		}

		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed 1")
	assert.Equal(t, int32(5), ran.Load())

	require.NoError(t, forEachBounded(0, func(int) error { return errors.New("never called") }))
}
