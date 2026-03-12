package proxy

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	"github.com/imunhatep/awslib/cache"
	v3 "github.com/imunhatep/awslib/provider/v3"
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
func FindAutoScaleGroups(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := autoscaling.NewAsgRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListAutoScalingGroupsAll()
		return slice.Map(items, cast[autoscaling.AutoScalingGroup]), err
	}
	items, err := repo.ListAutoScalingGroupsAll()
	return slice.Map(items, cast[autoscaling.AutoScalingGroup]), err
}

// FindBatchComputeEnvironments returns a list of Batch compute environments
func FindBatchComputeEnvironments(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := batch.NewBatchRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListComputeEnvironmentAll()
		return slice.Map(items, cast[batch.ComputeEnvironment]), err
	}
	items, err := repo.ListComputeEnvironmentAll()
	return slice.Map(items, cast[batch.ComputeEnvironment]), err
}

// FindBatchJobQueues returns a list of Batch job queues
func FindBatchJobQueues(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := batch.NewBatchRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListJobQueueAll()
		return slice.Map(items, cast[batch.JobQueue]), err
	}
	items, err := repo.ListJobQueueAll()
	return slice.Map(items, cast[batch.JobQueue]), err
}

// FindCloudWatchLogGroups returns a list of CloudWatch log groups
func FindCloudWatchLogGroups(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudwatchlogs.NewCloudWatchLogsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListLogGroupsAll()
		return slice.Map(items, cast[cloudwatchlogs.LogGroup]), err
	}
	items, err := repo.ListLogGroupsAll()
	return slice.Map(items, cast[cloudwatchlogs.LogGroup]), err
}

// FindDbInstances returns a list of RDS instances
func FindDbInstances(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := rds.NewRdsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDbInstancesAll()
		return slice.Map(items, cast[rds.DbInstance]), err
	}
	items, err := repo.ListDbInstancesAll()
	return slice.Map(items, cast[rds.DbInstance]), err
}

// FindDynamodbTables returns a list of DynamoDB tables
func FindDynamodbTables(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := dynamodb.NewDynamoDBRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListTablesAll()
		return slice.Map(items, cast[dynamodb.Table]), err
	}
	items, err := repo.ListTablesAll()
	return slice.Map(items, cast[dynamodb.Table]), err
}

// FindDbSnapshots returns a list of RDS snapshots
func FindDbSnapshots(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := rds.NewRdsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDbSnapshotsAll()
		return slice.Map(items, cast[rds.DbSnapshot]), err
	}
	items, err := repo.ListDbSnapshotsAll()
	return slice.Map(items, cast[rds.DbSnapshot]), err
}

// FindEc2Snapshots returns a list of EBS snapshots
func FindEc2Snapshots(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListSnapshotsAll()
		return slice.Map(items, cast[ec2.Snapshot]), err
	}
	items, err := repo.ListSnapshotsAll()
	return slice.Map(items, cast[ec2.Snapshot]), err
}

// FindEc2Volumes returns a list of EBS volumes
func FindEc2Volumes(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListVolumesAll()
		return slice.Map(items, cast[ec2.Volume]), err
	}
	items, err := repo.ListVolumesAll()
	return slice.Map(items, cast[ec2.Volume]), err
}

// FindEc2Vpcs returns a list of VPC
func FindEc2Vpcs(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListVpcsAll()
		return slice.Map(items, cast[ec2.Vpc]), err
	}
	items, err := repo.ListVpcsAll()
	return slice.Map(items, cast[ec2.Vpc]), err
}

// FindEbsCCVolumes returns a list of EBS volumes via CloudControl
func FindEbsCCVolumes(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudcontrol.NewCloudControlRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListVolumesAll()
		return slice.Map(items, cast[cloudcontrol.Volume]), err
	}
	items, err := repo.ListVolumesAll()
	return slice.Map(items, cast[cloudcontrol.Volume]), err
}

// FindEc2Instances returns a list of EC2 instances
func FindEc2Instances(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListInstancesAll()
		return slice.Map(items, cast[ec2.Instance]), err
	}
	items, err := repo.ListInstancesAll()
	return slice.Map(items, cast[ec2.Instance]), err
}

