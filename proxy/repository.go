package proxy

import (
	"context"
	"fmt"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/imunhatep/awslib/cache"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	cfgEntity "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/dict"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

type RepoProxyInterface interface {
	GetAccountID() ptypes.AwsAccountID
	GetRegion() ptypes.AwsRegion
	GetClient() *v3.Client
	GetContext() context.Context
	FindAll(resourceType cfg.ResourceType) ([]service.EntityInterface, error)
}

type RepoProxyPool struct {
	gateways []RepoProxyInterface
}

func NewRepoProxyPool(ctx context.Context, clients []*v3.Client) *RepoProxyPool {
	var services []RepoProxyInterface
	for _, client := range clients {
		log.Trace().
			Str("accountID", client.GetAccountID().String()).
			Str("region", client.GetRegion().String()).
			Msg("[RepoProxyPool.NewRepoProxyPool] adding client to the pool")

		services = append(services, NewRepoGateway(ctx, client))
	}

	return &RepoProxyPool{services}
}

func (e *RepoProxyPool) WithCache(cache *cache.DataCache) *RepoProxyPool {
	services := []RepoProxyInterface{}

	for _, gw := range e.gateways {
		cacheNS := fmt.Sprintf("%s:%s", gw.GetAccountID(), gw.GetRegion())
		services = append(services, NewRepoProxyCached(gw, cache.WithNamespace(cacheNS)))
	}

	return &RepoProxyPool{services}
}

func (e *RepoProxyPool) List(resourceType cfg.ResourceType) []RepoProxyInterface {
	// nothing to filter
	if slice.IsEmpty(e.gateways) {
		return e.gateways
	}

	// no filtering for regional resources
	if !slice.Contains(cfgEntity.ResourceTypeListGlobal(), resourceType) {
		return e.gateways
	}

	// for global resources prefer eu-central-1
	filterEuCentral1 := func(p RepoProxyInterface) bool {
		return p.GetRegion() == ptypes.AwsRegion(types.VPCRegionEuCentral1)
	}

	regionalGws := slice.Filter(e.gateways, filterEuCentral1)
	if !slice.IsEmpty(regionalGws) {
		return regionalGws
	}

	// eu-central-1 not in the list, then any region will do
	anyGwMap := map[ptypes.AwsAccountID]RepoProxyInterface{}
	for _, gw := range e.gateways {
		if _, ok := anyGwMap[gw.GetAccountID()]; ok {
			continue
		}

		anyGwMap[gw.GetAccountID()] = gw
	}

	return dict.Values(anyGwMap)
}

// RepoGateway is proxy to aws repositories to get all aws resources
type RepoGateway struct {
	ctx    context.Context
	client *v3.Client
}

func NewRepoGateway(ctx context.Context, client *v3.Client) *RepoGateway {
	return &RepoGateway{
		ctx:    ctx,
		client: client,
	}
}

func (e RepoGateway) GetAccountID() ptypes.AwsAccountID {
	return e.client.GetAccountID()
}

func (e RepoGateway) GetRegion() ptypes.AwsRegion {
	return e.client.GetRegion()
}

func (e RepoGateway) GetClient() *v3.Client {
	return e.client
}

func (e *RepoGateway) GetContext() context.Context {
	return e.ctx
}

func (e *RepoGateway) FindAll(resourceType cfg.ResourceType) (items []service.EntityInterface, err error) {
	switch resourceType {
	case cfg.ResourceTypeAutoScalingGroup:
		items, err = FindAutoScaleGroups(e.ctx, e.client)
	case cfg.ResourceTypeBatchComputeEnvironment:
		items, err = FindBatchComputeEnvironments(e.ctx, e.client)
	case cfg.ResourceTypeBatchJobQueue:
		items, err = FindBatchJobQueues(e.ctx, e.client)
	case cfgEntity.ResourceTypeGlueDatabase:
		items, err = FindGlueDatabases(e.ctx, e.client)
	case cfg.ResourceTypeGlueJob:
		items, err = FindGlueJobs(e.ctx, e.client)
	case cfgEntity.ResourceTypeGlueTable:
		items, err = FindGlueTables(e.ctx, e.client)
	case cfg.ResourceTypeBucket:
		items, err = FindS3Buckets(e.ctx, e.client)
	case cfg.ResourceTypeDBInstance:
		items, err = FindDbInstances(e.ctx, e.client)
	case cfg.ResourceTypeDBSnapshot:
		items, err = FindDbSnapshots(e.ctx, e.client)
	case cfg.ResourceTypeTable:
		items, err = FindDynamodbTables(e.ctx, e.client)
	case cfg.ResourceTypeECSCluster:
		items, err = FindEcsClusters(e.ctx, e.client)
	case cfg.ResourceTypeECSService:
		items, err = FindEcsServices(e.ctx, e.client)
	case cfg.ResourceTypeEKSCluster:
		items, err = FindEksClusters(e.ctx, e.client)
	case cfgEntity.ResourceTypeEmrCluster:
		items, err = FindEmrClusters(e.ctx, e.client)
	case cfgEntity.ResourceTypeEmrServerlessApplication:
		items, err = FindEmrServerlessApplications(e.ctx, e.client)
	case cfgEntity.ResourceTypeEmrServerlessJobRun:
		items, err = FindEmrServerlessJobRuns(e.ctx, e.client)
	case cfg.ResourceTypeFunction:
		items, err = FindLambdaFunctions(e.ctx, e.client)
	case cfg.ResourceTypeInstance:
		items, err = FindEc2Instances(e.ctx, e.client)
	case cfgEntity.ResourceTypeCloudWatchLogGroup:
		items, err = FindCloudWatchLogGroups(e.ctx, e.client)
	case cfg.ResourceTypeRoute53HostedZone:
		items, err = FindRoute53HostedZones(e.ctx, e.client)
	case cfgEntity.ResourceTypeSnapshot:
		items, err = FindEc2Snapshots(e.ctx, e.client)
	case cfg.ResourceTypeVolume:
		items, err = FindEc2Volumes(e.ctx, e.client)
	case cfg.ResourceTypeEFSFileSystem:
		items, err = FindEfsFileSystems(e.ctx, e.client)
	case cfg.ResourceTypeLoadBalancerV2:
		items, err = FindLoadBalancers(e.ctx, e.client)
	case cfg.ResourceTypeSecret:
		items, err = FindSecretManagerSecrets(e.ctx, e.client)
	case cfg.ResourceTypeQueue:
		items, err = FindSqsQueues(e.ctx, e.client)
	case cfg.ResourceTypeTopic:
		items, err = FindSnsTopics(e.ctx, e.client)
	case cfg.ResourceTypeUser:
		items, err = FindIamUsers(e.ctx, e.client)
	case cfg.ResourceTypeVpc:
		items, err = FindEc2Vpcs(e.ctx, e.client)
	default:
		err = fmt.Errorf("resource type %s not supported", cfgEntity.ResourceTypeToString(resourceType))
	}

	log.Info().
		Str("accountID", e.client.GetAccountID().String()).
		Str("region", e.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(resourceType)).
		Msgf("[RepoGateway.FindAll] aws resources found: %d", len(items))

	return items, err
}

func (e *RepoGateway) FindAllCC(resourceType cfg.ResourceType) (items []service.EntityInterface, err error) {
	switch resourceType {
	case cfg.ResourceTypeBucket:
		items, err = FindS3CCBuckets(e.ctx, e.client)
	case cfg.ResourceTypeInstance:
		items, err = FindEc2CCInstances(e.ctx, e.client)
	case cfg.ResourceTypeVolume:
		items, err = FindEbsCCVolumes(e.ctx, e.client) // FindEbsCCVolumes(e.ctx, e.client)
	default:
		err = fmt.Errorf("resource type %s not supported", cfgEntity.ResourceTypeToString(resourceType))
	}

	log.Info().
		Str("accountID", e.client.GetAccountID().String()).
		Str("region", e.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(resourceType)).
		Msgf("[RepoGateway.FindAll] aws resources found: %d", len(items))

	return items, err
}
