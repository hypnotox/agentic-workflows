How awf render and awf check detect and report drift: per-file config-hash inputs, managed-output attribution and provenance, foreign-file backups, residue scanning, ancestor pruning, and uninstall cleanup.

## Claims

### `invariant: agent-guide-size-advisory`

GeneratedOutputChecker classifies only the deterministic expected bytes of a managed `AGENTS.md`: at a fixed 12 KiB threshold, an overage is a Warning protecting heuristic quality with zero exit. RepositoryChecker preserves its aggregate-only placement, and non-aggregate consumers remain excluded.
Backing: test

### `invariant: authoring-sync-transaction`

A semantic part edit or reset acquires the complete project lease before mutable authority reads, observes the selected source identity, and validates one candidate overlay through both configuration-tree and project-tree readers before any source effect. It then confines mutation to that observed source, reloads committed authority, and invokes ordinary leased synchronization. Pre-source refusal preserves source, output, and lock bytes; a later failure reports source, setup, publisher, and release effects with residue-first recovery and no rollback claim.
Backing: test

### `invariant: awf-bak-flagged`

A collision-backup file under .awf whose name ends in .awf-bak or .awf-bak.<N>, outside an owned resident root, is reported by awf check as drift with a distinct stale-backup detail rather than passing silently.
Backing: test

### `invariant: catalog-data-in-confighash`

A change to an artifact's catalog default data changes that artifact's lock configHash, so `awf check` reports the artifact stale exactly as it would for a template change.
Backing: test

### `invariant: check-invalid-frontmatter`

awf check reports an invalid-frontmatter drift entry for an on-disk skill or agent file that is otherwise in sync but whose frontmatter is missing, unparseable, or has an empty name or description; a clean synced tree reports no such entry, and at most one drift entry is reported per path.
Backing: test

### `invariant: closed-config-tree`

Every filesystem entry under .awf outside the selected governance footprint's claimed-path model is reported by awf check as orphaned drift, with owned resident roots exempt.
Backing: test

### `invariant: drift-source-set`

Each rendered file's stored ConfigHash projects only that file's effective inputs and selected governance footprint, so awf check reports it stale only when those inputs changed. A sidecar or part matching no selected artifact or target is an orphan.
Backing: test

### `invariant: managed-output-attribution`

A reader-injected declaration builder enumerates the standard footprint's managed writes before rendering, retaining sorted declarers and exact inputs; those declarations are the output plan's generated-path classification.
Backing: test

### `invariant: ordinary-render-freshness`

After an ordinary frozen output's template and config hashes match, awf check compares its current planned render bytes to the locked output hash before observing the worktree or staged bytes. A changed fresh render reports stale output, while an unchanged fresh render attributes only a differing observed output as hand-edited. Regenerated and in-place outputs retain their declared regeneration policy.
Backing: test

### `invariant: provenance-banner`

Every rendered file begins with the awf generated-by banner as its first line, except that it follows a leading construct where one exists: the closing frontmatter delimiter for targets carrying frontmatter, and the shebang line for shell hooks.
Backing: test

### `invariant: regeneration-checked-attribute`

The files excluded from the frozen-output-hash comparison are exactly those a first-class RegenChecked attribute marks on the rendered-file model; the config reference and domain docs carry it, as does every file containing an in-place-editable section, replacing the former hardcoded path list.
Backing: test

### `invariant: residue-exemptions-pinned-three`

The identity-exemption list for the rendered-output residue scan contains exactly three entries: the bootstrap template, the upgrade-script template, and the agents-doc template; extending it requires a successor decision.
Backing: test

### `invariant: scopes-in-confighash`

The resolved commit-scope list folds into the config hash of every artifact whose assembled template references `.commitScopes`, so editing `audit.allowedScopes` flags exactly those artifacts stale in `awf check` while non-referencing artifacts stay in sync.
Backing: test

### `invariant: generated-artifacts-tracked`

Repository and staged drift require every output-plan write and `.awf/awf.lock` in the Git index, independently of ignore rules. An absent indexed path reports blocking `untracked` drift before `missing`; tracked files remain valid when ignored. Repository drift reports a non-failing tracking-unavailable advisory outside Git, while nested adopters exclude resident outputs outside their subtree index authority.
Backing: test

### `invariant: staged-drift-rendered-output`

`awf check staged drift` renders from the staged config and compares staged generated outputs for index membership, stale content, and hand edits. An absent staged output or lock reports blocking `untracked` drift without consulting working-tree bytes; an invalid staged lock remains an operational failure.
Backing: test

### `invariant: sync-mutations-root-confined`

Ordinary render and first adoption discover immutable tracked and resident anchors, acquire the complete canonical lease set before mutable configuration loading and output planning, and hold it through complete or typed partial outcome construction. Publisher retains stable output, backup, prune, and final-lock-last policy; no Preparation mutator can publish a stale plan. Selected root-confined handles observe expected identities and perform exclusive creation, replacement, removal, parent creation, mode correction, backup publication, and empty-ancestor cleanup without changing outside bytes or modes. Every failure after a committed directory, mode, backup, output, prune, cleanup, or lock effect returns and presents that stable effect with a retry or recovery action; a pre-effect failure preserves the tree, and no crash-atomicity is claimed.
Backing: test

### `invariant: sync-backs-up-foreign`

During `awf render`, a target path that already exists on disk but is not recorded as awf-written in the lock at the start of the sync is copied through the selected root-confined handle and complete exclusive publication to a `.awf-bak` sibling and reported before being overwritten, while a path recorded in that lock is overwritten with no backup. One confined open observes source bytes and permission mode. A foreign final symlink is backed up only when that open safely reads its in-root target; an escaping, broken, or unreadable target refuses without backup or replacement. Backup suffixes retry only after a destination-exists refusal, preserve the source permission bits, and propagate every non-collision publication error without retry.
Backing: test

### `invariant: local-doc-prune-preserved`

Before prune removes a present outgoing local document, render copies its complete confined file to the first free `.awf-bak` sibling and advances the lock only after backup and removal succeed; unsafe, unreadable, escaping, publication, or removal failures retain the old lock.
Backing: test

### `invariant: target-prune-ancestors`

When a selected target-owned path disappears from the output plan and awf re-syncs, it deletes that path and every resulting empty ancestor directory.
Backing: test

### `invariant: uninstall-removes-lock-entries`

awf uninstall removes the in-tree files recorded in the lock and no file outside it, reporting the count it removed. A project-composed recognition policy preserves a present local-document entry through the same confined sibling backup operation before removal.
Backing: test

### `invariant: coverage-evaluation-unconditional`

awf check evaluates current-state topic coverage and fan-out in working and staged paths regardless of a currentState block.
Backing: test
