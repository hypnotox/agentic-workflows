---
format: current-state-v2
status: Implemented
date: 2026-07-30
---
# ADR-0180: State ownership and derived state lifetime

## Context

ADR-0178 established the first repository-wide code-design pattern, `code-design/dependency-composition`,
which settled where a dependency is selected and which direction it points. It said nothing about what a
value owns once constructed, or about where state derived during one operation lives. `internal/project`
shows why that gap matters.

`internal/project` is 8087 production lines across 30 production files. It imports seventeen internal
packages and is imported by exactly two production packages, `cmd/awf` and `cmd/releasecheck`. One type,
`Project`, carries 95 production methods, 23 exported and 72 private. Its fields split cleanly in two.
`Root`,
`residentRoot`, `Cfg`, `Cat`, `Targets`, and `standard` are construction inputs, written once and read
thereafter. `corpus`, `topics`, and `effSkills` are derived during an operation and cached on a value
that outlives it.

Those three cached derivations each need a discipline no compiler enforces.

`beginInvocation` (`internal/project/project.go:469`) nils `corpus` and `topics`, and the comment above
it states the obligation in prose: "Every public operation that reads ADRs calls it before its first
Corpus use." The same comment spells out the consequence of forgetting: a `Check` following a `Sync` would
miss an ADR written in between, "silently blinding the drift oracle rather than merely serving a stale
read." Five call sites honour it today (`check.go:28`, `check.go:398`,
`output_plan.go:457`, `topics.go:26`, `project.go:237`), but the discipline is already inexact: the
`topics.go:26` call is vestigial, because `QueryTopic` reads neither field and takes its corpora from
`workingCurrentState`; and the `output_plan.go:457` call fires again inside `Check`, nested mid-operation
after `check.go:398` already reset. No bug is live, because nothing loads a corpus between the two
resets, yet the invariant the comment states is not the invariant the code enforces. A sixth public
operation that reads a corpus and omits the reset introduces the failure the comment predicts, and no
gate catches it.

`effSkills` needs an ordering rather than an invalidation. It is nil until a render pass writes it, at
`output_plan.go:408` in `targetOutputDeclarations` and then at `render.go:477` in `renderAllBase`, in that
order within one `OutputPlan`. `Check` reads the field at `check.go:438` and is correct only because
`check.go:406` reached `OutputPlan` first. Every production reader today is preceded by a write in the
same call tree, so the hazard is latent rather than live: `checkDeadSkillRefs` takes the effective set as
a parameter (`check.go:538`), so it is the caller that must supply a populated map, and a second caller
that passes the field without rendering first would report no dead skill references at all. That is the
same silent-blinding failure mode as the corpus case, one function-signature step further along, with the
ordering obligation recorded nowhere but the call sequence.

The same three shapes appear together, and more sharply, in rendered adopter-facing code. The Pi
subagent extension's preference store (`templates/pi/awf-subagents/model-routing.ts.tmpl:163-186`) holds
`global`, `project`, `registryInvalid`, and `ready` as closure-mutable slots that `reload` and
`validateAgainstRegistry` write after construction. Its correctness depends on a remembered protocol
rather than a lifetime: `index.ts.tmpl:465-466` constructs the store and must then call `reload`, and
`:467-471` must call `reload`, then `validateAgainstRegistry`, then `state`, in that order, before the
result means anything. And `resolveChildModel` takes the whole store and re-derives from it
(`model-routing.ts.tmpl:206-208` `const state = store.state()`) although its caller at
`index.ts.tmpl:483` sits in a closure where the already-derived state is available and `:470` has just
returned it. The pure derivation this all routes around already exists and is exported:
`effectivePreferenceState(global, project, registryInvalid)`. This instance matters more than the
internal ones because it renders into every adopter's extension rather than staying inside awf's own
binary.

