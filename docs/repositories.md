# Service repositories

Repositories are the higher-level interface: typed AWS resources behind one
`ResourceInterface`, with pluggable caching and cross-account fanout. For types no
repository covers, see [cloudcontrol.md](cloudcontrol.md).

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
    GetType() cfg.ResourceType
    GetArn() string
    GetId() string
    GetIdOrArn() string
    GetName() string
    GetCreatedAt() time.Time
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

    // Create the repository and wrap it with the cache.
    // WithCache is generated per service; it namespaces entries as
    // "<accountID>:<region>:ec2" and only writes on success.
    repo := ec2.NewEc2Repository(ctx, client).WithCache(dataCache)

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

