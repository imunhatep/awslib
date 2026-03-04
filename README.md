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
### AWS Resources interface
The library provides a set of interfaces to interact with AWS resources. The interfaces are defined as follows:
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

type EntityInterface interface {
    ResourceInterface
    GetName() string
    GetTags() map[string]string
    GetTagValue(string) string
}
```

### AWS Client Pool
Initialize AWS client pool.


### Basic Client Usage

#### Direct service access with v3 client
```go
package main

import (
    "context"
    "fmt"
    "log"
    
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
    "github.com/imunhatep/awslib/provider/v3/clients/s3"
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
    
    // Use S3 service
    s3Client := s3.GetClient(client)
    buckets, err := s3Client.ListBuckets(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("S3 buckets found: %d\n", len(buckets.Buckets))
}
```

#### v3 Client Pool for cross-account access
```go
package main

import (
    "context"
    "fmt"
    
    ptypes "github.com/imunhatep/awslib/provider/types"
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
)

func NewV3ClientPool() (*v3.ClientPool, error) {
    ctx := context.Background()
    
    // Create client builder
    clientBuilder := v3.NewClientBuilder(ctx)
    
    // Define assumable roles for cross-account access
    assumableRoles := map[ptypes.AwsAccountID]ptypes.RoleArn{
        "123456789012": "arn:aws:iam::123456789012:role/awslib-assumed1",
        "123456789013": "arn:aws:iam::123456789013:role/awslib-assumed2",
    }
    
    // Create client pool
    clientPool := v3.NewClientPool(ctx, clientBuilder, assumableRoles)
    
    return clientPool, nil
}

func ExampleV3ClientPool() error {
    clientPool, err := NewV3ClientPool()
    if err != nil {
        return err
    }
    
    // Get clients for specific regions
    awsRegions := []ptypes.AwsRegion{"us-east-1", "us-west-2"}
    clients, err := clientPool.GetClients(awsRegions...)
    if err != nil {
        return err
    }
    
    for _, client := range clients {
        fmt.Printf("Client for account %s in region %s\n", 
            client.GetAccountID(), client.GetRegion())
        
        // Use services with this client
        ec2Client := ec2.GetClient(client)
        instances, err := ec2Client.DescribeInstances(context.Background(), nil)
        if err != nil {
            return err
        }
        fmt.Printf("Found %d reservations\n", len(instances.Reservations))
    }
    
    return nil
}
```

#### v3 Client with custom providers
```go
package main

import (
    "context"
    
    "github.com/aws/aws-sdk-go-v2/config"
    v3 "github.com/imunhatep/awslib/provider/v3"
)

func NewV3ClientWithCustomConfig() (*v3.Client, error) {
    ctx := context.Background()
    
    // Get default providers
    providers, err := v3.DefaultAwsClientProviders()
    if err != nil {
        return nil, err
    }
    
    // Add custom region
    providers = append(providers, config.WithRegion("eu-west-1"))
    
    // Create client with custom providers
    client, err := v3.NewClient(ctx, providers...)
    if err != nil {
        return nil, err
    }
    
    return client, nil
}
```

### v3 Service Client Architecture

The v3 provider uses a service client pattern where each AWS service is accessed through dedicated client packages. This provides type safety and efficient caching:

#### Available Service Clients

The following service clients are available in the `provider/v3/clients/` package:

- `accessanalyzer`, `acm`, `apigateway`, `athena`, `autoscaling`, `batch`
- `cloudcontrol`, `cloudformation`, `cloudtrail`, `cloudwatch`, `cloudwatchlogs`
- `configservice`, `costexplorer`, `dynamodb`, `ec2`, `ecs`, `efs`, `eks`
- `elasticache`, `elasticloadbalancingv2`, `emr`, `emrserverless`, `glue`
- `health`, `iam`, `lambda`, `pricing`, `rds`, `route53`, `s3`, `s3control`
- `secretsmanager`, `securityhub`, `servicequotas`, `sns`, `sqs`, `ssm`
- And many more...

#### Service Client Usage Pattern

Each service client follows the same pattern:

```go
package main

import (
    "context"
    
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/servicename"
)

func main() {
    ctx := context.Background()
    client, _ := v3.NewClient(ctx)
    
    // Get service client - automatically cached
    serviceClient := servicename.GetClient(client)
    
    // Use the service client (standard AWS SDK v2 interface)
    result, err := serviceClient.SomeOperation(ctx, &input{})
    // ... handle result
}
```

#### Service Client Features

1. **Automatic Caching**: Service clients are automatically cached per v3.Client instance
2. **Thread Safe**: All client operations are thread-safe
3. **Standard AWS SDK v2**: Each service client returns the standard AWS SDK v2 client
4. **Configuration Options**: Support for AWS SDK v2 option functions

#### Multiple Services Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
    "github.com/imunhatep/awslib/provider/v3/clients/lambda"
    "github.com/imunhatep/awslib/provider/v3/clients/s3"
    "github.com/imunhatep/awslib/provider/v3/clients/iam"
)

func ExampleMultipleServices() error {
    ctx := context.Background()
    
    // Create v3 client
    client, err := v3.NewClient(ctx)
    if err != nil {
        return err
    }
    
    // Use multiple services
    ec2Client := ec2.GetClient(client)
    lambdaClient := lambda.GetClient(client)
    s3Client := s3.GetClient(client)
    iamClient := iam.GetClient(client)
    
    // Get EC2 regions
    regions, err := ec2Client.DescribeRegions(ctx, nil)
    if err != nil {
        return err
    }
    fmt.Printf("Available regions: %d\n", len(regions.Regions))
    
    // List Lambda functions
    functions, err := lambdaClient.ListFunctions(ctx, nil)
    if err != nil {
        return err
    }
    fmt.Printf("Lambda functions: %d\n", len(functions.Functions))
    
    // List S3 buckets
    buckets, err := s3Client.ListBuckets(ctx, nil)
    if err != nil {
        return err
    }
    fmt.Printf("S3 buckets: %d\n", len(buckets.Buckets))
    
    // List IAM users
    users, err := iamClient.ListUsers(ctx, nil)
    if err != nil {
        return err
    }
    fmt.Printf("IAM users: %d\n", len(users.Users))
    
    return nil
}
```

### AWS Service RepoProxy
The RepoProxy provides a higher-level interface for fetching AWS resources from AWS services with built-in caching support.

```go
package main

import (
  "context"
  "fmt"
  "time"
  
  "github.com/allegro/bigcache/v3"
  "github.com/aws/aws-sdk-go-v2/service/configservice/types"
  
  "github.com/imunhatep/awslib/cache"
  "github.com/imunhatep/awslib/cache/handlers"
  "github.com/imunhatep/awslib/proxy"
  "github.com/imunhatep/awslib/provider/v3"
  ptypes "github.com/imunhatep/awslib/provider/types"
  "github.com/imunhatep/awslib/resources"
)

func InitRepoProxy() error {
  ctx := context.Background()
  awsRegions := []ptypes.AwsRegion{"us-east-1", "us-west-2"}
  
  // Create v3 client pool
  clientBuilder := v3.NewClientBuilder(ctx)
  assumableRoles := map[ptypes.AwsAccountID]ptypes.RoleArn{
    "123456789012": "arn:aws:iam::123456789012:role/awslib-assumed1",
  }
  clientPool := v3.NewClientPool(ctx, clientBuilder, assumableRoles)
  
  clients, err := clientPool.GetClients(awsRegions...)
  if err != nil {
    return err 
  }
    
  // Create proxy pool
  proxyPool := proxy.NewRepoProxyPool(ctx, clients)
  
  // Enable resource caching (optional)
  cacheTtl := 300 * time.Second
  
  bigCache, _ := bigcache.New(ctx, bigcache.DefaultConfig(cacheTtl))
  inMem := handlers.NewInMemory(bigCache)
  inFile, _ := handlers.NewInFile("/tmp", cacheTtl)
  dataCache := cache.NewDataCache().WithHandlers(inMem, inFile)
  
  proxyPool.WithCache(dataCache)

  // Fetch resources
  resourceType := types.ResourceTypeInstance
  awsProvider := resources.NewProvider(resourceType, proxyPool.List(resourceType)...)
  reader := awsProvider.Run()
  
  for _, resource := range reader.Read() {
    fmt.Printf("Resource ARN: %s\n", resource.GetArn())
  }
  
  return nil
}
```

### Repository Caching

The library supports caching of AWS API responses to reduce API calls and improve performance:

#### In-Memory Caching
```go
package main

import (
    "context"
    "time"
    
    "github.com/allegro/bigcache/v3"
    "github.com/imunhatep/awslib/cache"
    "github.com/imunhatep/awslib/cache/handlers"
    "github.com/imunhatep/awslib/service/ec2"
    v3 "github.com/imunhatep/awslib/provider/v3"
)

func ExampleInMemoryCache() error {
    ctx := context.Background()
    
    // Create bigcache
    cacheTtl := 300 * time.Second
    bigCache, err := bigcache.New(ctx, bigcache.DefaultConfig(cacheTtl))
    if err != nil {
        return err
    }
    
    // Create cache handler
    inMem := handlers.NewInMemory(bigCache)
    dataCache := cache.NewDataCache().WithHandlers(inMem)
    
    // Create client
    client, err := v3.NewClient(ctx)
    if err != nil {
        return err
    }
    
    // Use cached repository
    repo := ec2.NewCachedEc2Repository(ctx, client, dataCache)
    
    // First call hits AWS API
    instances, err := repo.ListInstancesAll()
    if err != nil {
        return err
    }
    
    // Second call uses cached result
    instances, err = repo.ListInstancesAll()
    if err != nil {
        return err
    }
    
    return nil
}
```

#### File System Caching
```go
package main

import (
    "context"
    "time"
    
    "github.com/imunhatep/awslib/cache"
    "github.com/imunhatep/awslib/cache/handlers"
    "github.com/imunhatep/awslib/service/s3"
    v3 "github.com/imunhatep/awslib/provider/v3"
)

func ExampleFileSystemCache() error {
    ctx := context.Background()
    
    // Create file cache
    cacheTtl := 300 * time.Second
    cacheDir := "/tmp/awslib-cache"
    inFile, err := handlers.NewInFile(cacheDir, cacheTtl)
    if err != nil {
        return err
    }
    
    dataCache := cache.NewDataCache().WithHandlers(inFile)
    
    // Create client
    client, err := v3.NewClient(ctx)
    if err != nil {
        return err
    }
    
    // Use cached repository
    repo := s3.NewCachedS3Repository(ctx, client, dataCache)
    
    // API results are cached to file system
    buckets, err := repo.ListBucketsAll()
    if err != nil {
        return err
    }
    
    return nil
}
```

#### Combined Caching (Memory + File)
```go
package main

import (
    "context"
    "time"
    
    "github.com/allegro/bigcache/v3"
    "github.com/imunhatep/awslib/cache"
    "github.com/imunhatep/awslib/cache/handlers"
    "github.com/imunhatep/awslib/service/iam"
    v3 "github.com/imunhatep/awslib/provider/v3"
)

func ExampleCombinedCache() error {
    ctx := context.Background()
    cacheTtl := 300 * time.Second
    
    // Create multiple cache handlers
    bigCache, _ := bigcache.New(ctx, bigcache.DefaultConfig(cacheTtl))
    inMem := handlers.NewInMemory(bigCache)
    inFile, _ := handlers.NewInFile("/tmp/awslib-cache", cacheTtl)
    
    // Combine handlers - tries memory first, then file, then API
    dataCache := cache.NewDataCache().WithHandlers(inMem, inFile)
    
    client, err := v3.NewClient(ctx)
    if err != nil {
        return err
    }
    
    // Use cached repository with multi-level caching
    repo := iam.NewCachedIamRepository(ctx, client, dataCache)
    
    users, err := repo.ListUsersAll()
    if err != nil {
        return err
    }
    
    return nil
}
```

### Code Generation

Generate cached repository wrappers for all AWS services:

```go
package main

import (
    "fmt"
    "os"
    "os/exec"
)

// Generate cached repositories
func generateCachedRepos() error {
    cmd := exec.Command("go", "run", "cmd/generate-cached/main.go")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to generate cached repos: %w", err)
    }
    
    return nil
}

// Generate service options
func generateOptions() error {
    cmd := exec.Command("go", "run", "cmd/generate-options/main.go")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to generate options: %w", err)
    }
    
    return nil
}
```

### AWS EC2

#### List All Instances

To list all EC2 instances using v3 provider:

```go
package main

import (
    "context"
    "fmt"
    
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
)

func main() {
    ctx := context.Background()
    
    client, err := v3.NewClient(ctx)
    if err != nil {
        fmt.Println("Error creating client:", err)
        return
    }
    
    ec2Client := ec2.GetClient(client)
    result, err := ec2Client.DescribeInstances(ctx, nil)
    if err != nil {
        fmt.Println("Error listing instances:", err)
        return
    }
    
    for _, reservation := range result.Reservations {
        for _, instance := range reservation.Instances {
            fmt.Printf("EC2 Instance ID: %s, State: %s\n", 
                *instance.InstanceId, 
                string(instance.State.Name))
        }
    }
}
```


#### Get Volume Tags
To get tags for a specific EC2 volume using the v3 provider and cached repository:

```go
package main

import (
  "context"
  "fmt"
  
  v3 "github.com/imunhatep/awslib/provider/v3"
  "github.com/imunhatep/awslib/service/ec2"
  "github.com/imunhatep/awslib/cache"
  "github.com/imunhatep/awslib/cache/handlers"
  "github.com/allegro/bigcache/v3"
  "time"
)

func main() {
  ctx := context.Background()
  
  // Create client and cache
  client, _ := v3.NewClient(ctx)
  
  cacheTtl := 300 * time.Second
  bigCache, _ := bigcache.New(ctx, bigcache.DefaultConfig(cacheTtl))
  inMem := handlers.NewInMemory(bigCache)
  dataCache := cache.NewDataCache().WithHandlers(inMem)
  
  // Create cached repository
  repo := ec2.NewCachedEc2Repository(ctx, client, dataCache)

  volumeID := "vol-0123456789abcdef0"
  
  // Get all volumes and find the one we want
  volumes, err := repo.ListVolumesAll()
  if err != nil {
    fmt.Printf("Error listing volumes: %v\n", err)
    return
  }
  
  for _, volume := range volumes {
    if volume.GetId() == volumeID {
      tags := volume.GetTags()
      for key, value := range tags {
        fmt.Printf("Key: %s, Value: %s\n", key, value)
      }
      break
    }
  }
}
```

### S3 (Simple Storage Service)

#### List All Buckets
To list all S3 buckets using v3 provider:

```go
package main

import (
    "context"
    "fmt"
    
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/provider/v3/clients/s3"
)

func main() {
    ctx := context.Background()
    
    client, err := v3.NewClient(ctx)
    if err != nil {
        fmt.Println("Error creating client:", err)
        return
    }
    
    s3Client := s3.GetClient(client)
    result, err := s3Client.ListBuckets(ctx, nil)
    if err != nil {
        fmt.Println("Error listing buckets:", err)
        return
    }
    
    for _, bucket := range result.Buckets {
        fmt.Printf("Bucket Name: %s, Created: %s\n", 
            *bucket.Name, 
            bucket.CreationDate.String())
    }
}
```

#### Get Bucket Tags
To get tags for a specific S3 bucket using v3 provider:

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/aws/aws-sdk-go-v2/service/s3"
    v3 "github.com/imunhatep/awslib/provider/v3"
    s3client "github.com/imunhatep/awslib/provider/v3/clients/s3"
)

func main() {
    ctx := context.Background()
    
    client, err := v3.NewClient(ctx)
    if err != nil {
        fmt.Println("Error creating client:", err)
        return
    }
    
    s3Client := s3client.GetClient(client)
    bucketName := "my-bucket"
    
    tagsResult, err := s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
        Bucket: &bucketName,
    })
    if err != nil {
        fmt.Println("Error getting bucket tags:", err)
        return
    }
    
    for _, tag := range tagsResult.TagSet {
        fmt.Printf("Key: %s, Value: %s\n", *tag.Key, *tag.Value)
    }
}
```


## Monitoring
The library integrates with Prometheus to monitor AWS API requests and errors. Metrics are collected and can be visualized using Prometheus-compatible tools.