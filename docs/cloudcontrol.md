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


## Writing resources

The same repository creates, updates and deletes resources through Cloud Control's write
operations. Every write is **asynchronous**: AWS accepts the request and returns a `ProgressEvent`
with a `RequestToken`, and the resource reaches its final state later. The `*AndWait` methods
hide that: they submit, poll to a terminal status, and for create and update fetch the resulting
properties, returning the same `Resource` the read path produces.

```go
repo := cloudcontrol.NewCloudControlRepository(ctx, client).
    WithPollInterval(2*time.Second, 10*time.Second) // SDK default is 5s growing to 120s

created, err := repo.CreateResourceAndWait("AWS::Logs::LogGroup",
    `{"LogGroupName":"demo","RetentionInDays":7}`, 5*time.Minute)
if err != nil {
    var failed *cloudcontrol.RequestFailedError
    if errors.As(err, &failed) && failed.Code() == types.HandlerErrorCodeAlreadyExists {
        // AWS accepted the request and the handler rejected it; failed.Event has the details
    }
    return err // or: rejected before acceptance (validation, permissions), or the wait timed out
}
fmt.Println(created.GetId(), created.GetAttributes()["RetentionInDays"])

// Update is an RFC 6902 JSON Patch over the resource's current properties.
updated, err := repo.UpdateResourceAndWait("AWS::Logs::LogGroup", "demo",
    `[{"op":"replace","path":"/RetentionInDays","value":30}]`, 5*time.Minute)

// Delete by primary identifier, the same value Resource.Identifier carries.
event, err := repo.DeleteResourceAndWait("AWS::Logs::LogGroup", "demo", 5*time.Minute)
```

The split forms are there when the caller wants to own the wait: `CreateResource`,
`UpdateResource` and `DeleteResource` return the accepted event; `WaitForResourceRequest` polls a
token; `DescribeResourceRequest` is a single poll; `CancelResourceRequest` cancels a pending CREATE or
UPDATE; `FindResourceRequests` lists recent requests, optionally by status. The `*ByInput`
variants take the full SDK input for `RoleArn`, `TypeVersionId` or a caller-chosen
`ClientToken`; otherwise the SDK fills the token in, so a retried call does not create twice.

### Tags

Cloud Control has no tagging call: tags are a property of the resource model, patched like any
other. `SetResourceTags` and `UntagResource` read the current properties, merge by key, and submit one
update, preserving the property's shape (a `[{Key,Value}]` list for most types, an object for a
few such as `AWS::SSM::Parameter`) and the order of tags already present:

```go
event, err := repo.SetResourceTags("AWS::Logs::LogGroup", "demo", map[string]string{"env": "prod"})
event, err = repo.UntagResource("AWS::Logs::LogGroup", "demo", "temporary")
```

For resources that have an ARN, `resourcegroupstagging.TagResources` is still the better tool:
synchronous, keyed, batched, and needing no knowledge of the type's schema.

### Caching and writes

`repo.WithCache(dc)` exposes the write methods too, and a write **evicts every cached Cloud
Control list for that account and region** once AWS accepts the request (and again after an
`*AndWait` completes). Eviction is by account and region rather than by type: the cache key folds
the type into a one-way hash with the other list parameters, so one type's entries cannot be
named. Other types in the same place refetch on their next read, which is the cheaper mistake
next to a list that still shows a deleted resource. A rejected write evicts nothing.
`InvalidateLists()` is exported for a caller that changed a resource some other way.

Only handlers that implement `cache.InvalidatorInterface` evict; both shipped handlers do. A
third-party handler without it keeps serving the stale entry until its TTL lapses, with a
warning.

Two more things to know:

- **Properties the type's schema marks `createOnlyProperties` cannot be patched.** Changing one
  needs a delete and a create; Cloud Control rejects the update instead of replacing the resource.
- **Permissions compound.** `cloudcontrol:CreateResource`/`UpdateResource`/`DeleteResource` plus
  `cloudcontrol:GetResourceRequestStatus`, `ListResourceRequests` and `CancelResourceRequest` on the
  Cloud Control side, and the underlying service's write permissions, because the handler runs as
  your identity, or as `RoleArn` if you pass one.
