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

The divergence survived because the ledger agreed with the code rather than with the decision. ADR-0183 shipped `config/configuration:severity-not-configurable`, whose final sentence asserts the gate as intended behaviour: "Whether the checks run at all is a separate concern: a tree that declares no currentState block requests neither." That claim is `Backing: test`, and its proof at `internal/project/currentstate_test.go:223` is `TestCheckCurrentStateNoPolicy`, which asserts that a tree with no block produces no coverage report at all. Its comment names itself "the site backing the claim's 'a tree that declares no currentState block requests neither' clause".

So the same decision that committed to unconditional evaluation in item 1 also shipped a claim asserting the opposite, and pinned that assertion with a passing test. This is not an unbacked decision item drifting from its implementation; it is a decision internally contradicting itself, with the contradiction backed on the wrong side. `awf check` stayed clean across two schema generations because every mechanical layer agreed: the code gated, the claim said it should gate, and the test proved it gated. Nothing asserted item 1 itself, so nothing dissented.

## Decision

1. The `awf check` current-state report evaluates topic coverage and topic fan-out unconditionally. Both `Cfg.CurrentState != nil` guards in `internal/project/currentstate.go` are removed, in the working-tree path and in the staged path alike, so evaluation no longer depends on whether the adopter's config declares a `currentState:` block. The two function doc comments that document the removed behaviour are corrected in the same commit: "Coverage runs only when the project configures a currentState policy" on `CheckCurrentState` and "Coverage runs only when the staged config declares a currentState policy" on `CheckStaged` each become an unconditional statement citing this ADR.

2. `coveragePolicy` accepts a nil `*config.CurrentStateConfig`. No new nil handling is required: `EffectiveMaxTopicsPerPath` already returns the default of 8 on a nil receiver, so a tree with no block evaluates against the same defaults as a tree that declares the block and sets nothing in it.

3. The final sentence of `config/configuration:severity-not-configurable` is struck. That sentence, "Whether the checks run at all is a separate concern: a tree that declares no currentState block requests neither", is exactly what item 1 falsifies. The rest of the claim, which is the severity contract proper, stays intact and keeps its other backing. After this decision the caller-level truth lives in one place, the new claim in item 4, rather than being asserted as an aside by a claim about severity.

4. A new claim asserts the caller-level contract so it cannot drift again. It lands in `rendering/sync-and-drift`, whose selectors match `internal/project/**`, because the claim has to surface in `awf context internal/project/currentstate.go` at the site where the defect recurred, and only a topic selecting that tree does so. Topic path ownership does not by itself determine a claim's home, and this corpus contains a counter-precedent: `invariants/current-state-authority:uncovered-lists-unowned-unignored` is a claim about the current-state coverage report proven from `internal/project/context_test.go` even though that topic selects only `internal/currentstate/**` and `internal/invariants/**`. The surfacing argument, not an ownership rule, is what places this claim. Because `rendering/sync-and-drift` is currently summarized as drift detection alone (hash inputs, attribution, backups, residue, pruning, cleanup), its summary in `.awf/topics/metadata/rendering/sync-and-drift.yaml` is widened in the same commit to cover current-state coverage reporting.

5. The new claim is test-backed on both paths. `TestCheckCurrentStateNoPolicy` in `internal/project/currentstate_test.go` is inverted rather than added to: it currently asserts that a tree with no `currentState:` block produces no coverage report, which item 1 makes false, so it becomes an assertion that coverage and fan-out findings ARE produced without the block. Its proof marker moves from `config/configuration:severity-not-configurable` to the new claim. The staged path is proven alongside it in `internal/project/staged_test.go`, so the claim's two-path clause is anchored at both sites rather than at the easier one.

6. The `config/migrations-and-locks:severity-keys-dropped` claim is updated to drop its now-false rationale. What generation 25 does is unchanged and its description stays accurate; only the clause explaining the seeding as necessary "because an absent block would stop coverage and fan-out evaluating" is corrected, since after item 1 an absent block stops nothing.

