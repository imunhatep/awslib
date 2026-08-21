package resources

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/proxy"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

const ResourceBusSize = 10000

// DefaultRegionTimeout bounds how long one proxy may take to answer.
//
// Without a bound, Read() waits on every proxy indefinitely, so a single region
// whose endpoint does not route holds the whole result set hostage and the caller
// gets nothing instead of every other region's resources. The calls behind a
// proxy are paginated list operations against one region, so a minute is
// generous. The cost of the bound is that a genuinely slow region is reported as
// a failure rather than waited for — the right trade for a caller that has a
// request deadline of its own.
const DefaultRegionTimeout = 60 * time.Second

// ProxyFailure records a proxy that produced no resources because it could not
// be queried, so a caller can tell a short answer from a complete one.
//
// These used to be logged and dropped, which left "this account holds no
// resources of that type" and "this account could not be reached"
// indistinguishable in the result.
type ProxyFailure struct {
	AccountID ptypes.AwsAccountID
	Region    ptypes.AwsRegion
	Err       error
}

type Provider struct {
	proxyPool    []proxy.RepoProxyInterface
	resourceType types.ResourceType
	timeout      time.Duration
}

func NewProvider(resourceType types.ResourceType, proxyPool ...proxy.RepoProxyInterface) Provider {
	ro := Provider{
		proxyPool:    proxyPool,
		resourceType: resourceType,
		timeout:      DefaultRegionTimeout,
	}

	return ro
}

// WithTimeout overrides the per-proxy timeout. A non-positive value keeps
// DefaultRegionTimeout: the timeout cannot be switched off, because waiting
// forever is the failure it exists to prevent.
func (r Provider) WithTimeout(timeout time.Duration) Provider {
	if timeout > 0 {
		r.timeout = timeout
	}

	return r
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
	stream := make(chan service.ResourceInterface, ResourceBusSize)

	// At most one failure per proxy, so reporting one can never block the proxy
	// goroutine even before the reader starts draining.
	failures := make(chan ProxyFailure, len(r.proxyPool)+1)

	// resource reader
	resourceReader := NewResourceReader(r.resourceType, stream, failures)

	// find resources and flush these to resource reader
	go r.findResources(stream, failures)

	return resourceReader
}

// findResources fetches resources from all regions
func (r Provider) findResources(stream chan<- service.ResourceInterface, failures chan<- ProxyFailure) {
	defer close(stream)
	defer close(failures)

	log.Trace().
		Str("type", cfg.ResourceTypeToString(r.resourceType)).
		Msg("[AwsProvider.findResources] resource update")

	var wg sync.WaitGroup
	for _, gw := range r.proxyPool {
		wg.Add(1)

		go func() {
			r.findResourcesInRegion(gw, stream, failures)
			wg.Done()
		}()

		// manual aws request throttle
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}

func (r Provider) findResourcesInRegion(
	gw proxy.RepoProxyInterface,
	stream chan<- service.ResourceInterface,
	failures chan<- ProxyFailure,
) {
	timeout := r.timeout
	if timeout <= 0 {
		timeout = DefaultRegionTimeout
	}

	type answer struct {
		resources []service.ResourceInterface
		err       error
	}

	// Buffered, so a call that outlives the timeout can still complete its send
	// and exit rather than leaking a goroutine blocked on an unread channel.
	done := make(chan answer, 1)

	go func() {
		found, err := gw.FindAll(r.resourceType)
		done <- answer{resources: found, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case got := <-done:
		if got.err != nil {
			log.Error().Err(got.err).
				Str("accountID", gw.GetAccountID().String()).
				Str("region", gw.GetRegion().String()).
				Str("type", cfg.ResourceTypeToString(r.resourceType)).
				Msg("[AwsProvider.findResourcesInRegion] failed to find resources")

			failures <- ProxyFailure{AccountID: gw.GetAccountID(), Region: gw.GetRegion(), Err: got.err}

			return
		}

		r.flush(got.resources, stream)

	case <-timer.C:
		// Only this proxy's result is given up on. The abandoned goroutine keeps
		// running, completes into the buffered channel and exits.
		err := errors.Errorf("timed out after %s", timeout)

		log.Error().Err(err).
			Str("accountID", gw.GetAccountID().String()).
			Str("region", gw.GetRegion().String()).
			Str("type", cfg.ResourceTypeToString(r.resourceType)).
			Msg("[AwsProvider.findResourcesInRegion] proxy timed out, skipping")

		failures <- ProxyFailure{AccountID: gw.GetAccountID(), Region: gw.GetRegion(), Err: err}
	}
}

func (r Provider) flush(resources []service.ResourceInterface, stream chan<- service.ResourceInterface) {
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
