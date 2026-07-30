This topic governs values introduced by new work and derived state deliberately converted under its authority. Existing post-construction field writes and cached derivations remain bounded future candidates until a concrete consumer brings them into scope; this authority does not require a wholesale conversion.

## Claims

### `invariant: construction-immutable-state`

A new or deliberately converted value that outlives one operation is immutable once construction completes: no field is written outside the function that constructs it, and a construction needing more than one step completes every step inside that one function.
Origin: ADR-0180
Backing: unbacked
Verify: For each changed type, list every write to each field and confirm each write is inside the function that constructs that value.

### `invariant: operation-owned-derivation`

State a new or deliberately converted operation derives is owned by that operation and threaded explicitly to the consumers that need it, including consumers behind an intermediate boundary, rather than stored on a value that outlives the operation.
Origin: ADR-0180
Backing: unbacked
Verify: For each changed derivation, identify the operation that derives it and trace every consumer; confirm no carrier of the derived value outlives that operation.

### `invariant: no-remembered-invalidation`

A new or deliberately converted derivation is never kept correct by an ordering or invalidation step each entry point must remember to perform; its lifetime makes staleness unrepresentable.
Origin: ADR-0180
Backing: unbacked
Verify: For each changed type, confirm no reset, reload, or begin-style step must run before a read is meaningful, and that no comment states such an obligation.

### `invariant: single-derivation-producer`

Within one operation, a new or deliberately converted derived value is produced exactly once and every consumer receives it rather than re-deriving it; the rule counts productions per value per operation, not producers per type.
Origin: ADR-0180
Backing: unbacked
Verify: For each changed derived value, enumerate its production sites within one operation and confirm exactly one, with every other consumer receiving the value.

### `invariant: project-derived-state-ownership`

No production function in `internal/project` writes a `*Project` field outside the function that constructs that value: the ADR corpus, topic corpus, and effective skill set are derived by the operation that needs them and threaded to their consumers, and `beginInvocation` no longer exists.
Origin: ADR-0180
Backing: test
