package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListVpcPeeringConnectionsAll() ([]VpcPeeringConnection, error) {
	return r.ListVpcPeeringConnectionsByInput(&ec2.DescribeVpcPeeringConnectionsInput{})
}

func (r *Ec2Repository) ListVpcPeeringConnectionsByInput(describeInput *ec2.DescribeVpcPeeringConnectionsInput) ([]VpcPeeringConnection, error) {
	if describeInput == nil {
		return []VpcPeeringConnection{}, nil
	}

	start := time.Now()
	var items []VpcPeeringConnection

	p := ec2.NewDescribeVpcPeeringConnectionsPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeVpcPeeringConnections", cfg.ResourceTypeVPCPeeringConnection)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeVpcPeeringConnections", cfg.ResourceTypeVPCPeeringConnection)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.VpcPeeringConnections {
			items = append(items, NewVpcPeeringConnection(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeVpcPeeringConnections", cfg.ResourceTypeVPCPeeringConnection)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListVpcPeeringConnectionsByInput", cfg.ResourceTypeVPCPeeringConnection)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
