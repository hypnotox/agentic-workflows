Always-on and toggleable singleton outputs: ADR-system files, bootstrap and hook payloads, resident-root gitignores, and executable-mode rules. The commit-msg payload remains a thin delegate to the profile-selected commit-message gate: Core checks shared commit rules, while Full also enforces stale-ADR merge authorization. Pre-merge-commit remains a thin staged check because Git has not exposed the final message and parents at that earlier hook.

## Claims

### `invariant: adr-system-singletons-rendered`

A Full render emits docs/decisions/README.md and docs/decisions/template.md from its selected ADR-system singletons; Core does not emit them.
Origin: ADR-0148
Revised-by: ADR-0251, ADR-0278
Backing: test

### `invariant: bootstrap-config-tree-path`

When the bootstrap singleton is enabled it renders at .awf/bootstrap.sh, and no rendered output path is the retired repo-root awf-bootstrap.sh location.
Origin: ADR-0148
Backing: test

### `invariant: bootstrap-two-files`

With the bootstrap singleton enabled, exactly two files render under it, `.awf/bootstrap.sh` and `.awf/upgrade.sh`, and no third file joins the unit.
Origin: ADR-0148
Backing: test

### `invariant: hook-payloads-rendered`

Exactly five payloads always render at .awf/hooks/pre-commit.sh, .awf/hooks/commit-msg.sh, .awf/hooks/pre-push.sh, .awf/hooks/pre-merge-commit.sh, and .awf/hooks/reference-transaction.sh.
Origin: ADR-0148
Revised-by: ADR-0202, ADR-0228, ADR-0253
Backing: test

### `invariant: commit-policy-hook-payloads`

The rendered reference-transaction payload buffers each prepared transaction and checks the deduplicated commit union introduced to local branches before refs move: existing branches use their old tips, new branches use the configured integration branch's local pre-transaction tip, and deletions and backward-only updates contribute none. It resolves required integration evidence from the exact local branch without remote or same-transaction fallback. The rendered pre-push payload buffers every update and checks the deduplicated commit union introduced by the push: existing refs use their advertised remote tips, new commit-bearing refs use a freshly resolved destination integration-branch tip, recursively peeled tags contribute commits, and deletions contribute none. It invokes the configured project gate only after policy succeeds. Both payloads remain inert until adopter-owned wiring activates them, resolve policy from the invoking worktree, and fail closed on malformed or unresolvable required evidence without rewriting history.
Origin: ADR-0228
Revised-by: ADR-0315, ADR-0316
Backing: test

### `invariant: memory-gitignore-always-on`

Every render declares a self-ignoring `.gitignore` for exactly the three repository-wide resident roots awf owns: efforts, worktrees, and effort-archive. Only each root marker is governed; dynamic descendants, including archive descendants, are preserved without recursive interpretation, and no render reintroduces a standalone memory root.
Origin: ADR-0148
Revised-by: ADR-0159, ADR-0164, ADR-0167, ADR-0175, ADR-0259
Backing: test

### `invariant: plain-singleton-via-renderkind`

Every catalog document that declares its own output path and is neither the agents document nor generated output renders once to its catalog-derived fixed path with its catalog TemplateID and nonempty content through the shared plainSingletons table and the common renderKind path rather than a hand-rolled per-kind loop.
Origin: ADR-0148
Revised-by: ADR-0169, ADR-0170, ADR-0171, ADR-0172, ADR-0251
Backing: test

### `invariant: shebang-rendered-executable`

A rendered file whose content begins with a shebang is written with executable mode 0755 and every other rendered file with 0644; the mode follows the shebang predicate and is re-enforced on every sync, correcting a pre-existing file's mode rather than only setting it at creation.
Origin: ADR-0148
Backing: test

### `invariant: singleton-kinds-complete`

The runner is a dedicated config-tree render block rather than a catalog docs entry, so it is excluded from the singleton-kind set, and the unified-doc-model completeness check asserts that set equals exactly the catalog documents declaring their own output paths.
Origin: ADR-0148
Revised-by: ADR-0251
Backing: test

### `invariant: resident-output-preservation`

The output plan preserves the effort, managed-worktree, and effort-archive resident roots and their dynamic descendants at the primary control root. Only each root marker is managed; archive descendants remain local and unmanaged.
Origin: ADR-0167
Revised-by: ADR-0175, ADR-0259, ADR-0303
Backing: test
