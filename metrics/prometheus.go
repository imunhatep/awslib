package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const AwslibSubsystem = "awslib"

var AwsApiDurationBuckets = []float64{.01, .1, .5, 1, 10, 30, 60, 120}

var (
	AwsMetricsEnabled             bool
	AwsApiRequests                *prometheus.CounterVec
	AwsApiRequestErrors           *prometheus.CounterVec
	AwsApiResourcesFetched        *prometheus.GaugeVec
	AwsRepoCallDuration           *prometheus.HistogramVec
	AwsPoolResourcePerRegionCount *prometheus.GaugeVec
	AwsObserverExecutionCount     *prometheus.GaugeVec
	AwsObserverResourceQueueFull  *prometheus.CounterVec
	AwsResourceCacheRead          *prometheus.CounterVec
	AwsResourceCacheWrite         *prometheus.CounterVec
	AwsResourceCacheHit           *prometheus.CounterVec
	AwsResourceCacheError         *prometheus.CounterVec
)

// InitMetrics initialize Prometheus metrics
func InitMetrics(subsystem string) {
	AwsMetricsEnabled = true

	AwsApiRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "api_request_count",
			Help:      "number of aws api requests",
		},
		[]string{"account_id", "region", "resource_type", "method"},
	)

	AwsApiRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "api_error_count",
			Help:      "number of aws api requests ended up with error",
		},
		[]string{"account_id", "region", "resource_type", "method"},
	)

	AwsApiResourcesFetched = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "api_fetched_resources_count",
			Help:      "number of aws resources fetched through api",
		},
		[]string{"account_id", "region", "resource_type", "method"},
	)

	AwsRepoCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "repo_call_duration",
			Help:      "time elapsed to process aws repository call in milliseconds ",
			Buckets:   AwsApiDurationBuckets,
		},
		[]string{"account_id", "region", "resource_type", "method"},
	)

	AwsPoolResourcePerRegionCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "resources_pool_resource_count",
			Help:      "AWS resources stored in resource pool per region",
		},
		[]string{"account_id", "region", "resource_type"},
	)

	AwsObserverExecutionCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "resources_observer_execution_count",
			Help:      "AWS observer executions count",
		},
		[]string{"resource_type"},
	)

	AwsObserverResourceQueueFull = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "resources_resource_queue_full_count",
			Help:      "AWS resources producer resources skipped cause queue is full",
		},
		[]string{"resource_type"},
	)

	AwsResourceCacheRead = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "resources_resource_cache_read_count",
			Help:      "Number of aws resources cache reads",
		},
		[]string{"ns", "name", "store"},
	)

	AwsResourceCacheWrite = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "resources_resource_cache_write_count",
			Help:      "Number of aws resources cache writes",
		},
		[]string{"ns", "name", "store"},
	)

	AwsResourceCacheHit = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "resources_resource_cache_hits",
			Help:      "Number of aws resources cache hits",
		},
		[]string{"ns", "name", "store"},
	)

	AwsResourceCacheError = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "resources_resource_cache_error_count",
			Help:      "Number of cache related errors in reconciler",
		},
		[]string{"ns", "name", "store"},
	)

	// repository
	prometheus.MustRegister(AwsApiRequests)
	prometheus.MustRegister(AwsApiRequestErrors)
	prometheus.MustRegister(AwsApiResourcesFetched)
	prometheus.MustRegister(AwsRepoCallDuration)

	// middleware/pool
	prometheus.MustRegister(AwsPoolResourcePerRegionCount)

	// observer
	prometheus.MustRegister(AwsObserverExecutionCount)
	prometheus.MustRegister(AwsObserverResourceQueueFull)

	// datacache
	prometheus.MustRegister(AwsResourceCacheRead)
	prometheus.MustRegister(AwsResourceCacheWrite)
	prometheus.MustRegister(AwsResourceCacheHit)
	prometheus.MustRegister(AwsResourceCacheError)
}
