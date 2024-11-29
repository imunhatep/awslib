package gateway

import (
	"context"
	"github.com/imunhatep/awslib/provider/v2"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/autoscaling"
	"github.com/imunhatep/awslib/service/batch"
	"github.com/imunhatep/awslib/service/cloudcontrol"
	"github.com/imunhatep/awslib/service/cloudwatchlogs"
	"github.com/imunhatep/awslib/service/dynamodb"
	"github.com/imunhatep/awslib/service/ec2"
	"github.com/imunhatep/awslib/service/ecs"
	"github.com/imunhatep/awslib/service/efs"
	"github.com/imunhatep/awslib/service/eks"
	"github.com/imunhatep/awslib/service/elb"
	"github.com/imunhatep/awslib/service/emr"
	"github.com/imunhatep/awslib/service/emrserverless"
	"github.com/imunhatep/awslib/service/glue"
	"github.com/imunhatep/awslib/service/iam"
	"github.com/imunhatep/awslib/service/lambda"
	"github.com/imunhatep/awslib/service/rds"
	"github.com/imunhatep/awslib/service/route53"
	"github.com/imunhatep/awslib/service/s3"
	"github.com/imunhatep/awslib/service/secretmanager"
	"github.com/imunhatep/awslib/service/sns"
	"github.com/imunhatep/awslib/service/sqs"
	"github.com/imunhatep/gocollection/slice"
)

// FindAutoScaleGroups returns a list of Auto Scaling groups
func FindAutoScaleGroups(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := autoscaling.NewAsgRepository(ctx, client).ListAutoScalingGroupsAll()

	return slice.Map(items, cast[autoscaling.AutoScalingGroup]), err
}

// FindBatchComputeEnvironments returns a list of Batch compute environments
func FindBatchComputeEnvironments(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := batch.NewBatchRepository(ctx, client).ListComputeEnvironmentAll()

	return slice.Map(items, cast[batch.ComputeEnvironment]), err
}

// FindBatchJobQueues returns a list of Batch job queues
func FindBatchJobQueues(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := batch.NewBatchRepository(ctx, client).ListJobQueueAll()

	return slice.Map(items, cast[batch.JobQueue]), err
}

// FindCloudWatchLogGroups returns a list of CloudWatch log groups
func FindCloudWatchLogGroups(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := cloudwatchlogs.NewCloudWatchLogsRepository(ctx, client).ListLogGroupsAll()

	return slice.Map(items, cast[cloudwatchlogs.LogGroup]), err
}

// FindDbInstances returns a list of RDS instances
func FindDbInstances(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := rds.NewRdsRepository(ctx, client).ListDbInstancesAll()

	return slice.Map(items, cast[rds.DbInstance]), err
}

// FindDynamodbTables returns a list of DynamoDB tables
func FindDynamodbTables(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := dynamodb.NewDynamoDBRepository(ctx, client).ListTablesAll()

	return slice.Map(items, cast[dynamodb.Table]), err
}

// FindDbSnapshots returns a list of RDS snapshots
func FindDbSnapshots(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := rds.NewRdsRepository(ctx, client).ListDbSnapshotsAll()

	return slice.Map(items, cast[rds.DbSnapshot]), err
}

// FindEc2Snapshots returns a list of EBS snapshots
func FindEc2Snapshots(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := ec2.NewEc2Repository(ctx, client).ListSnapshotsAll()

	return slice.Map(items, cast[ec2.Snapshot]), err
}

// FindEc2Volumes returns a list of EBS volumes
func FindEc2Volumes(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := ec2.NewEc2Repository(ctx, client).ListVolumesAll()

	return slice.Map(items, cast[ec2.Volume]), err
}

// FindEc2Vpcs returns a list of VPC
func FindEc2Vpcs(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := ec2.NewEc2Repository(ctx, client).ListVpcsAll()

	return slice.Map(items, cast[ec2.Vpc]), err
}

// FindEbsCCVolumes returns a list of EBS volumes
func FindEbsCCVolumes(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := cloudcontrol.NewCloudControlRepository(ctx, client).ListVolumesAll()

	return slice.Map(items, cast[cloudcontrol.Volume]), err
}

// FindEc2Instances returns a list of EC2 instances
func FindEc2Instances(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := ec2.NewEc2Repository(ctx, client).ListInstancesAll()

	return slice.Map(items, cast[ec2.Instance]), err
}

// FindEc2CCInstances returns a list of EC2 instances
func FindEc2CCInstances(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := cloudcontrol.NewCloudControlRepository(ctx, client).ListInstancesAll()

	return slice.Map(items, cast[cloudcontrol.Instance]), err
}

