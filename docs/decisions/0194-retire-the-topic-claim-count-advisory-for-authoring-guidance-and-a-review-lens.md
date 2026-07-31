---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0194: Retire the topic claim-count advisory for authoring guidance and a review lens

## Context

`currentState.maxClaimsPerTopic` is a positive integer with an effective default of 20.
`awf check` emits one non-failing note for each topic whose claim count is strictly above
it, naming the count, the limit, and the two paths to split. The staged check suppresses the
note, so it is a working-tree authoring advisory only.

The budget was introduced to stop a large topic from dominating an automatic context answer.
That harm no longer exists there. The current projection contract,
`tooling/context-and-topic:context-concise-projection`, states that context renders each
topic summary and active invariant/rule count once, that direct claim summaries deduplicate
globally, that non-direct invariant and rule summaries require their respective facets, and
that every visible claim has a deterministic bounded summary. The concise answer therefore
scales with what a request selects, not with how many claims the selected topic happens to
hold, and no configured count changes that.

The explicit drilldown is a separate surface and was never bounded either: `awf topic
<domain>/<topic>` prints every claim body in full, and the threshold never truncated query
output. So neither surface the reader can reach is protected by the count.

What the advisory still measures is a raw claim count standing in for topic cohesion, and
the count is a poor proxy that points the wrong way. Splitting one broad claim into two
precise ones is the hygiene the claim model wants, and it moves the topic toward the note
rather than away from it. A count also cannot distinguish twenty-one tightly related claims
about one mechanism from twelve unrelated ones spanning three concerns, though a reader
judges those oppositely. Exactly one topic trips the note today,
`rendering/workflow-skill-templates` at 21 claims, and splitting it is not the remedy that
topic needs. The advisory's only current effect is a standing note an adopter learns to
ignore, which is how a non-failing check loses its meaning.

Two structural facts make the removal small and safe now. `internal/config`'s
`SkeletonCurrentState` has exactly one field, so every scaffolded tree wrote a
`currentState` block containing only this key. Until recently, deleting the key would have
collapsed that block, and `internal/project/currentstate.go` gated coverage and fan-out
evaluation on the block being present, so an ordinary adopter would have lost both checks
silently. ADR-0192 removed that gate: evaluation is unconditional and block presence carries
no meaning. Separately, both this repository's `.awf/config.yaml` and the `examples/sundial`
adopter carry `maxTopicsPerPath` alongside the retired key, so their blocks survive the
removal regardless; only a freshly scaffolded tree loses its block, which is now inert.

The instinct behind the budget is still worth keeping. Topic cohesion is a real authoring
concern; it is simply not a countable one. It is answerable at the two moments a claim
population actually changes: when prose is authored, and when a decision record chooses the
destination topic for a claim it adds. ADR-0135 item 5 already requires that destination
topic's metadata to exist before a decision reaches Accepted, so the choice is already on
the table at review time.

## Decision

1. Retire `currentState.maxClaimsPerTopic` from the configuration surface entirely: the
   `MaxClaimsPerTopic` field on `config.CurrentStateConfig`, its decoder case, the
   `EffectiveMaxClaimsPerTopic` accessor, its positive-integer validation entry, its
   `internal/configspec` entry, its `internal/project/configreference.go` case, and the key
   in this repository's `.awf/config.yaml` and in `examples/sundial/.awf/config.yaml`.

2. Remove `topic.ClaimBudgetNotes` and its single production caller in
   `internal/project/currentstate.go`. `awf check` emits no claim-count note in any mode.
   Remove the `Advisories` field on `CurrentStateReport` with it, along with the
   `slices.Clone` and nil-guard prologue in `Notes()`, so `Notes()` builds directly from the
   warn-severity coverage findings. That caller is the field's only production writer, so
   retaining the field would leave state nothing can populate and a guard branch only a test
   can reach. `currentState.maxTopicsPerPath` is deliberately untouched: it detects a path
   matching too many topics, which is a different condition carrying real severity, and
   nothing here argues against it.

3. Delete `config.SkeletonCurrentState` and the `CurrentState` field on the skeleton it
   populates, and drop the seed in `internal/project/scaffold.go`. A newly scaffolded tree
   carries no `currentState` block. This is safe only because ADR-0192 made coverage and
   fan-out evaluation independent of block presence, and it is why the type is deleted
   rather than emptied: an empty struct would leave the dead-code gate with an unreachable
   type.

