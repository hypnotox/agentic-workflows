## Composing the gate

The gate is one command (`./x gate`) that must be green before every commit. Here it runs the
profiled test suite (`go test ./... -coverpkg=./...`), the 100%-coverage check
(`cmd/covercheck`, ADR-0012), the containerized Pi-extension strict type check and its 100%
line/function/branch coverage floor (ADR-0123, ADR-0126), `go vet`, `golangci-lint`, the dead-code gate (`cmd/deadcodecheck`, ADR-0063), the
workflow-pin check (`cmd/pincheck`, ADR-0079), and the plain-punctuation scan (`awf prose-gate`, ADR-0119, opt-in
for adopters and enabled here). Every step is deterministic: same tree in, same verdict out.

Rendered-file drift is not a gate step: `./x check` blocks separately through the pre-commit
hook payload (see the local-hooks section below). And there is no slower tier; `./x gate full`
runs the identical steps and exists only so the rendered pre-push hook payload works unchanged
(see [docs/testing.md](testing.md)).

The current-state bridge deliberately sits outside this gate. `awf upgrade --attest-current-state`
runs the readiness check and a clean-HEAD test, then journals its writes and commits the attested lock;
it never runs the project test suite or the gate and never claims to. Attest only after `./x check`,
`./x gate`, and the readiness check are green on a clean HEAD, and with the matching current-state
binary verified. `awf upgrade --recover` is the escape when a transaction is interrupted.
