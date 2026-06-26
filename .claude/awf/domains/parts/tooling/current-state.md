## Current state

All repo interactions go through the `./x` command runner (`gate`, `lint`, `fmt`, `test`, `sync`, `check`, `setup`, `build`, `install`), which runs awf from source rather than a stale binary. The gate is `go test ./... && go vet && golangci-lint`, with a hard 100% statement-coverage floor (`cmd/covercheck`; a genuinely-unreachable branch may carry a justified `// coverage-ignore:`). golangci-lint is a pinned `go tool` dependency; git hooks under `.githooks/` run the gate on commit/push.
