---
status: Implemented
date: 2026-06-29
supersedes: [3]
superseded_by: ""
tags: [git-hooks, schema-migration]
related: [23, 30, 31]
domains: [tooling, rendering]
retires_invariants: [setup-guards-hookspath]
---
# ADR-0032: Remove Automatic Git-Hook Handling

## Context

awf currently does two things with git hooks:

- **Renders** `.githooks/pre-commit` and `.githooks/pre-push` from `templates/hooks/` as a
  first-class `hook` kind (config `hooks:` array, catalog `hooks:` block, `awf add/remove/list
  hook`).
- **Activates** them by reaching into the adopter's git config: `awf init` calls `runSetup`
  ([ADR-0003](0003-binary-delivery-and-setup.md)) which runs `git config core.hooksPath
  .githooks`; `awf setup` is the standalone activation; [ADR-0023](0023-safe-adoption-existing-repos.md)
  then had to add a `--force-hooks` guard, a foreign-`core.hooksPath` refusal, subdirectory
  resolution, and an `awf uninstall` that unsets `core.hooksPath`: all to make that side effect
  safe.

Touching the adopter's git config on first run is the friction. It risks silently hijacking an
existing husky/lefthook setup, which is exactly why ADR-0023 grew the guard machinery. The
dogfooding seam ADR-0003 introduced (`checkCmd`/`gateCmd` so the rendered hook can call `./x`
here but `awf check` for adopters) exists only to make the rendered hook portable. The net cost
of awf *owning* hooks (config mutation, guards, the portability seam, an uninstall step)
outweighs the value. Adopters already know how to install a git hook; awf supplying the gate
commands in their guide is enough.

**Decision in brief:** remove automatic hook handling entirely, both rendering the hook files
and touching git config. Adopters install and manage hooks themselves; awf dogfoods by
hand-maintaining its own `.githooks/`.

Grounding discoveries that shape the design:

- The `hook` kind lives in `kindDescriptors` (`internal/project/kind.go`), with a render loop
  in `internal/project/render.go` (whose error path carries a now-moot `// coverage-ignore`),
  a `Hooks []string` config field (`internal/config/config.go`), and a `Hooks []string` catalog
  field (not a map, unlike skills/agents/docs). `templates/hooks` is named in `embed.go`'s
  `go:embed`. `Project.Sync` (`internal/project/project.go`) has a `.githooks` → `0o755`
  special-case. `cmd/awf/list_add.go` has an `if kind != "hook"` branch.
- `checkCmd` and `gateCmd` are **not** hook-only vars: both are referenced by
  `templates/agents-doc/AGENTS.md.tmpl` and several skill templates. Removing the hook templates
  does not orphan them; they stay.
- The config tree is schema-versioned via `internal/migrate` (current version **3**, a registry
  of migrations; `applyDropReplaceWith` is the precedent for a field-stripping step). `awf check`
  gates on the schema version; `migrate.Current()` stamps the lock.
- Doc sections are declared in `templates/catalog.yaml` (`sections:`) and marked in the template
  with `<!-- awf:section <name> -->...<!-- awf:end -->`; an override part lives at
  `.awf/docs/parts/<doc>/<section>.md`. `awf check` flags an override part whose section is not
  catalog-declared. A non-empty prose default body is publication-safe (ADR-0001).
- `invariant: setup-guards-hookspath` is backed in `cmd/awf/setup.go` and declared by ADR-0023, which
  also declares the live, untouched `inv: init-force-backs-up` and `inv: uninstall-removes-lock-tracked`.
  Retiring the one slug while keeping ADR-0023 `Implemented` uses the
  [ADR-0031](0031-invariant-retirement-via-successor-adr.md) retirement mechanism.

## Decision

1. **Remove the `hook` kind.** Drop the hook descriptor from `kindDescriptors` and the hook
   render loop (with its moot `coverage-ignore`) from `render.go`; this removes `awf
   add/remove/list hook`. Delete the `Hooks` field from the config struct and the catalog, the
   `templates/hooks/` directory and its `hooks:` catalog block, the `templates/hooks` entry in
   `embed.go`, and the `.githooks` → `0o755` special-case in `Sync`. With the `hook` kind gone,
   the now-always-true `if kind != "hook"` guard in `targetState` (`cmd/awf/list_add.go`) is
   simplified away so the unreachable hook branch does not strand a statement under the 100%
   coverage gate.

2. **Remove hook activation.** Delete `cmd/awf/setup.go` (the `setup` subcommand and `runSetup`),
   the `--force-hooks` flag from the `init`/`setup` dispatch, the `runSetup()` call in
   `runInit()`, and all of `cmd/awf/git.go` (`openWorktree`, `localHooksPath`, and
   `writeLocalHooksPath`) since removing `setup.go` (which also carries `awfHooksRel`) and
   `unsetAwfHooks` leaves every function in that file, and its `go-git` import, unreferenced. The
   `go-git` module itself stays in `go.mod`: `internal/audit` still uses it. `awf uninstall`
   continues to remove lock-tracked files and the lock, but no longer touches `core.hooksPath`
   (the `unsetAwfHooks` step is removed).

3. **Migrate the config schema.** Add a schema migration (To:4, "drop-hooks") in
   `internal/migrate` that strips the `hooks:` key from `.awf/config.yaml`; absent key is a
   no-op (idempotent). `migrate.Current()` becomes 4. awf's own `.awf/config.yaml` loses its
   `hooks:` block via this change.

4. **Dogfood with hand-maintained hooks.** awf's `.githooks/pre-commit` and `.githooks/pre-push`
   become plain checked-in files: the generated banner is stripped and they are removed from
   `.awf/awf.lock` (no longer rendered). awf's local `core.hooksPath` is left as-is: it is now
   an adopter-managed setting, not awf-managed. These files are the worked example of an adopter
   that wrote its own hooks.