Removing these caches costs nothing that ADR-0130 was protecting. ADR-0130 item 1 introduced the shared
corpus view and chose the field deliberately: "they already share a `*Project` receiver, so the threading
is a field, not a new parameter on every signature." But its Consequences state the goal plainly, "The
gain is correctness before speed... at 129 ADRs the wall-clock saving is not the reason to do it," and
its own Alternatives row rejects caching in terms this decision only completes: "a cache keyed on
directory path invites staleness across a sync that rewrites ADRs. Threading a value makes the sharing
explicit." The cache is in any case already per-operation rather than per-process. `cmd/awf/check.go`
calls `AdvisoryNotes`, `Check`, and `CheckCurrentState` on one `Project` (`:28`, `:37`, `:41`), and the
first two each re-derive both corpora from scratch, so a single `awf check` already performs two full
cache-driven derivations of each. The third, `CheckCurrentState`, derives independently through the
snapshot path (`workingCurrentState`, `currentstate.go:136`), which is never cached and is untouched by
this conversion. Threading one derivation per operation preserves exactly the sharing the field provides.

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
and five `adr.NewCorpus` production sites in the package are independent of these fields and survive any
conversion of them (a sixth, the `adr.LoadCorpus` call inside `Corpus` at `project.go:486`, is the field
loader itself and moves with it). A claim asserting that `internal/project` derives its corpora per
operation in general would be either already satisfied or still false; the claim must name the cached
derivations it converts.

Second, `topic.Corpus` itself completes construction in two steps for a real reason. `LoadCorpus`
assembles the corpus, builds the marker index from it, and then writes `c.Markers = markers`
(`internal/topic/corpus.go:102-111`; the snapshot loader does the same at
`internal/topic/tree.go:66-75`), because `BuildMarkerIndex` needs the assembled corpus to resolve claim
ids. Since `topic.Corpus` is one of the values this decision threads, an immutability claim phrased as
"no field written after construction" would condemn a converted consumer's own dependency. The claim must
permit completing construction inside the constructing function.

Discoverability needs deliberate handling, because every surface ADR-0178 built names its topic by id
rather than naming the domain. Reasoned claims carry no mechanical enforcement, so ADR-0178's authority
commit added a focus item named `dependency-composition-authority` to `.awf/agents/adr-reviewer.yaml`,
`code-reviewer.yaml`, and `plan-reviewer.yaml`; `awf check` validates claim provenance and the absence of
a proof marker, never claim content, so a new topic of reasoned claims without a parallel focus item has
no enforcement path at all. And `focusItems` replaces catalog defaults wholesale, so the addition must
compare and backfill rather than append blindly. The same is true design-side: the workflow chain part
directs agents changing dependency selection, ownership, or wiring to `code-design/dependency-composition`
by id, and will not fire for a second topic either.

Unlike ADR-0178, this decision needs no new governance surface. The pathless `code-design` domain, the
`code-design` commit scope, and the architecture wiring all already exist, so only the topic itself, its
per-topic pointers, and a glossary term for the new vocabulary are added.

The commit scope does need a meaning correction, though. `.awf/config.yaml` gives `code-design` the
meaning "dependency composition and cross-package code structure", and
`dependency-composition:dependency-composition-commit-classification` says the same. That wording was
written when `code-design` owned one topic about dependencies. It now names a domain, and it fits neither
of the two commit kinds this pattern series produces: an authority commit that adds a topic is not
dependency composition, and this decision's conversion is entirely inside `internal/project`, a path the
`rendering` domain owns, so it is not cross-package either. ADR-0178's own conversion commit was
genuinely cross-package, reaching `cmd/awf` and `internal/project` together, so the mismatch did not
surface there.

This decision establishes the pattern and converts two consumers. Package-level cohesion, including
whether a
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
   outside the function that constructs it. That is the whole rule. A construction that genuinely needs
   more than one step completes every step inside the one function that constructs the value, whether
   the later field derives from the assembled value or from a separate input.

3. State an operation derives is owned by that operation and threaded explicitly to the consumers that
   need it, rather than stored on a value that outlives the operation. Threading reaches every consumer
   in the operation's call tree, including consumers behind an intermediate boundary, so that one
   derivation serves the whole operation.

4. A derivation is never kept correct by an ordering or invalidation step that each entry point must
   remember to perform. Where such a step exists today, the fix is to give the derivation a lifetime that
   makes staleness unrepresentable, not to document the step more loudly.

5. Within one operation, a derived value is produced exactly once, and every consumer receives it rather
   than re-deriving it. The rule counts productions per value per operation, not producers per type, so a
   type that several unrelated operations legitimately construct is not a counterexample.

