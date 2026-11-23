package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListLaunchTemplatesAll() ([]LaunchTemplate, error) {
	return r.ListLaunchTemplatesByInput(&ec2.DescribeLaunchTemplatesInput{})
}

func (r *Ec2Repository) ListLaunchTemplatesByInput(describeInput *ec2.DescribeLaunchTemplatesInput) ([]LaunchTemplate, error) {
	if describeInput == nil {
		return []LaunchTemplate{}, nil
	}

	start := time.Now()
	var items []LaunchTemplate

	p := ec2.NewDescribeLaunchTemplatesPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeLaunchTemplates", cfg.ResourceTypeLaunchTemplate)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeLaunchTemplates", cfg.ResourceTypeLaunchTemplate)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.LaunchTemplates {
			items = append(items, NewLaunchTemplate(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeLaunchTemplates", cfg.ResourceTypeLaunchTemplate)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListLaunchTemplatesByInput", cfg.ResourceTypeLaunchTemplate)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
