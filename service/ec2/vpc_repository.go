package ec2

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *Ec2Repository) ListVpcsAll() ([]Vpc, error) {
	return r.ListVpcsByInput(&ec2.DescribeVpcsInput{})
}

func (r *Ec2Repository) ListVpcsByInput(describeInput *ec2.DescribeVpcsInput) ([]Vpc, error) {
	if describeInput == nil {
		return []Vpc{}, nil
	}

	start := time.Now()
	var volumes []Vpc

	p := ec2.NewDescribeVpcsPaginator(r.client.EC2(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeVpcs", cfg.ResourceTypeVpc)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeVpcs", cfg.ResourceTypeVpc)).
					Inc()
			}

			return volumes, errors.New(err)
		}

		for _, v := range resp.Vpcs {
			volumes = append(volumes, NewVpc(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeVpcs", cfg.ResourceTypeVpc)).
			Add(float64(len(volumes)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListVpcsByInput", cfg.ResourceTypeVpc)).
			Observe(time.Since(start).Seconds())
	}

	return volumes, nil
}

func (r *Ec2Repository) DeleteVpc(deleteInput *ec2.DeleteVpcInput) (*ec2.DeleteVpcOutput, error) {
	if deleteInput == nil {
		return &ec2.DeleteVpcOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DeleteVpc", cfg.ResourceTypeVpc)).
			Inc()
	}

	deleteVpcOutput, err := r.client.EC2().DeleteVpc(r.ctx, deleteInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DeleteVpc", cfg.ResourceTypeVpc)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteVpc", cfg.ResourceTypeVpc)).
			Observe(time.Since(start).Seconds())
	}

	return deleteVpcOutput, nil
}

func (r *Ec2Repository) CreateVpcTags(tagsInput *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	if tagsInput == nil {
		return &ec2.CreateTagsOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("CreateTags", cfg.ResourceTypeVpc)).
			Inc()
	}

	output, err := r.client.EC2().CreateTags(r.ctx, tagsInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("CreateTags", cfg.ResourceTypeVpc)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateVpcTags", cfg.ResourceTypeVpc)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

func (r *Ec2Repository) DeleteVpcTags(tagsInput *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error) {
	if tagsInput == nil {
		return &ec2.DeleteTagsOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DeleteVpcTags", cfg.ResourceTypeVpc)).
			Inc()
	}

	output, err := r.client.EC2().DeleteTags(r.ctx, tagsInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DeleteVpcTags", cfg.ResourceTypeVpc)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteVpcTags", cfg.ResourceTypeVpc)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}
