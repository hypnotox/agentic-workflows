How awf render and awf check detect and report drift: per-file config-hash inputs, managed-output attribution and provenance, foreign-file backups, residue scanning, ancestor pruning, and uninstall cleanup.

## Claims

### `invariant: agent-guide-size-advisory`

Only the deterministic expected bytes of a managed `AGENTS.md` feed aggregate `CheckReport.Notes`: at a fixed 12 KiB threshold, an overage is a warning-only zero-exit advisory. Non-aggregate consumers are excluded.
Origin: ADR-0241
Revised-by: ADR-0251
Backing: test

### `invariant: awf-bak-flagged`

A collision-backup file under .awf whose name ends in .awf-bak or .awf-bak.<N>, outside an owned resident root, is reported by awf check as drift with a distinct stale-backup detail rather than passing silently.
Origin: ADR-0148
Revised-by: ADR-0175
Backing: test

### `invariant: catalog-data-in-confighash`

A change to an artifact's catalog default data changes that artifact's lock configHash, so `awf check` reports the artifact stale exactly as it would for a template change.
Origin: ADR-0148
Backing: test

### `invariant: check-active-md-stale`

For Full, awf check regenerates the ADR status index at docs/decisions/INDEX.md from current ADR frontmatter and reports divergent on-disk bytes as stale drift. Core has no managed ADR index.
Origin: ADR-0148
Revised-by: ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: check-invalid-frontmatter`

awf check reports an invalid-frontmatter drift entry for an on-disk skill or agent file that is otherwise in sync but whose frontmatter is missing, unparseable, or has an empty name or description; a clean synced tree reports no such entry, and at most one drift entry is reported per path.
Origin: ADR-0148
Backing: test

### `invariant: closed-config-tree`

Every filesystem entry under .awf outside the selected profile's claimed-path model is reported by awf check as orphaned drift, with owned resident roots exempt.
Origin: ADR-0148
Revised-by: ADR-0175, ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: drift-source-set`

Each rendered file's stored ConfigHash projects only that file's effective inputs and selected profile, so awf check reports it stale only when those inputs changed. A sidecar or part matching no selected artifact or target is an orphan.
Origin: ADR-0148
Revised-by: ADR-0251, ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: managed-output-attribution`

A reader-injected declaration builder enumerates selected-profile managed writes before rendering, retaining sorted declarers and exact inputs and supplying context source/output edges; those declarations classify their paths as generated.
Origin: ADR-0148
Revised-by: ADR-0251, ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: ordinary-render-freshness`

After an ordinary frozen output's template and config hashes match, awf check compares its current planned render bytes to the locked output hash before observing the worktree or staged bytes. A changed fresh render reports stale output, while an unchanged fresh render attributes only a differing observed output as hand-edited. Regenerated and in-place outputs retain their declared regeneration policy.
Origin: ADR-0235
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

### `invariant: generated-artifacts-tracked`

Repository and staged drift require every output-plan write and `.awf/awf.lock` in the Git index, independently of ignore rules. An absent indexed path reports blocking `untracked` drift before `missing`; tracked files remain valid when ignored. Repository drift reports a non-failing tracking-unavailable advisory outside Git, while nested adopters exclude resident outputs outside their subtree index authority.
Origin: ADR-0277
Backing: test

### `invariant: staged-drift-rendered-output`

`awf check staged drift` renders from the staged config and compares staged generated outputs for index membership, stale content, and hand edits. An absent staged output or lock reports blocking `untracked` drift without consulting working-tree bytes; an invalid staged lock remains an operational failure.
Origin: ADR-0210
Revised-by: ADR-0277
Backing: test

### `invariant: sync-always-writes-active-md`

Full awf render writes the ADR status index at docs/decisions/INDEX.md, recording it when ADRs exist and otherwise rendering a placeholder; Core does not emit a managed ADR index.
Origin: ADR-0148
Revised-by: ADR-0159, ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: sync-mutations-root-confined`

During ordinary render and first adoption, sync opens selected root-confined handles for the tracked checkout and any distinct primary resident root before its first mutation. Every lock-relative output observation and replacement, parent creation and mode correction, foreign or runner backup publication, retired-output removal, empty-ancestor cleanup, and lock load and replacement uses that output's selected handle with its unchanged slash-relative path. Escaping or broken output, backup, prune, ancestor, and lock parents refuse without changing outside bytes or modes, and an incomplete mutation never advances the old lock.
Origin: ADR-0269
Backing: test

### `invariant: sync-backs-up-foreign`

During `awf render`, a target path that already exists on disk but is not recorded as awf-written in the lock at the start of the sync is copied through the selected root-confined handle and complete exclusive publication to a `.awf-bak` sibling and reported before being overwritten, while a path recorded in that lock is overwritten with no backup. One confined open observes source bytes and permission mode. A foreign final symlink is backed up only when that open safely reads its in-root target; an escaping, broken, or unreadable target refuses without backup or replacement. Backup suffixes retry only after a destination-exists refusal, preserve the source permission bits, and propagate every non-collision publication error without retry.
Origin: ADR-0148
Revised-by: ADR-0159, ADR-0258, ADR-0269
Backing: test

### `invariant: local-doc-prune-preserved`

Before prune removes a present outgoing local document, render copies its complete confined file to the first free `.awf-bak` sibling and advances the lock only after backup and removal succeed; unsafe, unreadable, escaping, publication, or removal failures retain the old lock.
Origin: ADR-0272
Backing: test

### `invariant: target-prune-ancestors`

When a selected target-owned path disappears from the output plan and awf re-syncs, it deletes that path and every resulting empty ancestor directory.
Origin: ADR-0148
Revised-by: ADR-0251, ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: uninstall-removes-lock-entries`

awf uninstall removes the in-tree files recorded in the lock and no file outside it, reporting the count it removed. A project-composed recognition policy preserves a present local-document entry through the same confined sibling backup operation before removal.
Origin: ADR-0148
Revised-by: ADR-0272
Backing: test

### `invariant: coverage-evaluation-unconditional`

Full awf check evaluates current-state topic coverage and fan-out in working and staged paths regardless of a currentState block. Core does not load current-state authority or run those governance checks.
Origin: ADR-0192
Revised-by: ADR-introduce-core-and-full-workflow-profiles
Backing: test

### `invariant: profile-config-hash`

The selected profile participates in configuration hashes, while prior lock membership remains the sole pruning authority.
Origin: ADR-introduce-core-and-full-workflow-profiles
Backing: test
