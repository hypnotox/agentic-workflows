How awf sync and awf check detect and report drift: per-file config-hash inputs, managed-output attribution and provenance, foreign-file backups, residue scanning, ancestor pruning, and uninstall cleanup.

## Claims

### `invariant: awf-bak-flagged`

A collision-backup file under .awf whose name ends in .awf-bak or .awf-bak.<N>, outside the memory directory, is reported by awf check as drift with a distinct stale-backup detail rather than passing silently.
Origin: ADR-0148
Backing: test

### `invariant: catalog-data-in-confighash`

A change to an artifact's catalog default data changes that artifact's lock configHash, so `awf check` reports the artifact stale exactly as it would for a template change.
Origin: ADR-0148
Backing: test

### `invariant: check-active-md-stale`

awf check regenerates the ADR status index at docs/decisions/INDEX.md from the current ADR frontmatter and reports it as stale drift when the on-disk file differs, for example after an ADR's status changes without a re-sync; a synced, unchanged index produces no drift.
Origin: ADR-0148
Backing: test

### `invariant: check-invalid-frontmatter`

awf check reports an invalid-frontmatter drift entry for an on-disk skill or agent file that is otherwise in sync but whose frontmatter is missing, unparseable, or has an empty name or description; a clean synced tree reports no such entry, and at most one drift entry is reported per path.
Origin: ADR-0148
Backing: test

### `invariant: closed-config-tree`

Every filesystem entry under .awf that falls outside the claimed-path model, with the memory directory exempt, is reported by awf check as failing orphaned drift.
Origin: ADR-0148
Backing: test

### `invariant: drift-source-set`

Each rendered file's stored ConfigHash is a per-target projection over only that file's own effective inputs (the skeleton fields it reads, its sidecar, and its consumed parts), so awf check reports a file stale only when one of its own inputs changed since the last sync and never flags unrelated targets; a sidecar or part file matching no enabled or declared target is reported as an orphan.
Origin: ADR-0148
Backing: test

### `invariant: managed-output-attribution`

A reader-injected declaration builder enumerates managed writes and local reservations before rendering, retains their sorted declarers and exact config, sidecar, convention-part, topic, and generated inputs, and supplies context artifact source/output edges; only non-reservation declarations classify a path as generated.
Origin: ADR-0148
Backing: test

### `invariant: part-scopes-in-confighash`

A raw convention-part body referencing a `{{=awf:commitScope...}}` placeholder folds the resolved scope data into its artifact's config hash, so editing `audit.allowedScopes` flags that artifact stale in `awf check` while a non-referencing part stays in sync.
Origin: ADR-0148
Backing: test

### `invariant: provenance-banner`

Every rendered file begins with the awf generated-by banner as its first line, except that it follows a leading construct where one exists: the closing frontmatter delimiter for targets carrying frontmatter, and the shebang line for shell hooks.
Origin: ADR-0148
Backing: test

### `invariant: regeneration-checked-attribute`

The files excluded from the frozen-output-hash comparison are exactly those a first-class RegenChecked attribute marks on the rendered-file model; the generated index, the config reference, and the domain docs carry it, as does every file containing an in-place-editable section, replacing the former hardcoded path list.
Origin: ADR-0148
Backing: test

### `invariant: residue-exemptions-pinned-three`

The identity-exemption list for the rendered-output residue scan contains exactly three entries: the bootstrap template, the upgrade-script template, and the agents-doc template; extending it requires a successor decision.
Origin: ADR-0148
Backing: test

### `invariant: scopes-in-confighash`

The resolved commit-scope list folds into the config hash of every artifact whose assembled template references `.commitScopes`, so editing `audit.allowedScopes` flags exactly those artifacts stale in `awf check` while non-referencing artifacts stay in sync.
Origin: ADR-0148
Backing: test

### `invariant: skills-set-in-confighash`

Changing the skills enable array changes the lock config hash of every artifact whose assembled template references the skills set, so awf check flags those artifacts stale.
Origin: ADR-0148
Backing: test

### `invariant: sync-always-writes-active-md`

awf sync writes the ADR status index at docs/decisions/INDEX.md for every decisions directory, recording it in the lock when the directory holds ADRs and rendering a placeholder index when it holds none.
Origin: ADR-0148
Backing: test

### `invariant: sync-backs-up-foreign`

During `awf sync`, a target path that already exists on disk but is not recorded as awf-written in the lock at the start of the sync is copied to a free `.awf-bak` sibling and reported before being overwritten, while a path recorded in that lock is overwritten with no backup.
Origin: ADR-0148
Backing: test

### `invariant: target-prune-ancestors`

Removing a target from the config and re-syncing deletes that target's rendered files and every resulting empty ancestor directory, not only the immediate parent.
Origin: ADR-0148
Backing: test

### `invariant: uninstall-removes-lock-entries`

awf uninstall removes the in-tree files recorded in the lock and no file outside it, reporting the count it removed.
Origin: ADR-0148
Backing: test
