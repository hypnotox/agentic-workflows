---
status: Proposed
date: 2026-07-18
supersedes: []
superseded_by: ""
tags: [audit-rules, commit-conformance, schema-migration, cli-dispatch]
related: [17, 25, 73, 92, 111]
domains: [tooling, config]
---
# ADR-0127: Explicit Audit Ranges and a Single Git Range Parser

## Context

`awf audit` walks the commits reachable from HEAD but not from a base, where the base comes
from the `audit.baseBranch` config key defaulting to `main` (ADR-0017 Decision 5). When HEAD
is already reachable from that base, `Collect` returns an empty range and every history rule
is inert; only the range-independent `uncommitted-changes` rule (ADR-0025) still reports.

That default encodes a branching strategy. On a project whose work lands directly on `main`,
HEAD *is* the base on every run, so the audit silently reports almost nothing while still
exiting cleanly. This is not hypothetical: ADR-0122 reached Implemented in this repo without
its `rendering` and `tooling` current-state narratives being refreshed, and the
`domain-doc-staleness` rule that exists precisely to catch that never fired, because the
range it was evaluated over was empty. The failure is silent by construction, which makes it
worse than a missing rule: the command reports "clean" and the operator reasonably believes
the conformance rules ran.

ADR-0017's own Context anticipated this. It states the audit "cannot live in a pre-push hook
or assume a particular base-branch workflow as policy", and then its Decision 5 shipped
exactly such an assumption as a default. The remedy is to stop guessing: an adopter's
integration branch, release cadence, and review boundary are theirs to name, and awf has no
way to infer them.

There is direct precedent for refusing rather than guessing. `awf context` (ADR-0092
Decision 2) errors on a no-argument invocation with a hint, on the stated grounds that it
"never guesses 'current changes'" because the intended moment differs between brainstorm and
review. The same reasoning applies to a conformance range.

A second, smaller problem surfaced while scoping this. Four sites parse a `<a>..<b>` range
string independently: `cmd/repoaudit` (ADR-0073), `internal/git`, `cmd/awf/changelog`, and
the parsing this change would add. Their rigour differs. `cmd/repoaudit` rejects three-dot
ranges, multi-`..` inputs, empty sides, and `-`-prefixed sides, documenting that
`strings.Cut` mangles the first two into bogus revisions and that a `-`-prefixed side would
reach git as an option-like argument; it also records that dots *inside* a revision
(`v0.10.0`) are legal, since git forbids `.`-leading, `..`-containing, and `-`-leading refs.
`internal/git` performs a bare `strings.Cut` with none of those guards. Adding a fifth
parser would entrench the drift.

## Decision

1. **`awf audit` takes the range as a required positional argument.** The argument is either
   a bare base revision, meaning `<base>..HEAD`, or an explicit `<a>..<b>` range. An argument
   containing `..` is a range; otherwise it is a base. Git forbids `..` inside ref names, so
   the discrimination is unambiguous.

2. **A no-argument invocation is a hard error** printing usage for both accepted forms, in
   the manner of `awf context` (ADR-0092 Decision 2). It does not fall back to a default
   range, and it does not degrade to running only the range-independent rules. An audit that
   silently does almost nothing is a worse failure than one that refuses, which is the defect
   this ADR exists to remove.

3. **The `audit.baseBranch` config key is removed**, along with its `main` default and the
   `--base` flag that overrode it. awf holds no opinion about which branch an adopter
   integrates into. This supersedes ADR-0017 Decision 5's `baseBranch` field.

4. **`uncommitted-changes` (ADR-0025) continues to run on every invocation**, regardless of
   the range argument. It inspects live working-tree state and is orthogonal to the range. It
   stays in `awf audit` rather than moving to `awf check`: `check` renders from the working
   tree and therefore passes in exactly the case the rule targets, a rendered file committed
   while its `.awf/` source stays uncommitted.

5. **One range parser, in `internal/git`.** `internal/git` already centralises awf's go-git
   access "so every awf command that reads git shares one open path"; range parsing joins it.
   The exported parser returns a base and a head, defaulting head to `HEAD` for a bare base,
   and adopts `cmd/repoaudit`'s guards as the shared contract: reject an empty side, a
   three-dot range, a multi-`..` input, and a `-`-prefixed side. All four existing call sites
   converge on it, so `cmd/repoaudit` and `cmd/awf/changelog` change without carrying a
   defect, purely to retire their duplicate implementations.

