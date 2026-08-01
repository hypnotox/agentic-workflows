---
format: current-state-v2
status: Proposed
date: 2026-08-01
---
# ADR-0209: Sanction authoring latitude in plans

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
sentence that follows. An author reading top to bottom is told to be exact three times
before being told they may be otherwise.

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
already state exhaustively. ADR-0108 removed the `Tech stack` field partly because its
`Key packages / files touched` bullet duplicated `File structure`, which means ADR-0108
treated `File structure` as that information's surviving non-duplicate home. This decision
retires the section ADR-0108 kept, on the ground that the tasks rather than a header
section are where the information is now maintained.

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
   transformations. The no-placeholder boundary is unchanged for implementation tasks:
   `TBD`, `implement later`, outcome-only summaries, and hidden design choices remain
   forbidden. The same four surfaces carry the task field vocabulary this decision
   introduces, so the `plan-task-detail-modes` contract covers both what content form a
   task supplies and how a task declares that form.

2. A task claims the exception with an explicit `Latitude: exact` field. Where item 1's
   categories apply the field is required, so a contract-bearing task states its own
   exactness rather than leaving a reviewer to infer it from content; elsewhere the field
   is permitted as a voluntary claim, and its absence means qualifying form. Item 1
   governs which content must be exact and item 2 governs how that is declared: a
   contract-bearing task omitting the field is a defect the reviewer flags, not a task
   that has become qualifying.

3. Every field this decision introduces is written as a nested line directly beneath its
   task's checkbox bullet, spelled `<Field>: <value>`, before the task's prose body. Fixing
   the position is what lets a reviewer find every declaration without reading the prose,
   and it is authored-format vocabulary rather than a parsing concern, so it is settled
   here.

4. Plans gain a spike task form for investigation, declared with `Kind: spike`. A spike
   task carries a `Question:` field stating what must be learned, carries no implementation
   content, and produces an answer rather than an edit. The `executability` lens is amended
   to accept it on that basis. The distinction from the outcome-only summaries item 1
   still forbids is that an outcome-only summary hides how a known change is made, while a
   spike declares that the change is not yet knowable. The spike form lands on the same
   four surfaces item 1 names, because the `plan-task-detail-modes` backing test pins them
   equal.

5. A plan containing at least one spike task carries the `## Notes` section, which is
   otherwise optional, and every spike's answer is recorded there. Notes is the spike's
   only acceptance target, so no field names it. A spike never constitutes a phase on its
   own: it rides an implementation phase whose transaction it precedes, and work depending
   on its answer sequences into a later phase. Phase transaction ownership is therefore
   unchanged.

6. The whole-plan `## File structure` section is retired from the plan section taxonomy. A
   task declares the paths it affects with a `Paths:` field. On a batch task, which
   declares itself with `Kind: batch`, `Paths:` IS the exhaustive affected-site set the
   batch convention already requires, now carrying a field name and available to non-batch
   tasks; the convention's separate affected-site-set wording is retired into it across the
   four surfaces. `Paths:` is required wherever the affected set is not unambiguous from
   the task's title and content, which is always true of a batch task. Outside that
   always-true case the judgment is the author's and the reviewer's, in keeping with the
   rest of this decision.

7. A `Paths:` field may hold a pathspec or glob in place of an enumeration. `Post-check:`
   names the deterministic post-check the batch convention already requires, and this
   decision extends its trigger: it is carried by a batch task, and by any task whose
   `Paths:` field holds a pathspec. One field, one home, two triggers. A pathspec without a
   post-check asserts breadth without proving it; with one it is stronger than an
   enumeration, because an enumeration cannot detect its own staleness.

8. The field in item 7 is named `Post-check:`. It is not named `Verify:`, which already
   names the manual confirmation line a `Backing: unbacked` invariant claim carries, and
   reusing it would give one field name two meanings across adjacent authored formats
   while echoing the `Verify and commit` ceremony ADR-0196 retired. The plan template
   already calls this concept a deterministic post-check, so the name is existing project
   vocabulary rather than a coinage. The same collision test is why item 5 records a
   spike's answer in `## Notes` rather than in a `Record:` field: `Record:` already names
   the verbatim user-provenance block in an effort's decision log.

9. Conditional and optional tasks are not added. A phase is one independently green
   coherent implementation transaction, and a task whose execution depends on another
   task's outcome breaks that property. An author needing conditional work states the
   condition as a spike task and sequences the dependent work into a later phase.

