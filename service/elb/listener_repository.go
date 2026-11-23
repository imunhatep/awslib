package elb

import (
	stderrors "errors"
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

// listenerFetchConcurrency bounds the per-load-balancer DescribeListeners
// calls of ListListenersAll, for the same account-wide describe rate as
// healthFetchConcurrency.
const listenerFetchConcurrency = 4

// ListListenersAll lists the listeners of every load balancer in the region.
// DescribeListeners requires a load balancer (or listener) ARN, so this is one
// call per load balancer. A load balancer whose listeners cannot be read fails
// the listing, as do unreadable tags.
func (r *LoadBalancerRepository) ListListenersAll() ([]Listener, error) {
	start := time.Now()

	balancers, err := r.describeLoadBalancerArns()
	if err != nil {
		return nil, err
	}

	perLb := make([][]types.Listener, len(balancers))
	errs := make([]error, len(balancers))

	sem := make(chan struct{}, listenerFetchConcurrency)
	var wg sync.WaitGroup
	for i, lbArn := range balancers {
		wg.Add(1)

		go func(idx int, lbArn string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			perLb[idx], errs[idx] = r.describeListeners(&awselbv2.DescribeListenersInput{LoadBalancerArn: aws.String(lbArn)})

			// Deleted between DescribeLoadBalancers and now: it has no listeners.
			var notFound *types.LoadBalancerNotFoundException
			if stderrors.As(errs[idx], &notFound) {
				log.Debug().Str("loadBalancer", lbArn).
					Msg("[LoadBalancerRepository.ListListenersAll] load balancer disappeared during the listing")
				perLb[idx], errs[idx] = nil, nil
			}
		}(i, lbArn)
	}

	wg.Wait()

	var listeners []types.Listener
	for i, err := range errs {
		if err != nil {
			return nil, errors.Errorf("reading listeners of %s: %w", balancers[i], err)
		}

		listeners = append(listeners, perLb[i]...)
	}

	items, err := r.withListenerTags(listeners)
	if err != nil {
		return nil, err
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListListenersAll", cfg.ResourceTypeListenerV2)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}

// ListListenersByInput lists the listeners selected by describeInput, which
// must name a LoadBalancerArn or ListenerArns, with their tags.
func (r *LoadBalancerRepository) ListListenersByInput(describeInput *awselbv2.DescribeListenersInput) ([]Listener, error) {
	if describeInput == nil {
		return []Listener{}, nil
	}

	start := time.Now()

	listeners, err := r.describeListeners(describeInput)
	if err != nil {
		return nil, err
	}

	items, err := r.withListenerTags(listeners)
	if err != nil {
		return nil, err
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListListenersByInput", cfg.ResourceTypeListenerV2)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}

func (r *LoadBalancerRepository) withListenerTags(listeners []types.Listener) ([]Listener, error) {
	arns := make([]string, 0, len(listeners))
	for _, l := range listeners {
		arns = append(arns, aws.ToString(l.ListenerArn))
	}

	tags, err := r.describeTags(arns, cfg.ResourceTypeListenerV2)
	if err != nil {
		return nil, err
	}

	items := make([]Listener, 0, len(listeners))
	for _, l := range listeners {
		items = append(items, NewListener(r.client, l, tags[aws.ToString(l.ListenerArn)]))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeListeners", cfg.ResourceTypeListenerV2)).
			Add(float64(len(items)))
	}

	return items, nil
}

func (r *LoadBalancerRepository) describeListeners(describeInput *awselbv2.DescribeListenersInput) ([]types.Listener, error) {
	var listeners []types.Listener

	p := awselbv2.NewDescribeListenersPaginator(r.elbv2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeListeners", cfg.ResourceTypeListenerV2)).Inc()
		}

		resp, err := p.NextPage(r.ctx, func(o *awselbv2.Options) { o.RetryMaxAttempts = healthFetchMaxAttempts })
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeListeners", cfg.ResourceTypeListenerV2)).Inc()
			}

			return nil, errors.New(err)
		}

		listeners = append(listeners, resp.Listeners...)
	}

	return listeners, nil
}

// describeLoadBalancerArns pages through every load balancer and returns only
// the ARNs. ListLoadBalancersAll is not used for this: it reads each load
// balancer's tags (one call per load balancer) that listeners do not need.
func (r *LoadBalancerRepository) describeLoadBalancerArns() ([]string, error) {
	var arns []string

	p := awselbv2.NewDescribeLoadBalancersPaginator(r.elbv2Client(), &awselbv2.DescribeLoadBalancersInput{})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeListenerV2)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeListenerV2)).Inc()
			}

			return nil, errors.New(err)
		}

		for _, lb := range resp.LoadBalancers {
			arns = append(arns, aws.ToString(lb.LoadBalancerArn))
		}
	}

	return arns, nil
}
