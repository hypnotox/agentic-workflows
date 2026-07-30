---
format: current-state-v2
status: Implemented
date: 2026-07-30
---
# ADR-0184: Preserve the currentState block and correct three severity claim scopes

## Context

ADR-0183 removed the two `currentState` severity configuration keys behind schema generation 25 and
declared five claims. Independent review of the implementation found one behavioural defect, one
claim sentence broader than its code, and one claim clause that nothing could fail on. All three need
an operation against a claim ADR-0183 already applied, and ADR-0183 is Implemented, so its meaning is
frozen and cannot carry them.

**The behavioural defect.** `config.RemoveMappingKey` drops a parent mapping when the removal empties
it, so a tree whose `currentState` block held only `topicCoverage` and `topicFanout` loses the whole
block on upgrade. `internal/project/currentstate.go` gates coverage evaluation on
`Cfg.CurrentState != nil` at both its working-tree and staged call sites, so such a tree stops
evaluating topic coverage AND fan-out entirely. That is the exact inverse of ADR-0183 item 1, whose
stated consequence is that a tree which had set `off` starts seeing findings after upgrading. The
migration's own fixture asserted the collapse, so the defect shipped with a passing test and a claim
sentence, `severity-keys-dropped`, that its own proof fixture contradicted.

The reachable population is hand-authored trees only, and for a structural reason rather than a
circumstantial one: no awf-produced tree can arrive at generation 24 with a block these two removals
would empty, because `internal/project/scaffold.go` writes `maxClaimsPerTopic` into every scaffolded
tree and the generation-16 migration adds it to every tree that predates scaffolding. So the defect is
that shipped behaviour contradicts the decision authorizing it, not that a known adopter is harmed.

**One claim broader than its code.** `severity-not-configurable` asserted universally that no
configuration value selects, suppresses, or reranks the two checks. Declining to declare a
`currentState` block suppresses both, through the same nil gate, so the universal is false; only the
severity-value half of it is true.

**One clause nothing could fail on, and why that turned out to be fixable.**
`coverage-evaluation-selects-checks` asserted that the uncovered report requests coverage only. Review
verified by mutation that flipping `internal/project/context.go` to `Fanout: true` leaves the whole
suite green. The first reading was that the clause is inherently unobservable and had to be dropped.
That reading was wrong. The clause is unobservable only because `assembleUncovered` filters its
results on `f.Kind == topic.Uncovered`, a guard that is dead today given the `Fanout: false` on the
line above it. With that filter deleted, the same mutation fails two existing assertions
(`TestUncovered` and `TestUncoveredScanRoot` in `internal/project/context_test.go`), because the
policy literal supplies no budget, so `count > 0` fires for any path carrying a scoped topic and the
fan-out findings reach the uncovered list. Both halves were verified by running them.

That filter is also the shape ADR-0183 item 2 argued against. Its selection split exists so that a
caller which does not want a finding class does not request it, rather than requesting it and
discarding the answer. A kind filter over an unrequested class is that discarded answer in its last
remaining form.

## Decision

1. When removing `currentState.topicCoverage` and `currentState.topicFanout` would empty the
   `currentState` block, the schema-25 migration seeds the explicit default `maxTopicsPerPath` and
   announces it, so the block survives and both checks keep evaluating. `maxTopicsPerPath` rather than
   the conventionally-materialized `maxClaimsPerTopic`, because it is the key the coverage evaluation
   actually reads through `coveragePolicy`, which makes the surviving block self-evidently about the
   checks being preserved.

2. The seed is scoped to repairing a block that was present: the migration must not change which trees
   have a `currentState` block, only keep one it would otherwise destroy. This is narrower than a
   principle that an absent block means a deliberate opt-out, which awf does not hold: scaffolding and
   the generation-16 migration both write the block into trees that never asked for one. The
   population that reaches this migration with no block is hand-authored trees, and the migration
   leaves them as they are.

3. The seed is written into the pre-removal bytes rather than added to the collapsed result.
   `config.SetMappingInteger` appends an absent parent at the end of the document, so repairing a
   collapsed block afterwards would silently relocate an adopter's `currentState` block.

4. `migrate.ConfigForCurrentSchema`'s generation-25 branch stays a pure removal and does not seed. It
   exists so a historical committed config parses under the current strict decoder. What makes the
   asymmetry safe is the version gate, not a before-side-only property: the function is reached from
   the working-tree and staged-after paths too, and both do evaluate coverage. The generation-25 branch
   fires only for a side whose own lock is below 25, and `gate` and `gateStaged` in `cmd/awf/gate.go`
   refuse every coverage-evaluating command against such a lock. The collapsed shape is therefore only
   ever a parse repair for a historical universe.

