package resources

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/proxy"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

const ResourceBusSize = 10000

// Bounding a fetch is the HTTP client's job, not this provider's. A region whose
// endpoint does not route is a connection that never establishes, which dial and TLS
// timeouts on the AWS client (v3.AwsDialTimeout, v3.AwsTLSHandshakeTimeout) cut off
// in seconds. A wall-clock deadline here cannot
// tell that apart from a legitimately long paginated read — S3 lists every bucket and
// then queries each one's location and tags — so it abandons real results and reports
// a working region as failed.

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

// MaxConcurrentProxies bounds how many (account, region) fetches run at once.
//
// It replaces a fixed 100ms pause before starting each proxy, which throttled
// nothing useful — every proxy targets its own account and region, and AWS rate
// limits are per account and region — while making the last of N proxies wait
// N/10 seconds to start: half a minute for a 19-account sweep, longer than a
// typical MCP client waits. A bound on concurrency keeps the burst of open
// connections in check without serialising the start.
var MaxConcurrentProxies = 32

// findResources fetches resources from all regions
func (r Provider) findResources(stream chan<- service.ResourceInterface, failures chan<- ProxyFailure) {
	defer close(stream)
	defer close(failures)

	log.Trace().
		Str("type", cfg.ResourceTypeToString(r.resourceType)).
		Msg("[AwsProvider.findResources] resource update")

	limit := MaxConcurrentProxies
	if limit < 1 {
		limit = 1
	}

	slots := make(chan struct{}, limit)

	var wg sync.WaitGroup
	for _, gw := range r.proxyPool {
		wg.Add(1)
		slots <- struct{}{}

		go func() {
			defer func() {
				<-slots
				wg.Done()
			}()

			r.findResourcesInRegion(gw, stream, failures)
		}()
	}
	wg.Wait()
}

func (r Provider) findResourcesInRegion(
	gw proxy.RepoProxyInterface,
	stream chan<- service.ResourceInterface,
	failures chan<- ProxyFailure,
) {
	found, err := gw.FindAll(r.resourceType)
	if err != nil {
		log.Error().Err(err).
			Str("accountID", gw.GetAccountID().String()).
			Str("region", gw.GetRegion().String()).
			Str("type", cfg.ResourceTypeToString(r.resourceType)).
			Msg("[AwsProvider.findResourcesInRegion] failed to find resources")

		// Clients are built without touching AWS, so this is often the first
		// request the pair has made: if it shows the whole region unusable,
		// the client's pool remembers that and later sweeps skip the pair.
		//
		// Not when the caller's own context has ended: a dial cut short by
		// the caller's deadline reads exactly like one cut short by the
		// dialer's timeout — same text, both errors.Is DeadlineExceeded — and
		// would remember every healthy region still connecting when a tool
		// call timed out.
		if ctx := gw.GetContext(); ctx == nil || ctx.Err() == nil {
			gw.GetClient().ReportFailure(err)
		}

		failures <- ProxyFailure{AccountID: gw.GetAccountID(), Region: gw.GetRegion(), Err: err}

		return
	}

	r.flush(found, stream)
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
