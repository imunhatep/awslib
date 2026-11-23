package elb

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awselbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// tagBatchSize is DescribeTags' limit on ResourceArns per call.
const tagBatchSize = 20

// describeTags reads the tags of the given ELBv2 resources (load balancers,
// target groups, listeners, rules), tagBatchSize ARNs per call, keyed by ARN.
// A batch that cannot be read fails the whole call: callers that returned the
// resources without tags would report "untagged" for "unknown".
func (r *LoadBalancerRepository) describeTags(arns []string, resourceType cfg.ResourceType) (map[string][]types.Tag, error) {
	out := make(map[string][]types.Tag, len(arns))

	for start := 0; start < len(arns); start += tagBatchSize {
		batch := arns[start:min(start+tagBatchSize, len(arns))]

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeTags", resourceType)).Inc()
		}

		resp, err := r.elbv2Client().DescribeTags(r.ctx, &awselbv2.DescribeTagsInput{ResourceArns: batch})
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeTags", resourceType)).Inc()
			}

			return nil, errors.Errorf("reading tags of %d resources starting at %s: %w", len(batch), batch[0], err)
		}

		for _, d := range resp.TagDescriptions {
			out[aws.ToString(d.ResourceArn)] = d.Tags
		}
	}

	return out, nil
}
