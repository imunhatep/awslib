package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type Snapshot struct {
	service.AbstractResource
	types.Snapshot
}

func NewSnapshot(client AwsClient, snapshot types.Snapshot) Snapshot {
	ebs := Snapshot{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(snapshot.SnapshotId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "snapshot/", snapshot.SnapshotId),
			CreatedAt: aws.ToTime(snapshot.StartTime),
			Type:      cfg.ResourceTypeSnapshot,
		},
		Snapshot: snapshot,
	}

	return ebs
}

func (e Snapshot) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Snapshot) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Snapshot) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