4. Add a schema migration at generation 28 that removes `currentState.maxClaimsPerTopic` and
   announces the removal on the command's output, modelled on the severity-key removal but
   without its block-preservation seed, which ADR-0192 made unnecessary. Advance the schema
   generation to 28 and map it onto the existing `project.Version` of `0.30.0` in
   `minVersionBySchema`. `project.Version` does not change: 0.30.0 is unreleased, and
   generations 26 and 27 already share it for that reason, so a new generation on an
   unreleased version raises no adopter's minimum. Both `.awf/awf.lock` and
   `examples/sundial/.awf/awf.lock` sit at schema 27 and take the new `schemaVersion` while
   their `awfVersion` stays `0.30.0`, and the sundial tree is migrated in the same commit as
   the rest of the transaction. The render-hash and lock-input consumers the retired claim
   names need no code change: they hash config content rather than reading this key by name,
   so removing it changes the recorded hashes and nothing else.

   Registering the generation also breaks three assertions that pin the current one, which
   land in the same transaction: `internal/project/version_test.go` asserts both that
   `minVersionBySchema[27]` equals `Version` and that `ValidateSchemaMinimumVersion(28, ...)`
   reports "no minimum", and `internal/migrate/dropworkflowtelemetry_test.go` pins
   `Current()` at 27. The first survives unchanged because 28 maps onto the same version; the
   unmapped-schema probe moves to 29 and the registry pin moves to 28.

5. Reject a surviving key rather than tolerating it. `config.yaml` is strict-parsed, so an
   unmigrated tree hard-fails on the new binary with an actionable error naming the key.
   There is no tolerate-and-ignore transition period, matching the severity-key precedent
   and the standing position that a stale config key is an error state, not a warning.

6. Keep the historical migration that added the key. Historical migrations are never deleted
   or edited; the pair sits beside the equivalent add/drop pairs already in the registry.

7. Delete the four proof markers citing the two retired claims, because an orphaned marker
   naming a removed claim id hard-fails `awf check` in the same commit. They are
   `cmd/awf/check_test.go` for the advisory claim, and `internal/config/config_test.go`,
   `internal/config/edit_test.go`, and `internal/configspec/spec_test.go` for the configured
   claim. Only one test is deleted outright: `cmd/awf/check_test.go`'s claim-budget-note test,
   together with its unmarked sibling asserting that the staged check suppresses the note,
   which tests behaviour that no longer exists. The other three keep the test and lose only
   the marker, because each also asserts something that survives: `internal/config/edit_test.go`
   marks the `SetMappingInteger` test, which item 6's retained migration still uses;
   `internal/config/config_test.go` also asserts `currentState` presence and absence and the
   `maxTopicsPerPath` default; and `internal/configspec/spec_test.go` also asserts the
   surviving `currentState` configspec key set. Fixture data that uses the retired key merely
   as a sample nested key is rewritten to a surviving one.

8. Add a topic-cohesion authoring rule to the `rules` section of the shipped
   `templates/docs/doc-standard.md.tmpl`. It directs the author to judge whether a topic's
   claims describe one mechanism a reader would look up together, and to split on subject
   rather than on size. It is written generically and publication-safe, naming no count and
   no project-specific number, because a check every adopter had is being withdrawn and the
   replacement guidance must reach every adopter. It goes in the shipped template rather than
   a project-local part because a part override replaces the whole `rules` section wholesale,
   which would fork the shipped rules to add one.

9. Add a cohesion focus lens to the `adr-reviewer` agent, asking the reviewer to judge
   whether each claim an ADR adds belongs in its chosen destination topic. It is added in
   both `internal/catalog/standard.go` and this repository's `.awf/agents/adr-reviewer.yaml`,
   because that sidecar replaces `focusItems` wholesale; adding it in one place alone would
   either ship it to every adopter while silently skipping this repository, or the reverse.

10. Update every source that states the retired behaviour, in the same commit. Two are
    shipped templates and therefore adopter-facing: `templates/docs/working-with-awf.md.tmpl`
    carries a paragraph describing the advisory and its explicit default, and
    `templates/docs/agents-md-standard.md.tmpl` carries a sentence on the note never
    truncating a projection. The authored project sources include
    `.awf/docs/parts/testing/gate.md` and the roadmap part item 12 withdraws; every edit is
    made in the authored source, never in a generated file. The `./x render` sweep then
    regenerates the derived surfaces, including `docs/testing.md`, `docs/roadmap.md`,
    `docs/agents-md-standard.md`, `docs/config-reference.md`, the two topic docs,
    `docs/decisions/INDEX.md`, the `examples/sundial` renders of the same templates, and both
    locks.