// FindEc2CCInstances returns a list of EC2 instances via CloudControl
func FindEc2CCInstances(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudcontrol.NewCloudControlRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListInstancesAll()
		return slice.Map(items, cast[cloudcontrol.Instance]), err
	}
	items, err := repo.ListInstancesAll()
	return slice.Map(items, cast[cloudcontrol.Instance]), err
}

// FindEcsClusters returns a list of ECS clusters
func FindEcsClusters(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ecs.NewEcsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListClustersAll()
		return slice.Map(items, cast[ecs.Cluster]), err
	}
	items, err := repo.ListClustersAll()
	return slice.Map(items, cast[ecs.Cluster]), err
}

// FindEksClusters returns a list of EKS clusters
func FindEksClusters(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := eks.NewEksRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListClustersAll()
		return slice.Map(items, cast[eks.Cluster]), err
	}
	items, err := repo.ListClustersAll()
	return slice.Map(items, cast[eks.Cluster]), err
}

// FindEcsServices returns a list of ECS services
func FindEcsServices(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ecs.NewEcsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListServicesAll()
		return slice.Map(items, cast[ecs.Service]), err
	}
	items, err := repo.ListServicesAll()
	return slice.Map(items, cast[ecs.Service]), err
}

// FindEfsFileSystems returns a list of EFS file systems
func FindEfsFileSystems(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := efs.NewEfsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListFileSystemsAll()
		return slice.Map(items, cast[efs.FileSystem]), err
	}
	items, err := repo.ListFileSystemsAll()
	return slice.Map(items, cast[efs.FileSystem]), err
}

// FindEmrClusters returns a list of EMR clusters
func FindEmrClusters(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := emr.NewEmrRepository(ctx, client)
	if dc != nil {
		// ListClustersLatest is overkill with FindAll; use the cached variant
		items, err := repo.WithCache(dc).ListClustersLatest(nil)
		return slice.Map(items, cast[emr.Cluster]), err
	}
	items, err := repo.ListClustersLatest(nil)
	return slice.Map(items, cast[emr.Cluster]), err
}

// FindEmrServerlessApplications returns a list of EMR serverless applications
func FindEmrServerlessApplications(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := emrserverless.NewEMRServerlessRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListApplicationsActive()
		return slice.Map(items, cast[emrserverless.Application]), err
	}
	items, err := repo.ListApplicationsActive()
	return slice.Map(items, cast[emrserverless.Application]), err
}

// FindEmrServerlessJobRuns returns a list of EMR serverless job runs
func FindEmrServerlessJobRuns(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := emrserverless.NewEMRServerlessRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListJobRunsAll()
		return slice.Map(items, cast[emrserverless.JobRun]), err
	}
	items, err := repo.ListJobRunsAll()
	return slice.Map(items, cast[emrserverless.JobRun]), err
}

// FindGlueDatabases returns a list of Glue databases
func FindGlueDatabases(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := glue.NewGlueRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDatabaseAll()
		return slice.Map(items, cast[glue.Database]), err
	}
	items, err := repo.ListDatabaseAll()
	return slice.Map(items, cast[glue.Database]), err
}

// FindGlueJobs returns a list of Glue jobs
func FindGlueJobs(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := glue.NewGlueRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListJobsAll()
		return slice.Map(items, cast[glue.Job]), err
	}
	items, err := repo.ListJobsAll()
	return slice.Map(items, cast[glue.Job]), err
}

// FindGlueTables returns a list of Glue tables
func FindGlueTables(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := glue.NewGlueRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListTablesAll()
		return slice.Map(items, cast[glue.Table]), err
	}
	items, err := repo.ListTablesAll()
	return slice.Map(items, cast[glue.Table]), err
}

// FindLambdaFunctions returns a list of Lambda functions
func FindLambdaFunctions(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := lambda.NewLambdaRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListFunctionsAll()
		return slice.Map(items, cast[lambda.Function]), err
	}
	items, err := repo.ListFunctionsAll()
	return slice.Map(items, cast[lambda.Function]), err
}

// FindIamUsers returns a list of IAM users
func FindIamUsers(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := iam.NewIamRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListUsersAll()
		return slice.Map(items, cast[iam.User]), err
	}
	items, err := repo.ListUsersAll()
	return slice.Map(items, cast[iam.User]), err
}

