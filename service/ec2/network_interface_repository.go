package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// ListNetworkInterfacesAll returns every network interface in the account/region, including
// the requester-managed ones that load balancers, NAT gateways and Lambda create.
func (r *Ec2Repository) ListNetworkInterfacesAll() ([]NetworkInterface, error) {
	return r.ListNetworkInterfacesByInput(&ec2.DescribeNetworkInterfacesInput{})
}

func (r *Ec2Repository) ListNetworkInterfacesByInput(describeInput *ec2.DescribeNetworkInterfacesInput) ([]NetworkInterface, error) {
	if describeInput == nil {
		return []NetworkInterface{}, nil
	}

	start := time.Now()
	var interfaces []NetworkInterface

	p := ec2.NewDescribeNetworkInterfacesPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeNetworkInterfaces", cfg.ResourceTypeNetworkInterface)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeNetworkInterfaces", cfg.ResourceTypeNetworkInterface)).
					Inc()
			}

			return interfaces, errors.New(err)
		}

		for _, v := range resp.NetworkInterfaces {
			interfaces = append(interfaces, NewNetworkInterface(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeNetworkInterfaces", cfg.ResourceTypeNetworkInterface)).
			Add(float64(len(interfaces)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListNetworkInterfacesByInput", cfg.ResourceTypeNetworkInterface)).
			Observe(time.Since(start).Seconds())
	}

	return interfaces, nil
}
