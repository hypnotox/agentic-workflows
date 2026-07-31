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

The budget was introduced to stop a large topic from dominating a context answer. That harm
no longer exists. The current projection contract,
`tooling/context-and-topic:context-concise-projection`, states that every visible claim has
a deterministic bounded summary and that non-direct invariant and rule summaries require
their respective facets; a topic contributes its summary and its active claim counts, and
its individual claims reach the reader only behind an explicit facet. The concise answer
therefore does not scale with topic size, and no configured count changes that.

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
   `currentState.maxTopicsPerPath` is deliberately untouched: it detects a path matching too
   many topics, which is a different condition carrying real severity, and nothing here
   argues against it.

3. Delete `config.SkeletonCurrentState` and the `CurrentState` field on the skeleton it
   populates, and drop the seed in `internal/project/scaffold.go`. A newly scaffolded tree
   carries no `currentState` block. This is safe only because ADR-0192 made coverage and
   fan-out evaluation independent of block presence, and it is why the type is deleted
   rather than emptied: an empty struct would leave the dead-code gate with an unreachable
   type.

4. Add a schema migration at the next generation that removes `currentState.maxClaimsPerTopic`
   and announces the removal on the command's output, modelled on the severity-key removal
   but without its block-preservation seed, which ADR-0192 made unnecessary. Advance the
   schema generation, add the matching `minVersionBySchema` entry, and bump `project.Version`.

5. Reject a surviving key rather than tolerating it. `config.yaml` is strict-parsed, so an
   unmigrated tree hard-fails on the new binary with an actionable error naming the key.
   There is no tolerate-and-ignore transition period, matching the severity-key precedent
   and the standing position that a stale config key is an error state, not a warning.

6. Keep the historical migration that added the key. Historical migrations are never deleted
   or edited; the pair sits beside the equivalent add/drop pairs already in the registry.

7. Add a topic-cohesion authoring rule to the `rules` section of the shipped
   `templates/docs/doc-standard.md.tmpl`. It directs the author to judge whether a topic's
   claims describe one mechanism a reader would look up together, and to split on subject
   rather than on size. It is written generically and publication-safe, naming no count and
   no project-specific number, because a check every adopter had is being withdrawn and the
   replacement guidance must reach every adopter. It goes in the shipped template rather than
   a project-local part because a part override replaces the whole `rules` section wholesale,
   which would fork the shipped rules to add one.

8. Add a cohesion focus lens to the `adr-reviewer` agent, asking the reviewer to judge
   whether each claim an ADR adds belongs in its chosen destination topic. It is added in
   both `internal/catalog/standard.go` and this repository's `.awf/agents/adr-reviewer.yaml`,
   because that sidecar replaces `focusItems` wholesale; adding it in one place alone would
   either ship it to every adopter while silently skipping this repository, or the reverse.

9. Withdraw the roadmap idea proposing to promote this advisory from a non-failing note to a
   fixed blocking rank. Retiring the check makes the idea incoherent rather than merely
   stale, so it is removed rather than rewritten.

10. Mint no claim for the authoring rule or the review lens. Neither is mechanically
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

The `adr-reviewer` lens must be added in two places that cannot be kept in sync
mechanically, because per-key sidecar merging replaces `focusItems` rather than appending to
it. Item 8 makes the duplication explicit; the same hazard is already recorded for this
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