7. The generation-25 migration's behaviour is left exactly as it is: historical migrations are never edited, and a tree replaying generation 25 receives a harmless explicit `maxTopicsPerPath` it would have defaulted to anyway. Its doc comment is not behaviour and is corrected in the same commit. `internal/migrate/dropseveritysettings.go` presently asserts in the present tense that "an ABSENT block suppresses coverage and fan-out outright" and that "a tree that never declared currentState is deliberately opted out"; both are rewritten to past tense, recording that the gate motivating the seed existed at generation 25 and was removed here, leaving the seed inert but harmless.

8. The `topic coverage` glossary entry in `.awf/docs/glossary.yaml` is updated in the same commit. Its sentence "A tree that declares a currentState block evaluates both checks" becomes an unconditional statement that every adopted tree evaluates both, citing this ADR alongside the existing ADR-0183 reference.

9. Every rendered output of the four authored `.awf/` sources this decision edits is regenerated by `./x render` in the same commit: the claim parts under `.awf/topics/parts/config/configuration/` and `.awf/topics/parts/config/migrations-and-locks/` and the metadata at `.awf/topics/metadata/rendering/sync-and-drift.yaml` regenerate their `docs/topics/` counterparts, `.awf/docs/glossary.yaml` regenerates `docs/glossary.md`, `docs/decisions/INDEX.md` regenerates on the status change, and `.awf/awf.lock` takes the resulting config-hash change. An implementer following items 3, 4, 6, and 8 without this step leaves the tree failing `awf check`.

## State changes

- add `rendering/sync-and-drift:coverage-evaluation-unconditional`
- update `config/configuration:severity-not-configurable`
- update `config/migrations-and-locks:severity-keys-dropped`

## Consequences

An adopter tree that declares no `currentState:` block begins receiving coverage findings at error rank and fan-out findings at warn rank where it previously received none. This can newly fail a gate that was passing. That outcome is the stated intent of ADR-0183 item 1 rather than a new policy, and the exposure is narrow: every tree scaffolded by `awf init` already carries the block, so only a hand-authored tree that never declared one or deliberately deleted it is affected. Such a tree was not opted out by any deliberate mechanism; it was opted out by an implementation slip.

The rule "a tree that never declared `currentState` is deliberately opted out", asserted in the generation-25 migration's own source comment and rewritten to past tense by item 7, is retired. There is no opt-out. An adopter that wants coverage findings suppressed has no supported way to do so, which is exactly what ADR-0183 items 1 and 2 decided when they removed the keys and removed `off` from the severity vocabulary.

Retiring the last key in the `currentState` block becomes possible, because the block's presence stops carrying meaning. That unblocks the separate decision to retire `currentState.maxClaimsPerTopic`, which is what surfaced this defect; that retirement is not part of this decision and lands on its own record.

The class of defect is worth naming beyond this instance, and it is sharper than an unbacked item. A single decision stated a commitment in its Decision section and contradicted it in the claim it shipped, then backed the contradicting side with a test. Every mechanical layer then agreed with every other, so no drift signal existed anywhere. The gate's backing validation is purely structural: it catches a claim whose declared proof marker is missing, misplaced, or mismatched, and it never compares a claim's prose to what its proof actually asserts. That is precisely why a claim backed on the wrong side stayed green, and it means even a sharper structural check would not have caught this. Reviewing a `State changes` claim against the Decision items of its own ADR is the check that would have caught it, and it is a reviewer obligation rather than a mechanical one. This ADR closes the one instance it found; it does not audit the rest of ADR-0183, or any other decision, for the same shape.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the gate and seed a surviving key, as the generation-25 migration does | Preserves the coupling that caused the problem and leaves the block permanently unable to empty, so the next attempt to retire a `currentState` key hits the same wall. |
| Reintroduce an explicit opt-out key for coverage and fan-out | Directly reverses ADR-0183 items 1 and 2, which removed configurable suppression on purpose. If suppression is wanted again it needs its own decision arguing against that one, not a quiet restoration. |
| Treat it as a bugfix needing no ADR | The correction changes adopter-visible behaviour and requires a claim add plus a claim update, and claim mutations are ADR-gated. |
| Fold the correction into the claim-budget retirement | Mixes a correction to an existing Implemented decision with a separate new decision, and the correction stands on its own regardless of whether the budget is retired. |

## Status history

- 2026-07-31: Proposed
