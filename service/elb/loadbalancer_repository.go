package elb

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"time"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	ELBv2() *elasticloadbalancingv2.Client
}

type LoadBalancerRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewLoadBalancerRepository(ctx context.Context, client AwsClient) *LoadBalancerRepository {
	repo := &LoadBalancerRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *LoadBalancerRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *LoadBalancerRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *LoadBalancerRepository) ListLoadBalancersAll() ([]LoadBalancer, error) {
	return r.ListLoadBalancersByInput(&elasticloadbalancingv2.DescribeLoadBalancersInput{})
}

func (r *LoadBalancerRepository) ListLoadBalancersByInput(query *elasticloadbalancingv2.DescribeLoadBalancersInput) ([]LoadBalancer, error) {
	start := time.Now()
	var balancers []LoadBalancer

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeLoadBalancerV2)).Inc()
	}

	resp, err := r.client.ELBv2().DescribeLoadBalancers(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeLoadBalancerV2)).Inc()
		}

		return balancers, errors.New(err)
	}

	for _, v := range resp.LoadBalancers {
		tags, _ := r.GetLoadBalancerTags(v)

		bucket := NewLoadBalancer(r.client, v, tags)
		balancers = append(balancers, bucket)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeLoadBalancerV2)).
			Add(float64(len(balancers)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListLoadBalancersByInput", cfg.ResourceTypeLoadBalancerV2)).
			Observe(time.Since(start).Seconds())
	}

	return balancers, nil
}

func (r *LoadBalancerRepository) GetLoadBalancerTags(lb types.LoadBalancer) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeTags", cfg.ResourceTypeLoadBalancerV2)).Inc()
	}
	
	tagOutput, err := r.client.ELBv2().DescribeTags(r.ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(lb.LoadBalancerArn)},
	})

	if err != nil {
		log.
			Debug().
			Str("elasticloadbalancer", aws.ToString(lb.LoadBalancerName)).
			Err(err).
			Msg("failed to fetch elb tags")

		return []types.Tag{}, errors.New(err)
	}

	return slice.Head(tagOutput.TagDescriptions).OrElse(types.TagDescription{}).Tags, nil
}