// FindLoadBalancers returns a list of Load Balancers
func FindLoadBalancers(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := elb.NewLoadBalancerRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListLoadBalancersAll()
		return slice.Map(items, cast[elb.LoadBalancer]), err
	}
	items, err := repo.ListLoadBalancersAll()
	return slice.Map(items, cast[elb.LoadBalancer]), err
}

// FindRoute53HostedZones returns a list of Route 53 hosted zones
func FindRoute53HostedZones(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := route53.NewRoute53Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListHostedZonesAll()
		return slice.Map(items, cast[route53.HostedZone]), err
	}
	items, err := repo.ListHostedZonesAll()
	return slice.Map(items, cast[route53.HostedZone]), err
}

// FindRoute53DomainSummaries returns a list of Route 53 registered domain summaries
func FindRoute53DomainSummaries(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := route53.NewRoute53Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDomainsAll()
		return slice.Map(items, cast[route53.DomainSummary]), err
	}
	items, err := repo.ListDomainsAll()
	return slice.Map(items, cast[route53.DomainSummary]), err
}

// FindRoute53Domains returns a list of Route 53 registered domains with full details
func FindRoute53Domains(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := route53.NewRoute53Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDomainsDetailsByInput(&route53domains.ListDomainsInput{})
		return slice.Map(items, cast[route53.Domain]), err
	}
	items, err := repo.ListDomainsDetailsByInput(&route53domains.ListDomainsInput{})
	return slice.Map(items, cast[route53.Domain]), err
}

// FindRoute53ResourceRecords returns a list of Route 53 resource records across all hosted zones
func FindRoute53ResourceRecords(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := route53.NewRoute53Repository(ctx, client)

	var hostedZones []route53.HostedZone
	var err error

	if dc != nil {
		cachedRepo := repo.WithCache(dc)
		hostedZones, err = cachedRepo.ListHostedZonesAll()
		if err != nil {
			return nil, err
		}

		var all []service.ResourceInterface
		for _, hz := range hostedZones {
			records, err := cachedRepo.ListResourceRecords(hz)
			if err != nil {
				return nil, err
			}
			all = append(all, slice.Map(records, cast[route53.ResourceRecord])...)
		}
		return all, nil
	}

	hostedZones, err = repo.ListHostedZonesAll()
	if err != nil {
		return nil, err
	}

	var all []service.ResourceInterface
	for _, hz := range hostedZones {
		records, err := repo.ListResourceRecords(hz)
		if err != nil {
			return nil, err
		}
		all = append(all, slice.Map(records, cast[route53.ResourceRecord])...)
	}

	return all, nil
}

// FindSecretManagerSecrets returns a list of Secrets Manager secrets
func FindSecretManagerSecrets(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := secretmanager.NewSecretManagerRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListSecretsAll()
		return slice.Map(items, cast[secretmanager.SecretEntry]), err
	}
	items, err := repo.ListSecretsAll()
	return slice.Map(items, cast[secretmanager.SecretEntry]), err
}

// FindSqsQueues returns a list of SQS queues
func FindSqsQueues(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := sqs.NewSqsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListQueuesAll()
		return slice.Map(items, cast[sqs.Queue]), err
	}
	items, err := repo.ListQueuesAll()
	return slice.Map(items, cast[sqs.Queue]), err
}

// FindSnsTopics returns a list of SNS topics
func FindSnsTopics(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := sns.NewSnsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListTopicsAll()
		return slice.Map(items, cast[sns.Topic]), err
	}
	items, err := repo.ListTopicsAll()
	return slice.Map(items, cast[sns.Topic]), err
}

// FindS3CCBuckets returns a list of S3 Buckets via CloudControl
func FindS3CCBuckets(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudcontrol.NewCloudControlRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListBucketsAll()
		return slice.Map(items, cast[cloudcontrol.Bucket]), err
	}
	items, err := repo.ListBucketsAll()
	return slice.Map(items, cast[cloudcontrol.Bucket]), err
}

// FindS3Buckets returns a list of S3 buckets
func FindS3Buckets(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := s3.NewS3Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListBucketsAll()
		return slice.Map(items, cast[s3.Bucket]), err
	}
	items, err := repo.ListBucketsAll()
	return slice.Map(items, cast[s3.Bucket]), err
}

// cast casts exact type of entities to ResourceInterface
func cast[T service.ResourceInterface](e T) service.ResourceInterface {
	return e
}
