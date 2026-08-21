# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`github.com/imunhatep/awslib` is a Go library (Go 1.24) for interacting with AWS across
multiple accounts and regions simultaneously. It wraps `aws-sdk-go-v2`, adds a pluggable
caching layer, Prometheus instrumentation, and a normalized resource abstraction so
heterogeneous AWS resources can be fetched and handled through common interfaces.

## Commands

```sh
make test          # go test -v ./...
make test-cover    # go test -cover ./...

# Run a single test
go test -v -run TestName ./service/ec2/

# Code generation (run after adding/changing service clients or repositories)
make generate-options                 # regenerate provider/v3/clients/*/*.go (GetClient wrappers)
go run cmd/generate-cached/main.go    # regenerate service/*/cached_repository.go
go run cmd/generate-gob/main.go       # regenerate service/*/gob_register_gen.go (also: go generate ./...)

make update-deps   # bump all direct deps + go mod tidy (+ vendor if vendor/ exists)
```

Generator `main.go` files carry `//go:build ignore`, so they only run via `go run`, never in a normal build.

## Architecture

The library is built in layers, from low-level SDK access up to a parallel cross-account resource fetcher.

### 1. Provider layer — client creation & multi-account fanout (`provider/`)

- `provider/v3/` holds the client implementation.
- `v3.Client` (`provider/v3/client.go`) wraps an `aws.Config` for one account+region. It lazily
  resolves account ID via STS `GetCallerIdentity`, and acts as a per-client cache (`sync.Map`) for
  instantiated SDK service clients.
- `v3.ClientBuilder` (`client_builder.go`) constructs clients: `DefaultClient()`, `LocalClient(region)`,
  and `AssumeClient(role, region)` (caches assumed-role credentials per role). `DefaultAwsClientProviders()`
  assembles the default config chain (retry settings, shared profile / static creds / web identity from env).
- **Two `ClientPool` types exist** — do not confuse them:
  - `v3.ClientPool` (`provider/v3/client_pool.go`) — takes a map of `AwsAccountID → RoleArn` and
    creates one client per account×region by assuming roles (concurrent). Used for explicit cross-account setups.
  - `provider.ClientPool` (`provider/client_pool.go`, top-level package) — thinner wrapper around a
    `v3.ClientBuilder` that uses default credentials only. Example CLIs (`cmd/resources`) use this one.
- `provider/types/resources.go` defines the core value types: `AwsAccountID`, `AwsRegion`, `RoleArn`.

### 2. Service layer — repositories & entities (`service/`)

Each AWS service has a package under `service/<name>/` (ec2, s3, rds, ...). Within a package:

- `<name>_repository.go` — hand-written `XxxRepository` struct holding `ctx` + `*v3.Client`,
  constructed via `NewXxxRepository(ctx, client)`. It obtains SDK clients through the generated
  `provider/v3/clients/<name>.GetClient(client)` wrappers.
- `*_repository.go` files — hand-written `Get*`/`List*` methods that paginate the SDK and emit
  Prometheus metrics (guarded by `metrics.AwsMetricsEnabled`) via a `promLabels(...)` helper.
- Entity files (e.g. `instance.go`, `volume.go`) — resource wrappers that embed both
  `service.AbstractResource` and the raw SDK type, and satisfy `service.ResourceInterface`
  (`service/resource.go`): `GetAccountID/GetRegion/GetType/GetArn/GetId/GetName/GetCreatedAt/GetTags`.
  This normalized interface is what lets unrelated resource types flow through the same channels.
- `cached_repository.go` — **generated**. `XxxRepositoryCached` wraps the repo; `repo.WithCache(dc)`
  returns it with a `<accountID>:<region>` cache namespace. Cached methods build their key with
  `cache.Key(methodName, params...)` and only cache on success.

### 3. Cache layer (`cache/`)

- `cache.DataCache` (`cache/datacache.go`) fans reads/writes across ordered `HandlerInterface`
  handlers; first hit wins on read. Namespacing via `WithNamespace`, handlers via `WithHandlers`.
- Handlers in `cache/handlers/`: `NewInMemory(bigcache)` and file-based. Values are serialized with
  `encoding/gob` — this is why gob registration exists (see below).
