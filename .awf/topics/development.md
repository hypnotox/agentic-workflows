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

Keep assurance focused on retained behavior: source loading, literal project composition, fixed output ownership and drift, topic matching and coverage, create-only artifact destinations, opaque effort memory, embedded-doc routing and non-mutation, CLI smoke, and release bootstrap/launcher behavior. Test starter contracts through observable status, routing meaning, collision preservation, and opacity rather than exact explanatory wording or heading counts. Native release fixtures use the actual candidate archives, checksums, shared downloader, and public launcher; do not replace first-download coverage with a preseeded cache. Prefer `go test ./...` and `go build ./...` over selectors, timing systems, policy checkers, or tests of exact explanatory prose.
