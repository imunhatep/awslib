package rds

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockClient struct{}

func (mockClient) GetRegion() ptypes.AwsRegion       { return ptypes.AwsRegion("eu-west-1") }
func (mockClient) GetAccountID() ptypes.AwsAccountID { return ptypes.AwsAccountID("123456789012") }

func auroraServerlessCluster() types.DBCluster {
	return types.DBCluster{
		DBClusterIdentifier:   aws.String("orders"),
		DBClusterArn:          aws.String("arn:aws:rds:eu-west-1:123456789012:cluster:orders"),
		ClusterCreateTime:     aws.Time(time.Date(2025, 11, 2, 9, 30, 0, 0, time.UTC)),
		Engine:                aws.String("aurora-postgresql"),
		EngineVersion:         aws.String("16.4"),
		EngineMode:            aws.String("provisioned"),
		Status:                aws.String("available"),
		Endpoint:              aws.String("orders.cluster-abc.eu-west-1.rds.amazonaws.com"),
		ReaderEndpoint:        aws.String("orders.cluster-ro-abc.eu-west-1.rds.amazonaws.com"),
		Port:                  aws.Int32(5432),
		StorageEncrypted:      aws.Bool(true),
		MultiAZ:               aws.Bool(true),
		DeletionProtection:    aws.Bool(true),
		BackupRetentionPeriod: aws.Int32(7),
		DBClusterMembers: []types.DBClusterMember{
			{DBInstanceIdentifier: aws.String("orders-1"), IsClusterWriter: aws.Bool(false)},
			{DBInstanceIdentifier: aws.String("orders-0"), IsClusterWriter: aws.Bool(true)},
			{DBInstanceIdentifier: aws.String("orders-2"), IsClusterWriter: aws.Bool(false)},
		},
		ServerlessV2ScalingConfiguration: &types.ServerlessV2ScalingConfigurationInfo{
			MinCapacity: aws.Float64(0.5),
			MaxCapacity: aws.Float64(16),
		},
		TagList: []types.Tag{{Key: aws.String("Team"), Value: aws.String("payments")}},
	}
}

func TestDBClusterIdentity(t *testing.T) {
	cluster := NewDBCluster(mockClient{}, auroraServerlessCluster())

	var _ service.ResourceInterface = cluster

	assert.Equal(t, "orders", cluster.GetId())
	assert.Equal(t, "orders", cluster.GetName(), "an untagged cluster is named by its identifier")
	assert.Equal(t, "arn:aws:rds:eu-west-1:123456789012:cluster:orders", cluster.GetArn())
	assert.Equal(t, cfg.ResourceTypeDBCluster, cluster.GetType())
	assert.Equal(t, time.Date(2025, 11, 2, 9, 30, 0, 0, time.UTC), cluster.GetCreatedAt())
	assert.Equal(t, "payments", cluster.GetTagValue("Team"))

	named := auroraServerlessCluster()
	named.TagList = append(named.TagList, types.Tag{Key: aws.String("Name"), Value: aws.String("Orders DB")})
	assert.Equal(t, "Orders DB", NewDBCluster(mockClient{}, named).GetName())
}

func TestDBClusterHelpers(t *testing.T) {
	cluster := NewDBCluster(mockClient{}, auroraServerlessCluster())

	assert.Equal(t, "aurora-postgresql", cluster.GetEngine())
	assert.Equal(t, "16.4", cluster.GetEngineVersion())
	assert.Equal(t, "provisioned", cluster.GetEngineMode())
	assert.Equal(t, "available", cluster.GetStatus())
	assert.Equal(t, "orders-0", cluster.GetWriter())
	assert.Equal(t, []string{"orders-1", "orders-2"}, cluster.GetReaders())
	assert.Equal(t, 3, cluster.GetMemberCount())
	assert.Equal(t, "orders.cluster-abc.eu-west-1.rds.amazonaws.com", cluster.GetEndpoint())
	assert.Equal(t, "orders.cluster-ro-abc.eu-west-1.rds.amazonaws.com", cluster.GetReaderEndpoint())
	assert.Equal(t, int32(5432), cluster.GetPort())
	assert.True(t, cluster.IsStorageEncrypted())
	assert.True(t, cluster.IsMultiAZ())
	assert.True(t, cluster.IsDeletionProtected())
	assert.Equal(t, int32(7), cluster.GetBackupRetentionDays())
	assert.True(t, cluster.IsServerlessV2())
	assert.Equal(t, 0.5, cluster.GetServerlessV2MinCapacity())
	assert.Equal(t, 16.0, cluster.GetServerlessV2MaxCapacity())
}

