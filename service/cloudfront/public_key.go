package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// PublicKeyList holds a list of PublicKey items.
type PublicKeyList struct {
	Items []PublicKey
}

// PublicKey is a public key CloudFront verifies signed URLs and cookies with,
// grouped into KeyGroups. EncodedKey is the PEM public key — public by
// definition, so carrying it is harmless. No ARN, not taggable.
type PublicKey struct {
	service.AbstractResource
	cftypes.PublicKeySummary
}

func NewPublicKey(client AwsClient, summary cftypes.PublicKeySummary) PublicKey {
	return PublicKey{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Id),
			CreatedAt: createdOrEpoch(summary.CreatedTime),
			Type:      ccfg.ResourceTypeCloudFrontPublicKey,
		},
		PublicKeySummary: summary,
	}
}

func (e PublicKey) GetName() string {
	return aws.ToString(e.Name)
}

// GetTags returns an empty map: public keys are not taggable.
func (e PublicKey) GetTags() map[string]string {
	return map[string]string{}
}
