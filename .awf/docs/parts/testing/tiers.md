## Tiers

awf has a single tier: `./x gate` runs everything, and `./x gate full` runs the
identical steps — the `full` argument is accepted only so the rendered pre-push hook
payload (which invokes `./x gate full`) works unchanged. There is no slower, fuller
tier to reach for; the whole gate is fast enough to run before every commit.

`./x check` — beside the gate at every commit via the pre-commit payload — also
gates the example adopter (ADR-0090): it re-checks `examples/sundial` with a
source-built awf (drift, invariants, zero advisory notes) and runs that module's
`go test ./...`, the only place the example's tests execute.
