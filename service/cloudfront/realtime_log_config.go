package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// RealtimeLogConfigList holds a list of RealtimeLogConfig items.
type RealtimeLogConfigList struct {
	Items []RealtimeLogConfig
}

// RealtimeLogConfig sends a sample of a cache behaviour's requests to Kinesis
// Data Streams. It has a name and an ARN but no separate ID, so the name serves
// as the ID; cache behaviours reference it by ARN (RealtimeLogConfigArn). Not
// taggable.
type RealtimeLogConfig struct {
	service.AbstractResource
	cftypes.RealtimeLogConfig
}

func NewRealtimeLogConfig(client AwsClient, config cftypes.RealtimeLogConfig) RealtimeLogConfig {
	return RealtimeLogConfig{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(config.Name),
			ARN:       parseArn(config.ARN),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontRealtimeLogConfig,
		},
		RealtimeLogConfig: config,
	}
}

func (e RealtimeLogConfig) GetName() string {
	return aws.ToString(e.Name)
}

// GetTags returns an empty map: real-time log configurations are not taggable.
func (e RealtimeLogConfig) GetTags() map[string]string {
	return map[string]string{}
}

// GetStreamArns returns the Kinesis stream every endpoint writes to.
func (e RealtimeLogConfig) GetStreamArns() []string {
	var out []string

	for _, endpoint := range e.EndPoints {
		if endpoint.KinesisStreamConfig != nil {
			out = append(out, aws.ToString(endpoint.KinesisStreamConfig.StreamARN))
		}
	}

	return out
}