// FindEcsClusters returns a list of ECS clusters
func FindEcsClusters(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := ecs.NewEcsRepository(ctx, client).ListClustersAll()

	return slice.Map(items, cast[ecs.Cluster]), err
}

// FindEksClusters returns a list of EKS clusters
func FindEksClusters(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := eks.NewEksRepository(ctx, client).ListClustersAll()

	return slice.Map(items, cast[eks.Cluster]), err
}

// FindEcsServices returns a list of ECS services
func FindEcsServices(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := ecs.NewEcsRepository(ctx, client).ListServicesAll()

	return slice.Map(items, cast[ecs.Service]), err
}

// FindEfsFileSystems returns a list of EFS file systems
func FindEfsFileSystems(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := efs.NewEfsRepository(ctx, client).ListFileSystemsAll()

	return slice.Map(items, cast[efs.FileSystem]), err
}

// FindEmrClusters returns a list of EMR clusters
func FindEmrClusters(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	// FIndAll() is overkill...
	items, err := emr.NewEmrRepository(ctx, client).ListClustersLatest(nil)

	return slice.Map(items, cast[emr.Cluster]), err
}

// FindEmrServerlessApplications returns a list of EMR serverless applications
func FindEmrServerlessApplications(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := emrserverless.NewEMRServerlessRepository(ctx, client).ListApplicationsActive()

	return slice.Map(items, cast[emrserverless.Application]), err
}

// FindEmrServerlessJobRuns returns a list of EMR serverless job runs
func FindEmrServerlessJobRuns(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := emrserverless.NewEMRServerlessRepository(ctx, client).ListJobRunsAll()

	return slice.Map(items, cast[emrserverless.JobRun]), err
}

// FindGlueDatabases returns a list of Glue databases
func FindGlueDatabases(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := glue.NewGlueRepository(ctx, client).ListDatabaseAll()

	return slice.Map(items, cast[glue.Database]), err
}

// FindGlueJobs returns a list of Glue jobs
func FindGlueJobs(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := glue.NewGlueRepository(ctx, client).ListJobsAll()

	return slice.Map(items, cast[glue.Job]), err
}

// FindGlueTables returns a list of Glue tables
func FindGlueTables(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := glue.NewGlueRepository(ctx, client).ListTablesAll()

	return slice.Map(items, cast[glue.Table]), err
}

// FindLambdaFunctions returns a list of Lambda functions
func FindLambdaFunctions(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := lambda.NewLambdaRepository(ctx, client).ListFunctionsAll()

	return slice.Map(items, cast[lambda.Function]), err
}

// FindIamUsers returns a list of IAM users
func FindIamUsers(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := iam.NewIamRepository(ctx, client).ListUsersAll()

	return slice.Map(items, cast[iam.User]), err
}

// FindLoadBalancers returns a list of Load Balancers
func FindLoadBalancers(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := elb.NewLoadBalancerRepository(ctx, client).ListLoadBalancersAll()

	return slice.Map(items, cast[elb.LoadBalancer]), err
}

// FindRoute53HostedZones returns a list of Route 53 hosted zones
func FindRoute53HostedZones(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := route53.NewRoute53Repository(ctx, client).ListHostedZonesAll()

	return slice.Map(items, cast[route53.HostedZone]), err
}

// FindSecretManagerSecrets returns a list of Secrets Manager secrets
func FindSecretManagerSecrets(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := secretmanager.NewSecretManagerRepository(ctx, client).ListSecretsAll()

	return slice.Map(items, cast[secretmanager.SecretEntry]), err
}

// FindSqsQueues returns a list of SQS queues
func FindSqsQueues(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := sqs.NewSqsRepository(ctx, client).ListQueuesAll()

	return slice.Map(items, cast[sqs.Queue]), err
}

// FindSnsTopics returns a list of SNS topics
func FindSnsTopics(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := sns.NewSnsRepository(ctx, client).ListTopicsAll()

	return slice.Map(items, cast[sns.Topic]), err
}

// FindS3CCBuckets returns a list of S3 Buckets
func FindS3CCBuckets(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := cloudcontrol.NewCloudControlRepository(ctx, client).ListBucketsAll()

	return slice.Map(items, cast[cloudcontrol.Bucket]), err
}

// FindS3Buckets returns a list of S3 buckets
func FindS3Buckets(ctx context.Context, client *v2.Client) ([]service.EntityInterface, error) {
	items, err := s3.NewS3Repository(ctx, client).ListBucketsAll()

	return slice.Map(items, cast[s3.Bucket]), err
}

// cast casts exact type of entities to EntityInterface
func cast[T service.EntityInterface](e T) service.EntityInterface {
	return e
}

// cast casts exact type of entities to EntityInterface
func castV2[T service.EntityInterface](e service.EntityInterface) T {
	return e.(T)
}