11. Add a `[Unreleased]` breaking-change entry to `changelog/CHANGELOG.md` naming the removed
    key, the withdrawn note, the new schema generation, and `awf upgrade` as the remedy for an
    unmigrated tree. Removing a config-schema key is adopter-facing, which is the same
    reasoning the severity-key removal applied.

12. Withdraw the roadmap idea proposing to promote this advisory from a non-failing note to a
    fixed blocking rank, by deleting it from the authored part `.awf/docs/parts/roadmap/ideas.md`
    rather than from its `docs/roadmap.md` render. Retiring the check makes the idea incoherent
    rather than merely stale, so it is removed rather than rewritten.

13. Mint no claim for the authoring rule or the review lens. Neither is mechanically
    enforceable, and minting a claim to describe advice is what inflates a claim population
    without adding a checkable contract. The net effect on the corpus is two claims fewer.

## State changes

- remove `tooling/cli:topic-claim-budget-advisory`
- remove `config/configuration:topic-claim-budget-configured`

## Consequences

The configuration surface loses a key, and `awf check` loses its only claim-count note. An
adopter who had tuned the limit loses that tuning; the migration announces the removal on
its output so the loss is readable from the upgrade rather than recovered from history.

An unmigrated tree fails hard on the new binary instead of degrading. That is the intended
cost of item 5: the failure is loud, names the key, and is fixed by running the upgrade.

Topic cohesion becomes a judged concern rather than a measured one. This is the real
trade-off: nothing mechanical will flag a topic that has quietly become two, and a reviewer
who skips the lens will not be caught by the gate. That is accepted because the mechanical
signal being withdrawn pointed away from the outcome it was meant to protect, and a wrong
signal is worse than an absent one. The two moments the replacement covers, prose authoring
and destination-topic choice at ADR review, are the two moments a claim population changes.

A freshly scaffolded tree now mentions `currentState` nowhere, because item 3 deletes the
skeleton's only producer of that block. A new adopter therefore gets no in-file signal that
`sources`, `testGlobs`, and `maxTopicsPerPath` exist, and discovers the family from the
generated config reference instead. This is accepted rather than mitigated: seeding a block
purely as documentation would reintroduce the write-a-key-to-keep-a-block-alive shape that
ADR-0192 just removed the need for, and the config reference is the intended discovery path
for every other key family already.

The `adr-reviewer` lens must be added in two places that cannot be kept in sync
mechanically, because per-key sidecar merging replaces `focusItems` rather than appending to
it. Item 9 makes the duplication explicit; the same hazard is already recorded for this
agent's `docCurrencyItems`.

`rendering/workflow-skill-templates` stops emitting its note. Whether that topic should be
split remains an open authoring question, now answerable on its subject rather than on its
count, and no longer forced by a threshold.

`config/configuration:config-serialization-owned` is unaffected: it names the config editors
generically rather than any key, and the nested-integer setter it names stays in use by the
retained historical migrations.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the counter and raise the limit | Moves the threshold without fixing the proxy; the next hygienic claim split walks the topic back over whatever number is chosen. |
| Re-measure the budget hygiene-neutrally, for example by counting distinct subjects | No mechanical definition of subject exists, and inventing one would encode the same judgement the review lens makes, with less context and no way to be argued with. |
| Keep it as a human-authoring-load guard rather than a context-size guard | Reframes the check without changing what it measures; claim count is no better a proxy for authoring load than for cohesion, and the note's remedy would still be to split. |
| Promote the note to a blocking rank | The roadmap idea this decision withdraws. Making a wrong signal blocking multiplies its cost; it would force splits on topics whose only fault is precise claims. |
| Tolerate and ignore the retired key for one generation | Leaves a key that means nothing readable in adopter configs and defers the failure to whenever someone next reads it. The strict-parse hard failure is louder and shorter-lived. |
| Add the authoring rule as a project-local doc part | A part override replaces the whole `rules` section, forking every shipped rule to add one, and would withhold the replacement guidance from the adopters losing the check. |
| Put the cohesion lens on `code-reviewer` | Its claim lenses judge prose truth and proof strength after the destination is already chosen; the topic choice is made in an ADR's `State changes`, so that is where it is answerable. |
| Retire `maxTopicsPerPath` in the same decision | A different check on a different condition, with real severity and no argument against it. Bundling it would put two unrelated commitments in one record. |

## Status history

- 2026-07-31: Proposed
