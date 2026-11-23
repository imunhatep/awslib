package elb

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awselbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/elasticloadbalancingv2"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type LoadBalancerRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewLoadBalancerRepository(ctx context.Context, client *v3.Client) *LoadBalancerRepository {
	repo := &LoadBalancerRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *LoadBalancerRepository) elbv2Client() *awselbv2.Client {
	return elasticloadbalancingv2.GetClient(r.client)
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
	return r.ListLoadBalancersByInput(&awselbv2.DescribeLoadBalancersInput{})
}

// ListLoadBalancersByInput pages through DescribeLoadBalancers and reads the
// tags in batches. Tags that cannot be read fail the listing: a load balancer
// returned without them would read as untagged.
func (r *LoadBalancerRepository) ListLoadBalancersByInput(query *awselbv2.DescribeLoadBalancersInput) ([]LoadBalancer, error) {
	start := time.Now()

	if query == nil {
		query = &awselbv2.DescribeLoadBalancersInput{}
	}

	var lbs []types.LoadBalancer

	p := awselbv2.NewDescribeLoadBalancersPaginator(r.elbv2Client(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeLoadBalancerV2)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeLoadBalancers", cfg.ResourceTypeLoadBalancerV2)).Inc()
			}

			return nil, errors.New(err)
		}

		lbs = append(lbs, resp.LoadBalancers...)
	}

	arns := make([]string, 0, len(lbs))
	for _, lb := range lbs {
		arns = append(arns, aws.ToString(lb.LoadBalancerArn))
	}

	tags, err := r.describeTags(arns, cfg.ResourceTypeLoadBalancerV2)
	if err != nil {
		return nil, err
	}

	balancers := make([]LoadBalancer, 0, len(lbs))
	for _, lb := range lbs {
		balancers = append(balancers, NewLoadBalancer(r.client, lb, tags[aws.ToString(lb.LoadBalancerArn)]))
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

	tagOutput, err := r.elbv2Client().DescribeTags(r.ctx, &awselbv2.DescribeTagsInput{
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