- `cache.Key(method, params...)` (`cache/key.go`) builds the keys the generated cached repositories
  use. It renders params **by value** via reflection (pointers dereferenced, map entries sorted,
  unexported fields included), because `%v` on an AWS SDK input embeds the addresses of its nested
  pointer fields. A type can override the walk by implementing `cache.Hashable`.

### 4. Resource fetching layer (`proxy/` + `resources/`)

- `proxy.RepoProxy` (`proxy/proxy.go`) maps a `configservice.ResourceType` → the right
  `Find<Resource>(ctx, client, cache)` function via a big switch in `FindAll`. This is the central
  registry tying resource types to repositories — **when adding a new fetchable resource type, wire it
  here**. `FindAllCC` is the switch-less Cloud Control alternative covering every type at once (see
  below).
- `proxy.RepoProxyPool` builds one `RepoProxy` per client and can `WithCache(...)` them all.

#### Scoping to one account

Both client pools expose `GetAccountClients(accountID, regions...)` next to
`GetClients(regions...)`, plus `PoolAccountIDs()` for the accounts they can serve.

This is deliberately *not* something a caller should do by filtering the result of
`GetClients`: building a client assumes that account's role (`v3.ClientPool`) or
resolves that account's default credentials (`provider.ClientPool`), so by the time
you have a slice to filter, every account in the pool has already been contacted.
Anything downstream — including the Cloud Control generic path, which is built from
whatever clients it is handed — inherits its account scope from this decision.

`GetAccountClients` returns an **error** for an account the pool cannot reach, not
an empty slice: a caller that asked about the wrong account must not be told the
account is empty. `PoolAccountIDs()` exists because `ListAccountIDs()` on
`provider.ClientPool` reports the accounts of clients *already created*, so it is
empty until the first `GetClients` call and cannot be used to validate a request.
- `resources.Provider` (`resources/provider.go`) runs the proxies **in parallel** for a single
  resource type, streaming results through a buffered channel (`ResourceBusSize = 10000`) to a
  `ResourceReader`. It throttles goroutine launch by 100ms and drops resources (with a metric) if the
  channel is full.

#### The generic Cloud Control path (`proxy/generic.go`)

The whole Cloud Control side of this library is **type-agnostic**: there are no hand-written CC
entities or per-type CC repositories, and `service/cloudcontrol/` holds exactly one entity
(`Resource`) plus the repository. Everything funnels through one implementation,
`proxy.FindGenericResources`, reached by two entry points:

- **`RepoProxy.FindAllCC(rt)`** — the Cloud Control counterpart of `FindAll`, switch-less, serving any
  resource type. It keeps `FindAll`'s exact signature on purpose, so replacing the big `FindAll`
  switch with the Cloud Control approach stays a mechanical substitution.
- **`GenericRepoProxy`** — a decorator over `RepoProxy` (`RepoProxy.Generic(detailed)`, or
  `NewGenericRepoProxyPool` for a whole pool) that satisfies `RepoProxyInterface`. That is the point:
  a generic pool drops straight into `resources.NewProvider`, inheriting the parallel fan-out, the
  cache and `RepoProxyPool.List`'s global-type collapsing without any of them knowing it exists. It
  differs from `FindAllCC` only in honouring `detailed`, which `FindAllCC` cannot express without
  changing its signature.

Treat it as a **fallback, not a replacement** for the typed path. Types whose Cloud Control registry
entry has no LIST handler error out rather than returning an empty list, nested types need a
`ResourceModel` the proxy does not supply, `detailed` costs one extra `GetResource` per resource (S3
buckets need it, EC2 instances do not), and a generic resource gives up three things a typed entity
has: a typed SDK struct, a creation time (Cloud Control reports none), and often an ARN.

**On the missing ARN — this is deliberate, do not "fix" it by synthesizing one.** `ResourceDescription`
carries only an opaque identifier, and the resource-path segment differs per type (`instance/`,
`role/`, `function:` with a colon, and S3 omits region and account entirely). The deleted per-type CC
entities are the cautionary tale: their hand-written ARNs were correct for EC2 but produced
`arn:aws:s3:<region>:<account>:<bucket>` for S3, which is not a valid bucket ARN. A guessed ARN is
worse than none, and specifically it breaks consumers that use an empty ARN region segment to detect a
global resource. `Resource` lifts an ARN only when the type exposes one as a property; otherwise
`GetIdOrArn()` falls back to the identifier, which is always set.

