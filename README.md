# AWS Library

This library provides a set of tools to interact with AWS services. 
The library also integrates with Prometheus for monitoring AWS API requests and errors.

## Installation

To install the library, use the following command:

```sh
go get github.com/imunhatep/awslib
```

## Usage

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

#### Use credentials to access account directly
```go
package main

import (
  "context"
  "github.com/allegro/bigcache/v3"
  "github.com/aws/aws-sdk-go-v2/service/configservice/types"
  "github.com/rs/zerolog/log"
  "github.com/imunhatep/awslib/cache"
  "github.com/imunhatep/awslib/cache/handlers"
  "github.com/imunhatep/awslib/gateway"
  "github.com/imunhatep/awslib/provider"
  ptypes "github.com/imunhatep/awslib/provider/types"
  "github.com/imunhatep/awslib/provider/v2"
  "github.com/imunhatep/awslib/resources"
)

type AwsClientPool interface {
  GetContext() context.Context
  GetClient(ptypes.AwsAccountID, ptypes.AwsRegion) (*v2.Client, error)
  GetClients(...ptypes.AwsRegion) ([]*v2.Client, error)
}

func NewClientPool() (AwsClientPool, error) {
  awsRegions := []ptypes.AwsRegion{ "us-east-1", "us-west-2" }

  ctx := context.Background()
  clientBuilder, err := v2.NewClientBuilder(ctx)
  if err != nil {
    return nil, err
  }
  
  localClientPool := provider.NewClientPool(ctx, clientBuilder)

  clients, err := localClientPool.GetClients(awsRegions...)
  if err != nil {
    return nil, err
  }

  for _, client := range clients {
    // Do something with the client per region
  }
  
  return localClientPool, nil
}
```

#### Use credentials to assume roles, e.g. cross-account access
```go
package main

import (
  "context"
  "github.com/allegro/bigcache/v3"
  "github.com/aws/aws-sdk-go-v2/service/configservice/types"
  "github.com/rs/zerolog/log"
  "github.com/imunhatep/awslib/cache"
  "github.com/imunhatep/awslib/cache/handlers"
  "github.com/imunhatep/awslib/gateway"
  "github.com/imunhatep/awslib/metrics"
  "github.com/imunhatep/awslib/provider"
  ptypes "github.com/imunhatep/awslib/provider/types"
  "github.com/imunhatep/awslib/provider/v2"
  "github.com/imunhatep/awslib/resources"
)

type AwsClientPool interface {
  GetContext() context.Context
  GetClient(ptypes.AwsAccountID, ptypes.AwsRegion) (*v2.Client, error)
  GetClients(...ptypes.AwsRegion) ([]*v2.Client, error)
}

func NewClientPool() (AwsClientPool, error) {
  // enable metrics, optional
  metrics.InitMetrics(metrics.AwslibSubsystem)
  
  awsRegions := []ptypes.AwsRegion{ "us-east-1", "us-west-2" }

  ctx := context.Background()
  clientBuilder, err := v2.NewClientBuilder(ctx)
  if err != nil {
    return nil, err
  }
  
  assumedClientPool := v2.NewClientPool(ctx, clientBuilder)

  clients, err := assumedClientPool.GetClients(awsRegions...)
  if err != nil {
    return nil, err
  }

  for _, client := range clients {
    // Do something with the client per region
  }
  
  return assumedClientPool, nil
}
```

##### AWS IAM Role
AWS IAM Role example for wiring with IRSA
```json
{
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
              "iam:GenerateCredentialReport",
              "iam:GenerateServiceLastAccessedDetails",
              "iam:Get*",
              "iam:List*",
              "iam:SimulateCustomPolicy",
              "iam:SimulatePrincipalPolicy"
            ],
            "Resource": "*"
        },
        {
            "Action": [
                "sts:TagSession",
                "sts:AssumeRole"
            ],
            "Effect": "Allow",
            "Resource": [
                "arn:aws:iam::123456789012:role/awslib-assumed1",
                "arn:aws:iam::123456789013:role/awslib-assumed2",
                "arn:aws:iam::123456789014:role/awslib-assumed3"
            ]
        }
    ],
    "Version": "2012-10-17"
}
```