6. Convert two concrete consumers, this one and the Pi preference store in item 12. Take the three
   cached derivations on `internal/project.Project` first:
   `corpus`, `topics`, and `effSkills`. Derive each in the operation that needs it and thread it to its
   consumers; delete `beginInvocation`; and delete or unexport `Corpus` and `Topics`, whose production
   callers all become threaded parameters.

   Derivation happens once per outermost operation, never once per public entry point. The general rule
   is that an operation reaching a derivation with no threaded value derives at its own entry, and every
   operation it calls receives that value. Concretely: `Check`, `syncReport`, `AdvisoryNotes`, and
   `ConfigReferenceModel` derive at their own entry, the last being `AdvisoryNotes`' twin in shape
   (`RenderAll` then `generateDomainDocs`, at `configreference.go:405` and `:409`). `OutputPlan` and
   `RenderAll` derive only when entered directly, as they are on the `PlannedOutputs` path that
   `InitCollisions` reaches from `awf init`, and receive the threaded value when reached from an
   operation that already derived; neither has a production caller outside `internal/project`, so
   parameterising them is available. `sweepConfigTree` and `generateDomainDocs` likewise receive rather
   than derive. This is item 3's intermediate-boundary clause applied by name: without it, `Check` and the
   `OutputPlan` it calls would each derive, turning one `BuildMarkerIndex` repository walk into two or
   three.

   Leave `QueryTopic`'s snapshot derivation and the rest of the `workingCurrentState` path unchanged
   beyond removing its vestigial reset at `topics.go:26`, so the one path that derives no cached corpus
   today gains nothing.

7. Back `code-design/state-ownership:project-derived-state-ownership` with one structural test that loads
   the production packages and asserts that no production function in `internal/project` writes a
   `*Project` field outside the function that constructs that value. The assertion covers package
   functions as well as methods, because `StagedContextRootOptions` is a function. The other four claims
   are reasoned contracts carrying `Backing: unbacked` and a `Verify:` instruction, with no proof marker.

   The item 12 conversion is covered by the existing TypeScript suite in `tools/pi-extension-test`,
   which `./x gate` already runs, and gains no `Backing: test` claim of its own. `currentState.testGlobs`
   is `**/*_test.go`, so a proof marker cannot live in a TypeScript test; backing a claim on that
   conversion would require either widening `testGlobs` to scan TypeScript, which is its own decision
   about what current-state authority covers, or a Go-side assertion string-matching rendered TypeScript,
   which is exactly the brittleness this project rejects elsewhere. The four reasoned claims govern that
   conversion, and its behaviour is proven where the code lives.

   Do not use a behavioural "`Check` after `Sync`" assertion as the proof: that property already holds,
   because `Check` calls `beginInvocation` for exactly that reason, so such a test passes before the
   change and proves nothing about ownership.

8. Give the topic both a review-side and a design-side anchor. Add one reviewer focus item naming
   `code-design/state-ownership` to `.awf/agents/adr-reviewer.yaml`, `.awf/agents/code-reviewer.yaml`, and
   `.awf/agents/plan-reviewer.yaml`, comparing against and backfilling the catalog defaults each list
   replaces, so the reasoned claims have an enforcement path. Extend the workflow chain part
   (`.awf/parts/workflow/chain.md`), which already directs agents changing dependency selection to
   `code-design/dependency-composition`, so that agents changing what a value owns or where derived state
   lives consult this topic before design or implementation. Add a glossary term for the new vocabulary.
   The development dependencies part stays unchanged: it is about selecting dependencies, which remains
   `dependency-composition`'s subject.

9. Re-derive every `coverage-ignore` justification whose unreachability argument rests on a shared cache
   or on a prior pass having already derived the same value. The known sites are `sweep.go:119`,
   `render.go:813`, `output_plan.go:532`, `check.go:34`, `check.go:40`, `check.go:446`, `check.go:451`,
   `check.go:456`, `configreference.go:410`, and `render.go:474`, whose exclusion exists only because the
   effective skill set is derived twice and so disappears with item 5. Treat that list as known rather
   than exhaustive: the terminal condition is that no surviving justification in `internal/project` names
   a shared cache or a prior pass's derivation. Threading removes the error return in most cases, which
   deletes the exclusion outright.

10. Treat the three remaining stepwise `*Project` constructions as conforming under item 2 and leave them
    out of the conversion: `Loader.Open` writes `p.Cat` after its literal at `project.go:164-175`,
    `ContextForOptions` writes `universe.Targets` and `universe.Cat` after its literal at
    `context.go:44-49`, and `StagedContextRootOptions` writes `p.Cfg`, `p.Targets`, and `p.Cat` after its
    literal at `context.go:61-71`. Each write is inside the function that constructs the value and is
    ordered by a real dependency, so item 2 permits all three. They are named here because item 7's
    structural test must permit them, and because a reviewer auditing item 2 against its exemplar type
    will find them.

