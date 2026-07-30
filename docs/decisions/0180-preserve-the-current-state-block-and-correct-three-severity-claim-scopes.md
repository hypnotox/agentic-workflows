---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0180: Preserve the currentState block and correct three severity claim scopes

## Context

ADR-0179 removed the two `currentState` severity configuration keys behind schema generation 24 and
declared five claims. Independent review of the implementation found one behavioural defect and two
claim sentences broader than the code they govern. All three findings need an operation against a
claim ADR-0179 already applied, and ADR-0179 is Implemented, so its meaning is frozen and cannot
carry them.

**The behavioural defect.** `config.RemoveMappingKey` drops a parent mapping when the removal empties
it, so a tree whose `currentState` block held only `topicCoverage` and `topicFanout` loses the whole
block on upgrade. `internal/project/currentstate.go` gates coverage evaluation on
`Cfg.CurrentState != nil` at both its working-tree and staged call sites, so such a tree stops
evaluating topic coverage AND fan-out entirely. That is the exact inverse of ADR-0179 item 1, whose
stated consequence is that a tree which had set `off` starts seeing findings after upgrading. The
migration's own fixture asserted the collapse, so the defect shipped with a passing test and a claim
sentence, `severity-keys-dropped`, that its own proof fixture contradicted.

Adopter exposure is nil: both in-repo trees carry `sources`, `testGlobs`, and both maxima, and no
adopter is migrated near this generation. The defect is that the shipped behaviour contradicts the
decision that authorized it, not that anyone is currently harmed.

**Two claim scopes broader than reality.** `severity-not-configurable` asserted universally that no
configuration value selects, suppresses, or reranks the two checks. Declining to declare a
`currentState` block suppresses both, through the same nil gate, so the universal is false; only the
severity-value half of it is true. `coverage-evaluation-selects-checks` asserted that the uncovered
report requests coverage only. Review verified by mutation that flipping
`internal/project/context.go` to `Fanout: true` leaves the whole suite green, and the reason is
structural rather than a missing test: the report filters on `f.Kind == topic.Uncovered`, so a
fan-out finding is discarded before it can be observed. The clause states a real design intent -
do not request work you throw away - but nothing observable distinguishes it, so `Backing: test`
cannot honestly carry it.

## Decision

1. When removing `currentState.topicCoverage` and `currentState.topicFanout` would empty the
   `currentState` block, the schema-24 migration seeds the explicit default `maxTopicsPerPath` and
   announces it, so the block survives and both checks keep evaluating. The seed fires only where a
   block was present and these removals emptied it: a tree that never declared `currentState` is
   deliberately opted out and must not have one invented for it.

2. The seed is written into the pre-removal bytes rather than added to the collapsed result.
   `config.SetMappingInteger` appends an absent parent at the end of the document, so repairing a
   collapsed block afterwards would silently relocate an adopter's `currentState` block.

3. `migrate.ConfigForCurrentSchema`'s generation-24 branch stays a pure removal and does not seed.
   It exists so a historical committed config parses under the current strict decoder, and no
   coverage is ever evaluated from a before-side config, so materializing a key the committed bytes
   never carried would put a value into a historical universe rather than fix a parse.

4. `config/migrations-and-locks:severity-keys-dropped` is updated to describe the seed, replacing a
   sentence its own proof fixture contradicted.

5. `config/configuration:severity-not-configurable` is narrowed to severity values, and states
   explicitly that whether the checks run at all is a separate concern which a tree declaring no
   `currentState` block answers by requesting neither.

6. `invariants/topics-and-markers:coverage-evaluation-selects-checks` drops the unobservable
   uncovered-report clause and keeps what `EvaluateCoverage` actually guarantees: the two checks are
   requested independently, an unrequested check produces none of its findings, and no rank value
   suppresses a requested one.

7. `internal/config` gains `HasMapping`, so a caller can distinguish an absent block from one its own
   removals just created without parsing `config.yaml` itself. Config serialization knowledge stays
   in `internal/config` (ADR-0026).

## State changes

- update `config/configuration:severity-not-configurable`
- update `config/migrations-and-locks:severity-keys-dropped`
- update `invariants/topics-and-markers:coverage-evaluation-selects-checks`

## Consequences

An upgraded tree keeps evaluating the checks ADR-0179 said always evaluate, and the three claims
describe what the code does. The rendered glossary entry and the changelog entry, which inherited the
same over-breadth from the claim they paraphrased, are corrected with them.

The cost is a `maxTopicsPerPath: 8` key appearing in a config that never set one. That is a visible
edit to an adopter's file, which is why it is announced, and it materializes a value that was already
in force as the implicit default rather than changing behaviour.

Item 6 leaves a design intent unpinned: nothing fails if a future edit makes the uncovered report
request fan-out it then discards. That is accepted rather than papered over. A claim whose proof
cannot fail is worse than an absent claim, because it reports safety the gate is not providing. The
intent survives as a comment at the call site.

Ruled out: relaxing the `CurrentState != nil` gate so an absent block still evaluates. That would
turn current-state coverage from opt-in into always-on for every adopter, which is a far larger
decision than this correction, and ADR-0179 did not take it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Let the block collapse and narrow the claim to match | Ships behaviour that contradicts ADR-0179 item 1 with no record reconciling the two; the claim would be honest about a defect rather than the defect being fixed. |
| Seed into the collapsed result instead of the original bytes | `SetMappingInteger` appends an absent parent at the document end, silently relocating the adopter's block. |
| Amend ADR-0179 rather than write this decision | Its meaning is frozen once it leaves Proposed; a later decision changes the claims an earlier one established rather than editing it. |
| Keep the uncovered-report clause and add a marker for it | No test can fail on it: the report filters fan-out findings out by kind, so the clause is unobservable and any marker would stay green through the regression it claims to catch. |
| Preserve the block by seeding in `ConfigForCurrentSchema` too | That function repairs parseability of history, not the tree; seeding there would inject a value into a historical universe. |

## Status history

- 2026-07-30: Proposed
