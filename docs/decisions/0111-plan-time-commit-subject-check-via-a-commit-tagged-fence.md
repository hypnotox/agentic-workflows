---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [commit-gate, plan-artifact, plan-taxonomy]
related: [36, 70, 98, 108]
domains: [adr-system, rendering, tooling]
---
# ADR-0111: Plan-time commit-subject check via a commit-tagged fence

## Context

Plans under `docs/plans/` write the exact commit subject for each phase's closing commit into
a fenced code block inside the "Verify and commit" task — for example
`docs/plans/2026-07-13-trim-plan-header-taxonomy-to-three-fields.md` holds
`feat(rendering): trim plan header taxonomy to three fields (ADR-0108)` in a bare ` ``` ` fence.
Plan reviews recurrently flag these planned subjects as over the length limit, and the agent
then rewrites them by hand. This is a mechanical, deterministic property being caught by eye.

The irony is that awf *already* enforces commit-subject conformance — but only at commit time.
`cmd/awf/commitgate.go` calls `audit.CheckConventionalCommit` (ADR-0036), which validates
subject length (`SubjectMaxLength`, default 72), disallowed type, disallowed scope, and
malformed shape against the project's `audit` settings. A plan, however, is authored long before
any commit exists, so nothing checks the subjects it proposes. The reviewer is doing by hand what
the commit gate does mechanically — one workflow phase too early.

The blocker to closing this gap deterministically is that plans use bare ` ``` ` fences for many
things — Go snippets, diffs, YAML, command output, *and* commit subjects — with no machine-locatable
marker distinguishing a commit-subject fence from any other. Across the committed corpus, dozens of
bare fences hold a first line matching the Conventional-Commits shape, so no heuristic on bare fences
could tell a planned subject from a diff hunk without false positives. Standardising the task format
so a planned subject is unambiguously marked is the prerequisite for a check.

A second force shapes the check's severity. The full commit-gate rule treats an unknown *scope* as a
hard rejection, which is correct at commit time. But a plan may be the very change that *adds* a
scope — e.g. a plan that introduces a new domain also extends `audit.allowedScopes`. If the plan-time
check rejected an unknown scope outright, it would forbid a plan from proposing the commit that
establishes its own new scope. Scope conformance must therefore be advisory at plan time and hard only
at commit time, where the scope is expected to already exist.

Grounding against the code confirms the mechanics. `audit.Finding` carries no structured
discriminator between a length, type, and scope violation — all share `Rule = "conventional-commits"`
and differ only in a free-text `Detail` string — so severity must be decided *inside* the rule, not
recovered afterward by string-matching. And awf's two output tiers already live in separate methods:
hard drift flows through `Project.Check()` (which `checkPlans` feeds, alongside the existing
`plan-frontmatter-validated` and `plan-adr-link-resolved` checks from ADR-0098), while non-failing
advisories flow through the separate `Project.AdvisoryNotes()` pass — exactly the split the tag
vocabulary already uses (`checkTagVocabulary` gate vs `tagHealthNotes` advisory). The stub/part-marker
advisories (ADR-0070, ADR-0083) established this `note:` tier as the home for non-failing findings.

## Decision

