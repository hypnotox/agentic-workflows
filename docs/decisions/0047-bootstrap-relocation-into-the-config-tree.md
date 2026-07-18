---
status: Implemented
date: 2026-07-02
tags: [bootstrap-porcelain, config-tree]
related: [9, 23, 32, 35, 40]
domains: [rendering, tooling]
---
# ADR-0047: Bootstrap relocation into the config tree

## Context

[ADR-0040](0040-self-pinning-rendered-bootstrap.md) Decision item 1 renders
`awf-bootstrap.sh` as a neutral **repo-root** artifact. The adopter-floor analysis flagged
the result as a trust problem: a default-enabled, self-downloading shell script lands at
every adopter's repo root with no adjacent explanation: the most alarming possible
placement for the least-explained file awf emits. The user's direction: move it out of the
root so it stops polluting the top level.

Grounding discoveries that shape the design:

- The output path is a single literal in `RenderAll` (`internal/project/render.go:263`);
  every scan exclusion keys on the template id `bootstrapTID`
  (`internal/project/render.go:21`, `check.go:200-201`), not the path, so relocation
  cannot re-include the script in the dead-reference or skill-reference scans.
- Migration is free: `SyncReport`'s prune loop removes any prior-lock path no longer
  produced (`internal/project/project.go:161-178`), so the old root file disappears on an
  adopter's next `awf sync`; `awf uninstall` is lock-driven and needs no change. Before
  that sync, `awf check` reports the old path as one `orphaned` drift: coherent, and
  `awf sync` resolves it.
- `PlannedOutputs` derives from `RenderAll`, so the new path automatically joins init's
  collision preflight and sync's foreign-backup logic (ADR-0023/ADR-0035) with no code.
- Rendered files are written `0o644` with no chmod anywhere (`project.go:147`): the
  `./awf-bootstrap.sh` invocation style never worked on a fresh render. The documented
  invocation must be `bash .awf/bootstrap.sh`; ADR-0032 deliberately removed awf's last
  file-mode special-case, and this ADR does not reintroduce one.
- `docs/workflow.md` and other managed rendered markdown are subject to the
  dead-reference scan (ADR-0020): an inline markdown *link* to the bootstrap path fails
  `awf check` wherever bootstrap is disabled (including this repo, which builds from
  source). Prose must reference the path in code-spans only.
- ADR-0040's two tagged invariants (`bootstrap-pin`, `bootstrap-checksum`) are behavioral;
  no slug encodes the path, so nothing retires: only the backing test's path assertion
  moves.
- The orphan scanner walks only `.awf/<kind>/` and `.awf/<kind>/parts/` directories, so a
  file at `.awf/bootstrap.sh` can never be misflagged as an orphan sidecar or part.

## Decision

1. **Render the bootstrap at `.awf/bootstrap.sh`** (supersedes
   [ADR-0040](0040-self-pinning-rendered-bootstrap.md) Decision item 1, `refines: ADR-0040#1`; recorded via
   `related`, ADR-0040 stays `Implemented`: its self-pinning and checksum items are
   placement-independent and remain in force). The `awf-` filename prefix is dropped as
   redundant inside awf's own directory. The template file and `bootstrapTID` keep their
   names: the embedded source path is not adopter-visible.

2. **`.awf/` holds rendered awf-owned tooling alongside authored config.** The tree
   ([ADR-0009](0009-tree-based-config-layout.md)) was authored-input-only apart from the
   generated lock; the bootstrap joins the lock on the generated side. The distinction is
   load-bearing for `awf uninstall`: lock-tracked files under `.awf/` (the bootstrap) are
   removed, authored files (config, sidecars, parts) are kept.

3. **Documented invocation is `bash .awf/bootstrap.sh`**: hooks and CI capture the
   printed path, e.g. `"$(bash .awf/bootstrap.sh)" check`. Rendered files stay `0o644`;
   no file-mode special-case returns ([ADR-0032](0032-remove-automatic-hook-handling.md)).

4. **Rendered prose references the path in code-spans only, never as a markdown link**:
   a link target that exists only when bootstrap is enabled would fail the dead-reference
   scan in every project that disables it.

5. **Docs travel with the change.** The implementing commits update every surface
   enumerated under Consequences, and the commit that flips this ADR's status regenerates
   `docs/decisions/ACTIVE.md` via `./x sync`.

## Invariants

- `invariant: bootstrap-config-tree-path`: when enabled, the bootstrap renders at
  `.awf/bootstrap.sh` and no rendered output path is `awf-bootstrap.sh` (the retired
  root location).
- ADR-0040's `bootstrap-pin` and `bootstrap-checksum` invariants continue to hold at the
  new path (their backing tests assert the relocated output).
- No awf-managed rendered markdown carries an inline markdown link to the bootstrap path
  (textual contract; the dead-reference scan enforces it wherever bootstrap is disabled).

## Consequences

Easier:
- The repo root gains nothing from awf beyond `AGENTS.md` and the adapter bridge; the
  script sits in the directory that already marks awf ownership, next to the lock.
- Existing adopters migrate by running `awf sync` once after upgrading: the prune loop
  removes the root file; until then `awf check` shows a single `orphaned` drift.

Harder / accepted trade-offs:
- Every live surface naming the old path moves in the implementing commits: the render
  literal and the Go comments naming the rendered path (`render.go`, `check.go:199`,
  `config.go:69`), the `awf list` bootstrap row label
  (`cmd/awf/list_add.go:261`), the path assertions in `bootstrap_test.go` and
  `list_add_test.go`, awf's own AGENTS.md `awf-setup` part and invariants bullet, and the
  `rendering`/`tooling` domain narratives ("repo-root singleton"). Frozen ADR and plan
  text keeps the old name as historical record.
- Old binaries keep rendering to the root until upgraded: no config-schema surface
  changes, so nothing gates; acceptable for a pre-1.0 tool with the binary-version gate
  ([ADR-0039](0039-binary-version-compatibility-gate.md)) nudging upgrades.
- The `bash` invocation is one word longer than a direct execution; the alternative
  (shipping an executable bit) would reintroduce the mode special-case ADR-0032 removed.
- Adopter-authored references to the old path (hook scripts, CI steps, wrappers capturing
  `"$(bash awf-bootstrap.sh)"`) are outside awf's rendering surface: `awf sync` removes
  the old file but cannot rewrite what points at it, so each adopter edits those by hand
  when they upgrade.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep at repo root, fix with documentation alone | The trust problem is placement, not only prose; an unexplained root script alarms before any doc is read. |
| `scripts/awf-bootstrap.sh` | Creates or co-opts a top-level directory outside awf's namespace; `.awf/` already marks ownership. |
| Ship the script executable (mode special-case) | Reintroduces exactly the file-mode machinery ADR-0032 removed; `bash` invocation costs one word. |
| Rename the template/tid to match | Buys nothing adopter-visible; touches `embed.go` and the tid-keyed coverage test for free churn. |
| Default-disable the bootstrap instead of relocating | Forfeits the self-pinning win ([ADR-0040](0040-self-pinning-rendered-bootstrap.md)'s root fix) for every adopter who never finds the toggle; the trust objection is the placement, not the artifact's existence. |
