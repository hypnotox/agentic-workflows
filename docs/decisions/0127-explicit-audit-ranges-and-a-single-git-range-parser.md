---
status: Proposed
date: 2026-07-18
supersedes: []
superseded_by: ""
tags: [audit-rules, commit-conformance, schema-migration, cli-dispatch]
related: [17, 25, 73, 92, 111, 120, 122]
domains: [tooling, config, rendering]
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

A second, smaller problem surfaced while scoping this. Three sites parse a `<a>..<b>` range
string independently today: `cmd/repoaudit` (ADR-0073), `internal/git`, and
`cmd/awf/changelog`; the parsing this change needs would be a fourth. Their rigour differs. `cmd/repoaudit` rejects three-dot
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
   integrates into. This **partial-supersedes ADR-0017 Decision item 5**
   (`supersedes: ADR-0017#5`) for its `baseBranch` field only; ADR-0017 stays live, and its
   `related:` gains 127 in the same commit as this ADR.

4. **`uncommitted-changes` (ADR-0025) continues to run on every invocation**, regardless of
   the range argument. It inspects live working-tree state and is orthogonal to the range. It
   stays in `awf audit` rather than moving to `awf check`: `check` renders from the working
   tree and therefore passes in exactly the case the rule targets, a rendered file committed
   while its `.awf/` source stays uncommitted.

5. **One range parser, in `internal/git`.** `internal/git` already centralises awf's go-git
   access "so every awf command that reads git shares one open path"; range parsing joins it.
   The exported parser returns a base and a head, defaulting head to `HEAD` for a bare base,
   and adopts `cmd/repoaudit`'s guards as the shared contract: reject an empty side, a
   three-dot range, a multi-`..` input, and a `-`-prefixed side. All three existing call
   sites converge on it, so `cmd/repoaudit` and `cmd/awf/changelog` change without carrying
   a defect, purely to retire their duplicate implementations. Bare-base acceptance is
   opt-in per caller rather than universal: `cmd/repoaudit` rejects a *supplied* bare base
   today and keeps that contract, so convergence does not silently widen what it accepts.

