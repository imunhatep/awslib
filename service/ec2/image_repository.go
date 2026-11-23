package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListImagesAll returns the AMIs owned by the account. The public catalogue is enormous and
// costs nothing to the caller; owned images are what bill, through the snapshots behind them.
func (r *Ec2Repository) ListImagesAll() ([]Image, error) {
	return r.ListImagesByInput(&ec2.DescribeImagesInput{Owners: []string{"self"}})
}

func (r *Ec2Repository) ListImagesByInput(describeInput *ec2.DescribeImagesInput) ([]Image, error) {
	if describeInput == nil {
		return []Image{}, nil
	}

	start := time.Now()
	var items []Image

	p := ec2.NewDescribeImagesPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeImages", ccfg.ResourceTypeEc2Image)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeImages", ccfg.ResourceTypeEc2Image)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.Images {
			items = append(items, NewImage(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeImages", ccfg.ResourceTypeEc2Image)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListImagesByInput", ccfg.ResourceTypeEc2Image)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
