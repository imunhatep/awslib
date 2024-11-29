package ec2

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *Ec2Repository) ListInstancesAll() ([]Instance, error) {
	return r.ListInstancesByInput(&ec2.DescribeInstancesInput{})
}

func (r *Ec2Repository) ListInstancesByInput(query *ec2.DescribeInstancesInput) ([]Instance, error) {
	start := time.Now()
	var instances []Instance

	p := ec2.NewDescribeInstancesPaginator(r.client.EC2(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeInstances", cfg.ResourceTypeInstance)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			// metrics
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeInstances", cfg.ResourceTypeInstance)).Inc()
			}

			return instances, errors.New(err)
		}

		for _, reservations := range resp.Reservations {
			for _, v := range reservations.Instances {
				instance := NewInstance(r.client, v)
				instances = append(instances, instance)
			}
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("DescribeInstances", cfg.ResourceTypeInstance)).Add(float64(len(instances)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.With(r.promLabels("DescribeInstances", cfg.ResourceTypeInstance)).Observe(time.Since(start).Seconds())
	}

	return instances, nil
}

func (r *Ec2Repository) GetInstanceTypes() ([]types.InstanceType, error) {
	start := time.Now()
	instanceTypes := []types.InstanceType{}

	paginator := ec2.NewDescribeInstanceTypesPaginator(r.client.EC2(), &ec2.DescribeInstanceTypesInput{})
	for paginator.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeInstanceTypes", cfg.ResourceTypeInstance)).Inc()
		}

		resp, err := paginator.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeInstanceTypes", cfg.ResourceTypeInstance)).Inc()
			}

			return instanceTypes, errors.New(err)
		}

		for _, v := range resp.InstanceTypes {
			instanceTypes = append(instanceTypes, v.InstanceType)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("DescribeInstanceTypes", cfg.ResourceTypeInstance)).Add(float64(len(instanceTypes)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.With(r.promLabels("DescribeInstanceTypes", cfg.ResourceTypeInstance)).Observe(time.Since(start).Seconds())
	}

	return instanceTypes, nil
}
