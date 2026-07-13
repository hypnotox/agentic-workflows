---
status: Implemented
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [tooling, adoption, scaffold]
related: [3, 16]
domains: [tooling]
---
# ADR-0023: Safe Adoption Into Existing Repositories

## Context

The first real adoption target is a repository that already has its own agentic workflow — a
hand-authored `AGENTS.md`, a `.claude/` tree, possibly a hooks manager. awf's first-run behaviour is
safe at the edges but has three gaps that bite exactly that adopter, surfaced by the pre-adoption
audit:

- `awf init` correctly refuses to overwrite a pre-existing non-managed file and names the
  collisions, but the only way forward is `--force`, which skips the collision check entirely and
  has `Sync` overwrite every colliding path with no backup, diff, or merge (cmd/awf/main.go
  `runInit`, internal/project Sync `os.WriteFile`). An adopter's hand-authored `AGENTS.md` is
  destroyed irrecoverably.
- `awf setup` runs `git config core.hooksPath .githooks` unconditionally (cmd/awf/setup.go), silently
  repointing an existing husky / lefthook / `core.hooksPath` setup and disabling the adopter's hook
  suite — and `awf init` chains `setup` automatically.
- There is no `awf uninstall`; backing awf out means manually deleting rendered files, pruning dirs,
  and unsetting `core.hooksPath`. "How do I cleanly remove this if it doesn't work out" is a top
  question for any evaluation, and the answer is undocumented.

A fourth, smaller defect: `awf setup` checks for `.git` at the working directory, so running it from
a subdirectory silently skips hook activation rather than resolving the repository root.

ADR-0003 governs binary delivery and `setup`'s `core.hooksPath` activation; ADR-0016 governs the
`awf init` collision pre-flight. This ADR refines both to harden `init`/`setup` for real-world
adoption — closing the destructive-overwrite, hook-hijack, and no-backout gaps, and the related
subdir-invocation defect that makes `setup` skip hook activation when run below the repository root.

## Decision

1. **Back up before a forced overwrite.** `awf init` always computes collisions (it no longer skips
   the check under `--force`). With `--force`, before `Sync` runs, each colliding **non-managed**
   file is copied to `<path>.awf-bak` (replacing any prior `.awf-bak`) and the backup is reported on
   stdout; then the overwrite proceeds. Without `--force`, the refusal is unchanged. A path already
   recorded in the lock (awf-managed) is not a collision and is not backed up — re-sync of awf's own
   output stays clean.

2. **Guard `core.hooksPath`.** `awf setup` reads the repository's current `core.hooksPath` before
   setting it. If it is unset or already `.githooks`, setup proceeds (idempotent, as today). If it
   points anywhere else, setup refuses with a message naming the existing value and the
   `--force-hooks` escape hatch; with `--force-hooks` it proceeds, reporting the value it replaced.
   `awf init` accepts `--force-hooks` and forwards it to the chained `setup`, so a one-command
   forced adoption (`awf init --force --force-hooks`) is possible; without the flag, an adopter's
   existing hooks manager halts init's hook step with an actionable message rather than being
   silently overwritten.

3. **Resolve the repository root.** `awf setup` resolves the git top level via
   `git rev-parse --show-toplevel` rather than checking for `.git` at the working directory, so it
   activates hooks correctly when run from a subdirectory. A directory not inside a git repository
   keeps the existing warn-and-skip behaviour so `awf init` chaining never breaks.

4. **Add `awf uninstall`.** A new subcommand removes every rendered file recorded in `.awf/awf.lock`,
   prunes parent directories left empty, removes the now-stale lock file itself, and unsets
   `core.hooksPath` when it points to `.githooks` (leaving a foreign value untouched). When not
   inside a git repository, or when `core.hooksPath` is unset, the hook step is a no-op; like
   `setup` (item 3) `uninstall` resolves the repository root via `git rev-parse --show-toplevel` so
   it works from a subdirectory. It does **not** delete the adopter's *authored* config — `config.yaml`,
   sidecars, and convention parts under `.awf/` — only the generated lock; so a subsequent `awf sync`
   re-renders cleanly, and a subsequent `awf init --force` correctly treats any new hand-authored
   file as a collision (backing it up) rather than mistaking a stale lock entry for managed output.
   It reports what it removed and that the authored config was left in place. Backing out is a single
   command.

