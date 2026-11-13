package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// KeyGroupList holds a list of KeyGroup items.
type KeyGroupList struct {
	Items []KeyGroup
}

// KeyGroup is a set of PublicKeys a cache behaviour trusts for signed URLs and
// cookies (CacheBehavior.TrustedKeyGroups). No ARN, not taggable.
type KeyGroup struct {
	service.AbstractResource
	cftypes.KeyGroup
}

func NewKeyGroup(client AwsClient, group cftypes.KeyGroup) KeyGroup {
	return KeyGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(group.Id),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontKeyGroup,
		},
		KeyGroup: group,
	}
}

func (e KeyGroup) GetName() string {
	if e.KeyGroupConfig == nil {
		return e.GetId()
	}

	return aws.ToString(e.KeyGroupConfig.Name)
}

// GetTags returns an empty map: key groups are not taggable.
func (e KeyGroup) GetTags() map[string]string {
	return map[string]string{}
}

// GetPublicKeyIds returns the IDs of the public keys in the group.
func (e KeyGroup) GetPublicKeyIds() []string {
	if e.KeyGroupConfig == nil {
		return nil
	}

	return append([]string(nil), e.KeyGroupConfig.Items...)
}
