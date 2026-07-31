---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0192: Topic coverage and fan-out evaluate independently of config block presence

## Context

ADR-0183 item 1 removed the `currentState.topicCoverage` and `currentState.topicFanout` configuration keys and committed to unconditional behaviour: "Topic coverage and topic fan-out always evaluate. Coverage reports at error, fan-out reports at warn. Both ranks are fixed in code and no longer configurable."

The implementation did not follow. Both call sites in `internal/project/currentstate.go` still guard the evaluation on the presence of the config block that used to hold the removed keys:

- line 146, the working-tree path: `if ws.Cfg.CurrentState != nil { report.Coverage = topic.EvaluateCoverage(...) }`
- line 214, the staged path: `if afterCfg.CurrentState != nil { report.Coverage = topic.EvaluateCoverage(...) }`

So the effective rule is not "always evaluate" but "evaluate if and only if the adopter's config declares a `currentState:` block". A tree with no such block receives no coverage and no fan-out findings at all, silently. Nothing in the block's contents participates in the decision: `coveragePolicy` returns `Coverage: true, Fanout: true` unconditionally, so the block's mere presence is the switch.

The library contract is not at fault. `invariants/topics-and-markers:coverage-evaluation-selects-checks` states that a coverage evaluation caller selects which checks run, and that is a deliberate and correct design: the uncovered report legitimately requests coverage only. The defect is that one caller, the `awf check` current-state report, selects conditionally where the governing decision requires it to select always.

Two consequences of the divergence are already visible in the corpus.

First, it has been treated as a constraint rather than a defect. When ADR-0184 removed the two severity keys, an emptied `currentState` block would have been dropped by the YAML edit and would have disabled both checks. The migration therefore seeds a default `maxTopicsPerPath` to keep the block alive, and `config/migrations-and-locks:severity-keys-dropped` records the reason as "because an absent block would stop coverage and fan-out evaluating". That is a workaround holding up the bug, and it encodes the buggy behaviour as though it were intended.

Second, the constraint compounds. Because block presence is load-bearing, the block can never be allowed to become empty, so some key must always remain scaffolded, so the last key in the block can never be retired. `config.SkeletonCurrentState` currently has exactly one field, `maxClaimsPerTopic`, which every fresh `awf init` writes. Any decision to retire that setting is blocked by a coupling that has nothing to do with it.

The divergence went unnoticed because no current-state claim backs the caller-level behaviour. ADR-0183 item 1 decided it, nothing asserted it, and the implementation drifted from it across two schema generations while `awf check` stayed clean.

## Decision

1. The `awf check` current-state report evaluates topic coverage and topic fan-out unconditionally. Both `Cfg.CurrentState != nil` guards in `internal/project/currentstate.go` are removed, in the working-tree path and in the staged path alike, so evaluation no longer depends on whether the adopter's config declares a `currentState:` block.

2. `coveragePolicy` accepts a nil `*config.CurrentStateConfig`. No new nil handling is required: `EffectiveMaxTopicsPerPath` already returns the default of 8 on a nil receiver, so a tree with no block evaluates against the same defaults as a tree that declares the block and sets nothing in it.

3. A new claim asserts the caller-level contract so it cannot drift again. `rendering/sync-and-drift` owns `internal/project/**` and reports how check detects and reports, which makes it the claim's home. The claim is test-backed; its proof exercises both the working-tree and the staged path with a config that declares no `currentState:` block, and confirms coverage and fan-out findings are still produced.

4. The `config/migrations-and-locks:severity-keys-dropped` claim is updated to drop its now-false rationale. What generation 25 does is unchanged and its description stays accurate; only the clause explaining the seeding as necessary "because an absent block would stop coverage and fan-out evaluating" is corrected, since after item 1 an absent block stops nothing.

5. The generation-25 migration itself is left exactly as it is. Historical migrations are never edited, and a tree replaying generation 25 receives a harmless explicit `maxTopicsPerPath` it would have defaulted to anyway. The seeding simply stops being load-bearing.

## State changes

- add `rendering/sync-and-drift:coverage-evaluation-unconditional`
- update `config/migrations-and-locks:severity-keys-dropped`

## Consequences

An adopter tree that declares no `currentState:` block begins receiving coverage findings at error rank and fan-out findings at warn rank where it previously received none. This can newly fail a gate that was passing. That outcome is the stated intent of ADR-0183 item 1 rather than a new policy, and the exposure is narrow: every tree scaffolded by `awf init` already carries the block, so only a hand-authored tree that never declared one or deliberately deleted it is affected. Such a tree was not opted out by any deliberate mechanism; it was opted out by an implementation slip.

The rule "a tree that never declared `currentState` is deliberately opted out", asserted in the generation-25 migration's own source comment, is retired. There is no opt-out. An adopter that wants coverage findings suppressed has no supported way to do so, which is exactly what ADR-0183 items 1 and 2 decided when they removed the keys and removed `off` from the severity vocabulary.

Retiring the last key in the `currentState` block becomes possible, because the block's presence stops carrying meaning. That unblocks the separate decision to retire `currentState.maxClaimsPerTopic`, which is what surfaced this defect; that retirement is not part of this decision and lands on its own record.

The class of defect is worth naming beyond this instance: a decision item that no claim backs can diverge from its implementation indefinitely with a green gate. This ADR closes the one instance it found and adds the missing backing, but it does not audit the rest of ADR-0183 or any other decision for similarly unbacked items.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the gate and seed a surviving key, as the generation-25 migration does | Preserves the coupling that caused the problem and leaves the block permanently unable to empty, so the next attempt to retire a `currentState` key hits the same wall. |
| Reintroduce an explicit opt-out key for coverage and fan-out | Directly reverses ADR-0183 items 1 and 2, which removed configurable suppression on purpose. If suppression is wanted again it needs its own decision arguing against that one, not a quiet restoration. |
| Treat it as a bugfix needing no ADR | The correction changes adopter-visible behaviour and requires a claim add plus a claim update, and claim mutations are ADR-gated. |
| Fold the correction into the claim-budget retirement | Mixes a correction to an existing Implemented decision with a separate new decision, and the correction stands on its own regardless of whether the budget is retired. |

## Status history

- 2026-07-31: Proposed