5. **Supersedence scope.** This ADR refines two Implemented ADRs by partial-item supersedence
   recorded through `related` (no full replacement); both predecessors keep their `Implemented`
   status. It narrows **ADR-0003**'s setup behaviour — the textual "unconditional `core.hooksPath`
   activation" and "idempotent" contracts now hold only when the existing value is unset or
   `.githooks`. It refines **ADR-0016 `inv: init-collision-guard`**: under `--force`, `init` now
   computes collisions and backs up rather than skipping the check, so that slug's backing test
   (`TestInitGuardBlocksAndForceOverrides`) is augmented to assert the `.awf-bak` backup alongside
   the overwrite, in the commit that flips this ADR to `Implemented`.

## Invariants

Tagged slugs are backed by tests landing with implementation (enforced by `awf check` once this ADR
is `Implemented`); the bullets below are the mechanically verifiable contracts those tests assert.

- `invariant: init-force-backs-up` — under `--force`, `awf init` copies every colliding non-managed file to
  `<path>.awf-bak` before any managed output overwrites it.
- `invariant: setup-guards-hookspath` — `awf setup` does not overwrite a `core.hooksPath` set to a value
  other than `.githooks` unless `--force-hooks` is passed.
- `invariant: uninstall-removes-lock-tracked` — `awf uninstall` removes exactly the files recorded in the
  lock plus the lock file itself, and unsets `core.hooksPath` only when it points to `.githooks`,
  leaving the authored `.awf/` config (config.yaml, sidecars, parts) intact.

## Consequences

- An adopter can `awf init --force` over an existing `AGENTS.md`/`CLAUDE.md` and recover the original
  from `<path>.awf-bak`; the destructive-overwrite footgun is closed without losing the
  force-the-overwrite affordance.
- An existing hooks manager is preserved by default; the adopter makes a deliberate `--force-hooks`
  choice to hand hooks to awf. This refines ADR-0003's unconditional activation.
- Evaluation risk drops: `awf uninstall` makes adoption reversible in one command, and the flow is
  documentable.
- The CLI surface grows: a new `uninstall` subcommand and two flags (`--force` backup behaviour,
  `--force-hooks`). The accompanying `--help` work (tracked separately) documents them; this ADR
  does not depend on it.
- `.awf-bak` files are a new artifact in an adopter's tree. They are not awf-managed (absent from the
  lock), so `awf check` ignores them and the adopter removes or commits them at will. Only a
  *colliding non-managed* file is backed up: a path already in the lock (awf's own prior output) is
  overwritten without a `.awf-bak`, so re-running `--force` never snapshots awf-managed output. A
  pre-existing `<path>.awf-bak` (a prior backup, or an adopter file that happens to bear that name)
  is replaced; no awf planned output ends in `.awf-bak`, so a backup never collides with a write
  `Sync` performs.
- New behaviour needs coverage to clear the 100% gate: forced-overwrite backup, the `core.hooksPath`
  guard and `--force-hooks`, subdir resolution, and `uninstall`. `git rev-parse`/`git config --get`
  are invoked through the same `exec.Command` seam `setup` already uses. `git config --get
  core.hooksPath` exits non-zero with empty output when the key is unset — the guard treats that as
  *unset* (proceed), distinct from a real git failure — and any genuinely-unreachable git-exec error
  branch is excluded with `// coverage-ignore: <reason>` rather than contrived in a test.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `tooling` domain narrative gains the backup, hooks-guard, and uninstall behaviours.
- README/adoption docs describe the existing-repo flow and backout (tracked as adoption-doc work).
- The CLI dispatch usage string in `cmd/awf/main.go` (`usage: awf <…>`) gains `uninstall`, and the
  `add`/dispatch surface learns the new subcommand plus the `--force-hooks` flag, in the implementing
  commit; that usage line is hand-maintained, not rendered.
- No `docs/decisions/README.md` index row is owed (the README is a how-to guide; `ACTIVE.md` is the
  generated index — ADR-0005), matching ADR-0003/0016.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Interactive prompts on collision / hooks | Breaks non-interactive, scriptable `init`; a backup + an explicit flag are deterministic. |
| `--force` overwrites with no backup (status quo) | Destroys a hand-authored `AGENTS.md` irrecoverably — the exact first-adopter footgun. |
| Refuse a foreign `core.hooksPath` with no escape hatch | An adopter who genuinely wants awf to own hooks needs a way through; `--force-hooks` is that. |
| `awf uninstall` also deletes `.awf/` | `.awf/` is the adopter's authored config, not generated output; deleting it loses customisations. Leave it and say so. |
| Per-file selective `--force` | More surface than the first adoption needs; a blanket backup-then-overwrite is recoverable and simpler. |
