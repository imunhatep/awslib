# AWS Library

`awslib` wraps `aws-sdk-go-v2` for tools that read AWS across many accounts and regions
at once. It does not replace the SDK — the underlying client is always one call away
(`ec2.GetClient(client)`) — it supplies the plumbing such a tool ends up writing by hand:

- **Multi-account, multi-region fanout** — `v3.ClientPool` builds the whole
  account × region matrix concurrently, caching assumed-role credentials per role.
- **Caching on every repository** — `repo.WithCache(dc)`, namespaced per account, region
  and service, written only on success, in-memory or file-backed.
- **Pagination handled internally** — `List*All()` returns complete, flattened slices,
  including for APIs that ship no SDK paginator at all.
- **One interface over unrelated resources** — 23 service packages implement
  `service.ResourceInterface`, so heterogeneous resource types flow through the same
  channels, cache and reports.
- **Any type, without a repository** — the Cloud Control path serves any
  `AWS::Service::Resource` with a `LIST` handler and no per-type code.
- **Observability** — 11 Prometheus metrics across API calls, fetching and cache.

See [docs/why-awslib.md](docs/why-awslib.md) for the full comparison against the raw SDK,
and what the cache and the parallel fetcher are worth in dollars and wall-clock time.

## Installation

```sh
go get github.com/imunhatep/awslib
```

## Quick start

```go
ctx := context.Background()

client, err := v3.NewClient(ctx)
if err != nil {
    return err
}

instances, err := ec2.NewEc2Repository(ctx, client).ListInstancesAll()
if err != nil {
    return err
}

for _, instance := range instances {
    fmt.Printf("%s %s %s\n", instance.GetId(), instance.GetRegion(), instance.GetName())
}
```

## Documentation

| Document | What's in it |
|---|---|
| [docs/provider.md](docs/provider.md) | clients, client pools, cross-account role assumption, log verbosity |
| [docs/repositories.md](docs/repositories.md) | `ResourceInterface`, cached repositories, `RepoProxy` fanout |
| [docs/cloudcontrol.md](docs/cloudcontrol.md) | fetching any resource type without a repository |
| [docs/savingsplans.md](docs/savingsplans.md) | Savings Plans inventory and offering rates |
| [docs/monitoring.md](docs/monitoring.md) | the Prometheus metrics and how to read them |
| [docs/code-generation.md](docs/code-generation.md) | the generators, and when to re-run them |
| [docs/why-awslib.md](docs/why-awslib.md) | the case against hand-rolling this on the SDK |
| [CLAUDE.md](CLAUDE.md) | architecture in depth: layers, invariants and the reasoning behind them |

## AWS service list

Services whose entities implement the normalized `service.ResourceInterface`, and so are
reachable through `proxy.RepoProxy` and the parallel fetcher:

athena, autoscaling, batch, cloudfront, cloudtrail, cloudwatchlogs, dynamodb, ec2, ecs,
efs, eks, elb, emr, emrserverless, glue, iam, lambda, rds, route53, s3, secretmanager,
sns, sqs

Services exposing typed, service-specific APIs instead — cost figures, health events,
price lists, savings plans and bulk tags are not resources, so they are fetched through
their own repositories rather than the `RepoProxy` fanout:

costexplorer, health, pricing, [savingsplans](docs/savingsplans.md), resourcegroupstagging

`resourcegroupstagging` is the odd one there: it answers about tags rather than
resources, in bulk. One `GetResources` page carries up to 100 ARN-and-tag pairs
regardless of type, so filling in tags for a whole region costs a handful of calls
instead of one per resource — which is the difference between a usable and an unusable
Cloud Control sweep, since most Cloud Control LIST handlers omit tags entirely. It
reports only resources that are currently tagged or that ever held a tag, so it enriches
a resource list and must never *be* one.

And **cloudcontrol**, which is not a service in the same sense: one generic repository
serving any resource type through the AWS Cloud Control API, with no per-type code. Use
it for types the list above does not cover — see
[docs/cloudcontrol.md](docs/cloudcontrol.md).

## Contributing

Repositories, cached wrappers and gob registrations are partly generated — run the
generators after adding or removing a service or an entity type, see
[docs/code-generation.md](docs/code-generation.md).

```sh
make test          # go test -v ./...
make test-cover    # go test -cover ./...
```
