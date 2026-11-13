package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListPublicKeysAll lists every public key in the account.
func (r *CloudFrontRepository) ListPublicKeysAll() ([]PublicKey, error) {
	return r.ListPublicKeysByInput(&awscf.ListPublicKeysInput{})
}

// ListPublicKeysByInput lists public keys, following NextMarker to the end.
func (r *CloudFrontRepository) ListPublicKeysByInput(query *awscf.ListPublicKeysInput) ([]PublicKey, error) {
	start := time.Now()

	input := awscf.ListPublicKeysInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListPublicKeys", ccfg.ResourceTypeCloudFrontPublicKey, input.Marker,
		func(marker *string) ([]cftypes.PublicKeySummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListPublicKeys(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.PublicKeyList == nil {
				return nil, nil, nil
			}

			return output.PublicKeyList.Items, output.PublicKeyList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	items := make([]PublicKey, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, NewPublicKey(r.client, summary))
	}

	r.observeList("ListPublicKeys", "ListPublicKeysByInput", ccfg.ResourceTypeCloudFrontPublicKey, len(items), start)

	return items, nil
}
