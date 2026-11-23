package elb

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/imunhatep/awslib/service"
)

// Target is one registered target of a target group with its health as
// DescribeTargetHealth reported it.
type Target struct {
	// Id is an instance ID, an IP address, a Lambda function ARN or an ALB ARN,
	// depending on the target group's TargetType.
	Id               string
	Port             int32
	AvailabilityZone string
	HealthCheckPort  string
	State            types.TargetHealthStateEnum
	Reason           types.TargetHealthReasonEnum
	Description      string
}

type TargetGroup struct {
	service.AbstractResource
	types.TargetGroup
	Tags []types.Tag

	// Targets is what DescribeTargetHealth returned, sorted by Id then Port.
	// It is only meaningful when HealthError is empty.
	Targets []Target

	// HealthCheckedAt is when Targets was read. Listings are cached for hours,
	// so the health is "as of" this time, not current. Zero when HealthError
	// is set.
	HealthCheckedAt time.Time

	// HealthError is set when the target health could not be read. The target
	// group is still listed, but its Targets are unknown rather than empty.
	HealthError string
}

func NewTargetGroup(client AwsClient, tg types.TargetGroup, tags []types.Tag, targets []Target, checkedAt time.Time, healthErr error) TargetGroup {
	tgArn, _ := arn.Parse(aws.ToString(tg.TargetGroupArn))

	group := TargetGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        "",
			ARN:       &tgArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeElasticLoadBalancingV2TargetGroup,
		},
		TargetGroup:     tg,
		Tags:            tags,
		Targets:         targets,
		HealthCheckedAt: checkedAt,
	}

	if healthErr != nil {
		group.Targets = nil
		group.HealthCheckedAt = time.Time{}
		group.HealthError = healthErr.Error()
	}

	return group
}

func (e TargetGroup) GetName() string {
	return aws.ToString(e.TargetGroupName)
}

func (e TargetGroup) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e TargetGroup) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}

// HealthKnown reports whether the target health was read. When false, the
// counts below are zero because nothing is known, not because there are no
// targets.
func (e TargetGroup) HealthKnown() bool {
	return e.HealthError == ""
}

// HealthyCount is the number of targets in state "healthy".
func (e TargetGroup) HealthyCount() int {
	return e.countTargets(func(s types.TargetHealthStateEnum) bool { return isHealthy(s) })
}

// UnhealthyCount is the number of targets failing health checks, including
// those being drained because they failed ("unhealthy.draining").
func (e TargetGroup) UnhealthyCount() int {
	return e.countTargets(func(s types.TargetHealthStateEnum) bool { return isUnhealthy(s) })
}

// OtherCount is the number of targets neither healthy nor unhealthy:
// initial, draining, unused (not attached to a load balancer, or in an AZ the
// load balancer does not serve) and unavailable (health checks disabled).
func (e TargetGroup) OtherCount() int {
	return e.countTargets(func(s types.TargetHealthStateEnum) bool { return !isHealthy(s) && !isUnhealthy(s) })
}

// HealthStateCounts counts targets per health state.
func (e TargetGroup) HealthStateCounts() map[string]int {
	counts := make(map[string]int)
	for _, t := range e.Targets {
		counts[string(t.State)]++
	}

	return counts
}

// TargetIDs lists the target IDs in Targets order. A target registered on
// several ports appears once per port.
func (e TargetGroup) TargetIDs() []string {
	ids := make([]string, 0, len(e.Targets))
	for _, t := range e.Targets {
		ids = append(ids, t.Id)
	}

	return ids
}

// GetLoadBalancerArns returns a copy of the ARNs of the load balancers that
// route to this target group.
func (e TargetGroup) GetLoadBalancerArns() []string {
	return append([]string{}, e.LoadBalancerArns...)
}

// LoadBalancerNames returns the names of the load balancers that route to this
// target group, parsed from their ARNs.
func (e TargetGroup) LoadBalancerNames() []string {
	names := make([]string, 0, len(e.LoadBalancerArns))
	for _, lbArn := range e.LoadBalancerArns {
		if _, name := parseLoadBalancerArn(lbArn); name != "" {
			names = append(names, name)
		}
	}

	return names
}

func (e TargetGroup) countTargets(match func(types.TargetHealthStateEnum) bool) int {
	n := 0
	for _, t := range e.Targets {
		if match(t.State) {
			n++
		}
	}

	return n
}

func isHealthy(s types.TargetHealthStateEnum) bool {
	return s == types.TargetHealthStateEnumHealthy
}

func isUnhealthy(s types.TargetHealthStateEnum) bool {
	return s == types.TargetHealthStateEnumUnhealthy || s == types.TargetHealthStateEnumUnhealthyDraining
}

// newTarget flattens one DescribeTargetHealth entry.
func newTarget(d types.TargetHealthDescription) Target {
	t := Target{HealthCheckPort: aws.ToString(d.HealthCheckPort)}

	if d.Target != nil {
		t.Id = aws.ToString(d.Target.Id)
		t.Port = aws.ToInt32(d.Target.Port)
		t.AvailabilityZone = aws.ToString(d.Target.AvailabilityZone)
	}

	if d.TargetHealth != nil {
		t.State = d.TargetHealth.State
		t.Reason = d.TargetHealth.Reason
		t.Description = aws.ToString(d.TargetHealth.Description)
	}

	return t
}
