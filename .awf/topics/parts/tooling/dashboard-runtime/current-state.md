The repository dashboard runtime freezes canonical read semantics at an explicit Git commit and publishes validated immutable artifacts independently of the mutable checkout.

## Claims

### `invariant: pinned-development-runtime-cache`

The development dashboard runtime resolves `refs/awf/dashboard-runtime` to exactly one commit, builds the awf binary and closed launcher from an isolated materialization under a normalized Go environment, and publishes them with a complete canonical telemetry-policy snapshot as an immutable content-addressed cache entry. Publication holds an OS advisory lock, validates private confined paths and digests, recovers only incomplete same-key staging, and atomically renames; advance publishes first and then compare-and-swap updates the ref, while dirty checkout bytes cannot affect an entry.
Origin: ADR-0150
Backing: test
