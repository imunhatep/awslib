# AWS Library

This library provides a set of tools to interact with AWS services. 
The library also integrates with Prometheus for monitoring AWS API requests and errors.

## Installation
To install the library, use the following command:

```sh
go get github.com/imunhatep/awslib
```

## AWS Service list
List of AWS Services that have normalized interface EntityInterface{}
 - athena
 - autoscaling
 - batch
 - cloudcontrol
 - cloudtrail
 - cloudwatchlogs
 - dynamodb
 - ec2
 - ecs
 - efs
 - eks
 - elb
 - emr
 - emrserverless
 - health
 - lambda
 - pricing
 - rds
 - s3
 - secretmanager
 - sns
 - sqs

## Code Generation

The library provides code generation tools to bootstrap AWS service clients and repositories:

```sh
# Generate cached repositories for all services
go run cmd/generate-cached/main.go

# Generate service options and configurations
go run cmd/generate-options/main.go
```

## Usage

There are 2 distinct approaches provided by this library:
1. **AWS Provider v3**: Direct access to AWS services, allowing connection to multiple regions and accounts simultaneously.
2. **Service Repositories**: High-level abstraction for fetching AWS resources with built-in caching.

### Approach 1: AWS Provider v3

The v3 provider focuses on managing clients across multiple accounts and regions efficiently.

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