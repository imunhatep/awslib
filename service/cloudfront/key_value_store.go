package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// KeyValueStoreList holds a list of KeyValueStore items.
type KeyValueStoreList struct {
	Items []KeyValueStore
}

// KeyValueStore is a CloudFront KeyValueStore that CloudFront Functions read
// (FunctionConfig.KeyValueStoreAssociations, by ARN). Only its metadata is
// listed; the keys live behind the separate cloudfront-keyvaluestore API. Not
// taggable through the CloudFront API.
type KeyValueStore struct {
	service.AbstractResource
	cftypes.KeyValueStore
}

func NewKeyValueStore(client AwsClient, store cftypes.KeyValueStore) KeyValueStore {
	return KeyValueStore{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(store.Id),
			ARN:       parseArn(store.ARN),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontKeyValueStore,
		},
		KeyValueStore: store,
	}
}

func (e KeyValueStore) GetName() string {
	return aws.ToString(e.Name)
}

// GetTags returns an empty map: key value stores are not taggable.
func (e KeyValueStore) GetTags() map[string]string {
	return map[string]string{}
}

func (e KeyValueStore) GetStatus() string {
	return aws.ToString(e.Status)
}
