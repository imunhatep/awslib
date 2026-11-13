package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListKeyGroupsAll lists every key group in the account.
func (r *CloudFrontRepository) ListKeyGroupsAll() ([]KeyGroup, error) {
	return r.ListKeyGroupsByInput(&awscf.ListKeyGroupsInput{})
}

// ListKeyGroupsByInput lists key groups, following NextMarker to the end.
func (r *CloudFrontRepository) ListKeyGroupsByInput(query *awscf.ListKeyGroupsInput) ([]KeyGroup, error) {
	start := time.Now()

	input := awscf.ListKeyGroupsInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListKeyGroups", ccfg.ResourceTypeCloudFrontKeyGroup, input.Marker,
		func(marker *string) ([]cftypes.KeyGroupSummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListKeyGroups(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.KeyGroupList == nil {
				return nil, nil, nil
			}

			return output.KeyGroupList.Items, output.KeyGroupList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]KeyGroup, 0, len(summaries))
	for _, summary := range summaries {
		if summary.KeyGroup == nil {
			continue
		}

		items = append(items, NewKeyGroup(r.client, *summary.KeyGroup))
	}

	r.observeList("ListKeyGroups", "ListKeyGroupsByInput", ccfg.ResourceTypeCloudFrontKeyGroup, len(items), start)

	return items, nil
}
