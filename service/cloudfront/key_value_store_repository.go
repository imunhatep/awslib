package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListKeyValueStoresAll lists every key value store in the account.
func (r *CloudFrontRepository) ListKeyValueStoresAll() ([]KeyValueStore, error) {
	return r.ListKeyValueStoresByInput(&awscf.ListKeyValueStoresInput{})
}

// ListKeyValueStoresByInput lists key value stores, following NextMarker to the end.
func (r *CloudFrontRepository) ListKeyValueStoresByInput(query *awscf.ListKeyValueStoresInput) ([]KeyValueStore, error) {
	start := time.Now()

	input := awscf.ListKeyValueStoresInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListKeyValueStores", ccfg.ResourceTypeCloudFrontKeyValueStore, input.Marker,
		func(marker *string) ([]cftypes.KeyValueStore, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListKeyValueStores(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.KeyValueStoreList == nil {
				return nil, nil, nil
			}

			return output.KeyValueStoreList.Items, output.KeyValueStoreList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]KeyValueStore, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, NewKeyValueStore(r.client, summary))
	}

	r.observeList("ListKeyValueStores", "ListKeyValueStoresByInput", ccfg.ResourceTypeCloudFrontKeyValueStore, len(items), start)

	return items, nil
}
