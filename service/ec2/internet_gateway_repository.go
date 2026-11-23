package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListInternetGatewaysAll() ([]InternetGateway, error) {
	return r.ListInternetGatewaysByInput(&ec2.DescribeInternetGatewaysInput{})
}

func (r *Ec2Repository) ListInternetGatewaysByInput(describeInput *ec2.DescribeInternetGatewaysInput) ([]InternetGateway, error) {
	if describeInput == nil {
		return []InternetGateway{}, nil
	}

	start := time.Now()
	var items []InternetGateway

	p := ec2.NewDescribeInternetGatewaysPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeInternetGateways", cfg.ResourceTypeInternetGateway)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeInternetGateways", cfg.ResourceTypeInternetGateway)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.InternetGateways {
			items = append(items, NewInternetGateway(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeInternetGateways", cfg.ResourceTypeInternetGateway)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListInternetGatewaysByInput", cfg.ResourceTypeInternetGateway)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