11. Widen the `code-design` commit scope from dependency composition to code-design authority generally.
    Change its `meaning` in `.awf/config.yaml` to cover code-design authority and cross-package code
    structure, and update
    `code-design/dependency-composition:dependency-composition-commit-classification` to match, leaving
    its second half (a structural change uses the existing `refactor` type, not a `refactor` scope)
    unchanged. Carry its `Verify:` line with the widening too: it currently names dependency-composition
    subjects and an ADR-0178-specific staged-scope-addition step, so after the widening it would verify a
    narrower claim than the one it belongs to. This lands with the authority batch, not the conversion.
    Without it the scope's documented
    meaning covers neither an authority commit that adds a code-design topic nor an intra-package
    conversion performed under one, so this ADR's own commits would have no correct scope. The conversion
    commits then use `code-design` rather than the owning domain's `rendering`, because their subject is
    the code-design authority they apply.

12. Convert the Pi preference store as the second concrete consumer, in
    `templates/pi/awf-subagents/model-routing.ts.tmpl` and `index.ts.tmpl`. Make `resolveChildModel` take
    the already-derived `EffectivePreferenceState` instead of the store, so it stops re-deriving at
    `model-routing.ts.tmpl:208` what its caller already holds; its one production call site is
    `index.ts.tmpl:483`, inside a closure where that state is in scope. Replace the closure-mutable
    `global`, `project`, `registryInvalid`, and `ready` slots with a load step that returns an immutable
    value, so reading preferences becomes one derivation rather than a construct-then-`reload`-then-
    `validateAgainstRegistry` sequence a caller must remember. Update the five `resolveChildModel` call
    sites in `tools/pi-extension-test/tests/index.test.ts` to pass a state.

    This conversion changes rendered adopter-facing output, so it lands with its regenerated artifacts
    like any other template change, and the example adopter's rendered diff is part of the review.

## State changes

- add `code-design/state-ownership:construction-immutable-state`
- add `code-design/state-ownership:operation-owned-derivation`
- add `code-design/state-ownership:no-remembered-invalidation`
- add `code-design/state-ownership:single-derivation-producer`
- update `code-design/dependency-composition:dependency-composition-commit-classification`
- add `code-design/state-ownership:project-derived-state-ownership`

## Consequences

awf gains a second code-design authority that answers a question `dependency-composition` left open: not
where a dependency comes from, but what a value keeps afterwards. The two compose, because a threaded
derivation is a dependency the consumer receives rather than discovers.

The conversion removes two latent defects. `Check`'s dead-skill-reference pass stops depending on
`OutputPlan` having run first, so no future caller can report zero dead skill references from a nil map.
And the drift oracle stops depending on every future public operation remembering to reset a cache; the
sixth operation that reads a corpus cannot introduce the staleness the current comment predicts, because
there is no cache to go stale. Threading also tends to delete error returns, since a parameter cannot
fail, which should retire several coverage exclusions rather than merely re-justify them.

Taking two consumers rather than one costs a wider transaction but buys a much better test of the
claims. The two are genuinely different: one is Go inside awf's own binary, the other is TypeScript that
renders into every adopter's extension, and the Pi store exhibits all four reasoned claims at once where
`internal/project` splits them across three fields. A pattern that reads well against only the code it
was derived from is worth less than one that survives a second, unlike consumer. The cost is that the
adopter-facing half has no mechanical anchor, for the `testGlobs` reason in item 7, so its conformance
rests on review rather than on a gate.

The cost is visible parameters. Roughly ten method signatures across six production files grow a corpus
or skill-set parameter, and about thirty-three test call sites move with them. `Corpus` and `Topics` must
be deleted or unexported in the same transaction, because the dead-code gate flags an exported method
whose production callers have become parameters, and that breaks the four `Topics` call sites in
`internal/project/topics_test.go`. Test-side risk is otherwise smaller than the package's test volume
suggests: no test sets `corpus` or `topics`, and the existing `Project` literals in tests never set them.

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
`workingCurrentState` snapshot path already does so and five corpus-construction sites in the package
survive the conversion. A reader looking for a blanket guarantee about corpus construction in that
package will not find one here. Item 5 is scoped the same way, per value per operation rather than per
producer per type, so the surviving sites are not counterexamples to it either.

