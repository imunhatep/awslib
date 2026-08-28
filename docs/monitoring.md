# Monitoring

The library instruments every AWS call, the parallel fetcher and the cache with
Prometheus metrics.

## Enabling

Nothing is emitted until `metrics.InitMetrics` is called — it both builds the collectors
and flips `metrics.AwsMetricsEnabled`, which every emission site is guarded by. The
argument becomes the Prometheus **subsystem**, so it prefixes every metric name below:

```go
import "github.com/imunhatep/awslib/metrics"

metrics.InitMetrics("cloudevo_aws") // -> cloudevo_aws_api_request_count, ...
```

Register the collectors with your own registry as usual, and expose them on whatever
handler your service already serves `/metrics` from.

## Metrics

### API calls

Labeled `account_id`, `region`, `resource_type`, `method` — `method` being the AWS API
call (`DescribeInstances`) or the repository method that wraps it.

| Metric | Type | Meaning |
|---|---|---|
| `api_request_count` | Counter | AWS API requests made |
| `api_error_count` | Counter | requests that returned an error |
| `api_fetched_resources_count` | Gauge | resources returned |
| `repo_call_duration` | Histogram | time to complete a repository call |

`api_request_count` counts *pages*, not calls: a paginated `List*All` increments once per
page, which is what makes it comparable with a per-request AWS bill.

### Resource fetching

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `resources_pool_resource_count` | Gauge | `account_id`, `region`, `resource_type` | resources held in the resource pool |
| `resources_observer_execution_count` | Gauge | `resource_type` | observer executions |
| `resources_resource_queue_full_count` | Counter | `resource_type` | resources **dropped** because the provider's channel was full |

`resources_resource_queue_full_count` is the one to alert on: a non-zero value means the
fetcher discarded resources, so any report built from that run is silently incomplete.

### Cache

Labeled `ns`, `name`, `store` — the namespace (`<accountID>:<region>:<service>`), the
cache key's method name, and which handler answered (in-memory or file).

| Metric | Type | Meaning |
|---|---|---|
| `resources_resource_cache_read_count` | Counter | cache reads attempted |
| `resources_resource_cache_write_count` | Counter | cache writes |
| `resources_resource_cache_hits` | Counter | reads served from cache |
| `resources_resource_cache_error_count` | Counter | cache read/write errors |

Hit rate is `cache_hits / cache_read_count`. A rate near zero with a healthy write count
usually means the cache key changes every call — the classic cause being a query window
that ends at `now` rather than on a rounded boundary.

Reads fan across handlers in order and stop at the first hit, so a file-handler read is
only attempted after the in-memory handler misses. That makes per-`store` hit rates
asymmetric by design, not by fault.

## Reading the numbers

Resource collection is bursty: a read cycle runs, then the counters stay flat until the
next one. Any panel built on these must use a rate window wider than the gap between
cycles, or the ratio of two zero rates yields no sample and the panel shows gaps rather
than a flat line.
