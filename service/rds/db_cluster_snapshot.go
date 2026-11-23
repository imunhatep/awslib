package rds

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/imunhatep/awslib/service"
)

// DBClusterSnapshot is a DB cluster snapshot as DescribeDBClusterSnapshots returns it.
type DBClusterSnapshot struct {
	service.AbstractResource
	types.DBClusterSnapshot
}

func NewDBClusterSnapshot(client AwsClient, snapshot types.DBClusterSnapshot) DBClusterSnapshot {
	var snapshotArn *arn.ARN
	if parsed, err := arn.Parse(aws.ToString(snapshot.DBClusterSnapshotArn)); err == nil {
		snapshotArn = &parsed
	}

	return DBClusterSnapshot{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(snapshot.DBClusterSnapshotIdentifier),
			ARN:       snapshotArn,
			CreatedAt: aws.ToTime(snapshot.SnapshotCreateTime),
			Type:      cfg.ResourceTypeDBClusterSnapshot,
		},
		DBClusterSnapshot: snapshot,
	}
}

func (e DBClusterSnapshot) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return aws.ToString(e.DBClusterSnapshotIdentifier)
}

func (e DBClusterSnapshot) GetTags() map[string]string {
	tags := make(map[string]string, len(e.TagList))
	for _, tag := range e.TagList {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e DBClusterSnapshot) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

func (e DBClusterSnapshot) GetClusterIdentifier() string {
	return aws.ToString(e.DBClusterIdentifier)
}

// GetSnapshotType is "manual", "automated", "shared", "public" or "awsbackup".
func (e DBClusterSnapshot) GetSnapshotType() string {
	return aws.ToString(e.SnapshotType)
}

func (e DBClusterSnapshot) GetEngine() string {
	return aws.ToString(e.DBClusterSnapshot.Engine)
}

func (e DBClusterSnapshot) GetEngineVersion() string {
	return aws.ToString(e.DBClusterSnapshot.EngineVersion)
}

func (e DBClusterSnapshot) GetStatus() string {
	return aws.ToString(e.DBClusterSnapshot.Status)
}

// GetAllocatedStorage is the allocated storage in GiB.
func (e DBClusterSnapshot) GetAllocatedStorage() int32 {
	return aws.ToInt32(e.DBClusterSnapshot.AllocatedStorage)
}

func (e DBClusterSnapshot) IsStorageEncrypted() bool {
	return aws.ToBool(e.DBClusterSnapshot.StorageEncrypted)
}
