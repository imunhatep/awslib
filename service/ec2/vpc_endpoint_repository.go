package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListVpcEndpointsAll() ([]VpcEndpoint, error) {
	return r.ListVpcEndpointsByInput(&ec2.DescribeVpcEndpointsInput{})
}

func (r *Ec2Repository) ListVpcEndpointsByInput(describeInput *ec2.DescribeVpcEndpointsInput) ([]VpcEndpoint, error) {
	if describeInput == nil {
		return []VpcEndpoint{}, nil
	}

	start := time.Now()
	var items []VpcEndpoint

	p := ec2.NewDescribeVpcEndpointsPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeVpcEndpoints", cfg.ResourceTypeVPCEndpoint)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeVpcEndpoints", cfg.ResourceTypeVPCEndpoint)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.VpcEndpoints {
			items = append(items, NewVpcEndpoint(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeVpcEndpoints", cfg.ResourceTypeVPCEndpoint)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListVpcEndpointsByInput", cfg.ResourceTypeVPCEndpoint)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
