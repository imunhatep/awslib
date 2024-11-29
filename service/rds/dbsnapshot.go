package rds

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/imunhatep/awslib/service"
)

func init() {
	gob.Register(DbSnapshot{})
}

type DbSnapshot struct {
	service.AbstractResource
	types.DBSnapshot
}

func NewDbSnapshot(client AwsClient, snapshot types.DBSnapshot) DbSnapshot {
	snapshotArn, _ := arn.Parse(aws.ToString(snapshot.DBSnapshotArn))

	ebs := DbSnapshot{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(snapshot.DBSnapshotIdentifier),
			ARN:       &snapshotArn,
			CreatedAt: aws.ToTime(snapshot.SnapshotCreateTime),
			Type:      cfg.ResourceTypeDBSnapshot,
		},
		DBSnapshot: snapshot,
	}

	return ebs
}

func (e DbSnapshot) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e DbSnapshot) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.DBSnapshot.TagList {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e DbSnapshot) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
