Always-on and toggleable singleton outputs: ADR-system files, bootstrap and hook payloads, resident-root gitignores, and executable-mode rules.

## Claims

### `invariant: adr-system-singletons-rendered`

A full render emits docs/decisions/README.md and docs/decisions/template.md from their always-on singletons, and omits either one when its sidecar sets local: true.
Origin: ADR-0148
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

With the hooks singleton enabled, exactly three payloads render at .awf/hooks/pre-commit.sh, .awf/hooks/commit-msg.sh, and .awf/hooks/pre-push.sh; with it absent or disabled, no path under .awf/hooks/ renders.
Origin: ADR-0148
Backing: test

### `invariant: memory-gitignore-always-on`

Every render declares a self-ignoring `.gitignore` for exactly the three repository-wide resident roots: efforts, memory, and worktrees. Only each root ignore file is governed; dynamic descendants are preserved.
Origin: ADR-0148
Revised-by: ADR-0159, ADR-0164, ADR-0167
Backing: test

### `invariant: plain-singleton-via-renderkind`

Every catalog document marked Mandatory that is neither the agents document nor generated output renders to its catalog-derived fixed path and content through the shared plainSingletons table and the common renderKind path rather than a hand-rolled per-kind loop.
Origin: ADR-0148
Revised-by: ADR-0169, ADR-0170
Backing: test

### `invariant: shebang-rendered-executable`

A rendered file whose content begins with a shebang is written with executable mode 0755 and every other rendered file with 0644; the mode follows the shebang predicate and is re-enforced on every sync, correcting a pre-existing file's mode rather than only setting it at creation.
Origin: ADR-0148
Backing: test

### `invariant: singleton-kinds-complete`

The runner is a dedicated config-tree render block rather than a catalog docs entry, so it is excluded from the singleton-kind set, and the unified-doc-model completeness check asserts that set equals exactly the mandatory doc entries.
Origin: ADR-0148
Backing: test

### `invariant: resident-output-preservation`

The output plan preserves exactly the effort, memory, and managed-worktree resident roots and their dynamic descendants at the primary control root. Generation 21 destructively removes only the obsolete metrics and assignments roots.
Origin: ADR-0167
Backing: test
