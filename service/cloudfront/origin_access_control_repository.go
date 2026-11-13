package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListOriginAccessControlsAll lists every origin access control in the account.
func (r *CloudFrontRepository) ListOriginAccessControlsAll() ([]OriginAccessControl, error) {
	return r.ListOriginAccessControlsByInput(&awscf.ListOriginAccessControlsInput{})
}

// ListOriginAccessControlsByInput lists origin access controls, following NextMarker to the end.
func (r *CloudFrontRepository) ListOriginAccessControlsByInput(query *awscf.ListOriginAccessControlsInput) ([]OriginAccessControl, error) {
	start := time.Now()

	input := awscf.ListOriginAccessControlsInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListOriginAccessControls", ccfg.ResourceTypeCloudFrontOriginAccessControl, input.Marker,
		func(marker *string) ([]cftypes.OriginAccessControlSummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListOriginAccessControls(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.OriginAccessControlList == nil {
				return nil, nil, nil
			}

			return output.OriginAccessControlList.Items, output.OriginAccessControlList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]OriginAccessControl, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, NewOriginAccessControl(r.client, summary))
	}

	r.observeList("ListOriginAccessControls", "ListOriginAccessControlsByInput", ccfg.ResourceTypeCloudFrontOriginAccessControl, len(items), start)

	return items, nil
}
