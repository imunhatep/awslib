package cloudfront

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListVpcOriginsAll lists every VPC origin in the account, with its tags.
func (r *CloudFrontRepository) ListVpcOriginsAll() ([]VpcOrigin, error) {
	return r.ListVpcOriginsByInput(&awscf.ListVpcOriginsInput{})
}

// ListVpcOriginsByInput lists VPC origins and reads their tags.
//
// Tags come from one tagging sweep, with the same rule as the distributions: an
// origin whose tags cannot be read fails the listing rather than being returned
// untagged.
func (r *CloudFrontRepository) ListVpcOriginsByInput(query *awscf.ListVpcOriginsInput) ([]VpcOrigin, error) {
	start := time.Now()

	input := awscf.ListVpcOriginsInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListVpcOrigins", ccfg.ResourceTypeCloudFrontVpcOrigin, input.Marker,
		func(marker *string) ([]cftypes.VpcOriginSummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListVpcOrigins(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.VpcOriginList == nil {
				return nil, nil, nil
			}

			return output.VpcOriginList.Items, output.VpcOriginList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	arns := make([]string, len(summaries))
	for i, summary := range summaries {
		arns[i] = aws.ToString(summary.Arn)
	}

	tags, err := r.bulkTags(arns, vpcOriginTagSource)
	if err != nil {
		return nil, err
	}

	origins := make([]VpcOrigin, 0, len(summaries))
	for i, summary := range summaries {
		origins = append(origins, NewVpcOrigin(r.client, summary, tags[i]))
	}

	r.observeList("ListVpcOrigins", "ListVpcOriginsByInput", ccfg.ResourceTypeCloudFrontVpcOrigin, len(origins), start)

	return origins, nil
}
