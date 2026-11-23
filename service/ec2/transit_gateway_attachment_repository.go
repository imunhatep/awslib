package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Ec2Repository) ListTransitGatewayAttachmentsAll() ([]TransitGatewayAttachment, error) {
	return r.ListTransitGatewayAttachmentsByInput(&ec2.DescribeTransitGatewayAttachmentsInput{})
}

func (r *Ec2Repository) ListTransitGatewayAttachmentsByInput(describeInput *ec2.DescribeTransitGatewayAttachmentsInput) ([]TransitGatewayAttachment, error) {
	if describeInput == nil {
		return []TransitGatewayAttachment{}, nil
	}

	start := time.Now()
	var items []TransitGatewayAttachment

	p := ec2.NewDescribeTransitGatewayAttachmentsPaginator(r.ec2Client(), describeInput)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeTransitGatewayAttachments", cfg.ResourceTypeTransitGatewayAttachment)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeTransitGatewayAttachments", cfg.ResourceTypeTransitGatewayAttachment)).
					Inc()
			}

			return items, errors.New(err)
		}

		for _, v := range resp.TransitGatewayAttachments {
			items = append(items, NewTransitGatewayAttachment(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeTransitGatewayAttachments", cfg.ResourceTypeTransitGatewayAttachment)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTransitGatewayAttachmentsByInput", cfg.ResourceTypeTransitGatewayAttachment)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
