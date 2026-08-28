# Cloud Control: any resource type, without a repository

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

