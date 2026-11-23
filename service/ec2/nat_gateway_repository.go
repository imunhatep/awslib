package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListNatGatewaysAll() ([]NatGateway, error) {
	return r.ListNatGatewaysByInput(&ec2.DescribeNatGatewaysInput{})
}

func (r *Ec2Repository) ListNatGatewaysByInput(describeInput *ec2.DescribeNatGatewaysInput) ([]NatGateway, error) {
	if describeInput == nil {
		return []NatGateway{}, nil
	}

	start := time.Now()
	var items []NatGateway

	p := ec2.NewDescribeNatGatewaysPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeNatGateways", cfg.ResourceTypeNatGateway)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeNatGateways", cfg.ResourceTypeNatGateway)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.NatGateways {
			items = append(items, NewNatGateway(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeNatGateways", cfg.ResourceTypeNatGateway)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListNatGatewaysByInput", cfg.ResourceTypeNatGateway)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