6. **The terminal review passes its session range.** `awf-reviewing-impl` already derives
   `headSha` and `baseSha` (the commit before the session's first implementation commit) in
   its `sha-range-detection` step, then discards them and audits "over the branch". It now
   passes `<baseSha>..<headSha>`. This is a session range, not a branch range, so it holds
   under any branching strategy. This supersedes ADR-0017 Decision 7's description of the
   invocation.

7. **A schema-11 migration removes the key from an adopter's config.** `.awf/config.yaml` is
   strict-parsed, so a stale `audit.baseBranch` would hard-fail on the new binary rather than
   warn. The migration prints the removal, departing from the silent `applyDropHooks`
   precedent, because deleting a value an adopter deliberately set must be readable from
   command output rather than recovered by git archaeology.

## Invariants

- `` `invariant: audit-requires-explicit-range` ``: `awf audit` with no positional argument
  exits non-zero without evaluating any rule, and its message names both the `<base>` and
  `<a>..<b>` forms.
- `` `invariant: audit-no-base-branch-config` ``: no config key, spec entry, or resolved
  setting supplies an audit base; the range reaches `Collect` only from the command line.
- `` `invariant: git-range-parser-single-definition` ``: exactly one exported range parser
  exists in `internal/git`, and no other package parses a `..` range string.
- `` `invariant: git-range-rejects-malformed` ``: the parser rejects an empty side, a
  three-dot range, a multi-`..` input, and a `-`-prefixed side on either side.
- `` `unbacked-invariant: audit-migration-announces-removal` ``: the schema-11 migration
  prints the removed key when it strips one. **Verify:** run `awf upgrade` on a schema-10
  tree whose `.awf/config.yaml` sets `audit.baseBranch` and read the output.

## Consequences

The audit now reports over a range the caller named, so an empty result means the range was
genuinely clean rather than accidentally empty. The `domain-doc-staleness`,
`domain-code-staleness`, and ADR co-change rules become effective on a trunk-based project,
which is where they were previously inert.

The terminal review improves as a side effect: it audits the session it just reviewed instead
of a branch-shaped approximation, which is both narrower and more accurate.

Ergonomics cost: every invocation must now name a range, including interactive ones. The
release runbook's bare `./x audit` becomes `awf audit <previous-tag>..HEAD`, which is
arguably what a release audit always meant. A worktree-only check remains available as
`awf audit HEAD` (an empty range still runs `uncommitted-changes`), though that is a
consequence of Decision 4 rather than a designed mode.

This is a breaking CLI and config change. awf is pre-1.0 with no external API stability
promise, and the schema-11 migration makes the config side mechanical. An adopter scripting
`awf audit --base <ref>` must update to the positional form; there is no deprecation period.

The migration is undogfooded: neither this repo's config nor `examples/sundial` sets
`audit.baseBranch`, so no sync or check run exercises it. It requires a dedicated fixture
test rather than relying on the adopter tree for coverage.

Converging `cmd/repoaudit` and `cmd/awf/changelog` on the shared parser touches two commands
that have no reported defect. The risk is accepted because the alternative is a fifth
divergent parser, and because the shared contract is strictly the strongest of the existing
four.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `baseBranch`, document the trunk-based caveat | Leaves a silent-failure default in place; a caveat in prose does not stop the audit reporting clean over an empty range. |
| Keep `--base` as an alias for the positional | A redundant second spelling that keeps implying a default base exists, which is the mental model being removed. |
| Two explicit flags (`--base` / `--range`), no positional | Diverges from `cmd/repoaudit`'s established positional convention for the same concept. |
| Bare invocation runs only range-independent rules | Reintroduces a quiet mode whose safety depends on the operator reading output; the silence is the defect. |
| Move `uncommitted-changes` to `awf check` | `check` renders from the working tree, so it passes precisely when an uncommitted source is the problem; it is also gate-wired and runs against dirty trees during normal work. |
| Adopt a feature-branch workflow so the existing default fires | Imposes a branching strategy on the project to satisfy a tool, and does not help any other adopter on trunk. |
| Leave the four range parsers in place | Entrenches known drift, with the weakest parser sitting in the package that owns the git seam. |
