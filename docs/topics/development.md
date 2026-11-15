---
type: Project Topic
title: Development and assurance
description: Checkout-source commands and verification boundaries for maintained AWF behavior.
paths:
  - 'x'
  - 'go.mod'
  - 'go.sum'
  - '.github/workflows/ci.yml'
  - '**/*_test.go'
---

# Development and assurance

`x` is the checkout-source runner: `./x gate` checks Go formatting, then runs `go test ./...` and `go build ./...`, and the AWF subcommands run `go run ./cmd/awf`. CI's `gate` job runs `./x gate` and `./x check` and fails if `./x render` changes tracked files; pushes to `main` also run the tests and check on macOS arm64.

Assurance is the Go test suite alongside each owning package, with no selectors, timing systems, or policy checkers. Tests pin observable contracts—status, routing meaning, collision preservation, ownership, opacity—rather than exact explanatory wording or heading counts.
