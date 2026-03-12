package resources

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/service"
	"github.com/rs/zerolog/log"
)

type ResourceReader struct {
	resourceType types.ResourceType

	// stored values
	values []service.ResourceInterface
	wg     sync.WaitGroup
}

func NewResourceReader(resourceType types.ResourceType, channel <-chan service.ResourceInterface) *ResourceReader {
	cr := &ResourceReader{
		resourceType: resourceType,
		values:       []service.ResourceInterface{},
		wg:           sync.WaitGroup{},
	}

	cr.wg.Add(1)
	go cr.await(channel)

	return cr
}

func (cr *ResourceReader) await(channel <-chan service.ResourceInterface) {
	defer cr.wg.Done()

	log.Trace().Msg("[ResourceReader.await] reading channel..")
	for v := range channel {
		cr.values = append(cr.values, v)
	}
	log.Trace().Msgf("[ResourceReader.await] resources found: %d", len(cr.values))
}

func (cr *ResourceReader) Read() []service.ResourceInterface {
	cr.wg.Wait()

	result := make([]service.ResourceInterface, len(cr.values))
	copy(result, cr.values)

	return result
}

func (cr *ResourceReader) ResourceType() types.ResourceType {
	return cr.resourceType
}
