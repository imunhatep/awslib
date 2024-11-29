package cloudwatchlogs

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"time"
)

type LogGroupList struct {
	Items []LogGroup
}

type LogGroup struct {
	service.AbstractResource
	types.LogGroup
	Tags map[string]string
}

func init() {
	gob.Register(LogGroup{})
}

func NewLogGroup(client AwsClient, logGroup types.LogGroup, tags map[string]string) LogGroup {
	lArn, _ := arn.Parse(aws.ToString(logGroup.Arn))

	return LogGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(logGroup.LogGroupName),
			ARN:       &lArn,
			CreatedAt: time.Unix(aws.ToInt64(logGroup.CreationTime), 0),
			Type:      cfg.ResourceTypeCloudWatchLogGroup,
		},
		LogGroup: logGroup,
		Tags:     tags,
	}
}

func (e LogGroup) GetName() string {
	return aws.ToString(e.LogGroupName)
}

func (e LogGroup) GetTags() map[string]string {
	return e.Tags
}

func (e LogGroup) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
