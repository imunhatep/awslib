package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListRealtimeLogConfigsAll lists every real-time log configuration in the account.
func (r *CloudFrontRepository) ListRealtimeLogConfigsAll() ([]RealtimeLogConfig, error) {
	return r.ListRealtimeLogConfigsByInput(&awscf.ListRealtimeLogConfigsInput{})
}

// ListRealtimeLogConfigsByInput lists real-time log configurations, following NextMarker to the end.
func (r *CloudFrontRepository) ListRealtimeLogConfigsByInput(query *awscf.ListRealtimeLogConfigsInput) ([]RealtimeLogConfig, error) {
	start := time.Now()

	input := awscf.ListRealtimeLogConfigsInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListRealtimeLogConfigs", ccfg.ResourceTypeCloudFrontRealtimeLogConfig, input.Marker,
		func(marker *string) ([]cftypes.RealtimeLogConfig, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListRealtimeLogConfigs(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.RealtimeLogConfigs == nil {
				return nil, nil, nil
			}

			return output.RealtimeLogConfigs.Items, output.RealtimeLogConfigs.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]RealtimeLogConfig, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, NewRealtimeLogConfig(r.client, summary))
	}

	r.observeList("ListRealtimeLogConfigs", "ListRealtimeLogConfigsByInput", ccfg.ResourceTypeCloudFrontRealtimeLogConfig, len(items), start)

	return items, nil
}
