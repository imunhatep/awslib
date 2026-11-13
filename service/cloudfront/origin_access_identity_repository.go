package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListOriginAccessIdentitiesAll lists every legacy origin access identity in the account.
func (r *CloudFrontRepository) ListOriginAccessIdentitiesAll() ([]OriginAccessIdentity, error) {
	return r.ListOriginAccessIdentitiesByInput(&awscf.ListCloudFrontOriginAccessIdentitiesInput{})
}

// ListOriginAccessIdentitiesByInput lists legacy origin access identitys, following NextMarker to the end.
func (r *CloudFrontRepository) ListOriginAccessIdentitiesByInput(query *awscf.ListCloudFrontOriginAccessIdentitiesInput) ([]OriginAccessIdentity, error) {
	start := time.Now()

	input := awscf.ListCloudFrontOriginAccessIdentitiesInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListCloudFrontOriginAccessIdentities", ccfg.ResourceTypeCloudFrontOriginAccessIdentity, input.Marker,
		func(marker *string) ([]cftypes.CloudFrontOriginAccessIdentitySummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListCloudFrontOriginAccessIdentities(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.CloudFrontOriginAccessIdentityList == nil {
				return nil, nil, nil
			}

			return output.CloudFrontOriginAccessIdentityList.Items, output.CloudFrontOriginAccessIdentityList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]OriginAccessIdentity, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, NewOriginAccessIdentity(r.client, summary))
	}

	r.observeList("ListCloudFrontOriginAccessIdentities", "ListOriginAccessIdentitiesByInput", ccfg.ResourceTypeCloudFrontOriginAccessIdentity, len(items), start)

	return items, nil
}
