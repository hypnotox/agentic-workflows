## Tiers

awf has a single tier: `./x gate` runs everything, including protocol parity, the in-memory Pi three-factory runtime seam, repository-runtime cache and launcher tests, and strict full-coverage tests for the selection-gated effort association extension. The latter prove capability degradation but do not claim an end-to-end portable runtime floor. `./x gate full` runs the
identical Go and containerized TypeScript steps; the `full` argument is accepted only as a
no-op legacy argument, and no rendered artifact passes it. There is no slower, fuller
tier to reach for; the whole gate is fast enough to run before every commit. The release-only real interactive Pi smoke remains the manual unbacked verification documented in the test layout; it is not mislabeled as a deterministic gate tier.

`./x check` (beside the gate at every commit via the pre-commit payload) also
gates the example adopter (ADR-0090): it re-checks `examples/sundial` with a
source-built awf (drift, invariants, zero advisory notes) and runs that module's
`go test ./...`, the only place the example's tests execute.
