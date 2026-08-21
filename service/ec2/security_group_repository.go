package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListSecurityGroupsAll() ([]SecurityGroup, error) {
	return r.ListSecurityGroupsByInput(&ec2.DescribeSecurityGroupsInput{})
}

func (r *Ec2Repository) ListSecurityGroupsByInput(describeInput *ec2.DescribeSecurityGroupsInput) ([]SecurityGroup, error) {
	if describeInput == nil {
		return []SecurityGroup{}, nil
	}

	start := time.Now()
	var items []SecurityGroup

	p := ec2.NewDescribeSecurityGroupsPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeSecurityGroups", cfg.ResourceTypeSecurityGroup)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeSecurityGroups", cfg.ResourceTypeSecurityGroup)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.SecurityGroups {
			items = append(items, NewSecurityGroup(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeSecurityGroups", cfg.ResourceTypeSecurityGroup)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListSecurityGroupsByInput", cfg.ResourceTypeSecurityGroup)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