10. The single `update` to `plans-template-taxonomy` carries three changes. The claim's
    "four narrative header fields" has been wrong since `fb1f392d`, because ADR-0108
    already fixed the header at three; this decision retires `File structure`, leaving two,
    Goal and Architecture summary; and the Notes tail stops being unconditionally optional
    per item 5. One operation per claim per ADR means these cannot be separated within this
    ADR, so the operation carries all three and this item is the record of that.

## State changes

- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- update `adr-system/plan-artifacts:plans-template-taxonomy`

## Consequences

Authors gain a default that matches what the corpus already shows good plans doing. Two of
the largest recent plans, `2026-07-30-git-seam-whole-area-conversion.md` and
`2026-07-31-decompose-internal-project.md`, carry zero transcribable code fences across 759
and 536 lines while remaining fully executable. The corpus also contains the opposite form
at greater scale: the largest recent plan is the 972-line severity plan this ADR's Context
cites for dictating three files byte for byte. Both forms coexist at comparable size with
nothing marking the difference, which is the unremarked-deviation problem this decision
fixes.

The honest net on ceremony: one whole-plan section is retired and five per-task fields are
added, of which a typical implementation task carries zero or one. `Latitude:` is written
only to claim the exception, `Kind:` only for a spike or a batch, `Question:` only on a
spike, `Paths:` wherever the affected set is ambiguous, which is always true of a batch
task, and `Post-check:` on a batch task or any task whose `Paths:` holds a pathspec. A
batch task already owed both declarations under different names. The argument for accepting this is that
marked-exception fields on a minority of tasks cost less than a hand-maintained section on
every plan, and that two of the five fields are renames of existing obligations rather than
new ones. This decision nonetheless adds vocabulary to a format ADR-0197 deliberately
thinned, and the Alternatives table rejects two options specifically for adding ceremony,
so the cost is real and is accepted rather than denied.

Plans lose a whole-plan blast-radius summary. A reader wanting one must read the phases or
wait for a derived projection; no tool provides that today. This is accepted because the
retired section was authored rather than derived, so nothing that existed is lost, and a
derived projection is the correct long-term home.

Retiring `File structure` is a same-commit sweep of six surfaces: the plan-reviewer
`section-taxonomy` focus item, which names the three header sections explicitly; the
`plans-template-taxonomy` golden assertion, which asserts the literal heading; the plans
README structure part; the writing-plans conventions-header part, which names the three
canonical fields; the plans-template header part carrying the section itself; and the
`examples/sundial` renders, which follow from its own config tree. The plans-template
catalog `Sections` list and `adr-singleton-section-parity` are deliberately unaffected,
because `File structure` sits inside the `header` section marker rather than owning one;
ADR-0108 needed that change and a reader will expect it here.

Every prose change lands on four surfaces in one commit, because the `plan-task-detail-modes`
backing test asserts its clause list across eight renders covering both the default and
this project's rendered form. A partial sweep reds the gate rather than shipping a
half-inverted gradient.

The spike task form creates a way to defer a decision inside a plan, which can be misused
to avoid settling a design that brainstorming should have settled. The mitigation is that a
spike carries no implementation content and cannot stand in for one: work depending on its
answer sequences into a later phase, so a plan that spikes its way through a design shows
that shape plainly to its reviewer.

Removing a section from a taxonomy claim while 72 existing plans still carry it constrains
any later structural check, which must tolerate the section on plans predating this
decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add named task kinds (`exact`, `qualified`, `spike`) each with required content | New ceremony months after ADR-0197 spent a decision trimming ceremony; the marked-case field in item 2 buys the same visibility at a fraction of the cost |
| Declare an exactness mode per phase, beside the execution mode | Wrong granularity: a phase routinely mixes one contract-bearing config change with several prose edits, which is the distinction being drawn |
| Soften the `executability` lens alone | The lens and the template carry identical prose pinned equal by the backing test, so a lens-only change alters nothing |
| Derive `File structure` from the tasks instead of retiring it | The better long-term shape, and what Consequences endorses, but nothing parses plans today; an authored section awaiting replacement by a derived one is worth removing rather than maintaining in the interim |
| Defer the field vocabulary until something parses it | The fields are authored-format vocabulary that a reader uses immediately; withholding them until a consumer exists would leave the gradient inverted with no way to declare the exception |
| Keep `File structure` and require it to agree with the tasks | Doubles the maintenance of a section with no consumer and no way to check the agreement |
| Name the item 7 field `Verify:` | Already the name of the manual confirmation line on an unbacked invariant claim; one name, two meanings, in adjacent authored formats |
| Require `Paths:` on every task | Reintroduces per-task ceremony on the majority of tasks, whose single file is already named in the title |
| Add conditional tasks alongside the spike form | Breaks the phase's independently green transaction property, which no current form violates |

## Status history

- 2026-08-01: Proposed
