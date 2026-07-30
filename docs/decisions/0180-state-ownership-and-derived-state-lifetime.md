---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0180: State ownership and derived state lifetime

## Context

ADR-0178 established the first repository-wide code-design pattern, `code-design/dependency-composition`,
which settled where a dependency is selected and which direction it points. It said nothing about what a
value owns once constructed, or about where state derived during one operation lives. `internal/project`
shows why that gap matters.

`internal/project` is 8086 production lines across 30 production files. It imports seventeen internal
packages and is imported by exactly two, `cmd/awf` and `cmd/releasecheck`. One type, `Project`, carries
95 production methods, 23 exported and 72 private. Its fields split cleanly in two. `Root`,
`residentRoot`, `Cfg`, `Cat`, `Targets`, and `standard` are construction inputs, written once and read
thereafter. `corpus`, `topics`, and `effSkills` are derived during an operation and cached on a value
that outlives it.

Those three cached derivations each need a discipline no compiler enforces.

`beginInvocation` (`internal/project/project.go:468`) nils `corpus` and `topics`, and the comment above
it states the obligation in prose: "Every public operation that reads ADRs calls it before its first
Corpus use." The same comment spells out the consequence of forgetting, that a `Check` following a `Sync`
would miss an ADR written in between and so would "silently blind the drift oracle rather than merely
serving a stale read." Five call sites honour it today (`check.go:28`, `check.go:398`,
`output_plan.go:457`, `topics.go:26`, `project.go:236`), but the discipline is already inexact: the
`topics.go:26` call is vestigial, because `QueryTopic` reads neither field and takes its corpora from
`workingCurrentState`; and the `output_plan.go:457` call fires again inside `Check`, nested mid-operation
after `check.go:398` already reset. No bug is live, because nothing loads a corpus between the two
resets, yet the invariant the comment states is not the invariant the code enforces. A sixth public
operation that reads a corpus and omits the reset introduces the failure the comment predicts, and no
gate catches it.

`effSkills` needs an ordering rather than an invalidation, and the corresponding bug is live rather than
latent. It is nil until `RenderAll` writes it (`render.go:477`, and again at `output_plan.go:408`).
`checkDeadSkillRefs` reads it at `check.go:438` and is correct only because `Check` reaches `OutputPlan`
first at `check.go:406`. Called directly, `checkDeadSkillRefs` sees a nil map and reports no dead skill
references at all. That is the same silent-blinding failure mode, with the ordering obligation recorded
nowhere but the call sequence.

Removing these caches costs nothing that ADR-0130 was protecting. ADR-0130 item 1 introduced the shared
corpus view and chose the field deliberately: "they already share a `*Project` receiver, so the threading
is a field, not a new parameter on every signature." But its Consequences state the goal plainly, "The
gain is correctness before speed... at 129 ADRs the wall-clock saving is not the reason to do it," and
its own Alternatives row rejects caching in terms this decision only completes: "a cache keyed on
directory path invites staleness across a sync that rewrites ADRs. Threading a value makes the sharing
explicit." The cache is in any case already per-operation rather than per-process. `cmd/awf/check.go`
calls `AdvisoryNotes`, `Check`, and `CheckCurrentState` on one `Project` (`:28`, `:37`, `:41`), and each
re-derives, so a single `awf check` performs three full ADR-corpus and three topic-corpus derivations
today. Threading one derivation per operation preserves exactly the sharing the field provides.

Where the derivation is genuinely expensive is the topic corpus, not the ADR parse. `topic.LoadCorpus`
calls `BuildMarkerIndex`, which walks the repository reading every file matching the `currentState.sources`
globs (`internal/topic/markers.go:56-92`). Within one `Check` there are three `Topics` call sites in
different subtrees, `generateTopicDocs` (`topics.go:59`), `generateDomainDocs` (`render.go:832`), and
`sweepConfigTree` through `buildClaimedModel` (`sweep.go:118`), and `AdvisoryNotes` reaches
`generateDomainDocs` a fourth time outside `OutputPlan` (`check.go:33`). Threading must therefore reach
from `Check` through the `OutputPlan` boundary and separately into `sweepConfigTree`, or one repository
walk becomes three.

Two boundaries constrain how wide the resulting claims may be drawn. First, `internal/project` already
contains a second, independent corpus-construction path: `workingCurrentState`
(`internal/project/currentstate.go:93-111`) builds both corpora from a snapshot tree through
`currentstate.LoadFromTree`, backing `QueryTopic`, `CheckCurrentState`, `CheckStaged`,
`ContextForOptions`, and `StagedContextRootOptions`. That path is already operation-owned and threaded,
and six `adr.NewCorpus` or `adr.LoadCorpus` production sites in the package survive any conversion of the
fields. A claim asserting that `internal/project` derives its corpora per operation in general would be
either already satisfied or still false; the claim must name the cached derivations it converts.

