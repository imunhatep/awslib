package proxy

import (
	"context"
	"fmt"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/cache"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	cfgEntity "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

type RepoProxyInterface interface {
	GetAccountID() ptypes.AwsAccountID
	GetRegion() ptypes.AwsRegion
	GetClient() *v3.Client
	GetContext() context.Context
	FindAll(resourceType cfg.ResourceType) ([]service.ResourceInterface, error)
}

// RepoProxy is proxy to aws repositories to get all aws resources
type RepoProxy struct {
	ctx    context.Context
	client *v3.Client
	cache  *cache.DataCache
}

func NewRepoProxy(ctx context.Context, client *v3.Client) *RepoProxy {
	return &RepoProxy{
		ctx:    ctx,
		client: client,
	}
}

// WithCache returns a new RepoProxy that passes the given DataCache to each repository.
func (e *RepoProxy) WithCache(dc *cache.DataCache) *RepoProxy {
	return &RepoProxy{
		ctx:    e.ctx,
		client: e.client,
		cache:  dc,
	}
}

func (e RepoProxy) GetAccountID() ptypes.AwsAccountID {
	return e.client.GetAccountID()
}

func (e RepoProxy) GetRegion() ptypes.AwsRegion {
	return e.client.GetRegion()
}

func (e RepoProxy) GetClient() *v3.Client {
	return e.client
}

func (e *RepoProxy) GetContext() context.Context {
	return e.ctx
}

func (e *RepoProxy) FindAll(resourceType cfg.ResourceType) (items []service.ResourceInterface, err error) {
	switch resourceType {
	case cfg.ResourceTypeAutoScalingGroup:
		items, err = FindAutoScaleGroups(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeBatchComputeEnvironment:
		items, err = FindBatchComputeEnvironments(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeBatchJobQueue:
		items, err = FindBatchJobQueues(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeGlueDatabase:
		items, err = FindGlueDatabases(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeGlueJob:
		items, err = FindGlueJobs(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeGlueTable:
		items, err = FindGlueTables(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeBucket:
		items, err = FindS3Buckets(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeDBInstance:
		items, err = FindDbInstances(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeDBSnapshot:
		items, err = FindDbSnapshots(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeTable:
		items, err = FindDynamodbTables(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeECSCluster:
		items, err = FindEcsClusters(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeECSService:
		items, err = FindEcsServices(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeEKSCluster:
		items, err = FindEksClusters(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeEmrCluster:
		items, err = FindEmrClusters(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeEmrServerlessApplication:
		items, err = FindEmrServerlessApplications(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeEmrServerlessJobRun:
		items, err = FindEmrServerlessJobRuns(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeFunction:
		items, err = FindLambdaFunctions(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeInstance:
		items, err = FindEc2Instances(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeCloudWatchLogGroup:
		items, err = FindCloudWatchLogGroups(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeRoute53HostedZone:
		items, err = FindRoute53HostedZones(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeRoute53DomainSummary:
		items, err = FindRoute53DomainSummaries(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeRoute53Domain:
		items, err = FindRoute53Domains(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeRoute53ResourceRecord:
		items, err = FindRoute53ResourceRecords(e.ctx, e.client, e.cache)
	case cfgEntity.ResourceTypeSnapshot:
		items, err = FindEc2Snapshots(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeVolume:
		items, err = FindEc2Volumes(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeEFSFileSystem:
		items, err = FindEfsFileSystems(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeLoadBalancerV2:
		items, err = FindLoadBalancers(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeSecret:
		items, err = FindSecretManagerSecrets(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeQueue:
		items, err = FindSqsQueues(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeTopic:
		items, err = FindSnsTopics(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeUser:
		items, err = FindIamUsers(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeVpc:
		items, err = FindEc2Vpcs(e.ctx, e.client, e.cache)
	default:
		err = fmt.Errorf("resource type %s not supported", cfgEntity.ResourceTypeToString(resourceType))
	}

	log.Info().
		Str("accountID", e.client.GetAccountID().String()).
		Str("region", e.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(resourceType)).
		Msgf("[RepoProxy.FindAll] aws resources found: %d", len(items))

	return items, err
}

func (e *RepoProxy) FindAllCC(resourceType cfg.ResourceType) (items []service.ResourceInterface, err error) {
	switch resourceType {
	case cfg.ResourceTypeBucket:
		items, err = FindS3CCBuckets(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeInstance:
		items, err = FindEc2CCInstances(e.ctx, e.client, e.cache)
	case cfg.ResourceTypeVolume:
		items, err = FindEbsCCVolumes(e.ctx, e.client, e.cache)
	default:
		err = fmt.Errorf("resource type %s not supported", cfgEntity.ResourceTypeToString(resourceType))
	}

	log.Info().
		Str("accountID", e.client.GetAccountID().String()).
		Str("region", e.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(resourceType)).
		Msgf("[RepoProxy.FindAll] aws resources found: %d", len(items))

	return items, err
}
