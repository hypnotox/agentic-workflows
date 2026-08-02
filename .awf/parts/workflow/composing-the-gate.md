## Composing the gate

The gate is one command (`./x gate`) that must be green before every commit. Here it runs the
profiled test suite (`go test ./... -coverpkg=./...`, writing the durable `coverage.out` CI
uploads), the 100%-coverage check (`cmd/covercheck`, ADR-0012), the containerized Pi-extension
strict type check and its 100% line/function/branch coverage floor (ADR-0123, ADR-0126),
`go vet`, `golangci-lint`, the dead-code gate (`cmd/deadcodecheck`, ADR-0063), and the
workflow-pin check (`cmd/pincheck`, ADR-0079). Every step is deterministic: same tree in, same
verdict out. The plain-punctuation scan (`awf check repo prose`, ADR-0119) and the working-memory
citation scan (`awf check repo memory`, ADR-0158), both opt-in for adopters and enabled here, are
not gate steps: the pre-commit hook payload runs them locally and CI is their enforcement
backstop (ADR-0196).

Rendered-file drift is not a gate step: `./x check` blocks separately through the pre-commit
hook payload (see the local-hooks section below). And there is no slower tier; `./x gate full`
runs the identical steps, and `full` is accepted only as a no-op legacy argument that no
rendered artifact passes (see [docs/testing.md](testing.md)).

The current-state cutover deliberately sits outside this gate. The preceding bridge release sealed the
prepared tree, and this binary's plain `awf upgrade` consumes that seal through a recoverable journal;
it never runs the project test suite or the gate and never claims to. `awf upgrade --recover` is the
escape when a cutover transaction is interrupted.
