# Code generation

The library provides code generation tools to bootstrap AWS service clients and repositories:

```sh
# Generate cached repositories for all services
go run cmd/generate-cached/main.go

# Generate service options and configurations
go run cmd/generate-options/main.go

# Register entity types with encoding/gob so cached repositories can serialize them
go run cmd/generate-gob/main.go
```

Run `generate-gob` after adding or removing an entity type — the cache handlers serialize with
`encoding/gob`, and a missing registration surfaces as a cache write failure rather than a compile
error. When *removing* an entity, delete its `gob_register_gen.go` before regenerating: the generators
load packages with `go/packages`, so a stale reference to a deleted type makes the package
uncompilable and the generators then skip it silently.
