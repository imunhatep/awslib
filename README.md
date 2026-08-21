# AWS Library

This library provides a set of tools to interact with AWS services. 
The library also integrates with Prometheus for monitoring AWS API requests and errors.

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

## Installation
To install the library, use the following command:

```sh
go get github.com/imunhatep/awslib
```

## AWS Service list

Services whose entities implement the normalized `service.ResourceInterface`:

athena, autoscaling, batch, cloudfront, cloudtrail, cloudwatchlogs, dynamodb, ec2,
ecs, efs, eks, elb, emr, emrserverless, glue, iam, lambda, rds, route53, s3, secretmanager, sns,
sqs

Services exposing typed, service-specific APIs instead — cost figures, health events and price
lists are not resources, so they are fetched through their own repositories rather than the
`RepoProxy` fanout:

costexplorer, health, pricing

And **cloudcontrol**, which is not a service in the same sense: it is one generic repository that
serves *any* resource type through the AWS Cloud Control API, with no per-type code. Use it for types
the list above does not cover — see
[Cloud Control: any resource type, without a repository](#cloud-control-any-resource-type-without-a-repository).

## Code Generation

The library provides code generation tools to bootstrap AWS service clients and repositories:

```sh
# Generate cached repositories for all services
go run cmd/generate-cached/main.go

# Generate service options and configurations
go run cmd/generate-options/main.go

# Register entity types with encoding/gob so cached repositories can serialize them
go run cmd/generate-gob/main.go
```

Run `generate-gob` after adding or removing an entity type — the cache handlers serialize with
`encoding/gob`, and a missing registration surfaces as a cache write failure rather than a compile
error. When *removing* an entity, delete its `gob_register_gen.go` before regenerating: the generators
load packages with `go/packages`, so a stale reference to a deleted type makes the package
uncompilable and the generators then skip it silently.

## Usage

There are 2 distinct approaches provided by this library:
1. **AWS Provider**: Direct access to AWS services, allowing connection to multiple regions and accounts simultaneously.
2. **Service Repositories**: High-level abstraction for fetching AWS resources with built-in caching.

### Approach 1: AWS Provider

The provider focuses on managing clients across multiple accounts and regions efficiently. It lives at
`provider/v3` and is conventionally imported as `v3`; that is just the package path, not a choice
between provider versions — it is the only client provider the library ships.

#### Basic Usage (Single Region/Account)
```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
)

func main() {
    ctx := context.Background()

    // Create a basic v3 client
    client, err := v3.NewClient(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Use EC2 service
    ec2Client := ec2.GetClient(client)
    instances, err := ec2Client.DescribeInstances(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("EC2 instances found: %d\n", len(instances.Reservations))
}
```

#### Multiple Regions and Accounts
To work with multiple accounts, you typically assume roles. The `ClientPool` manages these clients for you.

```go
package main

import (
    "context"
    "fmt"

    ptypes "github.com/imunhatep/awslib/provider/types"
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
)

func ExampleMultiRegionAccount() error {
    ctx := context.Background()

    // 1. Create client builder
    clientBuilder := v3.NewClientBuilder(ctx)

    // 2. Define assumable roles for cross-account access
    assumableRoles := map[ptypes.AwsAccountID]ptypes.RoleArn{
        "123456789012": "arn:aws:iam::123456789012:role/awslib-assumed1",
        "987654321098": "arn:aws:iam::987654321098:role/awslib-assumed2",
    }

    // 3. Create client pool
    clientPool := v3.NewClientPool(ctx, clientBuilder, assumableRoles)

    // 4. Get clients for specific regions
    awsRegions := []ptypes.AwsRegion{"us-east-1", "eu-central-1"}
    clients, err := clientPool.GetClients(awsRegions...)
    if err != nil {
        return err
    }

    // 5. Iterate over clients (each represents a unique account+region combination)
    for _, client := range clients {
        fmt.Printf("Client for account %s in region %s\n",
            client.GetAccountID(), client.GetRegion())

        // Use services with this client
        ec2Client := ec2.GetClient(client)
        instances, err := ec2Client.DescribeInstances(ctx, nil)
        if err != nil {
            return err
        }
        fmt.Printf("Found %d reservations\n", len(instances.Reservations))
    }

    return nil
}
```

### Approach 2: Service Repositories

Service repositories provide a higher-level interface `ResourceInterface` and `EntityInterface` to interact with AWS resources, along with caching capabilities.

#### AWS Resources Interface
The common interface for all resources:
```go
package service

import (
    "github.com/aws/aws-sdk-go-v2/aws/arn"
    cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
    ptypes "github.com/imunhatep/awslib/provider/types"
    "time"
)

type ResourceInterface interface {
    GetAccountID() ptypes.AwsAccountID
    GetRegion() ptypes.AwsRegion
    GetCreatedAt() time.Time
    GetArn() string
    GetId() string
    GetIdOrArn() string
    GetType() cfg.ResourceType
    GetTags() map[string]string
}
```

#### Fetching Resources with Caching
The repositories support pluggable caching (e.g., in-memory, file-based).

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/allegro/bigcache/v3"
    "github.com/imunhatep/awslib/cache"
    "github.com/imunhatep/awslib/cache/handlers"
    "github.com/imunhatep/awslib/service/ec2"
    v3 "github.com/imunhatep/awslib/provider/v3"
)