// DescribeDBClusters returns DocumentDB and Neptune clusters too; they are kept, and a cluster
// without instances or a serverless configuration reads as zero values, not a panic.
func TestDBClusterSparseNonAurora(t *testing.T) {
	cluster := NewDBCluster(mockClient{}, types.DBCluster{
		DBClusterIdentifier: aws.String("graph"),
		Engine:              aws.String("neptune"),
	})

	assert.Equal(t, "neptune", cluster.GetEngine())
	assert.Empty(t, cluster.GetArn())
	assert.Empty(t, cluster.GetWriter())
	assert.Nil(t, cluster.GetReaders())
	assert.False(t, cluster.IsServerlessV2())
	assert.Zero(t, cluster.GetServerlessV2MinCapacity())
	assert.Zero(t, cluster.GetServerlessV2MaxCapacity())
	assert.False(t, cluster.IsStorageEncrypted())
	assert.Empty(t, cluster.GetTags())
}

func TestDBClusterSnapshot(t *testing.T) {
	snapshot := NewDBClusterSnapshot(mockClient{}, types.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("rds:orders-2026-11-22-03-10"),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:eu-west-1:123456789012:cluster-snapshot:rds:orders-2026-11-22-03-10"),
		DBClusterIdentifier:         aws.String("orders"),
		SnapshotCreateTime:          aws.Time(time.Date(2026, 11, 22, 3, 10, 0, 0, time.UTC)),
		SnapshotType:                aws.String("automated"),
		Engine:                      aws.String("aurora-postgresql"),
		EngineVersion:               aws.String("16.4"),
		Status:                      aws.String("available"),
		AllocatedStorage:            aws.Int32(120),
		StorageEncrypted:            aws.Bool(true),
		TagList:                     []types.Tag{{Key: aws.String("Team"), Value: aws.String("payments")}},
	})

	var _ service.ResourceInterface = snapshot

	assert.Equal(t, "rds:orders-2026-11-22-03-10", snapshot.GetId())
	assert.Equal(t, "rds:orders-2026-11-22-03-10", snapshot.GetName())
	assert.Equal(t, "arn:aws:rds:eu-west-1:123456789012:cluster-snapshot:rds:orders-2026-11-22-03-10", snapshot.GetArn())
	assert.Equal(t, cfg.ResourceTypeDBClusterSnapshot, snapshot.GetType())
	assert.Equal(t, time.Date(2026, 11, 22, 3, 10, 0, 0, time.UTC), snapshot.GetCreatedAt())
	assert.Equal(t, "orders", snapshot.GetClusterIdentifier())
	assert.Equal(t, "automated", snapshot.GetSnapshotType())
	assert.Equal(t, "aurora-postgresql", snapshot.GetEngine())
	assert.Equal(t, "16.4", snapshot.GetEngineVersion())
	assert.Equal(t, "available", snapshot.GetStatus())
	assert.Equal(t, int32(120), snapshot.GetAllocatedStorage())
	assert.True(t, snapshot.IsStorageEncrypted())
	assert.Equal(t, "payments", snapshot.GetTagValue("Team"))
}

func TestDBClusterGetTagsReturnsCopy(t *testing.T) {
	cluster := NewDBCluster(mockClient{}, auroraServerlessCluster())

	cluster.GetTags()["Team"] = "changed"
	assert.Equal(t, "payments", cluster.GetTagValue("Team"))
}

func TestDBClusterGobRoundTrip(t *testing.T) {
	in := []DBCluster{NewDBCluster(mockClient{}, auroraServerlessCluster())}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))

	var out []DBCluster
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))
	require.Len(t, out, 1)
	assert.Equal(t, in[0].GetArn(), out[0].GetArn())
	assert.Equal(t, "orders-0", out[0].GetWriter())
	assert.Equal(t, 16.0, out[0].GetServerlessV2MaxCapacity())
}
