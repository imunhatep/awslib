# Provider: clients, accounts and regions

The provider manages AWS clients across accounts and regions. It lives at
`provider/v3` and is conventionally imported as `v3`; that is the package path, not a
choice between provider versions — it is the only client provider the library ships.

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