Second, `topic.Corpus` itself completes construction in two steps for a real reason. `LoadCorpus`
assembles the corpus, builds the marker index from it, and then writes `c.Markers = markers`
(`internal/topic/corpus.go:103-111`; the snapshot loader does the same at
`internal/topic/tree.go:66-75`), because `BuildMarkerIndex` needs the assembled corpus to resolve claim
ids. Since `topic.Corpus` is one of the values this decision threads, an immutability claim phrased as
"no field written after construction" would condemn the first consumer's own dependency. The claim must
permit completing construction inside the constructing function.

Two adjacent surfaces need deliberate handling. Reasoned claims carry no mechanical enforcement, so
ADR-0178's authority commit added a focus item named `dependency-composition-authority` to
`.awf/agents/adr-reviewer.yaml`, `code-reviewer.yaml`, and `plan-reviewer.yaml`. That item names its
topic specifically and will not fire for a second one, so a new topic of reasoned claims without a
parallel focus item has no enforcement path at all; `awf check` validates claim provenance and the
absence of a proof marker, never claim content. And `focusItems` replaces catalog defaults wholesale, so
the addition must compare and backfill rather than append blindly.

Unlike ADR-0178, this decision needs no new governance surface. The pathless `code-design` domain, the
`code-design` commit scope, the glossary entry, and the architecture and development wiring all already
exist. `dependency-composition:dependency-composition-commit-classification` already assigns
cross-package code-structure work to the `code-design` scope, so this decision adds no
commit-classification claim of its own.

This decision establishes the pattern and converts one type. Package-level cohesion, including whether a
method that reads no receiver field should be a function and whether `internal/project` should be split,
is a separate later decision; the fourteen production files in the package that touch no `Project` field
and the four synthetic partial `Project` literals (`currentstate.go:154`, `context.go:340`,
`context.go:61`, `context.go:44`) are evidence for that decision, not this one.

## Decision

1. Add `code-design/state-ownership` with `applies: global` to the existing pathless `code-design`
   domain. Its identified claims are the durable authority for the rules below; this ADR remains
   historical rationale. Like `dependency-composition`, the topic governs values introduced by new work
   and derived state deliberately converted under its authority, and does not make existing cached
   derivations nonconforming debt to sweep.

2. A value that outlives one operation is immutable once construction completes: no field is written
   outside the function that constructs it. A construction that genuinely needs two steps, because a
   later field is derived from the assembled value, completes both inside that one function.

3. State an operation derives is owned by that operation and threaded explicitly to the consumers that
   need it, rather than stored on a value that outlives the operation. Threading reaches every consumer
   in the operation's call tree, including consumers behind an intermediate boundary, so that one
   derivation serves the whole operation.

4. A derivation is never kept correct by an ordering or invalidation step that each entry point must
   remember to perform. Where such a step exists today, the fix is to give the derivation a lifetime that
   makes staleness unrepresentable, not to document the step more loudly.

5. A derived value has exactly one producer, and a consumer receives it rather than re-deriving it.

6. Convert the three cached derivations on `internal/project.Project` as the concrete first consumer:
   `corpus`, `topics`, and `effSkills`. Derive each in the operation that needs it and thread it to its
   consumers; delete `beginInvocation`; and delete or unexport `Corpus` and `Topics`, whose production
   callers all become threaded parameters. Derive at the entry of the operations where derivation is
   already unconditional, and leave `QueryTopic` and the rest of the `workingCurrentState` snapshot path
   untouched, so no path that short-circuits today gains a derivation it does not need.

7. Back the conversion with one structural test that loads the production packages and asserts no method
   on `*Project` writes a field outside the function that constructs it. Do not use a behavioural
   "`Check` after `Sync`" assertion as the proof: that property already holds, because `Check` calls
   `beginInvocation` for exactly that reason, so such a test passes before the change and proves nothing
   about ownership.

8. Add one reviewer focus item naming `code-design/state-ownership` to `.awf/agents/adr-reviewer.yaml`,
   `.awf/agents/code-reviewer.yaml`, and `.awf/agents/plan-reviewer.yaml`, comparing against and
   backfilling the catalog defaults each list replaces, so the reasoned claims have an enforcement path.

9. Re-derive every `coverage-ignore` justification that cites the shared cache as its unreachability
   argument (`sweep.go:119`, `render.go:813`, `output_plan.go:532`, `check.go:446`, `check.go:451`,
   `check.go:456`, `configreference.go:410`). Threading removes the error return in most cases, which
   deletes the exclusion; any exclusion that survives carries a justification that no longer names a
   cache.

10. Leave the remaining post-construction writes to `*Project` out of scope and named: `ContextForOptions`
    writes `universe.Targets` and `universe.Cat` after the literal at `context.go:44-48`, and
    `StagedContextRootOptions` writes `p.Cfg`, `p.Targets`, and `p.Cat` after the literal at
    `context.go:61-70`. Both are stepwise construction driven by a real ordering need, and both are
    bounded future candidates under item 1 rather than conforming sites.

## State changes

