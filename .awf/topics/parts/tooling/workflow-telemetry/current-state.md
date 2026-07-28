The telemetry package owns the protocol-2 privacy-minimal event contract, confined append-only resident ledger, transactional phase lifecycle and trajectory model, deterministic terminal-effort retention, and canonical metrics and effort-owned diagnostic projections. Selectors, aggregation, exact workflow violations, versioned heuristic signals, and safe repair or waiver inputs share one deterministic Go interpretation for CLI and runtime consumers.

## Claims

### `invariant: event-protocol-and-ledger`

Telemetry is an independent schema-1 session stream at the primary control root's `.awf/metrics/sessions/<session-id>.jsonl`. A closed header precedes closed, privacy-minimal usage, tool, gate, subagent, compaction, or handoff observations. The direct Pi writer holds a per-session lock, validates complete prior bytes, preserves a stable observation UUID across retry, fsyncs acknowledged writes, and never writes effort IDs, workflow state, paths, commands, conversation, or tool I/O. New observations join current binary session assignment at report time; protocol-1 and protocol-2 residents remain byte-preserved and read-only.
Origin: ADR-0146
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164
Backing: test

### `invariant: privacy-integrity-and-retention`

Session-v1 telemetry contains only its closed descriptor fields and is append-only, locked, and fsynced by Pi without an awf append command. `awf metrics doctor` reports deterministic path, header, record, version, and duplicate findings only. Legacy protocol-1 and protocol-2 data is never migrated, retained, purged, repaired, or rewritten.
Origin: ADR-0146
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164
Backing: test

### `invariant: canonical-projections-and-diagnostics`

Metrics reads new session streams and read-only legacy ledgers deterministically. Optional effort, session, since-inclusive, and until-exclusive selectors combine with AND; session selection reports an unassigned session successfully. Current counters join current assignment records, legacy counters retain embedded historical identity, and doctor emits only sorted bounded integrity findings.
Origin: ADR-0146
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164
Backing: test