6. **The terminal review passes its session range.** `awf-reviewing-impl` already derives
   `headSha` and `baseSha` (the commit before the session's first implementation commit) in
   its `sha-range-detection` step, then discards them and audits "over the branch". It now
   passes `<baseSha>..<headSha>`. This is a session range, not a branch range, so it holds
   under any branching strategy. This **partial-supersedes ADR-0017 Decision item 7**
   (`supersedes: ADR-0017#7`) for its description of the invocation only.

7. **A schema-11 migration removes the key from an adopter's config.** `.awf/config.yaml` is
   strict-parsed, so a stale `audit.baseBranch` would hard-fail on the new binary rather than
   warn. The migration prints the removal, departing from the silent `applyDropHooks`
   precedent, because deleting a value an adopter deliberately set must be readable from
   command output rather than recovered by git archaeology. The mechanism is a new nested
   remover in `internal/config`, a sibling to `SetMappingScalar`: the existing `RemoveKey`
   walks only top-level mapping entries and so cannot reach a key under `audit:`. Config
   serialization stays owned by `internal/config` (ADR-0026). An `audit:` mapping left empty
   by the removal is dropped rather than kept as an empty mapping, so the migration leaves no
   vestigial key behind.

8. **The rendered sources that invoke or describe the audit are updated in the same commit.**
   `.awf/docs/parts/releasing/content.md` (whose `./x gate && ./x check && ./x audit` becomes
   an erroring invocation), `.awf/agents-doc.yaml` (which describes the audit as reporting
   "over the branch's commits"), and `templates/skills/reviewing-impl/SKILL.md.tmpl` (whose
   `run-audit` step Decision 6 changes), and `.awf/docs/parts/development/command-runner.md`
   (whose `./x audit-local [range]` row documents the `origin/main..HEAD` default Decision 11
   removes) are edited at their sources and re-rendered via
   `awf sync`, per the docs-travel-with-the-change invariant. The re-render reaches every
   enabled target tree and `examples/sundial`, so those outputs land in the same commit.

9. **Every run states what it evaluated, not just its verdict.** The audit reports the
   resolved range and the number of commits in it on every invocation, clean or not, so a
   verdict is never readable without its scope. A bare `awf audit: clean` cannot distinguish
   "evaluated forty commits and found nothing" from "evaluated nothing", and that ambiguity
   is the defect this ADR exists to remove; requiring an explicit range does not by itself
   fix it, because a mistyped or wrongly-scoped range resolves cleanly and still reports
   clean.

10. **An empty resolved range announces itself.** When the range resolves to zero commits,
    the audit says so distinctly instead of printing the clean line, naming the range and
    stating that no history rule was evaluated. It still exits zero and still yields zero
    findings, so ADR-0017's `audit-empty-range-clean` invariant survives intact and is not
    retired. The range-independent rules (Decision 4) still run and can still report.

11. **`cmd/repoaudit` loses its default base too.** It currently falls back to
    `origin/main..HEAD` when given no argument, which is the same guess-the-base defect in
    repo-local clothing: a no-argument call reports over a range nobody named. It requires an
    explicit range on the same terms, so the safety property holds uniformly rather than
    stopping at the shipped/local boundary. Its caller already passes one: the
    `reviewing-impl` convention part invokes `./x audit-local ${baseSha}..${headSha}`.

## Invariants

Backed slugs carry their `// invariant: <slug>` proof markers on `*_test.go` files (this
repo sets `invariants.testGlobs` to `**/*_test.go`) in `internal/git` (the parser slugs),
`cmd/awf` (the CLI refusal, the evaluated-scope line, and the empty-range notice),
`cmd/repoaudit` (its own refusal, Decision 11), and `internal/configspec` (the
no-base-config assertion, which carries its single marker there while asserting across the
config field, the spec entry, and the resolved settings).

- `` `invariant: audit-requires-explicit-range` ``: `awf audit` with no positional argument
  exits non-zero without evaluating any rule, and its message names both the `<base>` and
  `<a>..<b>` forms.
- `` `invariant: audit-no-base-branch-config` ``: no config key, spec entry, or resolved
  setting supplies an audit base; the range reaches `Collect` only from the command line.
- `` `invariant: git-range-parser-single-definition` ``: no non-test `.go` file outside
  `internal/git/` splits a range string on a `".."` separator; a test walks the module's
  non-test sources and asserts the absence, so a fourth parser cannot reappear unnoticed.
- `` `invariant: git-range-rejects-malformed` ``: the parser rejects an empty side, a
  three-dot range, a multi-`..` input, and a `-`-prefixed side on either side.
- `` `invariant: audit-reports-evaluated-scope` ``: every `awf audit` run prints the resolved
  range and its commit count, so no verdict is emitted without the scope that produced it.
- `` `invariant: audit-empty-range-announced` ``: a range resolving to zero commits prints a
  distinct notice naming the range and stating that no history rule ran, and never the bare
  clean line. ADR-0017's `audit-empty-range-clean` still holds alongside it: the run yields
  zero findings and exits zero.
- `` `invariant: repoaudit-requires-explicit-range` ``: `cmd/repoaudit` invoked with no range
  argument exits non-zero without evaluating any rule.
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
arguably what a release audit always meant.

An earlier draft of this ADR advertised `awf audit HEAD` as a worktree-only check, since an
empty range still runs `uncommitted-changes`. That advice is withdrawn: it is precisely the
misplaced call Decisions 9 and 10 exist to make visible, and promoting it would teach the
false-success pattern the rest of this ADR removes. Such a call still works, but it now
announces that it evaluated no history.

This is a breaking CLI and config change. awf is pre-1.0 with no external API stability
promise, and the schema-11 migration makes the config side mechanical. An adopter scripting
`awf audit --base <ref>` must update to the positional form; there is no deprecation period.

The migration is undogfooded: neither this repo's config nor `examples/sundial` sets
`audit.baseBranch`, so no sync or check run exercises it. It requires a dedicated fixture
test rather than relying on the adopter tree for coverage.

The empty-range notice is deliberately not an Error. Making it one would give automation a
non-zero exit to trip over, but it would contradict ADR-0017's `audit-empty-range-clean`
invariant and force its retirement under ADR-0120 for a reporting improvement. The accepted
residual risk is therefore explicit: a caller reading only the exit code still sees success
on an empty range. Decision 9's always-on scope line is what closes that gap for a human or
an agent reading output, and the terminal review reads the digest rather than the exit code.

The tightening reaches beyond `awf audit`. `internal/git`'s bare `strings.Cut` backs
`awf context --range <a>..<b>` (ADR-0092), so a three-dot range or a `-`-prefixed side that
parses today begins to error there. That is a behaviour change on a second command, and a
deliberate one: those inputs reach git as bogus revisions or option-like arguments either
way, so the failure moves earlier and reports better.

Converging `cmd/repoaudit` and `cmd/awf/changelog` on the shared parser touches two commands
that have no reported defect. The risk is accepted because the alternative is a fourth
divergent parser, and because the shared contract is the strictest of the three on every
guard while preserving each caller's existing arity (bare-base acceptance is opt-in, so
`cmd/repoaudit` keeps rejecting a supplied bare base).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `baseBranch`, document the trunk-based caveat | Leaves a silent-failure default in place; a caveat in prose does not stop the audit reporting clean over an empty range. |
| Keep `--base` as an alias for the positional | A redundant second spelling that keeps implying a default base exists, which is the mental model being removed. |
| Two explicit flags (`--base` / `--range`), no positional | Diverges from `cmd/repoaudit`'s established positional convention for the same concept. |
| Bare invocation runs only range-independent rules | Reintroduces a quiet mode whose safety depends on the operator reading output; the silence is the defect. |
| Empty range as a blocking Error | Strongest signal, but contradicts ADR-0017's `audit-empty-range-clean` invariant and would force its retirement for what is a reporting fix. |
| Report scope only when the range is empty | A wrongly-scoped but non-empty range (one commit where twelve were meant) would still read as a confident clean. |
| Leave `cmd/repoaudit`'s default base in place | Keeps the guess-the-base defect alive in repo-local tooling, so the safety property would stop at the shipped/local boundary for no principled reason. |
| Move `uncommitted-changes` to `awf check` | `check` renders from the working tree, so it passes precisely when an uncommitted source is the problem; it is also gate-wired and runs against dirty trees during normal work. |
| Adopt a feature-branch workflow so the existing default fires | Imposes a branching strategy on the project to satisfy a tool, and does not help any other adopter on trunk. |
| Leave the three range parsers in place | Entrenches known drift and adds a fourth, with the weakest parser sitting in the package that owns the git seam. |
