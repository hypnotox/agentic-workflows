---
status: Implemented
date: 2026-07-12
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, context, domains, testing]
related: [14, 77, 86, 92, 94]
domains: [tooling]
---
# ADR-0102: Domain-Coverage Report Mode via awf context --uncovered

## Context

Domains (ADR-0014) are awf's coarse territory model: each domain declares `paths`
globs, and every domain-derived signal — the per-domain ADR index, the
domain-code-staleness audit (ADR-0077), and the ADR/pitfall relatedness that
`awf context <paths>` (ADR-0092) surfaces — is only as complete as that glob
coverage. Coverage is entirely adopter configuration, and **nothing surfaces the
gaps**. A repository can leave whole packages owned by no domain, silently, and
the only symptom is thin context for those paths.

awf itself demonstrates the problem: the five configured domains cover
`internal/adr`; `internal/config|migrate|manifest`; `internal/invariants`;
`internal/render`, `internal/catalog`, and top-level `templates/`; and `cmd/`,
`internal/audit|coverage|changelog|evals`, plus the top-level `x` runner — but
**not** several `internal/` packages (e.g. `internal/project`, `internal/plan`,
`internal/frontmatter`, `internal/pathglob`, `internal/git`, `internal/clispec`,
`internal/configspec`). Querying
`awf context internal/project/placeholders.go` today returns bare invariant slugs
and zero related ADRs, because the file belongs to no domain. The gap is invisible
until someone runs a query and notices the silence.

`awf context <paths>` already computes, per queried path, whether any domain glob
matches it (`ContextResult.Unowned`). What is missing is the **inverse, whole-tree**
question an adopter needs while configuring domains: *which tracked paths does no
domain own?* That is a coverage report, not a per-path lookup — a signal an adopter
runs on demand to decide where new domains are warranted.

## Decision

1. **Add an `--uncovered` mode to `awf context`.** It reports the repository's
   git-tracked paths that are matched by **no** configured domain `paths` glob —
   the inverse of the domain-ownership test `awf context <paths>` already performs.

2. **Argument contract in `--uncovered` mode.** Positional arguments are optional
   **scan-root directories** that restrict the report to tracked paths beneath them;
   with none given the scan root is the repository root. Matching is on
   slash-separated path-segment boundaries (a directory subtree), not raw string
   prefixes — `internal/git` scans that directory's subtree and never a sibling like
   `internal/gitlab`. `--staged` and `--range` (which resolve *changed* paths, a
   different intent) are rejected in this mode with a usage error. This inverts the
   normal-mode contract, where at least one path or selector is required.

3. **Scanned set is git-tracked files at HEAD.** A new read-only helper in
   `internal/git` lists the repository's tracked paths (walking the HEAD tree, a
   sibling to the existing `treeAt`), returned as clean slash-separated
   repo-relative paths. Untracked and ignored files are out of scope: tracked state
   is the deterministic, meaningful set, consistent with how `ChangedPaths` and the
   domain-code-staleness audit already reason in tracked paths.

4. **Collapse fully-uncovered directories.** A directory all of whose scanned
   tracked descendants are uncovered is reported as that single topmost directory,
   not as its individual files — mirroring the closed-config-tree collapse of
   ADR-0086. An uncovered file in an otherwise-covered directory is reported
   individually. The report is therefore a short, actionable mix of subtree
   directories and stray files, not an exhaustive file dump.

5. **Preserve `awf context`'s existing contracts.** `--uncovered` runs entirely
   under the command's read-only entry point (no writer dependency; ADR-0092's
   `context-read-only`), degrades to the static pre-adoption notice outside an
   adopted tree via the same fallback branch (ADR-0092's `context-static-fallback`),
   and its human and `--json` renderings derive from one assembled result.

6. **Advisory, not gating.** `--uncovered` is a pull report the adopter runs; it is
   not an audit rule and never affects an exit code. A project with zero configured
   domains reports its whole tracked tree as uncovered (collapsed to the root),
   which is correct — nothing is owned.

## Invariants

- `invariant: uncovered-lists-unowned-only` — In `--uncovered` mode the reported entries
  cover exactly the scanned tracked paths (under the given scan roots, or the whole
  tree) matched by no configured domain `paths` glob: every such uncovered path is
  represented by exactly one reported entry — itself or a reported ancestor
  directory — and no path owned by a domain is represented by any entry. (Whether an
  entry is a file or a collapsed directory is the separate
  `uncovered-collapses-directories` contract.)
- `invariant: uncovered-collapses-directories` — A directory all of whose scanned tracked
  descendants are uncovered is reported as that topmost directory, never as its
  individual descendant files.
- `invariant: uncovered-output-parity` — The human and `--json` renderings of
  `--uncovered` report the same uncovered set, because both derive from one
  assembled result (the `--uncovered`-mode analogue of `context-output-parity`).

The mode's read-only and static-fallback properties are the existing
`context-read-only` and `context-static-fallback` contracts of ADR-0092, unchanged;
this ADR adds no separate slug for them.

## Consequences

- Adopters gain an on-demand signal for where domains are missing, closing the loop
  on domain-coverage completeness that ADR-0014's model never surfaced.
- awf dogfoods it: the report is how the currently-unowned `internal/*` packages get
  found and given domains, which in turn strengthens every domain-derived signal.
- Implementation surfaces beyond the assembly, all updated in the same commit: a
  small new read-only helper in `internal/git` (tracked-path listing at HEAD); the
  `context` `CommandSpec` in `internal/clispec` (register `--uncovered`, extend the
  help/usage with the mode and its rejection of `--staged`/`--range`; ADR-0094); and
  the `awf context` entry in `docs/working-with-awf.md`. Normal-mode
  `awf context <paths>` behavior is unchanged.
- The `--json` output grows a coverage-report shape (a new mode result). Pre-1.0,
  a schema addition is acceptable.
- Deliberately *not* an audit rule: coverage is adopter judgment (many repos rightly
  leave docs, CI config, or vendored trees unowned), so a warning would be noisy and
  coercive. Promoting `--uncovered` to an opt-in audit rule later is possible and out
  of scope here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| An audit rule that warns on uncovered tracked files | Coverage completeness is adopter judgment, not a universal invariant; a warn would fire on legitimately-unowned trees (docs, CI, vendored code) and pressure over-configuration. A pull report the adopter runs on demand fits the intent; an opt-in audit can follow later. |
| A separate `awf coverage` command | The report is the exact inverse of `awf context`'s domain-ownership resolution and reuses its machinery and its read-only / static-fallback / output-parity contracts; a mode keeps the one domain-resolution surface cohesive rather than forking a near-duplicate command. |
| Scan the working-tree filesystem instead of git-tracked paths | Tracked-at-HEAD is deterministic and excludes build artifacts and untracked scratch; it matches how `ChangedPaths` and the domain-code-staleness audit already define the relevant path set. |
