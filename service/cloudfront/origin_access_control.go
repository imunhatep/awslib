package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// OriginAccessControlList holds a list of OriginAccessControl items.
type OriginAccessControlList struct {
	Items []OriginAccessControl
}

// OriginAccessControl (OAC) is how a distribution signs its requests to an S3,
// MediaStore, MediaPackage or Lambda URL origin. An origin references it through
// Origin.OriginAccessControlId. OACs have no ARN in the list response and are
// not taggable.
type OriginAccessControl struct {
	service.AbstractResource
	cftypes.OriginAccessControlSummary
}

func NewOriginAccessControl(client AwsClient, summary cftypes.OriginAccessControlSummary) OriginAccessControl {
	return OriginAccessControl{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontOriginAccessControl,
		},
		OriginAccessControlSummary: summary,
	}
}

func (e OriginAccessControl) GetName() string {
	return aws.ToString(e.Name)
}

// GetTags returns an empty map: origin access controls are not taggable.
func (e OriginAccessControl) GetTags() map[string]string {
	return map[string]string{}
}
