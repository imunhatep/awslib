package elb

import (
	"bytes"
	"encoding/gob"
	stderrors "errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct{}

func (fakeClient) GetRegion() ptypes.AwsRegion       { return "eu-west-1" }
func (fakeClient) GetAccountID() ptypes.AwsAccountID { return "123456789012" }

const (
	tgArn  = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:targetgroup/web/73e2d6bc24d8a067"
	albArn = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/front/50dc6c495c0c9188"
	nlbArn = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/net/edge/1a2b3c4d5e6f7a8b"
)

func target(id string, port int32, state types.TargetHealthStateEnum) Target {
	return Target{Id: id, Port: port, State: state}
}

func TestNewTargetGroup(t *testing.T) {
	checked := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	tg := NewTargetGroup(fakeClient{}, types.TargetGroup{
		TargetGroupArn:   aws.String(tgArn),
		TargetGroupName:  aws.String("web"),
		LoadBalancerArns: []string{albArn, nlbArn},
	}, []types.Tag{{Key: aws.String("service"), Value: aws.String("web")}}, []Target{
		target("i-1", 80, types.TargetHealthStateEnumHealthy),
		target("i-2", 80, types.TargetHealthStateEnumHealthy),
		target("i-3", 80, types.TargetHealthStateEnumUnhealthy),
		target("i-4", 80, types.TargetHealthStateEnumUnhealthyDraining),
		target("i-5", 80, types.TargetHealthStateEnumDraining),
		target("i-6", 80, types.TargetHealthStateEnumUnused),
	}, checked, nil)

	assert.Equal(t, "web", tg.GetName())
	assert.Equal(t, tgArn, tg.GetArn())
	assert.Equal(t, cfg.ResourceTypeElasticLoadBalancingV2TargetGroup, tg.GetType())
	assert.Equal(t, time.Unix(0, 0), tg.GetCreatedAt())
	assert.Equal(t, "web", tg.GetTagValue("service"))

	assert.True(t, tg.HealthKnown())
	assert.Equal(t, checked, tg.HealthCheckedAt)
	assert.Equal(t, 2, tg.HealthyCount())
	assert.Equal(t, 2, tg.UnhealthyCount(), "unhealthy.draining failed its checks")
	assert.Equal(t, 2, tg.OtherCount())
	assert.Equal(t, map[string]int{"healthy": 2, "unhealthy": 1, "unhealthy.draining": 1, "draining": 1, "unused": 1}, tg.HealthStateCounts())
	assert.Equal(t, []string{"i-1", "i-2", "i-3", "i-4", "i-5", "i-6"}, tg.TargetIDs())
	assert.Equal(t, []string{"front", "edge"}, tg.LoadBalancerNames())

	arns := tg.GetLoadBalancerArns()
	arns[0] = "changed"
	assert.Equal(t, albArn, tg.LoadBalancerArns[0], "GetLoadBalancerArns returns a copy")

	tags := tg.GetTags()
	tags["service"] = "changed"
	assert.Equal(t, "web", tg.GetTagValue("service"), "GetTags returns a copy")
}

// Unknown health must not read as "no targets": the group is kept, the error recorded.
func TestNewTargetGroupKeepsHealthError(t *testing.T) {
	tg := NewTargetGroup(fakeClient{}, types.TargetGroup{
		TargetGroupArn:  aws.String(tgArn),
		TargetGroupName: aws.String("web"),
	}, nil, []Target{target("i-1", 80, types.TargetHealthStateEnumHealthy)}, time.Now(), stderrors.New("AccessDenied"))

	assert.False(t, tg.HealthKnown())
	assert.Equal(t, "AccessDenied", tg.HealthError)
	assert.Nil(t, tg.Targets)
	assert.True(t, tg.HealthCheckedAt.IsZero())
	assert.Zero(t, tg.HealthyCount())
}

func TestNewTarget(t *testing.T) {
	got := newTarget(types.TargetHealthDescription{
		HealthCheckPort: aws.String("8080"),
		Target:          &types.TargetDescription{Id: aws.String("10.0.0.5"), Port: aws.Int32(8080), AvailabilityZone: aws.String("all")},
		TargetHealth: &types.TargetHealth{
			State:       types.TargetHealthStateEnumUnhealthy,
			Reason:      types.TargetHealthReasonEnumTimeout,
			Description: aws.String("Request timed out"),
		},
	})

	assert.Equal(t, Target{
		Id: "10.0.0.5", Port: 8080, AvailabilityZone: "all", HealthCheckPort: "8080",
		State: types.TargetHealthStateEnumUnhealthy, Reason: types.TargetHealthReasonEnumTimeout, Description: "Request timed out",
	}, got)

	assert.Equal(t, Target{}, newTarget(types.TargetHealthDescription{}))
}

// Listings are cached through gob; the SDK structs reachable from the entities must encode.
func TestEntitiesRoundTripThroughGob(t *testing.T) {
	tg := NewTargetGroup(fakeClient{}, types.TargetGroup{
		TargetGroupArn: aws.String(tgArn), TargetGroupName: aws.String("web"),
		Matcher: &types.Matcher{HttpCode: aws.String("200")},
	}, []types.Tag{{Key: aws.String("k"), Value: aws.String("v")}},
		[]Target{target("i-1", 80, types.TargetHealthStateEnumHealthy)}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil)

	l := NewListener(fakeClient{}, albListener(), nil)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	require.NoError(t, enc.Encode([]TargetGroup{tg}))
	require.NoError(t, enc.Encode([]Listener{l}))

	dec := gob.NewDecoder(&buf)
	var tgs []TargetGroup
	var ls []Listener
	require.NoError(t, dec.Decode(&tgs))
	require.NoError(t, dec.Decode(&ls))

	assert.Equal(t, tg.GetArn(), tgs[0].GetArn())
	assert.Equal(t, 1, tgs[0].HealthyCount())
	assert.Equal(t, tg.HealthCheckedAt, tgs[0].HealthCheckedAt)
	assert.Equal(t, l.GetName(), ls[0].GetName())
	assert.Equal(t, l.ForwardTargetGroupArns(), ls[0].ForwardTargetGroupArns())
}
