---
status: Implemented
date: 2026-07-13
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [audit-rules, changelog]
related: [41, 67, 73, 78]
domains: [tooling]
---
# ADR-0107: Downgrade the changelog-conformance audit rule to an advisory Warning

## Context

ADR-0073 added the repo-local changelog-conformance rule (`cmd/repoaudit` / `./x audit-local`) as a
**blocking Error**: over an effort's SHA range, if any commit touches an adopter-facing path and the
`[Unreleased]` section is unchanged, the rule fails with a non-zero exit and blocks the implementation
review. It made the rule blocking deliberately: a prose reminder to add a changelog entry kept failing
on **missed** entries (the failure mode ADR-0073 Context documents), so the rule was promoted onto a
deterministic rung.

That promotion weighed the missed-entry cost but not the symmetric one: **the rule over-fires on
benign changes**. Its detection is a path heuristic (`adopterFacingPrefixes`: `templates/`,
`cmd/awf/`, and the config/lock/catalog packages `internal/config/`, `internal/manifest/`,
`internal/catalog/`); it cannot distinguish a behavioral change from a benign one (a refactor, or a
comment/marker relocation) that touches the same path. A change that alters no adopter output still
trips the Error.

This is not hypothetical. The ADR-0105/0106 invariant-backing migration (2026-07-13) relocated
`// invariant:` markers to `// touches-invariant:` across `cmd/awf/` and `internal/config/` (comment-only
edits, zero adopter-facing behavior) and the rule hard-failed the implementation review (exit 1)
because the effort's changelog entry legitimately lived in a sibling plan's SHA range. A correctly-documented,
behaviorally-benign effort was blocked by a false positive, and the reviewer had to escalate a
non-issue to the user to conclude.

For a heuristic, that cost/benefit is inverted. A false Error blocks legitimate work and demands a
resolve-or-escalate when there is nothing to add; meanwhile the harm ADR-0073 targeted (a genuinely
missed entry) is still caught downstream: the `awf-retrospective` changelog step (ADR-0041/0067)
re-checks `[Unreleased]` at the end of every effort, and `releasecheck` (ADR-0078) refuses a release
whose `[Unreleased]` section is missing or non-empty. The per-effort check earns its keep as an
attention signal, not as a gate.

## Decision

1. **The changelog-conformance verdict becomes a Warning.** In `cmd/repoaudit`, the `changelogRule`
   finding for "an adopter-facing change in the range but `[Unreleased]` is unchanged" is emitted at
   Warning severity, not Error. It still prints in the audit output and surfaces in the
   `awf-reviewing-impl` digest (attention is preserved) but it no longer sets a non-zero exit or
   blocks the review. This **partial-supersedes ADR-0073 Decision item 2** ("emit one Error finding")
   and **item 4** ("Error blocks the review from concluding") for this rule only; ADR-0073 stays
   `Implemented` and gains a `related:` back-pointer.

2. **Infrastructure failures inside the rule stay Errors.** A `git merge-base`/`git diff` failure or a
   `[Unreleased]`-section read failure means the rule **cannot verify conformance** and must fail loud,
   exactly as ADR-0073 intended (its "a git or parse failure fails loud" clause). That is orthogonal to
   the conformance verdict's severity: only the "unchanged `[Unreleased]`" verdict downgrades; the four
   "cannot run" findings remain Errors.

3. **The `repo-audit-error-exit` invariant is preserved; its proof relocates.** ADR-0073's
   `repo-audit-error-exit` (the command exits non-zero iff at least one Error finding) is unaffected:
   the command still exits non-zero on Error findings, now sourced from the infrastructure-failure
   findings (item 2) and the `coverage-ignore-added` rule's error paths rather than the conformance
   verdict. Its proof marker moves from the conformance test (now a Warning/exit-0 case) onto an
   infrastructure-failure test that still produces an Error and a non-zero exit.

## Invariants

- `invariant: changelog-rule-advisory`: `cmd/repoaudit`'s changelog-conformance verdict (an
  adopter-facing change in range with an unchanged `[Unreleased]`) is a Warning that never sets a
  non-zero exit, while a git-or-read failure inside the same rule stays an Error.

## Consequences

- **Benign changes stop blocking implementation review.** A refactor or marker relocation touching an
  adopter-facing path no longer hard-fails the review; the reviewer still sees the Warning and adds an
  entry when one is actually warranted. The reviewer's judgment, not a path heuristic, decides.
- **A genuinely missed entry is no longer caught deterministically at impl review**: this walks back
  part of ADR-0073's prose→blocking-Error move. Mitigations, in order: the Warning keeps it visible at
  the exact review moment; the `awf-retrospective` changelog step (ADR-0041/0067) re-checks it per
  effort; `releasecheck` (ADR-0078) blocks any release on a missing/non-empty `[Unreleased]`. Be honest
  about the strength of these: the retrospective step is a *prose* mechanism, and ADR-0073's Context
  records prose reminders as unreliable at *landing* time: the ADR-0072 entry was missed during the
  effort and recovered only by this catch-net at the end. So the residual is a leaky-at-landing prose
  catch (which nonetheless did catch ADR-0072) plus a release-time *structural* check (`releasecheck`,
  which guards `[Unreleased]` well-formedness, not per-effort entry presence); no deterministic
  per-effort guarantee remains. The accepted trade: a false Error blocking legitimate work is both
  costlier and more frequent than a missed entry surviving a Warning plus those backstops, and the user
  explicitly accepted it.
- **The exit contract is unchanged.** `repo-audit-error-exit` still holds; only which findings source an
  Error changes, and its proof relocates to an infrastructure-failure test: no behavior change to the
  command's exit semantics.
- **Scope is unchanged.** This is repo-local dev tooling (`cmd/repoaudit` / `./x audit-local`), never
  the shipped `awf audit` (ADR-0073's standing scope boundary). No adopter-facing surface changes, so
  no changelog `[Unreleased]` entry is warranted for this ADR, and, fittingly, the downgraded rule
  would only Warning it in any case.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Behavioral detection: keep the Error, but fire only on non-comment/non-rename diffs | A heuristic layered on a heuristic: it needs per-hunk diff classification (comment-only? rename-only?), which is more code and more failure modes. The Warning already delivers the "attention without blocking" the change is after. |
| Narrow the `adopterFacingPrefixes` allowlist | Does not address the core issue: a genuinely adopter-facing file changed benignly (comment-only) still matches its path. The lever is severity, not the path set. |
| Keep the Error but whitelist marker-only changes | Special-casing the audit for one migration's shape is brittle and does not generalize to the next benign change. |
| Move the check to release time only | ADR-0073 already weighed and rejected release-only (too late, not per-effort), and `releasecheck` (ADR-0078) already covers release integrity. Keeping a per-effort *advisory* signal is strictly more than release-only. |
