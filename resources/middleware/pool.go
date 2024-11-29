package middleware

import (
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/resources"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/dict"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"sync"
)

type ResourcePoolMiddleware struct {
	resourceList map[types.ResourceType][]service.EntityInterface
	running      bool
	writeLock    sync.RWMutex
}

func NewResourcePoolMiddleware() *ResourcePoolMiddleware {
	return &ResourcePoolMiddleware{
		resourceList: map[types.ResourceType][]service.EntityInterface{},
	}
}

// GetResources returns all resources
func (m *ResourcePoolMiddleware) GetResources() []service.EntityInterface {
	m.writeLock.RLock()
	defer m.writeLock.RUnlock()

	resourceList := []service.EntityInterface{}
	for _, rsr := range dict.Values(m.resourceList) {
		resourceList = append(resourceList, rsr...)
	}

	return resourceList
}

// GetResourcesByType returns resources by type
func (m *ResourcePoolMiddleware) GetResourcesByType(resourceType types.ResourceType) []service.EntityInterface {
	m.writeLock.RLock()
	defer m.writeLock.RUnlock()

	if resourceList, ok := m.resourceList[resourceType]; ok {
		return resourceList
	}

	return []service.EntityInterface{}
}

// HandleResourceReader is a middleware that processes resources from the resource reader
func (m *ResourcePoolMiddleware) HandleResourceReader(next resources.HandlerFunc) resources.HandlerFunc {
	return func(reader *resources.ResourceReader) error {
		log.Debug().Msg("[ResourcePoolMiddleware.HandleResourceReader] processing resources")

		resourceType := reader.ResourceType()
		resourceList := reader.Read()

		log.Debug().
			Str("type", cfg.ResourceTypeToString(resourceType)).
			Msgf("[ResourcePoolMiddleware.HandleResourceReader] resources found: %d", len(resourceList))

		m.flush(resourceType, resourceList)
		go m.updateMetrics(resourceType)

		return next(reader)
	}
}

// resourceList receives unix timestamp with refresh initialized time
func (m *ResourcePoolMiddleware) flush(resourceType types.ResourceType, resourceList []service.EntityInterface) {
	if slice.IsEmpty(resourceList) {
		log.Debug().Msg("[ResourcePoolMiddleware.flush] flushing resources, list is empty")
	}

	log.Debug().
		Str("type", cfg.ResourceTypeToString(resourceType)).
		Msg("[ResourcePoolMiddleware.flush] flushing resources, from stream")

	m.writeLock.Lock()
	defer m.writeLock.Unlock()

	m.resourceList[resourceType] = resourceList
}

func (m *ResourcePoolMiddleware) updateMetrics(resourceType types.ResourceType) {
	resourceList, ok := m.resourceList[resourceType]
	if !ok {
		promQL := map[string]string{"resource_type": cfg.ResourceTypeToString(resourceType)}
		if metrics.AwsMetricsEnabled {
			metrics.AwsPoolResourcePerRegionCount.DeletePartialMatch(promQL)
		}

		return
	}

	countByAccountAndRegion := map[ptypes.AwsAccountID]map[ptypes.AwsRegion]int{}
	for _, resource := range resourceList {
		region := resource.GetRegion()
		accountId := resource.GetAccountID()

		if _, ok := countByAccountAndRegion[accountId]; !ok {
			countByAccountAndRegion[accountId] = map[ptypes.AwsRegion]int{}
		}

		countByAccountAndRegion[accountId][region]++
	}

	// metrics
	for accountId, regions := range countByAccountAndRegion {
		for region, cnt := range regions {
			if metrics.AwsMetricsEnabled {
				metrics.AwsPoolResourcePerRegionCount.
					WithLabelValues(accountId.String(), region.String(), cfg.ResourceTypeToString(resourceType)).
					Set(float64(cnt))
			}
		}
	}
}
