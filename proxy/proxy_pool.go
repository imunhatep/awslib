package proxy

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/imunhatep/awslib/cache"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	cfgEntity "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/dict"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

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

		services = append(services, NewRepoProxy(ctx, client))
	}

	return &RepoProxyPool{services}
}

func (e *RepoProxyPool) WithCache(dc *cache.DataCache) *RepoProxyPool {
	services := []RepoProxyInterface{}

	for _, gw := range e.gateways {
		if proxy, ok := gw.(*RepoProxy); ok {
			services = append(services, proxy.WithCache(dc))
		} else {
			services = append(services, gw)
		}
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
