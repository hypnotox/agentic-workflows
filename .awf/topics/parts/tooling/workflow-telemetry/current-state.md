The telemetry package owns the protocol-2 privacy-minimal event contract, confined append-only resident ledger, transactional phase lifecycle and trajectory model, deterministic terminal-effort retention, and canonical metrics and effort-owned diagnostic projections. Selectors, aggregation, exact workflow violations, versioned heuristic signals, and safe repair or waiver inputs share one deterministic Go interpretation for CLI and runtime consumers.

## Claims

### `invariant: event-protocol-and-ledger`

One embedded machine-readable descriptor defines protocol version 2.0, closed event, payload, activity, route-action, and lifecycle-request vocabularies, bounded identifiers and categories, recursive privacy exclusions, compatible-minor preservation, and the append-only per-session JSONL ledger. Creation and append validate confinement, ownership, leases, durability, idempotency, and corrupt-stream evidence without protocol-1 compatibility or inferred missing state.
Origin: ADR-0146
Revised-by: ADR-0149
Backing: test

### `invariant: effort-lifecycle-and-routes`

Effort identity, route selection, transactional phase transitions, terminal epochs, repairs, and waivers change only through the closed explicit lifecycle request union. A normal chain edge appends one `phase_transitioned` event naming the unmatched start, closing and successor phases, optional route effect, and current causal predecessor frontier; identical retries are idempotent, conflicts remain evidence, and structurally valid illegal events have no state effect.
Origin: ADR-0146
Revised-by: ADR-0149
Backing: test

### `invariant: trajectory-and-derived-effort-model`

Trajectories preserve parent and fork ancestry so current-path projection follows the active ancestry while all-work projection retains discarded branches. Independent, derived, and reopened efforts use explicit immutable lineage or terminal-epoch operations without reconstructing or double-counting origin work.
Origin: ADR-0146
Backing: test

### `invariant: privacy-integrity-and-retention`

Resident protocol-2 telemetry excludes conversational content and every repository path, rejects unsafe paths and unsupported protocol interpretations, and never rewrites protocol-1 resident data. Retention prunes only terminal efforts through deterministic age/count selection, leased tombstones, private trash, and explicit confirmed purge; repository preflight refuses automatic cleanup when any protocol-1 effort exists.
Origin: ADR-0146
Revised-by: ADR-0149
Backing: test

### `invariant: canonical-projections-and-diagnostics`

Canonical metrics and doctor results use one validated selector and deterministic projection over every resident effort. They distinguish current-path from all-trajectory work, preserve integrity evidence, and expose stable exact and heuristic findings with an owning `effortId`, thresholds, baselines, confidence, and typed remediation. Repair and waiver inputs re-resolve that effort-owned finding and require eligible reason, matching evidence and scope, and the current nonempty causal frontier; no projection derives an opaque health score.
Origin: ADR-0146
Revised-by: ADR-0149
Backing: test
