The upgrade package runs the current-state migration: it verifies the bridge seal, journals the cutover, and writes the permanent lock. The claims below capture the current upgrade-runtime contracts.

## Claims

### `invariant: current-state-cutover-is-atomic`

Schema upgrade enables the topic authority engine only after every readiness predicate succeeds and never leaves a partial or compatibility state.
Origin: ADR-0133
Backing: unbacked
Verify: Upgrade fixtures that each fail one predicate (topic coverage, legacy-invariant mapping, marker resolution, in-flight ADR resolution, static or staged validation) leave the old lock unchanged, and only when every predicate passes does the upgraded project run topic-based context and invariant checks.

### `invariant: every-live-legacy-invariant-adjudicated`

Each nonretired legacy invariant slug maps to exactly one current-state invariant with the same backed or unbacked class and exactly one authored approval, with no exception path; a genuine class change instead requires reviewed retirement rationale and a distinct unmapped claim, and every retired slug has valid history evidence or migration rationale and no approval.
Origin: ADR-0136
Backing: unbacked
Verify: Fixtures with missing, duplicate, unknown, retired-key, malformed, and destination-mismatch approvals, ambiguous or class-changed mappings, exception metadata, rationale-backed distinct claims, and invalid retirement evidence all resolve correctly, mapping is derived before approval, every approval failure reports invariant-approval at .awf/current-state-migration.yaml, and JSON computes approved rather than reading it.

### `invariant: legacy-format-set-is-closed`

The lock's cutoff plus recorded lower-number gaps exactly close the attested legacy identity set, and every later ADR uses the new format.
Origin: ADR-0136
Backing: unbacked
Verify: Missing-format ADRs attempted at an existing lower identity, a recorded lower gap, the cutoff, and above it are accepted only at the attested existing identity.

### `invariant: migration-approval-artifact-is-ephemeral`

The migration approval artifact has one bridge-only parser, is required at readiness even when its approvals list is empty, invalidates the attestation digest on any path, mode, or content change, is not mutated by sealing, is deleted by an explicit final journal operation, and has no permanent parser, claim, or consumer.
Origin: ADR-0136
Backing: unbacked
Verify: Bridge fixtures with absent, empty, and populated approval files, post-seal path, mode, and content mutations that fail final verification, pre and post attestation image comparison, a single final deletion, and the permanent boundary scan together prove no permanent parser, path claim, or consumer remains.

### `invariant: migration-does-not-infer-authority`

Migration inventories and validates legacy obligations but never writes generated ADR summaries as authoritative claims.
Origin: ADR-0133
Backing: unbacked
Verify: Upgrade preflight on a fixture with an unmigrated invariant and no topic claim refuses completion, reports the obligation, and leaves the topic authoring tree unchanged.

### `invariant: migration-never-authors-claims`

Migration commands scaffold empty structure and report obligations but never generate normative claim prose or provenance.
Origin: ADR-0136
Backing: unbacked
Verify: Topic scaffolding and upgrade preflight on a legacy fixture invent no claim heading, body, Origin, or revision metadata.

### `invariant: no-unqualified-markers-after-upgrade`

A migrated project's configured current-state source paths contain no active unqualified proof or touches marker, while frozen declarations may remain in historical ADR text outside that authority scan.
Origin: ADR-0136
Backing: unbacked
Verify: Each legacy marker form placed in every configured source language and in one historical ADR requires source-site rewrites at readiness but leaves the historical declaration untouched.

### `invariant: upgrade-failure-is-recoverable`

A failed upgrade either restores the pre-transaction bytes and modes or preserves a journal from which recovery completes before any project command may run; every valid journal permits only awf upgrade --recover, and postcommit recovery never rolls authority back.
Origin: ADR-0136
Backing: unbacked
Verify: Failures injected during preparation, rename, prune, lock replacement, rollback, and cleanup recover to matching tree digests, every other project command refuses in every journal phase including lock-committed, and postcommit recovery removes only transaction residue.

### `invariant: upgrade-requires-complete-current-state`

The bridge cannot attest and the new lock cannot be written until every awf-owned readiness predicate succeeds; configured tests and the full project gate remain workflow prerequisites for committing the attestation rather than lock-stored facts.
Origin: ADR-0136
Backing: unbacked
Verify: Bridge fixtures failing each internal readiness predicate, plus an invalidated clean-HEAD attestation, are all refused, and the rendered migration workflow and pre-commit payload carry the configured test and gate steps.

### `invariant: upgraded-runtime-has-one-authority-engine`

After the new lock lands, normal context and invariant reporting cannot consume legacy ADR tags, supersession edges, or invariant declarations.
Origin: ADR-0136
Backing: unbacked
Verify: A migrated fixture retaining contradictory legacy metadata affects output and enforcement only through its topic claims.
