# Observer and middleware

`resources.ResourceObserver` fetches a list of resource types across every account and
region and pushes each type's results through a chain of middleware into your handler.

For a single resource type, or when you just want a slice back,
`resources.NewProvider(...).Run().Read()` is the shorter path — see
[repositories.md](repositories.md). Use the observer when you want a pipeline over many
types.

- [Quick start](#quick-start)
- [What ships](#what-ships)
- [Collecting the results](#collecting-the-results)
- [Writing your own middleware](#writing-your-own-middleware)
- [How it behaves](#how-it-behaves)
- [Related: who created this resource?](#related-who-created-this-resource)
- [Related: CloudTrail lookups](#related-cloudtrail-lookups)

## Quick start

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/configservice/types"
    "github.com/imunhatep/awslib/proxy"
    "github.com/imunhatep/awslib/resources"
    "github.com/imunhatep/awslib/resources/middleware"
)

clients, err := clientPool.GetClients("eu-central-1", "us-east-1")
if err != nil {
    return err
}

proxyPool := proxy.NewRepoProxyPool(ctx, clients).WithCache(dataCache)

observer := resources.NewResourceObserver(proxyPool, middleware.LoggerHandler())

err = observer.Serve([]types.ResourceType{
    types.ResourceTypeInstance,
    types.ResourceTypeBucket,
})
```

That logs every instance and bucket in every account and region the pool covers. The
handler is the end of the chain — swap it for your own function to do something else:

```go
observer := resources.NewResourceObserver(proxyPool, func(reader resources.ResourceReaderInterface) error {
    for _, resource := range reader.Read() {
        fmt.Println(resource.GetType(), resource.GetAccountID(), resource.GetRegion(), resource.GetIdOrArn())
    }

    return nil
})
```

Add middleware with `Use` — stages run in the order added, before the handler:

```go
observer.Use(middleware.NewLoggerMiddleware())
```

## What ships

**Handlers** — terminal, exactly one, passed to `NewResourceObserver`:

| Handler | Does |
|---|---|
| `middleware.SummaryHandler()` | logs the count per resource type (debug) |
| `middleware.LoggerHandler()` | logs every resource's ARN (info) |
| `middleware.NullHandler()` | does nothing — use it when the middleware *is* the work |
| `middleware.WaitHandler(ch)` | sleeps 3s, then signals `ch`; a demo, not production plumbing |

**Middleware** — stackable, passed to `Use`:

| Middleware | Does |
|---|---|
| `middleware.NewLoggerMiddleware()` | logs type, ARN and tags per resource (trace), then continues |
| `middleware.NewResourcePoolMiddleware()` | keeps the latest results per type in memory, exports per-account/region counts as Prometheus gauges, then continues |

To sweep everything the typed path covers, feed `Serve` from
`cfg.ResourceTypeListRegional()` or `cfg.ResourceTypeListGlobal()`.

## Collecting the results

`ResourcePoolMiddleware` is how you get the resources out of a run. It stores the latest
results per type and is safe to read from another goroutine while a `Serve` is in flight —
so an HTTP handler can answer from the last sweep while the next one runs.

```go
pool := middleware.NewResourcePoolMiddleware()

observer := resources.NewResourceObserver(proxyPool, middleware.NullHandler())
observer.Use(pool)

if err := observer.Serve(resourceTypes); err != nil {
    return err
}

all := pool.GetResources()
buckets := pool.GetResourcesByType(types.ResourceTypeBucket)
```

Two things to plan for:

- **Each sweep replaces a type's results wholesale**, so a type that returned nothing this
  run reads as empty rather than stale-but-populated. Right for an inventory — and the
  reason an unreachable region matters (see [below](#how-it-behaves)).
- **It holds every resource of every type in memory** for the observer's lifetime. For a
  large estate that is the process's memory ceiling; aggregate and discard in your own
  middleware if that bites.

## Writing your own middleware

Anything with a `HandleResourceReader` method qualifies — no registration, no base type:

```go
type UntaggedCounter struct {
    Missing map[types.ResourceType]int
}

func (m *UntaggedCounter) HandleResourceReader(next resources.HandlerFunc) resources.HandlerFunc {
    return func(reader resources.ResourceReaderInterface) error {
        for _, r := range reader.Read() {
            if _, ok := r.GetTags()["Team"]; !ok {
                m.Missing[reader.ResourceType()]++
            }
        }

        return next(reader)
    }
}
```

Call `next(reader)` unless you deliberately want to end the chain — dropping it silently
skips every later stage *and* the handler. A stage can run code before and after `next`,
which is how you'd time a sweep or wrap it in a transaction.

The three types involved, if you want them spelled out:

```go
// What a stage receives: one resource type's worth of results.
type ResourceReaderInterface interface {
    ResourceType() types.ResourceType
    Read() []service.ResourceInterface
}

// The end of the chain — your code.
type HandlerFunc func(c ResourceReaderInterface) error

// A stage.
type MiddlewareInterface interface {
    HandleResourceReader(next HandlerFunc) HandlerFunc
}
```

## How it behaves

Details that matter once a run is more than a demo.

**Types run sequentially, regions run in parallel.** Each type fans out across every
account × region proxy (staggered 100ms apart as a crude throttle), but the next type does
not start until the current type's chain has returned. So the API request rate scales with
the region count, not with `types × regions`.

**The chain runs once per type**, not once per resource. A stage receives the reader and
decides whether to call `Read()` at all.

**The first error aborts the run.** `Serve` returns as soon as a stage or the handler
returns non-nil, and the remaining types are never fetched. A stage that wants a failure to
be survivable must swallow it and record it.

**A short result is not proof of an empty estate.** A region that could not be queried
produces no resources and no error at this level. Cast the reader to
`*resources.ResourceReader` and check `Failures()` — each entry is a
`resources.ProxyFailure{AccountID, Region, Err}` — before reporting "none found":

```go
if failed, ok := reader.(*resources.ResourceReader); ok {
    for _, f := range failed.Failures() {
        log.Warn().Err(f.Err).Msgf("%s/%s could not be queried", f.AccountID, f.Region)
    }
}
```

**`Read()` returns a fresh slice each call, and the resources in it are read-only.** The
slice is a shallow copy, so a stage can sort or filter in place without disturbing what
later stages see; calling it in five stages costs five slice copies but only one fetch.
The resources themselves cannot be mutated through `ResourceInterface`: entities are value
types with value receivers and no setters, and their `GetTags()`/`GetAttributes()` hand back
copies. What is still shared is the embedded SDK struct's pointer and slice fields, reachable
only after a deliberate type assertion — don't write through those.

## Related: who created this resource?

`resources.AwsBlame` reads CloudTrail around each resource's creation time and reports the
non-read-only event's username:

```go
blame := resources.NewAwsBlame(ctx, clientPool).
    WithTtl(7 * 24 * time.Hour).
    WithResourceTypeList([]types.ResourceType{types.ResourceTypeInstance})

events, err := blame.LookupAll(pool.GetResources()...)
if err != nil {
    return err
}

for id, e := range events {
    fmt.Println(id, e.GetUsername(), e.GetUser()) // GetUser strips the @domain
}
```

- **One CloudTrail lookup per resource** — `LookupAll` iterates, it does not batch, and
  `LookupEvents` is heavily rate-limited. Filter hard before calling it.
- **`WithTtl` is the filter that matters** (default 30 days): resources created before the
  window are skipped without an API call, and CloudTrail retains only 90 days anyway.
- **`WithTtl` and `WithResourceTypeList` return copies, and `WithTtl` drops a type list set
  earlier** — set the TTL first, then the type list.
- Unknown creators come back as `resources.ResourceCreatorUnknown` (`"unknown"`).

It accepts any `resources.AwsClientPool` (`GetContext()` + `GetClient(accountID, region)`),
which both client pools satisfy.

## Related: CloudTrail lookups

`cloudtrail.NewLookupMiddleware()` is a different kind of middleware: it builds a
`LookupEventsInput` rather than wrapping a handler. `AwsBlame` uses it, and it is directly
useful on its own:

```go
lookup := cloudtrail.NewLookupMiddleware().
    WithStartTime(time.Now().Add(-24 * time.Hour)).
    WithEndTime(time.Now()).
    WithEventName("RunInstances"). // one attribute only — see below
    WithLimit(50)

if errs, ok := lookup.Errors(); !ok {
    return errs[0]
}

events, err := cloudtrail.NewCloudTrailRepository(ctx, client).ListEventsByLookup(lookup)
```

- **Errors are collected, not returned** — every `With*` returns the builder, so check
  `Errors()` once before use. The second return value is `true` when there are none.
- **CloudTrail accepts exactly one lookup attribute** per call. A second is refused: the
  value is dropped and an error lands in `Errors()`, which is why that check is not
  optional. Combine at most one of
  `WithEventName`/`WithUsername`/`WithResourceId`/`WithResourceType`/`WithReadOnly` with the
  time window, and filter the rest yourself.
- **`Hash()`** renders the query as a stable key (attributes sorted) — what the cached
  repository uses, so a builder is safe to reuse as a cache identity.
- `WithResource(resource)` derives the right attribute from a `service.ResourceInterface`.
