The telemetry topic describes independent session-v1 streams and their read-only relationship to older residents. Pi writes a session stream at `.awf/metrics/sessions/<session-id>.jsonl`: a closed header is followed by closed, privacy-minimal observations. The writer uses a per-session lock, stable observation IDs for retry, complete-byte validation, fsync, and hard-link publication. It never writes effort identity, workflow state, paths, commands, conversation, or tool I/O.

Current assignment joins are a reporting concern. A session observation does not embed an effort ID; metrics joins the current binary session assignment when it reports current streams. An explicitly selected unassigned session remains reportable. Legacy protocol-1 and protocol-2 residents retain their embedded historical identity and are read-only.

Integrity reporting is deterministic and bounded. Readers report malformed headers, malformed or unterminated records, unsupported versions, unsafe ownership, and duplicate observation IDs without repairing the stream. Read failures preserve the bytes already present. No lifecycle, trajectory, retention, or heuristic behavior is active in this topic.

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