5. **Add an opt-in customization slot.** Add a catalog-declared optional `local-hooks` section to
   the `workflow.md` doc template, with a short non-empty default body pointing adopters at
   installing their own hooks (publication-safe). Adopters fill it via
   `.awf/docs/parts/workflow/local-hooks.md`; awf itself provides such a part describing its
   hand-maintained hooks.

6. **Retire `setup-guards-hookspath`.** This ADR's frontmatter declares
   `retires_invariants: [setup-guards-hookspath]` (mechanism: ADR-0031). Deleting
   `cmd/awf/setup.go` removes that slug's backing comment while ADR-0023 stays `Implemented`; the
   retirement takes effect when this ADR flips to `Implemented`.

7. **Supersedence scope.** This ADR **fully supersedes ADR-0003**: ADR-0003's entire Decision
   (the `checkCmd` hook-template default, `awf setup`, and `awf init` → setup) is hook
   activation; binary delivery is governed by [ADR-0030](0030-prebuilt-binary-distribution-and-release.md)
   and was only a stated assumption in ADR-0003. ADR-0003's status flips to `Superseded by
   ADR-0032`. It **partially supersedes ADR-0023** (recorded via `related`; ADR-0023 keeps
   `Implemented`): item 2 (`core.hooksPath` guard) and item 3 (subdir resolution for `setup`) go
   away with the `setup` command, and item 4's "unset `core.hooksPath`" clause is dropped
   (`uninstall` now only removes lock-tracked files). ADR-0023's `inv: init-force-backs-up` and
   the lock-tracked-removal substance of `inv: uninstall-removes-lock-tracked` remain in force
   and backed; only the retired slug above is dropped.

## Invariants

- `invariant: hooks-config-dropped`: the schema-4 migration removes the `hooks:` key from a
  `.awf/config.yaml`, and is a no-op on a config that has no `hooks:` key (idempotent).
- awf renders no files under `.githooks/`, and the catalog and config schema declare no `hook`
  kind or `hooks` key (textual contract; verified by the golden render no longer producing
  `.githooks/*` and by the config struct lacking the field).

## Consequences

Easier:
- awf no longer mutates an adopter's git config; the `--force-hooks` guard, the foreign-hooks
  refusal, and the `core.hooksPath` unset in `uninstall` all disappear. Adoption has one fewer
  surprising side effect.
- The render pipeline loses a kind; the CLI loses the `setup` subcommand and a flag.

Harder / accepted trade-offs:
- Adopters who want a pre-commit gate must install a hook themselves (e.g.
  `git config core.hooksPath <dir>` over a hook they author, or a script in `.git/hooks/`). The
  new `local-hooks` workflow section and README adoption guidance cover this; awf's own
  hand-maintained `.githooks/` is the example.
- Existing adopters who previously ran `awf setup` keep a stale `core.hooksPath` pointing at the
  no-longer-rendered `.githooks/`. The migration cannot safely touch git config, so the README
  documents `git config --unset core.hooksPath` (or keeping the now hand-owned files) as a manual
  step. The schema migration only strips the `hooks:` config key.
- The 100% coverage gate: the migration step and the removed branches must leave no uncovered
  statements; the moot `coverage-ignore` in the old render loop is deleted with the loop.

Doc-currency obligations the implementing commit(s) must satisfy:
- `README.md` adoption section is rewritten: remove `awf setup`, `--force-hooks`, and the
  hook-activation/uninstall-unset narrative; add manual hook-installation guidance and the
  `core.hooksPath --unset` note for prior adopters.
- The agent-guide templates (`templates/agents-doc/`, its layout/setup parts) drop the
  "git hooks ... are rendered by awf" / `awf setup` references.
- `docs/architecture.md` drops hooks from the overview, the config-arrays list, the CLI entry
  points (`setup`), and component descriptions.
- The `tooling` domain narrative drops the setup/uninstall-hooks/hook-rendering behaviour; the
  `rendering` domain narrative drops the `hook` kind; the `config` domain narrative drops `hooks`
  from the enable-array enumeration (four arrays, not five) and reflects the schema-4 migration.
- The user-facing kind enumerations drop `hook`: the `awf --help` text (`kind ∈ {...}`) and the
  `unknown kind` error message in `cmd/awf` (`main.go`, `list_add.go`), and the `(kinds: ...)` list
  in the agent-guide template.
- The hand-maintained CLI strings in `cmd/awf/main.go` drop `setup`, `--force-hooks`, the `setup`
  entry in `argSpecs`, the `init` help line's "activate git hooks" phrasing, and the `uninstall`
  help line's "unset core.hooksPath" phrasing.
- ADR-0003's status flip and this ADR's eventual `Implemented` flip each regenerate
  `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep rendering hooks; stop only the auto-activation | Leaves the `hook` kind, the `.githooks/*` output, and the `checkCmd` portability seam: half the friction (a generated dir adopters must wire up anyway) with none of the simplicity of full removal. |
| Keep `awf setup` as an opt-in command, drop only the `init` chaining | Retains all the git-config-mutation code and guards for a command few would run; the user chose full removal. |
| Tolerate-and-ignore the legacy `hooks:` field instead of migrating | Leaves dead config in adopter trees and is dishonest about the breaking change; the schema-version + `awf upgrade` mechanism exists precisely for this. |
| Drop awf's own hooks entirely and rely on CI | Loses the local pre-commit safety net the "green gate before every commit" invariant leans on; hand-maintaining `.githooks/` keeps it and doubles as the adopter example. |
