This topic records the current ownership contract for generated executable adapter-runtime outputs.

## Claims

### `invariant: generated-adapter-runtime-ownership`

Enabled target extension outputs under `.pi/extensions/**` are owned by this topic even though their generated-output classification excludes them from whole-tree coverage eligibility.
Origin: ADR-0144
Backing: test

### `invariant: pi-workflow-telemetry-runtime`

The generated Pi telemetry runtime writes schema-1 session streams directly at the primary control root under a locked, validated, fsynced per-session state machine. It stores no effort ID or lifecycle projector state, uses binary assignment for optional selected-effort context, and leaves malformed, incomplete, locked, or unsafe bytes untouched.
Origin: ADR-0162
Revised-by: ADR-0164
Backing: test