The topic's own coverage view will be empty, exactly as `dependency-composition`'s is, because the
pathless domain declares no selectors and a global topic's applicability is computed against them.
Discovery therefore rests on four pointers that name the topic by id, the three reviewer focus items and
the workflow chain part, plus `awf context`, which attaches global topics to a queried path. The only
mechanical anchor is the proof marker on the test-backed claim.

Widening the `code-design` scope reaches outside this decision's own topic: it mutates a claim ADR-0178
established. That is the intended mechanism for changing a prior decision's current-state claim rather
than editing the earlier record, but it does mean this ADR's authority batch changes the commit taxonomy
that governs its own later commits, so the config change, the claim update, and the rendered scope tables
travel together or `awf check` fails on the rendered workflow doc.

Finally, this decision creates downstream work it does not perform. Package-level cohesion in
`internal/project` becomes the next code-design pattern, and it now has a stated prerequisite: a value
whose fields are all construction inputs is far easier to split than one carrying three derived caches.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the caches and document the reset obligation more strongly | The obligation is already documented and already inexact: one of its five call sites is vestigial and another is nested-redundant. Prose cannot be gated. |
| Keep the caches but add a gate that every public operation resets first | It enforces a mechanism instead of removing the need for one, and it cannot distinguish an operation that reads a corpus from one that does not. |
| Convert `corpus` and `topics` only, leaving `effSkills` | `effSkills` is the only genuine two-production case in the package, so excluding it would leave item 5 with no consumer at all, and would leave its ordering hazard in place. |
| Include the four synthetic partial `Project` literals and the fourteen zero-field files | That is package cohesion, a separate decision. Including it would make this effort the `internal/project` decomposition rather than its prerequisite. |
| Prove the claim with a behavioural `Check`-after-`Sync` test | The property already holds today, so the test passes before the change and cannot be written failing first. |
| Prove the claim with an approved-call-set test, as ADR-0130 did for `ParseDir` | It governs where a corpus is constructed rather than whether a value keeps it, which is not what the claim states. |
| Fold these rules into `code-design/dependency-composition` | Dependency selection and state lifetime are separate subjects; merging them would make one topic's claims answer two questions and blur which authority a reviewer is applying. |
| Leave the `code-design` scope meaning alone and commit the conversion as `rendering` | The scope's documented meaning would still cover no authority commit, and a conversion's code-design character would be invisible in its subject line. |
| Convert only `internal/project` and leave the Pi preference store to a follow-on | It is the strongest instance of this decision's own anti-pattern, it exhibits all four reasoned claims at once, and it ships to adopters, so establishing the rule while leaving it in place would weaken both the rule and the record. |
| Widen `currentState.testGlobs` to scan TypeScript so the Pi conversion can carry a proof marker | That decides what current-state authority covers, which is a separate question from state ownership and should not ride along inside this decision. |
| Add a generic section on immutability to `docs/maintainable-code-design.md` instead | That document's sections are a fixed list and its value is generic guidance. The whole point of a topic is being specific and mechanically reviewable where the guide cannot be. |
| Derive eagerly at the entry of every public operation | `QueryTopic` derives from the snapshot path and touches neither field, so a blanket rule would add a repository walk to the one operation that currently avoids it; and deriving at a nested entry as well as its caller's would multiply the walk rather than share it. |

## Status history

- 2026-07-30: Proposed
- 2026-07-30: Implementing; content-sha256: c0d35a88e9e7b0778184de45b27931f7be17fb74ee7ea4f3afc914ca9134b12c
- 2026-07-30: Applied; operations: add `code-design/state-ownership:construction-immutable-state`, add `code-design/state-ownership:operation-owned-derivation`, add `code-design/state-ownership:no-remembered-invalidation`, add `code-design/state-ownership:single-derivation-producer`, update `code-design/dependency-composition:dependency-composition-commit-classification`
- 2026-07-30: Applied; operations: add `code-design/state-ownership:project-derived-state-ownership`
- 2026-07-30: Implemented; content-sha256: c0d35a88e9e7b0778184de45b27931f7be17fb74ee7ea4f3afc914ca9134b12c
