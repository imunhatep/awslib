package gateway

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

type RepoGatewayCached struct {
	RepoGatewayInterface
	cache *cache.DataCache
}

func NewRepoGatewayCached(gw RepoGatewayInterface, cache *cache.DataCache) *RepoGatewayCached {
	cacheNS := fmt.Sprintf("%s:%s", gw.GetAccountID(), gw.GetRegion())

	return &RepoGatewayCached{
		RepoGatewayInterface: gw,
		cache:                cache.WithNamespace(cacheNS),
	}
}

// FindAll a wrapper of RepoGateway method with reading and writing results into a cache
func (e *RepoGatewayCached) FindAll(resourceType types.ResourceType) (items []service.EntityInterface, err error) {
	items = []service.EntityInterface{}

	resourceTypeString := cfg.ResourceTypeToString(resourceType)
	if found := e.cache.Read(resourceTypeString, &items); !found {
		items, err = e.RepoGatewayInterface.FindAll(resourceType)

		if err == nil {
			err = e.cache.Write(resourceTypeString, items)
		}
	}

	log.Info().
		Str("accountID", e.GetClient().GetAccountID().String()).
		Str("region", e.GetClient().GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(resourceType)).
		Msgf("[RepoGatewayCached.DescribeResources] resources found: %d", len(items))

	return items, err
}
