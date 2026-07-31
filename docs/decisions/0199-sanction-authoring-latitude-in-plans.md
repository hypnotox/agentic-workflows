---
format: current-state-v2
status: Proposed
date: 2026-08-01
---
# ADR-0199: Sanction authoring latitude in plans

## Context

A workflow audit of the plan corpus on 2026-08-01 measured the 42 plans dated 2026-07-21
and later, roughly 19,000 lines. The floor is high: a sweep for vague acceptance language
(`as appropriate`, `as needed`, `reasonable`, `TBD`, `somehow`, `etc.`) returned two hits,
both benign, and no task was found whose acceptance is unverifiable. ADR-0196's ceremony
trim is visible in the same corpus: the per-task `Verify and commit` plus `Expected:`
restatement appears in 15 plans dated before 2026-07-29 and in none after.

The defect is the opposite of vagueness. Authors default to maximum precision even where
precision buys nothing. `2026-07-30-drop-configurable-severity-and-unify-the-finding-rank.md`
dictates three complete files byte for byte, including package doc-comment rationale prose
the implementer must transcribe unchanged. Commit `6f1e5178` had to drop ADR-0180's
test-line count because the literal was drift-prone, which is the "assert a method, never
a count" rule failing to reach an artifact it was written for.

The sanctioned less-concrete form already exists. The plan template permits qualifying
instructions for non-contractual prose, and the `executability` lens carries the identical
sentence, pinned equal to the template by the `plan-task-detail-modes` backing test. Only
4 of the 42 plans use any latitude language. The rule is right and the gradient is wrong:
the guidance opens by demanding exact paths, symbols, and commands, asserts that exact form
remains mandatory across a list of eight categories, and permits latitude only in the
eighth clause of that sentence. An author reading top to bottom is told to be exact three
times before being told they may be otherwise.

Three forms have no sanctioned expression at all. An investigation task must leave the plan
structure entirely, because every task must carry implementation-ready content and the
`executability` lens rejects outcome-only summaries. A task facing genuine authoring-time
uncertainty must over-specify a guess, because `TBD` and `implement later` are forbidden in
three independent places. Neither prohibition is wrong on its own; together they leave an
author with no way to say "this is not yet knowable" except by inventing an answer.

Separately, the whole-plan `## File structure` header section has no machine consumer. The
only reference in Go is `internal/project/golden_test.go:60`, which asserts the heading
exists in the rendered template. It is a hand-maintained snapshot of information the tasks
already carry, and for a batch task it duplicates an affected-site set the task must
already state exhaustively. ADR-0108 removed the `Key packages / files touched` field for
duplicating `File structure`; the same argument applies one level up.

The `plans-template-taxonomy` claim also carries an error. It reads "the four narrative
header fields" while its own proof asserts three headings, and ADR-0108 decided the header
is three fields. ADR-0108 is not the source: it predates State-changes governance and its
plan recorded the correct intent. The wrong wording entered at the current-state cutover
commit `fb1f392d`, so the claim has contradicted its proof since migration.

## Decision

1. The task-detail guidance in the four surfaces the `plan-task-detail-modes` claim pins
   (the plan-authoring skill, the plan reviewer, the implementation-plans README, and the
   plan template) states qualifying form as the default for a task's content and presents
   the contract-bearing categories as the named exception to it. The closed category list
   is unchanged in membership: machine-consumed configuration and manifests,
   contract-bearing declarations, fixtures, golden output, commands, mechanical
   replacements, required literal prose, and a batch task's representative and edge
   transformations. The no-placeholder boundary is unchanged: `TBD`, `implement later`,
   outcome-only summaries, and hidden design choices remain forbidden.

2. A task claims the exception with an explicit `Latitude: exact` field. The field is
   written only to claim exactness; its absence means qualifying form. The marked case is
   therefore the exception rather than the default, so a normal task carries no latitude
   ceremony and a reviewer can see every exactness claim without inferring it.

3. Plans gain a spike task form for investigation, declared with `Kind: spike`. A spike
   task carries a `Question:` field stating what must be learned and a `Record:` field
   naming where the answer lands. It carries no implementation content, and the
   `executability` lens accepts it on that basis. A spike task deliberately does not
   declare how the question will be answered: a spike exists because the route is not yet
   known, and prescribing one would reintroduce the over-specification this decision
   removes.

