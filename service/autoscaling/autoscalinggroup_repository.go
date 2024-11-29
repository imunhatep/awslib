package autoscaling

import (
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *AutoscalingRepository) ListAutoScalingGroupsAll() ([]AutoScalingGroup, error) {
	return r.ListAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
}

func (r *AutoscalingRepository) ListAutoScalingGroups(query *autoscaling.DescribeAutoScalingGroupsInput) ([]AutoScalingGroup, error) {
	start := time.Now()
	var groups []AutoScalingGroup

	p := autoscaling.NewDescribeAutoScalingGroupsPaginator(r.client.Autoscaling(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeAutoScalingGroups", cfg.ResourceTypeAutoScalingGroup)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeAutoScalingGroups", cfg.ResourceTypeAutoScalingGroup)).
					Inc()
			}

			return groups, errors.New(err)
		}

		for _, v := range resp.AutoScalingGroups {
			asg := NewAutoScalingGroup(r.client, v)
			groups = append(groups, asg)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeAutoScalingGroups", cfg.ResourceTypeAutoScalingGroup)).
			Add(float64(len(groups)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListAutoScalingGroups", cfg.ResourceTypeAutoScalingGroup)).
			Observe(time.Since(start).Seconds())
	}

	return groups, nil
}
