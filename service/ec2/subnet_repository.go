package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListSubnetsAll() ([]Subnet, error) {
	return r.ListSubnetsByInput(&ec2.DescribeSubnetsInput{})
}

func (r *Ec2Repository) ListSubnetsByInput(describeInput *ec2.DescribeSubnetsInput) ([]Subnet, error) {
	if describeInput == nil {
		return []Subnet{}, nil
	}

	start := time.Now()
	var subnets []Subnet

	p := ec2.NewDescribeSubnetsPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeSubnets", cfg.ResourceTypeSubnet)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeSubnets", cfg.ResourceTypeSubnet)).
					Inc()
			}

			return subnets, errors.New(err)
		}

		for _, v := range resp.Subnets {
			subnets = append(subnets, NewSubnet(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeSubnets", cfg.ResourceTypeSubnet)).
			Add(float64(len(subnets)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListSubnetsByInput", cfg.ResourceTypeSubnet)).
			Observe(time.Since(start).Seconds())
	}

	return subnets, nil
}
