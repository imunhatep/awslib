package elb

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awselbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

// healthFetchConcurrency and healthFetchMaxAttempts shape the per-target-group
// DescribeTargetHealth reads. There is no bulk form, so a listing costs one call
// per target group; ELB's describe calls share an account-wide request rate, so
// the fan-out is kept small and each call gets a larger retry budget than the
// SDK's default 3 attempts.
const (
	healthFetchConcurrency = 4
	healthFetchMaxAttempts = 10
)

func (r *LoadBalancerRepository) ListTargetGroupsAll() ([]TargetGroup, error) {
	return r.ListTargetGroupsByInput(&awselbv2.DescribeTargetGroupsInput{})
}

// ListTargetGroupsByInput lists target groups with their tags and target health.
//
// Tags that cannot be read fail the listing. Target health that cannot be read
// for one target group does not: that group is returned with HealthError set,
// so "health unknown" is never presented as "no targets".
func (r *LoadBalancerRepository) ListTargetGroupsByInput(describeInput *awselbv2.DescribeTargetGroupsInput) ([]TargetGroup, error) {
	if describeInput == nil {
		return []TargetGroup{}, nil
	}

	start := time.Now()
	var groups []types.TargetGroup

	p := awselbv2.NewDescribeTargetGroupsPaginator(r.elbv2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeTargetGroups", cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeTargetGroups", cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)).
					Inc()
			}

			return nil, errors.New(err)
		}

		groups = append(groups, resp.TargetGroups...)
	}

	arns := make([]string, 0, len(groups))
	for _, tg := range groups {
		arns = append(arns, aws.ToString(tg.TargetGroupArn))
	}

	tags, err := r.describeTags(arns, cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)
	if err != nil {
		return nil, err
	}

	items := make([]TargetGroup, len(groups))

	sem := make(chan struct{}, healthFetchConcurrency)
	var wg sync.WaitGroup
	for i, tg := range groups {
		wg.Add(1)

		go func(idx int, tg types.TargetGroup) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			targets, err := r.describeTargetHealth(aws.ToString(tg.TargetGroupArn))
			if err != nil {
				log.Warn().Err(err).
					Str("targetGroup", aws.ToString(tg.TargetGroupName)).
					Msg("[LoadBalancerRepository.ListTargetGroupsByInput] failed to read target health; target group returned with HealthError")
			}

			items[idx] = NewTargetGroup(r.client, tg, tags[aws.ToString(tg.TargetGroupArn)], targets, time.Now().UTC(), err)
		}(i, tg)
	}

	wg.Wait()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeTargetGroups", cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)).
			Add(float64(len(items)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTargetGroupsByInput", cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}

// describeTargetHealth reads the health of every target registered with one
// target group, sorted by target ID then port so a cached and a fresh listing
// compare equal.
func (r *LoadBalancerRepository) describeTargetHealth(targetGroupArn string) ([]Target, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeTargetHealth", cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)).
			Inc()
	}

	resp, err := r.elbv2Client().DescribeTargetHealth(r.ctx, &awselbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(targetGroupArn),
	}, func(o *awselbv2.Options) { o.RetryMaxAttempts = healthFetchMaxAttempts })
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DescribeTargetHealth", cfg.ResourceTypeElasticLoadBalancingV2TargetGroup)).
				Inc()
		}

		return nil, errors.New(err)
	}

	targets := make([]Target, 0, len(resp.TargetHealthDescriptions))
	for _, d := range resp.TargetHealthDescriptions {
		targets = append(targets, newTarget(d))
	}

	slices.SortFunc(targets, func(a, b Target) int {
		return cmp.Or(cmp.Compare(a.Id, b.Id), cmp.Compare(a.Port, b.Port))
	})

	return targets, nil
}