5. `assembleUncovered` in `internal/project/context.go` drops its `f.Kind == topic.Uncovered` filter.
   The call requests coverage only, so the filter guards against a class the evaluator was never asked
   to produce. Removing it makes the selection contract load-bearing: a later edit that requests
   fan-out here fails a test instead of being silently swallowed. Output is unchanged today, verified
   by running `internal/project` with the filter removed and the policy untouched.

6. `config/migrations-and-locks:severity-keys-dropped` is updated to describe the seed, replacing a
   sentence its own proof fixture contradicted.

7. `config/configuration:severity-not-configurable` is narrowed to severity values, and states
   explicitly that whether the checks run at all is a separate concern which a tree declaring no
   `currentState` block answers by requesting neither.

8. `invariants/topics-and-markers:coverage-evaluation-selects-checks` KEEPS its uncovered-report
   clause, now backed by the assertions item 5 makes load-bearing, and additionally states that an
   unrequested check produces none of its findings.

9. `internal/config` gains `HasMapping`, so a caller can distinguish an absent block from one its own
   removals just created without parsing `config.yaml` itself. Config serialization knowledge stays in
   `internal/config` (ADR-0026). No operation against
   `config/configuration:config-serialization-owned` is needed: that claim governs how config.yaml is
   constructed and mutated, and this is a read-only probe. `RemoveMappingKey` is deliberately not
   extended to report or avoid the parent drop instead: its collapse is a documented contract other
   migrations rely on, and giving a shared editor a mode for one caller is the wrong direction.

10. The same implementation commits the artifacts this decision falsifies: the authored
    `.awf/docs/glossary.yaml` entry, the `[Unreleased]` changelog entry, the migration fixture case
    that asserts the collapse, and the regenerated `docs/decisions/INDEX.md` and lock from `./x render`
    at every status transition. Rendered files are never hand-edited.

## State changes

- update `config/configuration:severity-not-configurable`
- update `config/migrations-and-locks:severity-keys-dropped`
- update `invariants/topics-and-markers:coverage-evaluation-selects-checks`

## Consequences

An upgraded tree keeps evaluating the checks ADR-0183 said always evaluate, and the three claims
describe what the code does. The changelog entry both loses its inherited over-breadth and gains the
seed: an adopter reading it to predict what `awf upgrade` does to their `config.yaml` needs to know a
line they never authored can appear, under what condition, and that it is announced.

The clause that first looked unbackable ends up backed, and the codebase loses a defensive filter in
exchange. That is a deliberate trade: the filter made a wrong policy silent, and the tests now make it
loud. It is also the narrower reading of ADR-0183 item 2 applied to awf's own call site.

The seed's cost is a `maxTopicsPerPath: 8` line in a config that never set one, which is why it is
announced. It does not change what the coverage gate evaluates, since `EffectiveMaxTopicsPerPath`
already returned 8 for an unset pointer. It does change one rendered row: `configreference.go` marks
that row `(default)` only while the pointer is nil, so the regenerated `docs/config-reference.md`
prints `8` instead of `8 (default)`, and an adopter who upgrades before re-rendering sees that doc
reported stale.

Ruled out: relaxing the `CurrentState != nil` gate so an absent block still evaluates. That would turn
current-state coverage from opt-in into always-on for every adopter, which is a far larger decision
than this correction, and ADR-0183 did not take it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Let the block collapse and narrow the claim to match | Ships behaviour that contradicts ADR-0183 item 1 with no record reconciling the two; the claim would be honest about a defect rather than the defect being fixed. |
| Seed into the collapsed result instead of the original bytes | `SetMappingInteger` appends an absent parent at the document end, silently relocating the adopter's block. |
| Seed `maxClaimsPerTopic` instead, matching scaffold and the generation-16 migration | Consistent with precedent but says the wrong thing: the block is being kept alive for the coverage checks, and `maxTopicsPerPath` is the key those checks read. |
| Amend ADR-0183 rather than write this decision | Its meaning is frozen once it leaves Proposed; a later decision changes the claims an earlier one established rather than editing it. |
| Drop the uncovered-report clause as unbackable | The premise was false. The clause is unobservable only behind a dead kind filter, and deleting that one line pins it against two existing assertions. |
| Keep the kind filter and add a marker for the clause anyway | The marker would stay green through the exact regression it claims to catch, which is worse than no claim. |
| Preserve the block by seeding in `ConfigForCurrentSchema` too | That function repairs the parseability of history, not the tree; seeding there would inject a value into a historical universe. |
| Extend `RemoveMappingKey` to avoid or report the parent drop | Its collapse behaviour is a documented contract other migrations depend on; a per-caller mode on a shared editor is worse than a read-only probe. |

## Status history

- 2026-07-30: Proposed
- 2026-07-30: Implemented; content-sha256: 563553c07eee9661819cf7106738255a59092423ad02d8cfa213362aae8c4cc3; state-sequence: 96
