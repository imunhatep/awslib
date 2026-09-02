//go:build tools

// Package tools pins build-time-only dependencies so `go mod tidy` keeps them.
//
// The code generators under cmd/ import golang.org/x/tools/go/packages, but their
// main.go files carry `//go:build ignore` and are therefore invisible to `go mod
// tidy`, which used to prune x/tools from go.mod and vendor/ and break every
// generator run. This file is excluded from normal builds by the `tools` tag, yet
// tidy and `go mod vendor` consider all build tags, so the dependency stays.
package tools

import _ "golang.org/x/tools/go/packages"