4. A plan containing at least one spike task carries the `## Notes` section, which is
   otherwise optional. The section is the spike's acceptance target, so it becomes
   required exactly where something depends on it.

5. The whole-plan `## File structure` section is retired from the plan section taxonomy. A
   task declares the paths it affects with a `Paths:` field. The field is required on a
   batch task, which the reader identifies by its representative and edge declarations,
   and optional elsewhere, because breadth is what a batch asserts while a single-file
   task normally names its file in its title.

6. A `Paths:` field may hold a pathspec or glob in place of an enumeration. A task whose
   `Paths:` field holds a pathspec carries a `Post-check:` field: a deterministic check
   that pins the resolved set at execution time. A pathspec without one asserts breadth
   without proving it; with one it is stronger than an enumeration, because an enumeration
   cannot detect its own staleness.

7. The field in item 6 is named `Post-check:`. It is not named `Verify:`, which already
   names the manual confirmation line a `Backing: unbacked` invariant claim carries, and
   reusing it would give one field name two meanings across adjacent authored formats
   while echoing the `Verify and commit` ceremony ADR-0196 retired. The plan template
   already calls this concept a deterministic post-check, so the name is existing project
   vocabulary rather than a coinage.

8. Conditional and optional tasks are not added. A phase is one independently green
   coherent implementation transaction, and a task whose execution depends on another
   task's outcome breaks that property. An author needing conditional work states the
   condition as a spike task and sequences the dependent work into a later phase.

9. The `plans-template-taxonomy` claim's narrative-header wording is corrected from four
   fields to the two that survive this decision, Goal and Architecture summary, in the
   same operation that retires `File structure` from the taxonomy.

## State changes

- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- update `adr-system/plan-artifacts:plans-template-taxonomy`

## Consequences

Authors gain a default that matches what the corpus shows good plans already do. The two
largest recent plans, `2026-07-30-git-seam-whole-area-conversion.md` and
`2026-07-31-decompose-internal-project.md`, carry zero transcribable code fences across
759 and 536 lines while remaining fully executable; this decision makes their form the
sanctioned default rather than an unremarked deviation.

Plans lose a whole-plan blast-radius summary. A reader wanting one must read the phases or
wait for a derived projection; no tool provides that today. This is accepted because the
retired section was authored rather than derived, so nothing that existed is lost, and a
derived projection is the correct long-term home.

Every prose change lands on four surfaces in one commit, because the `plan-task-detail-modes`
backing test asserts its clause list across eight renders covering both the default and
this project's rendered form. A partial sweep reds the gate rather than shipping a
half-inverted gradient. The same change must land in `examples/sundial`, which carries its
own config tree.

The spike task form creates a way to defer a decision inside a plan, which can be misused
to avoid settling a design that brainstorming should have settled. The mitigation is that a
spike carries no implementation content and cannot stand in for one: work depending on its
answer sequences into a later phase, so a plan that spikes its way through a design shows
that shape plainly to its reviewer.

Retiring `File structure` invalidates the `section-taxonomy` reviewer focus item and the
golden test's literal assertion, both of which name the section explicitly. Removing a
section from a taxonomy claim while 72 existing plans still carry it also constrains any
later structural check, which must tolerate the section on plans predating this decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add named task kinds (`exact`, `qualified`, `spike`) each with required content | New ceremony months after ADR-0197 spent a decision trimming ceremony; the marked-case field in item 2 buys the same visibility at a fraction of the cost |
| Declare an exactness mode per phase, beside the execution mode | Wrong granularity: a phase routinely mixes one contract-bearing config change with several prose edits, which is the distinction being drawn |
| Soften the `executability` lens alone | The lens and the template carry identical prose pinned equal by the backing test, so a lens-only change alters nothing |
| Keep `File structure` and require it to agree with the tasks | Doubles the maintenance of a section with no consumer and no way to check the agreement |
| Name the item 6 field `Verify:` | Already the name of the manual confirmation line on an unbacked invariant claim; one name, two meanings, in adjacent authored formats |
| Require `Paths:` on every task | Reintroduces per-task ceremony on the majority of tasks, whose single file is already named in the title |
| Add conditional tasks alongside the spike form | Breaks the phase's independently green transaction property, which no current form violates |

## Status history

- 2026-08-01: Proposed
