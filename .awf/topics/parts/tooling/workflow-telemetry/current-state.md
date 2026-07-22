The telemetry package owns the versioned privacy-minimal event protocol, confined append-only resident ledger, explicit lifecycle and trajectory model, and deterministic terminal-effort retention. The initial CLI exposes only protocol negotiation, lifecycle mutation, and confirmed maintenance; canonical queries and diagnostics are not yet current behavior.

## Claims

### `invariant: event-protocol-and-ledger`

One embedded machine-readable descriptor defines protocol version 1.0, closed event and payload vocabularies, bounded identifiers and categories, privacy exclusions, compatible-minor preservation, and the append-only per-session JSONL ledger. Creation and append validate confinement, ownership, leases, durability, idempotency, and corrupt-stream evidence without inferring missing state.
Origin: ADR-0146
Backing: test

### `invariant: effort-lifecycle-and-routes`

Effort identity, route selection, phase transitions, terminal epochs, repairs, and waivers change only through the closed explicit lifecycle request union. Causal predecessor frontiers rather than timestamps define ordering, and structurally valid illegal events remain evidence while their state effects are excluded.
Origin: ADR-0146
Backing: test

### `invariant: trajectory-and-derived-effort-model`

Trajectories preserve parent and fork ancestry so current-path projection follows the active ancestry while all-work projection retains discarded branches. Independent, derived, and reopened efforts use explicit immutable lineage or terminal-epoch operations without reconstructing or double-counting origin work.
Origin: ADR-0146
Backing: test

### `invariant: privacy-integrity-and-retention`

Resident telemetry excludes conversational content and repository paths other than the bounded checkpoint identifier, rejects unsafe paths and unsupported interpretations, and prunes only terminal efforts through deterministic age/count selection, leased tombstones, private trash, and explicit confirmed purge.
Origin: ADR-0146
Backing: test
