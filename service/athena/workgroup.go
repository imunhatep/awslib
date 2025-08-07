package athena

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type WorkGroupList struct {
	Items []WorkGroup
}

type WorkGroup struct {
	service.AbstractResource
	types.WorkGroup
	Tags []types.Tag
}

func NewWorkGroup(client AwsClient, workGroup types.WorkGroup, tags []types.Tag) WorkGroup {
	return WorkGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(workGroup.Name),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "athena", "workgroup/", workGroup.Name),
			CreatedAt: aws.ToTime(workGroup.CreationTime),
			Type:      cfg.ResourceTypeAthenaWorkGroup,
		},
		WorkGroup: workGroup,
		Tags:      tags,
	}
}

func (e WorkGroup) GetName() string {
	return aws.ToString(e.Name)
}

func (e WorkGroup) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e WorkGroup) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
