package rds

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/imunhatep/awslib/service"
)

// DBCluster is an RDS DB cluster as DescribeDBClusters returns it. That call also returns
// Neptune and DocumentDB clusters, which share the RDS control plane; GetEngine tells them
// apart ("aurora-postgresql", "neptune", "docdb", ...).
type DBCluster struct {
	service.AbstractResource
	types.DBCluster
}

func NewDBCluster(client AwsClient, cluster types.DBCluster) DBCluster {
	var clusterArn *arn.ARN
	if parsed, err := arn.Parse(aws.ToString(cluster.DBClusterArn)); err == nil {
		clusterArn = &parsed
	}

	return DBCluster{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(cluster.DBClusterIdentifier),
			ARN:       clusterArn,
			CreatedAt: aws.ToTime(cluster.ClusterCreateTime),
			Type:      cfg.ResourceTypeDBCluster,
		},
		DBCluster: cluster,
	}
}

func (e DBCluster) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return aws.ToString(e.DBClusterIdentifier)
}

func (e DBCluster) GetTags() map[string]string {
	tags := make(map[string]string, len(e.TagList))
	for _, tag := range e.TagList {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e DBCluster) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

func (e DBCluster) GetEngine() string {
	return aws.ToString(e.DBCluster.Engine)
}

func (e DBCluster) GetEngineVersion() string {
	return aws.ToString(e.DBCluster.EngineVersion)
}

// GetEngineMode is "provisioned", "serverless" (Aurora Serverless v1), "parallelquery",
// "global" or "multimaster". Aurora Serverless v2 clusters are "provisioned"; see
// IsServerlessV2.
func (e DBCluster) GetEngineMode() string {
	return aws.ToString(e.DBCluster.EngineMode)
}

func (e DBCluster) GetStatus() string {
	return aws.ToString(e.DBCluster.Status)
}

// GetWriter is the DB instance identifier of the cluster writer, empty when there is none
// (a cluster with no instances, or a secondary cluster of a global database).
func (e DBCluster) GetWriter() string {
	for _, member := range e.DBClusterMembers {
		if aws.ToBool(member.IsClusterWriter) {
			return aws.ToString(member.DBInstanceIdentifier)
		}
	}

	return ""
}

// GetReaders are the DB instance identifiers of the cluster's reader instances.
func (e DBCluster) GetReaders() []string {
	var readers []string
	for _, member := range e.DBClusterMembers {
		if !aws.ToBool(member.IsClusterWriter) {
			readers = append(readers, aws.ToString(member.DBInstanceIdentifier))
		}
	}

	return readers
}

func (e DBCluster) GetMemberCount() int {
	return len(e.DBClusterMembers)
}

func (e DBCluster) GetEndpoint() string {
	return aws.ToString(e.DBCluster.Endpoint)
}

func (e DBCluster) GetReaderEndpoint() string {
	return aws.ToString(e.DBCluster.ReaderEndpoint)
}

func (e DBCluster) GetPort() int32 {
	return aws.ToInt32(e.DBCluster.Port)
}

func (e DBCluster) IsStorageEncrypted() bool {
	return aws.ToBool(e.DBCluster.StorageEncrypted)
}

func (e DBCluster) IsMultiAZ() bool {
	return aws.ToBool(e.DBCluster.MultiAZ)
}

func (e DBCluster) IsDeletionProtected() bool {
	return aws.ToBool(e.DBCluster.DeletionProtection)
}

func (e DBCluster) GetBackupRetentionDays() int32 {
	return aws.ToInt32(e.DBCluster.BackupRetentionPeriod)
}

// IsServerlessV2 reports whether the cluster carries an Aurora Serverless v2 scaling
// configuration; its instances use the db.serverless class.
func (e DBCluster) IsServerlessV2() bool {
	return e.ServerlessV2ScalingConfiguration != nil
}

// GetServerlessV2MinCapacity is the minimum capacity in ACUs, zero when not serverless v2.
func (e DBCluster) GetServerlessV2MinCapacity() float64 {
	if e.ServerlessV2ScalingConfiguration == nil {
		return 0
	}

	return aws.ToFloat64(e.ServerlessV2ScalingConfiguration.MinCapacity)
}

// GetServerlessV2MaxCapacity is the maximum capacity in ACUs, zero when not serverless v2.
func (e DBCluster) GetServerlessV2MaxCapacity() float64 {
	if e.ServerlessV2ScalingConfiguration == nil {
		return 0
	}

	return aws.ToFloat64(e.ServerlessV2ScalingConfiguration.MaxCapacity)
}

// GetGlobalClusterIdentifier names the global database the cluster belongs to, if any.
func (e DBCluster) GetGlobalClusterIdentifier() string {
	return aws.ToString(e.DBCluster.GlobalClusterIdentifier)
}
