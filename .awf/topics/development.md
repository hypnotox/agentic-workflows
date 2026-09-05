---
paths:
  - 'x'
  - 'go.mod'
  - 'go.sum'
  - '.golangci*.yml'
  - '.github/workflows/ci.yml'
  - '**/*_test.go'
---

# Development and assurance

AWF development uses `./x` to run the checkout source. The generated `./awf` wrapper is reserved for the released bootstrap path.

Keep assurance focused on retained behavior: source loading, literal project composition, fixed output ownership and drift, topic matching, effort memory, CLI smoke, and release bootstrap behavior. Prefer `go test ./...` and `go build ./...` over selectors, timing systems, policy checkers, or tests of exact explanatory prose.
