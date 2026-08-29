---
title: Pin the Go toolchain when preview compilers break lint
domains: ["tooling"]
---
A locally installed preview Go compiler newer than the `go.mod` directive can make pinned golangci-lint analyzers panic even when the source is valid. Confirm `go version`, rerun with a stable toolchain matching the directive through `GOTOOLCHAIN`, and never bypass lint or treat an analyzer panic as a source finding.