- add `code-design/state-ownership:construction-immutable-state`
- add `code-design/state-ownership:operation-owned-derivation`
- add `code-design/state-ownership:no-remembered-invalidation`
- add `code-design/state-ownership:single-derivation-producer`
- add `code-design/state-ownership:project-derived-state-ownership`

## Consequences

awf gains a second code-design authority that answers a question `dependency-composition` left open: not
where a dependency comes from, but what a value keeps afterwards. The two compose, because a threaded
derivation is a dependency the consumer receives rather than discovers.

The conversion removes a live defect and a latent one. `checkDeadSkillRefs` stops depending on `Check`
having called `OutputPlan` first, so it can no longer report zero dead skill references from a nil map.
And the drift oracle stops depending on every future public operation remembering to reset a cache; the
sixth operation that reads a corpus cannot introduce the staleness the current comment predicts, because
there is no cache to go stale. Threading also tends to delete error returns, since a parameter cannot
fail, which should retire several coverage exclusions rather than merely re-justify them.

The cost is visible parameters. Roughly ten method signatures across six production files grow a corpus
or skill-set parameter, and about thirty-three test call sites move with them. `Corpus` and `Topics` must
be deleted or unexported in the same transaction, because the dead-code gate flags an exported method
whose production callers have become parameters, and that breaks the four `Topics` call sites in
`internal/project/topics_test.go`. Test-side risk is otherwise smaller than the package's 17815 lines of
test code suggests: no test sets `corpus` or `topics`, and the existing `Project` literals in tests never
set them.

This decision changes the mechanism ADR-0130 item 1 chose. That reversal is deliberate and completes
ADR-0130's own stated preference for explicit threading over a cache, but it is worth recording that no
mechanical handshake enforces the change: ADR-0130's live claim `adr-system/adr-lifecycle:corpus-parsed-once`
governs `ParseDir` call sites, which this conversion does not touch, so `awf check` stays green while a
prior decision's stated rationale stops describing the code. A reader of ADR-0130 needs this ADR to
understand why its item 1 no longer holds.

Four of the five claims are reasoned contracts rather than test-backed ones, so their enforcement is a
reviewer focus item and their `Verify` instructions, not a gate. That is the same posture ADR-0178 took,
and it carries the same risk: a global topic of vague claims produces repository-wide noise. The claims
are kept narrow by naming what they govern, an operation's derived state and a value's fields, and by
carrying the new-or-deliberately-converted qualifier inline. The wholesale-list audit on the three agent
sidecars is deliberate work, because an appended focus item silently erases the catalog defaults.

One claim is deliberately narrower than its subject. `project-derived-state-ownership` names the cached
derivations rather than asserting that `internal/project` derives corpora per operation, because the
`workingCurrentState` snapshot path already does so and six corpus-construction sites in the package
survive the conversion. A reader looking for a blanket guarantee about corpus construction in that
package will not find one here.

The topic's own coverage view will be empty, exactly as `dependency-composition`'s is, because the
pathless domain declares no selectors and a global topic's applicability is computed against them.
Discovery works through `awf context`, which attaches global topics to a queried path, and the only
concrete anchor is the proof marker on the test-backed claim.

Finally, this decision creates downstream work it does not perform. Package-level cohesion in
`internal/project` becomes the next code-design pattern, and it now has a stated prerequisite: a value
whose fields are all construction inputs is far easier to split than one carrying three derived caches.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the caches and document the reset obligation more strongly | The obligation is already documented and already inexact: one of its five call sites is vestigial and another is nested-redundant. Prose cannot be gated. |
| Keep the caches but add a gate that every public operation resets first | It enforces a mechanism instead of removing the need for one, and it cannot distinguish an operation that reads a corpus from one that does not. |
| Convert `corpus` and `topics` only, leaving `effSkills` | `effSkills` carries the one live defect and is the only genuine two-producer case, so excluding it would leave the strongest exemplar of two claims unproven and a real bug in place. |
| Include the four synthetic partial `Project` literals and the fourteen zero-field files | That is package cohesion, a separate decision. Including it would make this effort the `internal/project` decomposition rather than its prerequisite. |
| Prove the claim with a behavioural `Check`-after-`Sync` test | The property already holds today, so the test passes before the change and cannot be written failing first. |
| Prove the claim with an approved-call-set test, as ADR-0130 did for `ParseDir` | It governs where a corpus is constructed rather than whether a value keeps it, which is not what the claim states. |
| Fold these rules into `code-design/dependency-composition` | Dependency selection and state lifetime are separate subjects; merging them would make one topic's claims answer two questions and blur which authority a reviewer is applying. |
| Add a generic section on immutability to `docs/maintainable-code-design.md` instead | That document's sections are a fixed list and its value is generic guidance. The whole point of a topic is being specific and mechanically reviewable where the guide cannot be. |
| Derive eagerly at the entry of every public operation | `tagHealthNotes` returns before deriving when an adopter declares no tags, and `QueryTopic` never touches the fields, so a blanket rule would add a repository walk to operations that currently avoid one. |

## Status history

- 2026-07-30: Proposed
