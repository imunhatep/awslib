package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListTransitGatewaysAll() ([]TransitGateway, error) {
	return r.ListTransitGatewaysByInput(&ec2.DescribeTransitGatewaysInput{})
}

func (r *Ec2Repository) ListTransitGatewaysByInput(describeInput *ec2.DescribeTransitGatewaysInput) ([]TransitGateway, error) {
	if describeInput == nil {
		return []TransitGateway{}, nil
	}

	start := time.Now()
	var items []TransitGateway

	p := ec2.NewDescribeTransitGatewaysPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeTransitGateways", cfg.ResourceTypeTransitGateway)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeTransitGateways", cfg.ResourceTypeTransitGateway)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.TransitGateways {
			items = append(items, NewTransitGateway(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeTransitGateways", cfg.ResourceTypeTransitGateway)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTransitGatewaysByInput", cfg.ResourceTypeTransitGateway)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
