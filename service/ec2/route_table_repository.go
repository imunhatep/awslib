package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListRouteTablesAll() ([]RouteTable, error) {
	return r.ListRouteTablesByInput(&ec2.DescribeRouteTablesInput{})
}

func (r *Ec2Repository) ListRouteTablesByInput(describeInput *ec2.DescribeRouteTablesInput) ([]RouteTable, error) {
	if describeInput == nil {
		return []RouteTable{}, nil
	}

	start := time.Now()
	var items []RouteTable

	p := ec2.NewDescribeRouteTablesPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeRouteTables", cfg.ResourceTypeRouteTable)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeRouteTables", cfg.ResourceTypeRouteTable)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.RouteTables {
			items = append(items, NewRouteTable(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeRouteTables", cfg.ResourceTypeRouteTable)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListRouteTablesByInput", cfg.ResourceTypeRouteTable)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
