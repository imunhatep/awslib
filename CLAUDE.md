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
go run cmd/generate-gob/main.go       # regenerate gob_register_gen.go (also: go generate ./...)

make update-deps   # bump all direct deps + go mod tidy (+ vendor if vendor/ exists)
```

Generator `main.go` files carry `//go:build ignore`, so they only run via `go run`, never in a normal build.

## Architecture

The library is built in layers, from low-level SDK access up to a parallel cross-account resource fetcher.

### 1. Provider layer — client creation & multi-account fanout (`provider/`)

- `provider/v3/` is the current client implementation. **`provider/v2/` is legacy** — prefer v3.
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
  `Find<Resource>(ctx, client, cache)` function via a big switch in `FindAll` (and `FindAllCC` for
  Cloud Control API variants). This is the central registry tying resource types to repositories —
  **when adding a new fetchable resource type, wire it here**.
- `proxy.RepoProxyPool` builds one `RepoProxy` per client and can `WithCache(...)` them all.
- `resources.Provider` (`resources/provider.go`) runs the proxies **in parallel** for a single
  resource type, streaming results through a buffered channel (`ResourceBusSize = 10000`) to a
  `ResourceReader`. It throttles goroutine launch by 100ms and drops resources (with a metric) if the
  channel is full.

### Resource types (`service/cfg/resources.go`)

Resource types reuse `aws-sdk-go-v2/service/configservice/types.ResourceType` (CloudFormation-style
strings like `AWS::EC2::Instance`). `service/cfg/resources.go` extends the SDK's set with custom
constants (EMR Serverless, Glue, Route53 records/domains, etc.) and provides
`ResourceTypeToString/ToUrl/FromUrl/List/ListGlobal/ListRegional`. Global vs regional distinction
matters for how resources are enumerated.

### Gob registration (`gob_register.go`, `gob_register_gen.go`)

Because cache handlers gob-encode concrete types, every entity/SDK type stored in cache must be
registered. `gob_register_gen.go` is generated from all `service/*` packages; an `init()` in
`gob_register.go` calls it automatically. Regenerate with `go run cmd/generate-gob/main.go` after
adding entity types.

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
6. Run `go run cmd/generate-gob/main.go` so cached entities can be serialized.
