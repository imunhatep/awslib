# Provider: clients, accounts, regions and IAM

The provider manages AWS clients across accounts and regions. It lives at
`provider/v3` and is conventionally imported as `v3`; that is the package path, not a
choice between provider versions — it is the only client provider the library ships.

- [The pieces](#the-pieces)
- [A single client](#a-single-client)
- [Client builder: local, assumed and default clients](#client-builder-local-assumed-and-default-clients)
- [Regions](#regions)
- [Single account, many regions](#single-account-many-regions-providerclientpool)
- [Many accounts, many regions](#many-accounts-many-regions-v3clientpool)
- [Discovering assumable roles](#discovering-assumable-roles)
- [Using any AWS service client](#using-any-aws-service-client) — [the 56 ready-made clients](#the-56-ready-made-clients), [service packages](#service-packages-repositories-on-top-of-those-clients)
- [IAM policies](#iam-policies) — caller, trust policy, read permissions
- [Failure caching and region timeouts](#failure-caching-and-region-timeouts)
- [Custom endpoints (LocalStack, S3-compatible)](#custom-endpoints-localstack-s3-compatible)
- [Logging verbosity](#logging-verbosity)

## The pieces

| Type | What it is |
|---|---|
| `v3.Client` | one `aws.Config` for exactly one account **and** one region, plus a cache of instantiated SDK service clients |
| `v3.ClientBuilder` | makes `v3.Client`s — default, local (default credentials) or assumed-role — and caches assumed-role credentials per role |
| `provider.ClientPool` | one account (whatever the default credential chain resolves to) × many regions |
| `v3.ClientPool` | many accounts (via assumed roles) × many regions, built concurrently |
| `provider/types` | `AwsAccountID`, `AwsRegion`, `RoleArn`, and the region table |

Two things about `v3.Client` are worth knowing before anything else:

- **It is per-region.** There is no "switch region" call; a second region means a second client.
- **Constructing one calls STS.** `NewClient` resolves the account ID via
  `sts:GetCallerIdentity` before returning, so it fails immediately on missing credentials
  or a region the account has not enabled, rather than later at the first API call.

## A single client

```go
ctx := context.Background()

// Region comes from the usual chain: AWS_REGION, then the shared profile.
client, err := v3.NewClient(ctx)
if err != nil {
    return err
}

fmt.Println(client.GetAccountID(), client.GetRegion())
```

Pin the region and pick up the library's default retry/credential wiring explicitly:

```go
providers, err := v3.DefaultAwsClientProviders(config.WithRegion("eu-central-1"))
if err != nil {
    return err
}

client, err := v3.NewClient(ctx, providers...)
```

`v3.DefaultAwsClientProviders(extra...)` builds the config chain the library expects and
appends whatever you pass. It sets 5 retry attempts with a 3s max backoff
(`v3.AwsRetryAttempts`, `v3.AwsRetryMaxBackoffDelay`) and then reads the environment:

| Environment | Effect |
|---|---|
| `AWS_PROFILE` | `config.WithSharedConfigProfile(...)` — the profile from `~/.aws/config` |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` (+ `AWS_SESSION_TOKEN`) | static credentials |
| `AWS_ROLE_ARN` (+ `AWS_WEB_IDENTITY_TOKEN_FILE`) | web identity, session name `aws_reporting@$HOSTNAME` — the EKS/IRSA path |

Options are applied in order and the last one wins, so anything you pass overrides the
defaults. `NewClient` takes plain `config.LoadOptionsFunc` values, so every option in
`aws-sdk-go-v2/config` is available — see `provider/example_test.go` for a catalogue
(`WithSharedConfigFiles`, `WithCredentialsCacheOptions`, `WithHTTPClient`,
`WithAssumeRoleCredentialOptions` for MFA, and so on).

## Client builder: local, assumed and default clients

`ClientBuilder` is the thing to hold onto in a long-running process; it is what the pools
are built from.

```go
providers, err := v3.DefaultAwsClientProviders()
if err != nil {
    return err
}

builder := v3.NewClientBuilder(ctx, providers...)
```

| Method | Credentials | Region | Cached |
|---|---|---|---|
| `builder.DefaultClient()` | default chain | **always `us-east-1`** (`types.DefaultAwsRegion`) | yes, one instance |
| `builder.LocalClient(region)` | default chain | as given | no — each call is a fresh client and a fresh STS call |
| `builder.AssumeClient(role, region)` | `sts:AssumeRole` on `role` | as given | credentials per role, client no |

`DefaultClient()` forces `us-east-1` regardless of `AWS_REGION`, because its job is to be
the identity that mints assumed-role credentials, not to do work. For work in a specific
region use `LocalClient(region)` — or a pool, which caches per account and region.

```go
// Same credentials, one client per region.
euClient, err := builder.LocalClient("eu-central-1")
usClient, err := builder.LocalClient("us-east-1")

// Another account, via a role, in a chosen region.
roClient, err := builder.AssumeClient(
    "arn:aws:iam::123456789012:role/awslib-reader",
    "eu-west-1",
)
```

Assumed-role credentials are cached per role ARN (`aws.NewCredentialsCache` over
`stscreds.NewAssumeRoleProvider`), so N regions in one account cost one `AssumeRole` call,
not N — and the credentials refresh themselves as they near expiry. The session name is
the SDK default (`aws-go-sdk-<nanos>`); a trust policy that conditions on
`sts:RoleSessionName` will not match it.

## Regions

Regions are `types.AwsRegion`, a string type. The library ships the region table rather
than calling `ec2:DescribeRegions`:

```go
types.DefaultAwsRegion                        // "us-east-1"
types.GetAwsRegionList()                      // []AwsRegion, every known region
types.GetAwsRegionStringList()                // []string, same set
types.GetAwsRegionDescription("eu-central-1") // "Europe (Frankfurt)", or "undefined"
```

Both `GetAwsRegionList` and `GetAwsRegionStringList` iterate a map, so the order is
**not** stable — sort before printing or comparing. The list is the commercial partition
only (no GovCloud, no China) and includes opt-in regions, which an account that has not
enabled them will reject; see [failure caching](#failure-caching-and-region-timeouts) for
what that costs and how it is contained.

## Single account, many regions (`provider.ClientPool`)

The top-level `provider.ClientPool` uses default credentials only — no role assumption.
This is the pool for a CLI running against whatever profile the operator has active.

```go
import (
    "github.com/imunhatep/awslib/provider"
    ptypes "github.com/imunhatep/awslib/provider/types"
    "github.com/imunhatep/awslib/provider/v3"
)

providers, err := v3.DefaultAwsClientProviders()
if err != nil {
    return err
}

pool := provider.NewClientPool(ctx, v3.NewClientBuilder(ctx, providers...)).
    WithFailureTTL(15 * time.Minute)

clients, err := pool.GetClients("eu-central-1", "us-east-1")
if err != nil {
    return err
}

for _, client := range clients {
    // one client per region, all in the same account
}
```

Regions that fail to produce a client are logged and skipped, so one disabled region does
not fail the call. `GetClients` returns an error only when the default credentials
themselves cannot resolve.

## Many accounts, many regions (`v3.ClientPool`)

`v3.ClientPool` takes a map of account ID → role ARN and builds the account × region
matrix concurrently.

```go
assumableRoles := map[ptypes.AwsAccountID]ptypes.RoleArn{
    "123456789012": "arn:aws:iam::123456789012:role/awslib-reader",
    "987654321098": "arn:aws:iam::987654321098:role/awslib-reader",
}

pool := v3.NewClientPool(ctx, builder, assumableRoles).
    WithFailureTTL(15 * time.Minute)

clients, err := pool.GetClients("eu-central-1", "us-east-1")
if err != nil {
    return err
}

for _, client := range clients {
    fmt.Printf("account %s region %s\n", client.GetAccountID(), client.GetRegion())
}
```

One role per account: the map is keyed by account ID, so two roles in the same account
cannot both be held. Pass an **empty map** and the pool falls back to default credentials
for its own account, which makes it a drop-in for `provider.ClientPool`.

### Scoping to one account

```go
accounts, err := pool.PoolAccountIDs()                                  // every account the pool can serve
clients, err := pool.GetAccountClients("123456789012", "eu-central-1")  // just that one
```

Do this here, not by filtering the result of `GetClients`: building a client assumes that
account's role, so by the time you hold a slice every account in the pool has already been
contacted. An account the pool cannot serve is an **error**, not an empty slice — a caller
that asked about the wrong account must not be told the account is empty.

`PoolAccountIDs()` is the one to validate against. `ListAccountIDs()` on
`provider.ClientPool` reports the accounts of clients *already created*, so it is empty
until the first `GetClients` call.

## Discovering assumable roles

`DiscoverAssumableRolesFromCurrentRole` reads the IAM policies attached to the role the
process is running as and extracts the role ARNs it is allowed to assume. It is the
IRSA/EKS convenience: the deployment's role is the single source of truth for which
accounts the tool covers, so adding an account is a policy change and not a code change.

It lives in the top-level `provider` package (some doc comments in the source say `v3.` —
that is stale):

```go
import "github.com/imunhatep/awslib/provider"

defaultClient, err := builder.DefaultClient()
if err != nil {
    return err
}

roles, err := provider.DiscoverAssumableRolesFromCurrentRole(ctx, defaultClient)
if err != nil {
    return err
}

pool := v3.NewClientPool(ctx, builder, roles)
```

It reads `iam:ListAttachedRolePolicies` → `iam:GetPolicy` → `iam:GetPolicyVersion` and
scans the default version of each document. That narrows what it can see, so the policy
has to be written for it:

- **The identity must be a role.** The role name is taken from the caller identity ARN, so
  an IAM user (`arn:aws:iam::…:user/bob`) sends `RoleName=bob` and fails.
- **Attached managed policies only.** Inline role policies, group policies and permission
  boundaries are not read.
- **The action must be exactly `sts:AssumeRole`.** `"sts:*"` and `"sts:AssumeRole*"` do not
  match, and `NotAction` is not understood.
- **`Resource` must list explicit role ARNs.** `"Resource": "*"` yields nothing usable and
  is skipped with a warning.
- **`Effect` is not checked.** A `Deny` statement naming role ARNs is read as if it granted
  them; keep assume-role denials out of the attached policies, or pass the map by hand.
- A policy that cannot be read or parsed is warned about and skipped, so partial discovery
  succeeds quietly — log at debug to see the per-role decisions.

When any of that does not hold, build the map explicitly. It is two lines either way.

## Using any AWS service client

Every wrapper in `provider/v3/clients/<name>` exposes one function, `GetClient`, which
returns a `*<name>.Client` from the aws-sdk-go-v2 package of the same name and caches it on
the `v3.Client`:

```go
import (
    awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/imunhatep/awslib/provider/v3/clients/ec2"
)

svc := ec2.GetClient(client)

out, err := svc.DescribeInstances(ctx, &awsec2.DescribeInstancesInput{})
```

Two caveats:

- **`optFns` are honoured only on the first call.** The cache is keyed by service name
  alone, so `GetClient(client, withSomething)` after a plain `GetClient(client)` returns the
  already-cached client and silently drops the options. Build the client directly from
  `client.Config()` when you need per-call options.
- **The returned client is shared** across everything holding that `v3.Client`. That is the
  point — one HTTP client, one credential cache — but do not mutate it.

### The 56 ready-made clients

Import path is `github.com/imunhatep/awslib/provider/v3/clients/<package>`; the returned
type is the SDK client of the same name.

| Package | SDK client | Package | SDK client |
|---|---|---|---|
| `accessanalyzer` | `*accessanalyzer.Client` | `route53` | `*route53.Client` |
| `acm` | `*acm.Client` | `route53domains` | `*route53domains.Client` |
| `apigateway` | `*apigateway.Client` | `s3` | `*s3.Client` |
| `athena` | `*athena.Client` | `s3control` | `*s3control.Client` |
| `autoscaling` | `*autoscaling.Client` | `s3outposts` | `*s3outposts.Client` |
| `batch` | `*batch.Client` | `savingsplans` | `*savingsplans.Client` |
| `cloudcontrol` | `*cloudcontrol.Client` | `secretsmanager` | `*secretsmanager.Client` |
| `cloudformation` | `*cloudformation.Client` | `securityhub` | `*securityhub.Client` |
| `cloudfront` | `*cloudfront.Client` | `servicecatalog` | `*servicecatalog.Client` |
| `cloudtrail` | `*cloudtrail.Client` | `servicediscovery` | `*servicediscovery.Client` |
| `cloudwatch` | `*cloudwatch.Client` | `servicequotas` | `*servicequotas.Client` |
| `cloudwatchlogs` | `*cloudwatchlogs.Client` | `ses` | `*ses.Client` |
| `configservice` | `*configservice.Client` | `sfn` | `*sfn.Client` |
| `costexplorer` | `*costexplorer.Client` | `shield` | `*shield.Client` |
| `dynamodb` | `*dynamodb.Client` | `signer` | `*signer.Client` |
| `ec2` | `*ec2.Client` | `sns` | `*sns.Client` |
| `ecs` | `*ecs.Client` | `sqs` | `*sqs.Client` |
| `efs` | `*efs.Client` | `ssm` | `*ssm.Client` |
| `eks` | `*eks.Client` | `storagegateway` | `*storagegateway.Client` |
| `elasticache` | `*elasticache.Client` | `swf` | `*swf.Client` |
| `elasticloadbalancingv2` | `*elasticloadbalancingv2.Client` | `synthetics` | `*synthetics.Client` |
| `emr` | `*emr.Client` | `timestreamwrite` | `*timestreamwrite.Client` |
| `emrserverless` | `*emrserverless.Client` | `transfer` | `*transfer.Client` |
| `glue` | `*glue.Client` | `waf` | `*waf.Client` |
| `health` | `*health.Client` | `wafregional` | `*wafregional.Client` |
| `iam` | `*iam.Client` | `wafv2` | `*wafv2.Client` |
| `lambda` | `*lambda.Client` | `pricing` | `*pricing.Client` |
| `rds` | `*rds.Client` | `resourcegroupstaggingapi` | `*resourcegroupstaggingapi.Client` |

STS has no wrapper because the client already owns one: `client.Sts()`, plus
`client.GetCallerIdentity(ctx)` which caches its answer.

### Service packages: repositories on top of those clients

A wrapper gives you the raw SDK client. `service/<name>` goes a step further — pagination
handled, Prometheus metrics emitted, `WithCache(dc)` available, and entities that implement
`service.ResourceInterface` where the concept fits. Prefer these over the raw client, and
drop to the raw client for the calls they do not cover.

Resource-shaped packages, reachable through `proxy.RepoProxy` and the parallel fetcher —
see [docs/repositories.md](repositories.md):

| Package | Constructor | Package | Constructor |
|---|---|---|---|
| `service/athena` | `NewAthenaRepository` | `service/glue` | `NewGlueRepository` |
| `service/autoscaling` | `NewAsgRepository` | `service/iam` | `NewIamRepository` |
| `service/batch` | `NewBatchRepository` | `service/lambda` | `NewLambdaRepository` |
| `service/cloudfront` | `NewCloudFrontRepository` | `service/rds` | `NewRdsRepository` |
| `service/cloudtrail` | `NewCloudTrailRepository` | `service/route53` | `NewRoute53Repository` |
| `service/cloudwatchlogs` | `NewCloudWatchLogsRepository` | `service/s3` | `NewS3Repository` |
| `service/dynamodb` | `NewDynamoDBRepository` | `service/secretmanager` | `NewSecretManagerRepository` |
| `service/ec2` | `NewEc2Repository` | `service/sns` | `NewSnsRepository` |
| `service/ecs` | `NewEcsRepository` | `service/sqs` | `NewSqsRepository` |
| `service/efs` | `NewEfsRepository` | `service/elb` | `NewLoadBalancerRepository` |
| `service/eks` | `NewEksRepository` | `service/emr` | `NewEmrRepository` |
| `service/emrserverless` | `NewEMRServerlessRepository` | | |

Packages with typed, service-specific APIs instead — costs, health events, price lists,
savings plans and bulk tags are not resources, so they are used directly rather than
through the `RepoProxy` fanout:

| Package | Constructor | What it answers | Docs |
|---|---|---|---|
| `service/costexplorer` | `NewCostExplorerRepository` | cost and usage, forecasts, dimension values | [docs/costexplorer.md](costexplorer.md) |
| `service/savingsplans` | `NewSavingsPlansRepository` | Savings Plans inventory and offering rates | [docs/savingsplans.md](savingsplans.md) |
| `service/pricing` | `NewPricingRepository` | public price list |  |
| `service/health` | `NewHealthRepository` | AWS Health events |  |
| `service/resourcegroupstagging` | `NewResourceGroupsTaggingRepository` | tags for many resources per call, and tag writes |  |
| `service/cloudcontrol` | `NewCloudControlRepository` | any resource type, no per-type code | [docs/cloudcontrol.md](cloudcontrol.md) |

`service/cfg` is not a service: it holds the `cfg.ResourceType` constants
(`AWS::EC2::Instance` and friends, extended past the SDK's set) that the proxy layer and
the tagging filters key off.

Every repository takes the same two arguments, so swapping the client swaps the account and
region:

```go
repo := ec2.NewEc2Repository(ctx, client)          // uncached
cachedRepo := repo.WithCache(dc)                   // same methods, cache-backed
```

### A service with no wrapper

`client.Config()` is a plain `aws.Config` with the right region and credentials, so any SDK
package works without touching this library:

```go
import "github.com/aws/aws-sdk-go-v2/service/kms"

svc := kms.NewFromConfig(client.Config())
```

To get caching and the `GetClient` convention instead, add the service to `awsServices` in
`cmd/generate-options/main.go` and run `make generate-options` — see
[docs/code-generation.md](code-generation.md).

## IAM policies

Three policies matter for multi-account use: what the **caller** may assume, what the
**target** role trusts, and what the target role may **read**. Nothing here needs write
access unless you call `TagResources`/`UntagResources`.

### 1. The caller: permission to assume

Attach this to the identity the process runs as — the IRSA role, the CI role, or the
operator's role. Listing target ARNs explicitly is what makes
[role discovery](#discovering-assumable-roles) able to read it back.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AssumeAwslibReaders",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": [
        "arn:aws:iam::123456789012:role/awslib-reader",
        "arn:aws:iam::987654321098:role/awslib-reader"
      ]
    }
  ]
}
```

Add this second statement **only** if you use `DiscoverAssumableRolesFromCurrentRole`; it
reads policies in the caller's own account:

```json
{
  "Sid": "DiscoverOwnPolicies",
  "Effect": "Allow",
  "Action": [
    "iam:ListAttachedRolePolicies",
    "iam:GetPolicy",
    "iam:GetPolicyVersion"
  ],
  "Resource": [
    "arn:aws:iam::111111111111:role/awslib-caller",
    "arn:aws:iam::111111111111:policy/*"
  ]
}
```

`sts:GetCallerIdentity` needs no permission — every principal may call it — but a `Deny`
that catches it breaks client construction outright, since `NewClient` calls it eagerly.

### 2. The target account: the role's trust policy

In each member account, create the role (same name everywhere keeps the caller policy
short) with this trust policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "AWS": "arn:aws:iam::111111111111:role/awslib-caller" },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": { "sts:ExternalId": "awslib" }
      }
    }
  ]
}
```

The `ExternalId` condition is optional — if you set it, pass it through:

```go
providers, err := v3.DefaultAwsClientProviders(
    config.WithAssumeRoleCredentialOptions(func(o *stscreds.AssumeRoleOptions) {
        o.ExternalID = aws.String("awslib")
    }),
)
```

Do **not** condition on `sts:RoleSessionName`: the library lets the SDK generate the
session name (`aws-go-sdk-<nanos>`).

### 3. The target account: what the role may read

`arn:aws:iam::aws:policy/ReadOnlyAccess` covers everything the library does on the read
path and is the pragmatic choice for an audit role. If you scope it down, these are the
cross-cutting permissions the library depends on beyond the obvious per-service
`Describe*`/`List*`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "BulkTags",
      "Effect": "Allow",
      "Action": [
        "tag:GetResources",
        "tag:GetTagKeys",
        "tag:GetTagValues"
      ],
      "Resource": "*"
    },
    {
      "Sid": "CloudControlGenericPath",
      "Effect": "Allow",
      "Action": [
        "cloudcontrol:ListResources",
        "cloudcontrol:GetResource"
      ],
      "Resource": "*"
    },
    {
      "Sid": "S3Buckets",
      "Effect": "Allow",
      "Action": [
        "s3:ListAllMyBuckets",
        "s3:GetBucketLocation",
        "s3:GetBucketTagging"
      ],
      "Resource": "*"
    }
  ]
}
```

What each is load-bearing for:

- **`tag:GetResources`** is not optional in practice. Tags for most resource types come
  from one bulk `resourcegroupstagging` sweep per region rather than a call per resource,
  and the S3 bucket lister depends on it directly. Without it, bucket listing logs a
  warning and falls back to `s3:GetBucketTagging` per bucket; other types simply come back
  untagged. `tag:GetTagKeys`/`tag:GetTagValues` are only needed if you call those methods.
- **`cloudcontrol:*`** covers the generic path only. Cloud Control executes the type's
  handler under *your* identity, so it also needs the underlying service's read
  permissions — `cloudcontrol:ListResources` alone lists nothing.
- **`s3:GetBucketLocation`** is a fallback: bucket region scoping is done server-side via
  the `BucketRegion` request parameter, and only a bucket whose region the response omits
  triggers the call.

Writing tags needs a separate grant, and only if you use it:

```json
{
  "Sid": "BulkTagWrites",
  "Effect": "Allow",
  "Action": ["tag:TagResources", "tag:UntagResources"],
  "Resource": "*"
}
```

`tag:TagResources` is a router — it also requires the target service's own tagging
permission (`ec2:CreateTags`, `s3:PutBucketTagging`, …) in the same policy.

### Region opt-in is not an IAM problem

A region the account has not enabled rejects **every** STS call with
`InvalidClientTokenId`, which reads like a credential problem but is not fixable with a
policy. Enable the region in the account, or leave it out of the region list.

## Failure caching and region timeouts

A wide sweep's expensive failures are the ones the resource cache never sees, because it
only caches successful results. Three things bound the cost:

- **`v3.FailureCache`** — both pools remember per (account, region) client-creation
  failures for `v3.DefaultClientFailureTTL`, so a disabled region costs one rejected STS
  call per TTL instead of one per request, and an unroutable endpoint costs one TCP timeout
  instead of one per request. Pass your own window with `WithFailureTTL(ttl)` so it ages on
  the same clock as your resource cache. Credential failures are deliberately **not**
  cached — an expired SSO session fails identically everywhere and is fixed by
  re-authenticating, so caching it would turn a 30-second `aws sso login` into a TTL-long
  outage.
- **Per-fetch deadlines belong on the HTTP client, not on the fetcher.**
  `resources.Provider` deliberately has no wall-clock timeout: it cannot tell an
  unroutable endpoint from a legitimately long paginated read (S3 lists every bucket and
  then queries each one's tags), so a deadline there abandons real results and reports a
  working region as failed. Bound it where the distinction is visible — dial and TLS
  timeouts on the AWS HTTP client:

  ```go
  providers, err := v3.DefaultAwsClientProviders(
      config.WithHTTPClient(awshttp.NewBuildableClient().
          WithTimeout(2 * time.Minute).
          WithDialerOptions(func(d *net.Dialer) { d.Timeout = 5 * time.Second })),
  )
  ```

- **`ResourceReader.Failures()`** reports the proxies that could not be queried at all,
  each as a `resources.ProxyFailure{AccountID, Region, Err}`. An empty slice is what
  licenses treating a short list as complete; check it before reporting "no resources".

## Custom endpoints (LocalStack, S3-compatible)

`v3.Client` construction hits STS, so an endpoint override has to be in the config, not
applied afterwards:

```go
providers, err := v3.DefaultAwsClientProviders(
    config.WithRegion("us-east-1"),
    config.WithBaseEndpoint("http://localhost:4566"),
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
)
if err != nil {
    return err
}

client, err := v3.NewClient(ctx, providers...)
```

`WithBaseEndpoint` applies to every service built from that config, STS included. To
redirect one service only, build it from `client.Config()` with a per-service
`o.BaseEndpoint` rather than going through the `GetClient` wrapper — the wrapper's cache
would hand the override to every other caller.

## Logging verbosity

The library logs through `zerolog`, with bracketed `[Type.Method]` prefixes. Verbosity is
global:

```go
package internal

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

`Debug` shows every client construction and cache decision; `Trace` adds the role and
region of each assume. Both are worth turning on when a region or account is unexpectedly
missing from a result.