Three invariants worth keeping:

- **`RepoProxyPool.WithCache` needs a branch per cacheable proxy type.** It type-switches, and the
  fallthrough passes the proxy along *uncached* rather than failing — a new proxy type missed there
  silently stops caching.
- **`cloudcontrol.Resource` must keep `Attributes` and `Tags` exported**, and the hand-written
  `service/cloudcontrol/gob_register.go` must keep registering `map[string]interface{}` and
  `[]interface{}`. The cache serializes with gob: unexported fields are dropped *without an error*
  (a cache hit returns a resource with no properties at all), and a nested JSON object in an
  interface-typed field fails to encode outright without those two registrations. Both are pinned by
  `service/cloudcontrol/resource_test.go`.
- **Deleting an entity type means deleting its `gob_register_gen.go` before regenerating.** Both
  generators load packages with `go/packages`, so a stale generated file referencing a removed type
  makes its package uncompilable and the generators then silently *skip* that package rather than
  fixing it. Remove the stale file first, then run `generate-cached` followed by `generate-gob`.

### Resource types (`service/cfg/resources.go`)

Resource types reuse `aws-sdk-go-v2/service/configservice/types.ResourceType` (CloudFormation-style
strings like `AWS::EC2::Instance`). `service/cfg/resources.go` extends the SDK's set with custom
constants (EMR Serverless, Glue, Route53 records/domains, etc.) and provides
`ResourceTypeToString/ToUrl/FromUrl/List/ListGlobal/ListRegional`. Global vs regional distinction
matters for how resources are enumerated.

### Gob registration (`service/*/gob_register_gen.go`)

Cache handlers serialize with `encoding/gob`, so each service package **self-registers** its own
types from an `init()` in its generated `gob_register_gen.go` — the `image/png` / `database/sql`
driver pattern. Importing `service/ec2` is therefore sufficient to make `ec2.Instance` cacheable,
and a consumer only pays for the SDK packages it actually imports. There is deliberately **no root
`awslib` package**: a central registry had to import all ~30 service packages, dragging the whole
SDK into any build that touched it. Regenerate with `go run cmd/generate-gob/main.go` after adding
entity types.

`gob.Register` only *needs* an explicit call for values held in **interface-typed fields**;
concrete types (what the cached repositories actually encode) are handled by reflection. The one
real case is `types.JobRun.JobDriver`, registered by hand in
`service/emrserverless/gob_register.go`. Registering the entity types is belt-and-braces — free now
that it costs no imports, and it covers a caller who caches `[]service.ResourceInterface` through
the public `DataCache` API.

## Conventions

- Errors are wrapped with `github.com/go-errors/errors` (`errors.New(err)`) to capture stack traces.
- Logging uses `github.com/rs/zerolog` with bracketed `[Type.Method]` message prefixes; verbosity is
  set globally via `zerolog.SetGlobalLevel` (see the `setLogLevel` example in README.md).
- Collection helpers come from `github.com/imunhatep/gocollection` (`slice`, `dict`).
- Prometheus metrics live in `metrics/` and are always guarded by `if metrics.AwsMetricsEnabled`.
- Use `podman`, not `docker`, for any container operations.

## Adding a new AWS service

1. Add the SDK dependency and register a client wrapper: add the service to `cmd/generate-options`
   config, then `make generate-options` to emit `provider/v3/clients/<name>/<name>.go`.
2. Create `service/<name>/` with a `NewXxxRepository`, `Get*/List*` methods, and entity types
   implementing `service.ResourceInterface`.
3. Run `go run cmd/generate-cached/main.go` to emit the cached wrapper.
4. Add a `Find<Resource>` function and wire the resource type into `proxy.RepoProxy.FindAll`.
5. If new custom resource types are needed, add them to `service/cfg/resources.go`.
6. Run `go run cmd/generate-gob/main.go` so cached entities can be serialized (emits
   `service/<name>/gob_register_gen.go`).
