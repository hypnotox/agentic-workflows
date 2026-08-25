The upgrade package plans supported live-schema migrations and commits them through a root-confined journal with the replacement lock last. When a schema advance discards resident state, the journal records each proven resident as a quarantine-by-rename operation alongside tracked file images, so failure restores the resident whole and successful cleanup discards it only after the lock commits. The claims below capture the current upgrade-runtime contracts.

## Claims

### `invariant: initial-adoption-version-immutable`

The first-adoption binary version is sealed once and preserved unchanged by ordinary sync, zero-migration upgrade, staged authority checks, and forced initialization. ADR format is authored in each record and no cutoff or legacy-gap set forms permanent lock authority.
Origin: ADR-0139
Revised-by: ADR-0206
Backing: test

### `invariant: upgrade-failure-is-recoverable`

A failed upgrade either restores the pre-transaction bytes and modes or preserves a journal from which recovery completes before any project command may run; every valid journal permits only awf upgrade --recover, and postcommit recovery never rolls authority back.
Origin: ADR-0136
Backing: unbacked
Verify: Failures injected during preparation, rename, prune, lock replacement, rollback, and cleanup recover to matching tree digests, every other project command refuses in every journal phase including lock-committed, and postcommit recovery removes only transaction residue.
### `invariant: upgraded-runtime-has-one-authority-engine`

After the new lock lands, normal context and invariant reporting cannot consume legacy ADR tags, supersession edges, or invariant declarations.
Origin: ADR-0136
Backing: unbacked
Verify: A migrated fixture retaining contradictory legacy metadata affects output and enforcement only through its topic claims.

### `rule: installed-release-compatibility-floor`

Owner-managed installations support the current awf release and one previous release; at adoption current is 0.39.2 and the floor is 0.39.1. Older installed releases are unsupported. The rolling floor does not advance until every managed adopter pin is upgraded to remain at or above it.
Origin: ADR-0297

### `rule: managed-compatibility-removal-gate`

Compatibility removal remains blocked while any managed adopter pin or live source is below its applicable floor, or while any current managed input, active resident, cutover state, or reachable managed history inside the audit horizon requires the candidate component. Advancing a floor does not itself authorize deletion.
Origin: ADR-0297

### `rule: managed-cutover-format-support`

Retain the generic journaled upgrade commit point, rollback, quarantine, postcommit cleanup, and recovery as current mutation safety. Permanent locks are the only live authority; unsupported lock fields refuse before upgrade mutation. Hypothetical external adoption is not retention evidence.
Origin: ADR-0297
Revised-by: ADR-0303