func ExampleRepositoryUsage() error {
    ctx := context.Background()

    // Setup caching (optional)
    cacheTtl := 300 * time.Second
    bigCache, _ := bigcache.New(ctx, bigcache.DefaultConfig(cacheTtl))
    inMem := handlers.NewInMemory(bigCache)
    dataCache := cache.NewDataCache().WithHandlers(inMem)

    // Create client
    client, err := v3.NewClient(ctx)
    if err != nil {
        return err
    }

    // Create cached repository
    // This wrapper handles caching logic automatically
    repo := ec2.NewCachedEc2Repository(ctx, client, dataCache)

    // Fetch instances (returns []ec2.Entity)
    // First call hits AWS API, subsequent calls within TTL hit cache
    instances, err := repo.ListInstancesAll()
    if err != nil {
        return err
    }

    for _, instance := range instances {
        fmt.Printf("Instance: %s (ID: %s)\n", instance.GetName(), instance.GetId())
    }

    return nil
}
```

#### Using RepoProxy for Cross-Account Fetching
`RepoProxy` combines `ClientPool` and `Repositories` to fetch resources across multiple accounts and regions in waiting structure.

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/aws/aws-sdk-go-v2/service/configservice/types"
    "github.com/imunhatep/awslib/proxy"
    "github.com/imunhatep/awslib/resources"
    // ... imports for v3, cache, etc.
)

func ExampleRepoProxy(ctx context.Context, clients []*v3.Client, dataCache *cache.DataCache) {

  // Create proxy pool
  proxyPool := proxy.NewRepoProxyPool(ctx, clients)
  
  // Attach cache
  proxyPool.WithCache(dataCache)

  // Fetch specific resource type across all configured accounts/regions
  resourceType := types.ResourceTypeInstance
  
  // The Provider runs the fetchers in parallel
  awsProvider := resources.NewProvider(resourceType, proxyPool.List(resourceType)...)
  reader := awsProvider.Run()

  for _, resource := range reader.Read() {
    fmt.Printf("Resource: %s | Account: %s | Region: %s\n", 
        resource.GetId(), resource.GetAccountID(), resource.GetRegion())
  }
}
```

#### Cloud Control: any resource type, without a repository

`RepoProxy.FindAll` can only serve a resource type that someone has written a repository for. The
**Cloud Control** path removes that constraint: one generic code path serves any `AWS::Service::Resource`
type whose CloudFormation registry entry implements the `LIST` handler, with no per-type Go code at all.

Results come back as `cloudcontrol.Resource`, which implements the same `service.ResourceInterface` as
every typed entity — so it flows through the identical pipeline. The resource's own fields stay in
`Attributes` as the JSON object Cloud Control returned:

```go
repo := cloudcontrol.NewCloudControlRepository(ctx, client)

// Any type, same call. Add .WithCache(dataCache) for the usual caching.
streams, err := repo.ListResourcesByType("AWS::Kinesis::Stream")
if err != nil {
    return err
}

for _, r := range streams {
    fmt.Println(r.GetId(), r.GetName(), r.GetTags(), r.GetAttributes())
}
```

To fan out across accounts and regions, swap the pool constructor — `GenericRepoProxy` satisfies
`RepoProxyInterface`, so the `Provider`, the cache and the global-type handling all work unchanged:

```go
// instead of proxy.NewRepoProxyPool(ctx, clients)
proxyPool := proxy.NewGenericRepoProxyPool(ctx, clients, false).WithCache(dataCache)

resourceType := types.ResourceType("AWS::Kinesis::Stream")
reader := resources.NewProvider(resourceType, proxyPool.List(resourceType)...).Run()
```

`RepoProxy.FindAllCC(resourceType)` is the same lookup with `FindAll`'s exact signature, for callers
that want to switch a single proxy over.

**Use it as a fallback, not a default.** Compared with a typed repository:

| | Typed repository | Cloud Control `Resource` |
|---|---|---|
| Coverage | Only wired types | Any type with a `LIST` handler |
| Fields | Typed SDK struct | `map[string]interface{}` of the raw properties |
| ARN | Always | Only when the type exposes one as a property |
| Creation time | Usually | Never — Cloud Control reports none |
| Extra calls | None | `detailed` costs one `GetResource` per resource |

Two caveats worth knowing before you rely on it. Types that implement only `READ`, and nested types
needing a parent identifier (`AWS::ApiGateway::Method`), return an error rather than an empty list —
pass a `ResourceModel` via `ListResourcesByInput` for the latter. And the `detailed` flag exists
because some types' `LIST` returns identifiers only (S3 buckets) while others return full properties
(EC2 instances); there is no way to know which without trying.

### Logging verbosity
Use this func example to set logging verbosity
```go
package internal

import (
	"github.com/imunhatep/awslib/provider/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"time"
)

func setLogLevel(level int) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})

	switch level {
	case 0:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 3:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 4:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}

```

## Monitoring
The library integrates with Prometheus to monitor AWS API requests and errors. Metrics are collected and can be visualized using Prometheus-compatible tools.