#### AWS Service RepoGateway
This structure helps fetching AWS resources from AWS services.
```go
package main

import (
  "context"
  "github.com/allegro/bigcache/v3"
  "github.com/aws/aws-sdk-go-v2/service/configservice/types"
  "github.com/rs/zerolog/log"
  "github.com/imunhatep/awslib/cache"
  "github.com/imunhatep/awslib/cache/handlers"
  "github.com/imunhatep/awslib/gateway"
  "github.com/imunhatep/awslib/provider"
  ptypes "github.com/imunhatep/awslib/provider/types"
  "github.com/imunhatep/awslib/provider/v2"
  "github.com/imunhatep/awslib/resources"
    "github.com/imunhatep/awslib/service"
  "fmt"
  "time"
)

type AwsClientPool interface {
  GetContext() context.Context
  GetClient(ptypes.AwsAccountID, ptypes.AwsRegion) (*v2.Client, error)
  GetClients(...ptypes.AwsRegion) ([]*v2.Client, error)
}

func InitRepo() error {
  awsRegions := []ptypes.AwsRegion{ "us-east-1", "us-west-2" }
  
  clientPool, _ := NewClientPool()

  clients, err := clientPool.GetClients(awsRegions...)
  if err != nil {
    return err 
  }
    
  ctx := context.Background()
  gatewayPool := gateway.NewRepoGatewayPool(ctx, clients)
  
  // enable resource cache
  cacheTtl := 300 * time.Second
  
  bigCache, _ := bigcache.New(ctx, bigcache.DefaultConfig(cacheTtl))
  inMem := handlers.NewInMemory(bigCache)

  inFile, _ := handlers.NewInFile("/tmp", cacheTtl)

  dataCache := cache.NewDataCache().WithHandlers(inMem, inFile)

  // enable resource cache
  gatewayPool.WithCache(dataCache)

  resourceType := types.ResourceTypeInstance
  awsProvider := resources.NewProvider(resourceType, gatewayPool.List(resourceType)...)
  reader := awsProvider.Run()
  
  // resource service.EntityInterface
  for _, resource := range reader.Read() {
    fmt.Println(resource.GetArn())
    }
  
  return nil
}
```


### AWS EC2

#### List All Instances

To list all EC2 insatnces:

```go
package main

import (
  "context"
  "fmt"
  "github.com/imunhatep/awslib/service/ec2"
)

func main() {
  ctx := context.Background()
  
  client := NewAwsClient() // Assume NewAwsClient is a function that returns an initialized AWS client
  repo := ec2.NewEc2Repository(ctx, client)

  instances, err := repo.ListInstancesAll()
  if err != nil {
    fmt.Println("Error listing instances:", err)
    return
  }

  for _, instance := range instances {
    fmt.Println("EC2 ID:", instance.GetID())
  }
}
```

#### Get Volume Tags
To get tags for a specific EC2 volume:
```go
package main

import (
  "context"
  "fmt"
  "github.com/imunhatep/awslib/service/ec2"
)

func main() {
  ctx := context.Background()
  client := NewAwsClient() // Assume NewAwsClient is a function that returns an initialized AWS client
  repo := ec2.NewEc2Repository(ctx, client)

  volumeID := "vol-0123456789abcdef0"
  volume := ec2.Volume{ID: volumeID}

  tags := volume.GetTags()
  for key, value := range tags {
    fmt.Printf("Key: %s, Value: %s\n", key, value)
  }
}
```

### S3 (Simple Storage Service)

#### List All Buckets
To list all S3 buckets:
```go
package main

import (
  "context"
  "fmt"
  "github.com/imunhatep/awslib/service/s3"
)

func main() {
  ctx := context.Background()
  client := NewAwsClient() // Assume NewAwsClient is a function that returns an initialized AWS client
  repo := s3.NewS3Repository(ctx, client)
  
  buckets, err := repo.ListBucketsAll()
  if err != nil {
    fmt.Println("Error listing buckets:", err)
    return
  }
  
  for _, bucket := range buckets {
    fmt.Println("Bucket Name:", bucket.GetName())
    
    for key, value := range bucket.GetTags() {
      fmt.Printf("Key: %s, Value: %s\n", key, value)
    }
  }
}
```

#### Get Bucket Tags
To get tags for a specific S3 bucket:
```go
package main

import (
  "context"
  "fmt"
  "github.com/imunhatep/awslib/service/s3"
)

func main() {
  ctx := context.Background()
  client := NewAwsClient() // Assume NewAwsClient is a function that returns an initialized AWS client
  repo := s3.NewS3Repository(ctx, client)

  bucketName := "my-bucket"
  bucket := s3.Bucket{Name: &bucketName}
  
  tags, err := repo.GetTags(bucket)
  if err != nil {
   fmt.Println("Error getting bucket tags:", err)
   return
  }

  for key, value := range tags {
    fmt.Printf("Key: %s, Value: %s\n", key, value)
  }
}
```

## Monitoring
The library integrates with Prometheus to monitor AWS API requests and errors. Metrics are collected and can be visualized using Prometheus-compatible tools.

## License
This library is licensed under the MIT License. See the `LICENSE` file for more details.