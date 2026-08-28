# Why awslib instead of the raw AWS SDK v2?

What the library adds over `aws-sdk-go-v2`, and what those additions are worth in
money and wall-clock time.

## Why awslib instead of the raw AWS SDK v2?

`awslib` wraps `aws-sdk-go-v2`, it does not replace it — the underlying SDK client is always one call
away (`ec2.GetClient(client)`), so nothing is off limits. What the library adds is the plumbing that
every multi-account AWS tool ends up writing by hand:

| Concern | Raw `aws-sdk-go-v2` | `awslib` |
|---|---|---|
| **Multi-account, multi-region** | One `aws.Config` per account and per region; STS assume-role, credential refresh and fanout are yours to wire | `v3.ClientPool` takes a `map[AwsAccountID]RoleArn` and builds the whole account × region matrix concurrently, caching assumed-role credentials per role |
| **Caching** | None | `repo.WithCache(dc)` on every repository — generated, namespaced `<accountID>:<region>`, pluggable in-memory (bigcache) or file handlers, and only written on success |
| **Cache keys** | — | `cache.Key` renders arguments *by value*: pointers dereferenced, maps sorted, unexported fields included. Formatting an SDK input with `%v` instead embeds pointer addresses, giving keys that change on every call and collide once the allocator reuses an address |
| **Pagination** | A paginator wired up at each call site — and some APIs ship none at all (Cost Explorer's `GetCostAndUsage` and `GetDimensionValues` have no SDK paginator) | `List*All()` / `Get*` methods drive pagination internally and return complete, flattened slices |
| **Heterogeneous resources** | Every service returns its own unrelated struct | 24 service packages implement one `service.ResourceInterface` (`GetAccountID`, `GetRegion`, `GetArn`, `GetId`, `GetType`, `GetTags`, `GetCreatedAt`), so unrelated resource types flow through the same channels and reports |
| **Cross-account fetching** | Your own goroutine fanout, channels, throttling and error handling | `proxy.RepoProxy` maps 39 resource types to the right repository; `resources.Provider` runs them in parallel and streams results over a buffered channel |
| **Unsupported resource types** | Read the service's API docs and write another lister | `proxy.NewGenericRepoProxyPool` serves *any* `AWS::Service::Resource` type via the Cloud Control API, with no per-type code — same interface, same fanout, same cache |
| **Observability** | None | 11 Prometheus metrics — request and error counts, resources fetched, call duration, cache read/write/hit/error — labeled by `account_id`, `region`, `resource_type` and `method` |
| **Errors and retries** | Bare SDK errors, SDK default retries | Errors wrapped with `go-errors` to carry stack traces; 5 retry attempts with a 3s max backoff configured on every client |
| **Adding a service** | Hand-written boilerplate per service | Generators emit the client wrappers, cached repositories and gob registrations |

### What the cache is worth in dollars

Nearly every AWS API this library wraps is free per call — `DescribeInstances`, `DescribeLogGroups`,
`GetProducts`, the Health and Cloud Control APIs. For those the cache buys wall-clock time, not
money.

The exception is **Cost Explorer, billed at $0.01 per paginated request** (check the
[current pricing](https://aws.amazon.com/aws-cost-management/pricing/) — figures below are
us-east-1 at the time of writing). Every *page* is billable, so a query spanning five pages costs
$0.05. `GetCostAndUsage` follows pagination internally and caches the **accumulated** result, so a
single cache hit saves every page of that query rather than just the first.

```
uncached $/month = accounts × queries_per_refresh × pages_per_query × refreshes_per_month × $0.01
cached refreshes = min(refreshes_per_month, 720 / ttl_hours)
```

Worked at one single-page query per account, against a 6h TTL:

| Scenario | Refresh rate | Raw `aws-sdk-go-v2` | `awslib`, 6h TTL | Saved / month |
|---|---|---|---|---|
| 10 accounts | hourly | 7,200 req — $72 | 1,200 req — $12 | **$60** |
| 50 accounts | hourly | 36,000 req — $360 | 6,000 req — $60 | **$300** |
| 200 accounts | hourly | 144,000 req — $1,440 | 24,000 req — $240 | **$1,200** |
| 50 accounts, 3-panel dashboard | every 5 min, 10h/day | 396,000 req — $3,960 | 18,000 req — $180 | **$3,780** |

Two caveats, so the numbers stay honest:

- **A cache only pays when the query repeats.** A window ending at `now` yields a new cache key on
  every call, so it never hits — and it samples cost data that is still settling. Round the window
  to a day or hour boundary and both problems go away.
- **The table assumes one page per query.** Real queries grouped by service or tag often span
  several pages, which scales both columns up together: the ratio holds and the absolute saving
  grows.

Beyond the bill, `CostQuery.Validate()` rejects malformed queries locally — missing metrics, more
than two `GroupBy` entries, match options `GetCostAndUsage` does not accept — so a bad query costs
you nothing instead of a round trip and a slot in your request budget.

### What the parallel fetcher is worth in compute

For the free APIs the saving lands on your own compute bill instead. `resources.Provider` fans the
repositories out across the account × region matrix concurrently (staggering launches by 100ms)
rather than walking it serially. At 50 accounts × 4 regions = 200 client-regions and ~1.5s per
call, that is roughly 5 minutes of sequential wall-clock against ~25s — the difference between a
CI job you wait on and one you do not.
