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

	// proxies that could not be queried. Kept alongside the values so a caller
	// reading a short list can tell whether it is short because the estate is
	// small or because part of it was unreachable.
	failures []ProxyFailure

	wg sync.WaitGroup
}

func NewResourceReader(
	resourceType types.ResourceType,
	channel <-chan service.ResourceInterface,
	failures <-chan ProxyFailure,
) *ResourceReader {
	cr := &ResourceReader{
		resourceType: resourceType,
		values:       []service.ResourceInterface{},
		failures:     []ProxyFailure{},
		wg:           sync.WaitGroup{},
	}

	// Two producers, both drained to completion before Read or Failures answers.
	// Each goroutine owns its own field, so no further synchronisation is needed.
	cr.wg.Add(2)
	go cr.await(channel)
	go cr.awaitFailures(failures)

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

func (cr *ResourceReader) awaitFailures(channel <-chan ProxyFailure) {
	defer cr.wg.Done()

	for f := range channel {
		cr.failures = append(cr.failures, f)
	}

	if len(cr.failures) > 0 {
		log.Warn().Msgf("[ResourceReader.awaitFailures] proxies that could not be queried: %d", len(cr.failures))
	}
}

func (cr *ResourceReader) Read() []service.ResourceInterface {
	cr.wg.Wait()

	result := make([]service.ResourceInterface, len(cr.values))
	copy(result, cr.values)

	return result
}

// Failures reports the proxies that could not be queried during this run: a
// region that errored or timed out. An empty slice means every proxy answered,
// so the resource list is complete rather than merely short.
func (cr *ResourceReader) Failures() []ProxyFailure {
	cr.wg.Wait()

	result := make([]ProxyFailure, len(cr.failures))
	copy(result, cr.failures)

	return result
}

func (cr *ResourceReader) ResourceType() types.ResourceType {
	return cr.resourceType
}
