package ec2

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *Ec2Repository) ListVolumesAll() ([]Volume, error) {
	return r.ListVolumesByInput(&ec2.DescribeVolumesInput{})
}

func (r *Ec2Repository) ListVolumesByInput(describeInput *ec2.DescribeVolumesInput) ([]Volume, error) {
	if describeInput == nil {
		return []Volume{}, nil
	}

	start := time.Now()
	var volumes []Volume

	p := ec2.NewDescribeVolumesPaginator(r.client.EC2(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeVolumes", cfg.ResourceTypeVolume)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeVolumes", cfg.ResourceTypeVolume)).
					Inc()
			}

			return volumes, errors.New(err)
		}

		for _, v := range resp.Volumes {
			volumes = append(volumes, NewVolume(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeVolumes", cfg.ResourceTypeVolume)).
			Add(float64(len(volumes)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListVolumesByInput", cfg.ResourceTypeVolume)).
			Observe(time.Since(start).Seconds())
	}

	return volumes, nil
}

func (r *Ec2Repository) DeleteVolume(deleteInput *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error) {
	if deleteInput == nil {
		return &ec2.DeleteVolumeOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DeleteVolume", cfg.ResourceTypeVolume)).
			Inc()
	}

	deleteVolumeOutput, err := r.client.EC2().DeleteVolume(r.ctx, deleteInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DeleteVolume", cfg.ResourceTypeVolume)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteVolume", cfg.ResourceTypeVolume)).
			Observe(time.Since(start).Seconds())
	}

	return deleteVolumeOutput, nil
}

func (r *Ec2Repository) CreateVolumeTags(tagsInput *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	if tagsInput == nil {
		return &ec2.CreateTagsOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("CreateTags", cfg.ResourceTypeVolume)).
			Inc()
	}

	output, err := r.client.EC2().CreateTags(r.ctx, tagsInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("CreateTags", cfg.ResourceTypeVolume)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateVolumeTags", cfg.ResourceTypeVolume)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

func (r *Ec2Repository) DeleteVolumeTags(tagsInput *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error) {
	if tagsInput == nil {
		return &ec2.DeleteTagsOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DeleteVolumeTags", cfg.ResourceTypeVolume)).
			Inc()
	}

	output, err := r.client.EC2().DeleteTags(r.ctx, tagsInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DeleteVolumeTags", cfg.ResourceTypeVolume)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteVolumeTags", cfg.ResourceTypeVolume)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}
