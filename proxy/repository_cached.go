package proxy

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

type RepoProxyCached struct {
	RepoProxyInterface
	cache *cache.DataCache
}

func NewRepoProxyCached(px RepoProxyInterface, cache *cache.DataCache) *RepoProxyCached {
	cacheNS := fmt.Sprintf("%s:%s", px.GetAccountID(), px.GetRegion())

	return &RepoProxyCached{
		RepoProxyInterface: px,
		cache:              cache.WithNamespace(cacheNS),
	}
}

// FindAll a wrapper of RepoProxy method with reading and writing results into a cache
func (e *RepoProxyCached) FindAll(resourceType types.ResourceType) (items []service.EntityInterface, err error) {
	items = []service.EntityInterface{}

	resourceTypeString := cfg.ResourceTypeToString(resourceType)
	if found := e.cache.Read(resourceTypeString, &items); !found {
		items, err = e.RepoProxyInterface.FindAll(resourceType)

		if err == nil {
			err = e.cache.Write(resourceTypeString, items)
		}
	}

	log.Info().
		Str("accountID", e.GetClient().GetAccountID().String()).
		Str("region", e.GetClient().GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(resourceType)).
		Msgf("[RepoProxyCached.DescribeResources] resources found: %d", len(items))

	return items, err
}
