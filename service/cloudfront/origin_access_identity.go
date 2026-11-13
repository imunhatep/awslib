package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// OriginAccessIdentityList holds a list of OriginAccessIdentity items.
type OriginAccessIdentityList struct {
	Items []OriginAccessIdentity
}

// OriginAccessIdentity is a legacy CloudFront origin access identity (OAI), the
// predecessor of OriginAccessControl for S3 origins. An origin references it as
// "origin-access-identity/cloudfront/<id>" in S3OriginConfig.OriginAccessIdentity;
// S3CanonicalUserId is the principal a bucket policy grants. OAIs carry only a
// comment, no name, and are not taggable.
type OriginAccessIdentity struct {
	service.AbstractResource
	cftypes.CloudFrontOriginAccessIdentitySummary
}

func NewOriginAccessIdentity(client AwsClient, summary cftypes.CloudFrontOriginAccessIdentitySummary) OriginAccessIdentity {
	return OriginAccessIdentity{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontOriginAccessIdentity,
		},
		CloudFrontOriginAccessIdentitySummary: summary,
	}
}

// GetName returns the comment, the only human-readable label an OAI has, and
// falls back to the ID.
func (e OriginAccessIdentity) GetName() string {
	if comment := aws.ToString(e.Comment); comment != "" {
		return comment
	}

	return e.GetId()
}

// GetTags returns an empty map: origin access identities are not taggable.
func (e OriginAccessIdentity) GetTags() map[string]string {
	return map[string]string{}
}
