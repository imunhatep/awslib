package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListNetworkAclsAll() ([]NetworkAcl, error) {
	return r.ListNetworkAclsByInput(&ec2.DescribeNetworkAclsInput{})
}

func (r *Ec2Repository) ListNetworkAclsByInput(describeInput *ec2.DescribeNetworkAclsInput) ([]NetworkAcl, error) {
	if describeInput == nil {
		return []NetworkAcl{}, nil
	}

	start := time.Now()
	var items []NetworkAcl

	p := ec2.NewDescribeNetworkAclsPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeNetworkAcls", cfg.ResourceTypeNetworkAcl)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeNetworkAcls", cfg.ResourceTypeNetworkAcl)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.NetworkAcls {
			items = append(items, NewNetworkAcl(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeNetworkAcls", cfg.ResourceTypeNetworkAcl)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListNetworkAclsByInput", cfg.ResourceTypeNetworkAcl)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
