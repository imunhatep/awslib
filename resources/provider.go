package resources

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/proxy"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

const ResourceBusSize = 10000

type Provider struct {
	proxyPool    []proxy.RepoProxyInterface
	resourceType types.ResourceType
}

func NewProvider(resourceType types.ResourceType, proxyPool ...proxy.RepoProxyInterface) Provider {
	ro := Provider{
		proxyPool:    proxyPool,
		resourceType: resourceType,
	}

	return ro
}

// Run fetches aws resources and sends to resource channel
func (r Provider) Run() *ResourceReader {
	log.Trace().
		Str("type", cfg.ResourceTypeToString(r.resourceType)).
		Msg("[AwsProvider.Run] processing resource type")

	if metrics.AwsMetricsEnabled {
		metrics.AwsObserverExecutionCount.WithLabelValues(cfg.ResourceTypeToString(r.resourceType)).Inc()
	}

	// resource transition channel
	stream := make(chan service.EntityInterface, ResourceBusSize)

	// resource reader
	resourceReader := NewResourceReader(r.resourceType, stream)

	// find resources and flush these to resource reader
	go r.findResources(stream)

	return resourceReader
}

// findResources fetches resources from all regions
func (r Provider) findResources(stream chan<- service.EntityInterface) {
	defer close(stream)

	log.Trace().
		Str("type", cfg.ResourceTypeToString(r.resourceType)).
		Msg("[AwsProvider.findResources] resource update")

	var wg sync.WaitGroup
	for _, gw := range r.proxyPool {
		wg.Add(1)

		go func() {
			r.findResourcesInRegion(gw, stream)
			wg.Done()
		}()

		// manual aws request throttle
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}

func (r Provider) findResourcesInRegion(gw proxy.RepoProxyInterface, stream chan<- service.EntityInterface) {
	resources, err := gw.FindAll(r.resourceType)
	if err != nil {
		log.Error().Err(err).
			Str("type", cfg.ResourceTypeToString(r.resourceType)).
			Msg("[AwsProvider.findResourcesInRegion] failed to find resources")
		return
	}

	r.flush(resources, stream)
}

func (r Provider) flush(resources []service.EntityInterface, stream chan<- service.EntityInterface) {
	for _, resource := range resources {
		select {
		case stream <- resource:
		default:
			if metrics.AwsMetricsEnabled {
				metrics.AwsObserverResourceQueueFull.WithLabelValues(string(resource.GetType())).Inc()
			}
			log.Warn().
				Str("arn", resource.GetArn()).
				Msg("[AwsProvider.flush] resource channel is full, value is discarded")
		}
	}
}