1. **A `commit`-tagged fence marks a planned commit subject; an `awf-ignore` token opts out.** In a
   plan's phase tasks, the planned closing-commit subject is written in a fenced code block whose info
   string's first token is `commit` (` ```commit `) — the alias GitHub Linguist maps to its Git-Commit
   grammar, so the block keeps commit-message syntax highlighting. `awf check` reads the first non-empty
   line of every such block in a plan file as one planned subject and validates it, **unless** the info
   string carries the explicit opt-out token `awf-ignore` (` ```commit awf-ignore `). A display-only
   commit example uses that opt-out to keep highlighting while suppressing the check for that one block.
   Highlighting is unaffected either way because a highlighter selects the language from the info
   string's first token and ignores the rest. An empty or whitespace-only block yields no subject and no
   finding. The check is presence-triggered: it validates the ` ```commit ` blocks it finds and requires
   none to exist — a bare ` ``` ` fence, or any first token other than `commit`, is never read as a
   subject, so the entire existing bare-fence corpus is grandfathered with zero findings.

2. **Plan-time validation reuses the commit-gate rule with scope relaxed to advisory.** Each planned
   subject is validated against the project's resolved `audit` settings (`audit.Resolve(p.Cfg.Audit)`)
   using the same Conventional-Commits rule the commit gate uses. An over-length subject, a disallowed
   type, and a malformed (non-`type(scope)?: subject`) shape are **hard drift** that fails `awf check`.
   A disallowed **scope** is a non-failing **`note:`** advisory, because a plan may be the change that
   adds the scope.

3. **The shared rule is parameterized by scope severity; the commit gate is unchanged.**
   `audit.CheckConventionalCommit(c, s)` keeps its exact signature and its scope=Error behaviour — the
   commit gate (ADR-0036) and its `commit-gate-shared-rule` proof continue to call it untouched. The
   scope-severity choice is threaded through a shared core the public wrapper delegates to; the
   plan-time path invokes it with scope=Warning. Length, type, and malformed-shape severities are
   identical on both paths.

4. **Drift and advisory route through the two existing check tiers.** The hard findings (length, type,
   malformed) are emitted as `manifest.Drift` from the `Project.Check()` path, extending `checkPlans`.
   The scope advisory is emitted as a `note:` from the separate `Project.AdvisoryNotes()` path. A
   single shared plan-parse helper extracts the ` ```commit ` subjects so both passes read one
   extraction rather than duplicating fence parsing.

5. **The plans template and the writing-plans skill teach the fence, in prose.** The `plans-template`
   singleton's phases section (`templates/plans-template/template.md.tmpl`) and the `awf-writing-plans`
   skill both instruct authors — in prose, referencing the fence via inline code — to write the planned
   subject in a ` ```commit ` fence in the closing-commit task. Neither embeds an un-opted-out
   (checkable) ` ```commit ` block, so an `awf new plan` scaffold carries nothing the check will
   validate; a fresh, unedited scaffold therefore cannot fail on a placeholder subject. Any literal
   commit example either skill or template does show carries the ` ```commit awf-ignore ` opt-out, which
   the check skips. Both edits land in the same commit as the check. The rendered `docs/plans/template.md` and
   the example adopter's rendered template are additionally excluded from validation because plan
   parsing already skips `template.md`/`README.md` by filename.

6. **No new configuration.** The check reads the existing `audit.SubjectMaxLength`,
   `audit.allowedScopes`, and `audit.allowedTypes`; a project that tightens those automatically
   tightens its plan checks. No new config key, sidecar field, or var is introduced.

7. **All frontmatter-bearing plans are validated, regardless of `status`.** The check does not exempt
   `Implemented` (frozen) plans, matching the existing `plan-frontmatter-validated` and
   `plan-adr-link-resolved` checks, which skip only a plan without frontmatter. A frozen plan's
   ` ```commit ` subjects were conformant when authored and stay so.

## Invariants

- `` `invariant: plan-commit-subject-length-checked` `` — `awf check` fails a plan under `docs/plans/`
  carrying a validated ` ```commit ` fence whose first non-empty line exceeds the resolved
  `audit.SubjectMaxLength`, reporting the offending length and limit.
- `` `invariant: plan-commit-subject-shape-checked` `` — `awf check` fails a plan under `docs/plans/`
  whose validated ` ```commit ` fence first non-empty line is malformed (not `type(scope)?: subject`)
  or names a type outside a non-empty `audit.allowedTypes`.
- `` `invariant: plan-commit-subject-scope-advisory` `` — a validated ` ```commit ` fence subject
  naming a scope outside a non-empty `audit.allowedScopes` yields a non-failing `awf check` `note:`,
  never drift; the plan-time path is the only relaxation — `audit.CheckConventionalCommit`, and
  therefore the commit gate, keeps treating an unknown scope as a hard finding.
- `` `invariant: plan-commit-subject-marker-scoped` `` — a fence is read as a planned subject only when
  its info string's first token is `commit`; a bare ` ``` ` fence, any other first token, an
  empty/whitespace-only ` ```commit ` block, and `template.md`/`README.md` are never validated, so the
  grandfathered bare-fence corpus produces no finding.
- `` `invariant: plan-commit-subject-optout-honored` `` — a ` ```commit ` fence whose info string also
  carries the `awf-ignore` token is never validated (no drift, no note), so a display-only commit
  example is suppressed while keeping its first-token `commit` highlighting.

## Consequences

- The recurring "commit subject too long" plan-review finding becomes a deterministic gate failure the
  author fixes before review, freeing the reviewer for judgment-level concerns.
- The plan task format gains a small, learnable convention (` ```commit `) that keeps GitHub's
  Git-Commit highlighting and fails safe toward checking: a real planned subject written in the natural
  ` ```commit ` block is validated by default, so the reachable mistake is a caught finding, not a
  silent miss. Authors who write the subject in a bare fence still lose the check for it — the trade
  accepted for backward compatibility and a presence-trigger that cannot false-positive on the existing
  corpus. The template and the writing-plans skill steer hard toward the fence.
- The default-checked design has a small cost: a genuine display-only ` ```commit ` example an author
  forgets to mark ` ```commit awf-ignore ` is validated and may surface a finding. Accepted because the
  opt-out is a single token, the false-positive is a visible nudge rather than silent breakage, and
  display-only commit examples are rare inside `docs/plans/` (they live mostly in docs and skills,
  which this check does not scan).
- The shared conventional-commit rule gains a gate-side consumer for the first time, so two
  `internal/audit/audit.go` doc comments are updated in the same implementation commit: the
  `CheckConventionalCommit` function doc, which already names the commit-gate consumer, additionally
  names the new plan-time `awf check` consumer; and the package doc, which today states the package is
  "advisory … never wired into the gate", is corrected, since `awf check` now consumes the rule.
- The scope-advisory split means a plan proposing a not-yet-declared scope surfaces a `note:` rather
  than blocking, correctly modelling a plan that establishes its own new scope; the commit gate still
  enforces the scope once the plan has landed it in `audit.allowedScopes`.
- The example adopter's zero-notes gate (ADR-0090) now also covers this advisory: an
  `examples/sundial` plan carrying a validated ` ```commit ` fence with an out-of-vocabulary scope would
  emit a `note:` and fail the example check. The example's plans must not carry such a block (or must
  opt out) — a constraint recorded here, currently satisfied (the example's plans use no commit-subject
  fence).
- The `plans-template-taxonomy` golden guard (ADR-0108/0097) constrains the phases section: the new
  ` ```commit ` guidance must avoid the bare phrase "the gate" (the `gateCmd` interpolation guard) and
  introduce no unresolved `{{ }}` token.
- No new configuration surface is added; the check inherits the project's audit settings.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Heuristic detection of Conventional-Commits-shaped bare fences | Dozens of existing bare fences hold commit-shaped first lines (diffs, examples); would false-positive across the whole corpus and still miss untagged subjects. An explicit marker is what "standardise the task format" asks for. |
| A namespaced `awf-commit` info string (no opt-out) | Forfeits GitHub's Git-Commit syntax highlighting (`awf-commit` is unknown to Linguist) and makes the unsafe default *silent*: an author reaching for the natural ` ```commit ` for a real planned subject would have it skipped unchecked. Keeping `commit` (highlighted, checked-by-default) with a per-block ` ```commit awf-ignore ` opt-out fails safe and preserves highlighting. |
| A structured inline line (`Commit: type(scope): subject`) | Awkward for subjects containing backticks/colons and less natural than the fence authors already write; a fence reads cleanly in rendered markdown and holds an optional body. |
| Collect subjects into plan frontmatter (`commits: [...]`) | Divorces the subject from its task context, duplicates information authors must keep in sync, and reads worse than an in-task fence. |
| Scope as hard drift at plan time (reuse the gate rule verbatim) | Would forbid a plan from proposing the commit that establishes its own new scope (e.g. a new domain). Scope must be advisory until the scope exists. |
| Softening `CheckConventionalCommit`'s scope severity globally | The commit gate depends on scope=Error; a global change would weaken the commit-time gate. Only the plan-time path relaxes, via a scope-severity parameter. |
