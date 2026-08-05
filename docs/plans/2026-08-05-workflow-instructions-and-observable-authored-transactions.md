---
format: plan-v2
date: 2026-08-05
adrs: [validate-authored-transactions-by-observable-operations]
status: Proposed
---
# Plan: Workflow Instructions and Observable Authored Transactions

## Goal

Let authored authority transactions carry independently observable batches and legal lifecycle history, then place adopter-facing verification, checkout, freeze, and integration guidance at the moments where an agent can act on it. Do not weaken same-claim materiality, append-only history, identity, provenance, authorization, lock, or final-state checks, and do not attempt to lint arbitrary shell pipelines.

## Architecture summary

First update the existing `internal/currentstate` transition model and its current-state claim under the approved ADR, retaining `TransitionMode` only for same-claim chain folding. Then update the planning and review surfaces, followed by execution and integration surfaces. Each guidance phase changes generic catalog/template sources, mirrors project list overrides where required, pins the rendered outputs, retires only pitfalls whose prevention and recovery have become canonical, and carries its own changelog entry.

## Phase 1: Validate observable authored transactions

**Execution mode: inline.**

Completes: ["observable-authored-transactions"]

### Task 1.1: Pin the relaxed authored contract before changing production code
Latitude: exact
Applying: ["validate-authored-transactions-by-observable-operations:observable-authored-transaction", "validate-authored-transactions-by-observable-operations:same-claim-authored-boundary", "validate-authored-transactions-by-observable-operations:merge-chain-boundary", "validate-authored-transactions-by-observable-operations:historical-authored-interpretation"]
Paths: ["internal/currentstate/aggregate_test.go", "internal/currentstate/transition_test.go", "internal/adr/format_test.go", "internal/adr/pending_test.go", "internal/project/mergeaggregate_test.go", "internal/project/spine_test.go", "internal/audit/history_test.go"]

Change the existing transition fixtures before production code so focused tests fail for the removed cardinality rules. In `internal/currentstate/aggregate_test.go`, make the several-distinct-target-batches and multi-step-history cases require both `AuthoredCommit` and `MergeAggregate` to pass. Keep `TestMergeAggregateFoldsClaimChains` proving an authored add/update chain is refused, and change corrective-reapplication authored expectations from the removed batch-cap diagnostic to the same-claim duplicate-target diagnostic. In `internal/currentstate/transition_test.go`, replace the standalone multi-batch refusal assertion with acceptance for distinct targets while retaining the adjacent duplicate-target rejection.

Update `internal/adr/format_test.go` and `internal/adr/pending_test.go` so `HistoryTransitionValid` accepts an exact-prefix legal multi-event replay and still rejects truncation, replacement, illegal lifecycle order, and rewritten retained events. Rework `internal/project/mergeaggregate_test.go` to distinguish modes with a legal merge-only same-claim chain rather than several distinct-target batches, preserving proof that `MERGE_HEAD` selects aggregate chain folding. Add a non-merge case to `internal/audit/history_test.go` using its existing historical-operation fixtures: a retained commit that appends distinct-target batches plus a legal multi-event history produces no current-state transition finding, while the sibling same-claim chain still produces one. In `internal/project/spine_test.go`, add a rendered-contract assertion that the generic ADR scaffold and both enabled ADR lifecycle skill outputs permit several Applied or Reapplied batches in one authored transaction only when their operation targets are distinct, retain a separately observable authored boundary for a repeated same-claim occurrence, permit multiple Status events when the prior history remains an exact prefix and the appended events replay as a legal ordered lifecycle, and no longer prescribe exactly one batch or a fixed Status-event count per transaction.

Run `go test ./internal/adr ./internal/currentstate ./internal/project ./internal/audit`. Before implementation, the new authored acceptance and rendered-instruction cases must fail for the old cardinality behavior while all retained negative cases remain meaningful.

### Task 1.2: Remove only the unobservable choreography checks
Latitude: exact
Applying: ["validate-authored-transactions-by-observable-operations:observable-authored-transaction", "validate-authored-transactions-by-observable-operations:same-claim-authored-boundary", "validate-authored-transactions-by-observable-operations:merge-chain-boundary", "validate-authored-transactions-by-observable-operations:historical-authored-interpretation"]
Paths: ["internal/currentstate/transition.go", "internal/adr/format.go"]

In `internal/currentstate/transition.go`, remove the `AuthoredCommit` per-ADR newly appended batch cap and its diagnostic. Keep `AuthoredCommit` rejecting a chain with more than one operation occurrence for the same claim, and keep `MergeAggregate` using `foldChain`; update `TransitionMode`, `pairOps`, `historyTransitionValid`, and nearby ownership comments so the distinction names same-claim observability rather than one-commit-one-step cardinality.

In `internal/adr/format.go`, make `HistoryTransitionValid` own exact-prefix legal ordered replay for every transition. Fold or remove `HistoryTransitionValidAggregate` rather than retaining two identical public helpers, and update callers and comments accordingly. Do not change operation parsing, lifecycle legality, frozen-content checks, mutation reconciliation, dominated-update empty-result behavior, or after-state `Check`.

Run the focused command from Task 1.1. It must pass with authored distinct-target batches and legal multi-event histories accepted, authored same-claim chains refused, merge chains accepted, and historical audit reflecting the same authored rule.

### Task 1.3: Apply the ADR operation and publish the revised authority contract
Latitude: exact
Applying: ["validate-authored-transactions-by-observable-operations:observable-authored-transaction", "validate-authored-transactions-by-observable-operations:same-claim-authored-boundary", "validate-authored-transactions-by-observable-operations:merge-chain-boundary", "validate-authored-transactions-by-observable-operations:historical-authored-interpretation"]
Paths: ["docs/decisions/validate-authored-transactions-by-observable-operations.md", ".awf/topics/parts/invariants/current-state-authority/current-state.md", "internal/currentstate/aggregate_test.go", "templates/adr-template/template.md.tmpl", ".awf/parts/adr-template/body.md", "templates/skills/adr-lifecycle/SKILL.md.tmpl", ".claude/skills/awf-adr-lifecycle/SKILL.md", ".pi/skills/awf-adr-lifecycle/SKILL.md", "changelog/CHANGELOG.md", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/decisions/template.md", "docs/topics/invariants/current-state-authority.md", "docs/domains/invariants.md"]

Update `invariants/current-state-authority:merge-transition-ordered-aggregate` so its authored clause permits any number of distinct-target batches and exact-prefix legal Status events, retains one operation occurrence per claim, and states that merge provenance alone permits same-claim net folding. Preserve `Origin`, append the pending ADR slug to `Revised-by`, keep `Backing: test`, and make the aggregate test markers cover the authored acceptance and same-claim rejection clauses. Mutation-check the proof by temporarily restoring the batch cap and fixed event validator separately; each restoration must make its corresponding marked test fail, and each temporary mutation must be asserted present before the test is trusted and removed without discarding real edits.

Update the generic ADR scaffold and awf's full body override to replace "one checked batch per commit" and any fixed Status-event shape with the observable rule: several batches may share a transaction only across distinct claim IDs, a repeated same-claim occurrence needs a separately observable authored transaction, and multiple Status events may share a transaction when the prior history remains an exact prefix and the appended events replay as a legal ordered lifecycle. Update the generic ADR lifecycle skill at the transition, history-edit, claim-mutation, and commit-subject instructions so one authored transaction may append several Applied or Reapplied batches with their matching distinct-target claim mutations and may carry the corresponding legal ordered Status progression, while each repeated same-claim occurrence still requires a separately observable authored transaction; render and inspect both enabled target outputs. Add an Unreleased changelog entry for the authored transaction relaxation and retroactive audit interpretation.

Using `awf-adr-lifecycle`, append the first `Implementing` event and one `Applied` event for `update invariants/current-state-authority:merge-transition-ordered-aggregate` in the same staged transaction as the claim and proof changes; obtain the content stamp from the final ADR bytes rather than precomputing it in the plan. Run `./x render`, inspect the listed generated outputs and lock, then run `go test ./internal/adr ./internal/currentstate ./internal/project ./internal/audit`, `./x check`, `./awf check staged`, and `./x gate`; every command must be clean.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and `./x gate` pass.

```commit
feat(invariants): accept observable authored transactions
```

## Phase 2: Put verification guidance at planning and review moments

**Execution mode: inline.**

Completes: ["planning-verification-guidance"]

### Task 2.1: Pin generic planning and reviewer guidance
Latitude: exact
Paths: ["internal/project/spine_test.go", "internal/project/plan_detail_modes_test.go", "internal/catalog/batch_test.go"]

Add rendered-contract assertions before editing templates or catalog data. The writing-plans assertions must require each material `Post-check:` to name its input population, exclusions, lifecycle snapshot, and expected terminal set or lifecycle-authorized residual findings; require a probe success sentinel or checked exit status before empty output counts as absence; and require mutation targets to be read back after compound mutating commands. They must also classify each material check as an authority, state, or choreography check, preserve authority checks, select the least restrictive state validation that proves the durable property, and omit a choreography-only constraint unless a named authority or state property requires it. Keep the existing prohibition on frozen authoring-time counts.

Add catalog assertions that the plan reviewer executes material census and post-check commands against the exact intermediate snapshot declared by the plan and rejects premature zero requirements, while the code reviewer requires every added or changed mechanical check to demonstrate a negative case and requires a temporary falsification to prove its mutation landed before its verdict counts. Both reviewer contracts must apply the same authority/state/choreography taxonomy: preserve authority checks, require state checks to be no stricter than the durable property they prove, and flag choreography-only enforcement that has no named authority or state obligation. Rendered agent assertions must cover both generic catalog defaults and awf's enabled target outputs.

Run `go test ./internal/catalog ./internal/project`; the new assertions must fail against the old guidance.

### Task 2.2: Implement and dogfood the planning-time guidance
Kind: batch
Latitude: exact
Paths: ["templates/skills/writing-plans/SKILL.md.tmpl", ".awf/skills/parts/writing-plans/conventions-tasks.md", "internal/catalog/standard.go", ".awf/agents/plan-reviewer.yaml", ".awf/agents/code-reviewer.yaml", "glob:.claude/skills/awf-writing-plans/**", "glob:.pi/skills/awf-writing-plans/**", ".claude/agents/plan-reviewer.md", ".pi/agents/plan-reviewer.md", ".claude/agents/code-reviewer.md", ".pi/agents/code-reviewer.md", "changelog/CHANGELOG.md", ".awf/awf.lock"]
Representative: Add the complete Post-check snapshot and terminal-set rule to the generic writing-plans section, then preserve it through awf's `sectionDefault`-based conventions override.
Edge: Add each reviewer focus item to `internal/catalog/standard.go` and mirror it into the corresponding project-local `data:` list override, because list overrides replace rather than extend catalog defaults.
Post-check: After `./x render`, the generic render tests and all four enabled reviewer outputs contain the new focus items, the two writing-plans outputs contain the snapshot rule, no output contains an authoring-time expected count, and `git diff --check`, `go test ./internal/catalog ./internal/project`, `./x check`, and `./awf check staged` are clean.

Implement the Task 2.1 wording in the generic writing-plans template and awf conventions part without duplicating contradictory forms. Put the authority/state/choreography taxonomy beside the material Post-check authoring rule so classification happens when the check is designed. Add concise adopter defaults to the plan-reviewer and code-reviewer catalog entries, including the taxonomy and its preserve-authority, least-restrictive-state, and no-unjustified-choreography consequences. Mirror the new plan-reviewer defaults into `.awf/agents/plan-reviewer.yaml`; strengthen the existing `verification-instrument-can-fail` entry in `.awf/agents/code-reviewer.yaml` with the mutation-landed obligation and mirror the taxonomy item while keeping the other catalog defaults explicitly mirrored. Add an Unreleased changelog entry describing the adopter-facing planning and reviewer guidance.

### Task 2.3: Retire planning pitfalls now owned by canonical guidance
Latitude: exact
Paths: [".awf/docs/pitfalls.yaml", "docs/pitfalls.md", ".awf/awf.lock"]

Remove the active entries `An intermediate search expectation must respect deferred claim operations` and `A mechanical check proves nothing until you have seen it fail`: their prevention and complete recovery now live in writing-plans and reviewer instructions. Keep `A census number is only as good as its stated query` as the short cross-artifact residual warning, and keep `An empty scan result only counts once the probe provably ran` because it applies beyond plan execution; do not duplicate the new procedures into either retained entry. Run `./x render`, inspect the source and rendered removals, and require `./x check`, `./awf check staged`, and `./x gate` to be clean.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and `./x gate` pass.

```commit
feat(rendering): place verification guidance at planning moments
```

## Phase 3: Put checkout, freeze, and integration guidance at execution moments

**Execution mode: inline.**

Completes: ["execution-boundary-guidance", "workflow-pitfall-consolidation"]

### Task 3.1: Pin execution-boundary guidance before template edits
Latitude: exact
Paths: ["internal/project/spine_test.go"]

Extend the focused rendered contracts so subagent-driven development validates the explicitly intended worktree, branch, reported commit, and clean status before review. Require reviewing-impl to reconcile implemented public shapes and deviations into mutable plan Notes before the terminal freeze and, after a divergent integration, to re-read enumerative workflow prose against contracts changed on the other side before renewed review can settle.

Extend the effort-workflow contract with: one checkout is one writer boundary; parallel work uses separate worktrees; before the first mutation the operator verifies the exact managed-worktree prefix; a suspected path slip triggers inspection of the primary checkout; and a residual shared-checkout commit requires fresh status plus comparison of staged and worktree copies of shared generated files. Extend the workflow-doc render test so preserving long gate output uses a direct log redirect and separately captured command status rather than a status-losing pipeline. Run `go test ./internal/project`; the new assertions must fail against the old templates.

### Task 3.2: Implement the post-child, pre-freeze, and post-integration instructions
Kind: batch
Latitude: exact
Paths: ["templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/skills/effort-workflow/SKILL.md.tmpl", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", "glob:.claude/skills/awf-subagent-driven-development/**", "glob:.pi/skills/awf-subagent-driven-development/**", "glob:.claude/skills/awf-reviewing-impl/**", "glob:.pi/skills/awf-reviewing-impl/**", "glob:.claude/skills/awf-effort-workflow/**", "glob:.pi/skills/awf-effort-workflow/**", "docs/workflow.md", "docs/working-with-awf.md", "changelog/CHANGELOG.md", ".awf/awf.lock"]
Representative: Put each instruction in the existing procedure step that owns the action: post-child validation in subagent status handling, deviation reconciliation immediately before the deferred terminal transaction, divergent-prose revalidation in the integration branch, and checkout-prefix validation before effort mutation.
Edge: Keep instructions target-neutral and publication-safe, retain the governed switch to the target checkout for integration and closure, and show gate output capture without claiming arbitrary shell pipelines are mechanically checked.
Post-check: After `./x render`, both enabled target skills and the generic workflow docs contain the new instruction at the asserted step, unset-variable renders remain coherent, no target-specific Pi tool leaks into the target-neutral effort skill, and `git diff --check`, `go test ./internal/project`, `./x check`, and `./awf check staged` are clean.

In `templates/docs/working-with-awf.md.tmpl`, add the awf-specific rendering transaction reminder near the render/check loop: stage `.awf/awf.lock` with every regenerated output and treat its atomic manifest as part of the one render transaction rather than slicing it across independent commits. Add an Unreleased changelog entry for the adopter-facing execution and integration guidance.

### Task 3.3: Retire workflow pitfalls whose complete remedy is now canonical
Kind: batch
Latitude: exact
Paths: [".awf/docs/pitfalls.yaml", "docs/pitfalls.md", ".awf/awf.lock"]
Representative: Remove a pitfall only after reading the newly rendered instruction that prevents it and gives its recovery at the exact workflow moment.
Edge: Retain implementation-specific hazards and any entry whose residual case still requires judgment beyond the new instruction; shorten rather than delete when only the common case is covered.
Post-check: The source and rendered pitfalls contain none of the retired titles, every retained workflow-related entry names a residual hazard not completely handled by canonical instructions, `./x render` reports only the expected output/lock changes, and `./x check`, `./awf check staged`, and `./x gate` are clean.

Retire these entries because their complete remedies are now canonical: `Stage the lock with every render output`, `Parallel efforts can collide on schema generations at integration`, `A piped gate run reports the pipe's exit code, not the gate's`, `Verify checkout identity after a commit-capable child`, `The atomic .awf/awf.lock forces multi-scope rendering work into one commit`, `Parallel sessions share one git index`, `Record implementation deviations before the terminal artifact transaction`, `A Decision amendment discovered mid-implementation must be reviewed before the freeze commit`, `Inspect shared generated files before pathspec commits`, `A long-lived branch's prose goes stale against the other side with no merge conflict`, and `In a managed worktree, a primary-checkout path silently splits the transaction`. Preserve `Verify compound-chain side effects by reading the target back`, `Make custom staged-slice hooks explicit about branch and cleanup`, `Port a stale branch before merging a breaking marker grammar`, and other specialized integration hazards because their residual conditions still require general or domain-specific judgment.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and `./x gate` pass.

```commit
feat(rendering): place guidance at execution boundaries
```

## Definition of done

- `dod: observable-authored-transactions` Ordinary staged and historical transitions accept multiple distinct-target batches and legal multi-event histories, continue refusing same-claim authored chains, and preserve every authority and after-state check named by the ADR.
- `dod: planning-verification-guidance` Adopter-facing plan authoring and review require snapshot-scoped terminal-set checks and demonstrated negative cases, with awf dogfooding every catalog default despite list overrides.
- `dod: execution-boundary-guidance` Post-child, checkout, commit, pre-freeze, and divergent-integration instructions appear in the generic workflow surface that owns each action and render coherently for every enabled target.
- `dod: workflow-pitfall-consolidation` Workflow pitfalls fully covered by canonical prevention and recovery are absent, while retained entries describe only residual human hazards.

## Notes

The grounding check rejected a full mode unification because same-claim endpoint folding would weaken per-occurrence update and corrective-reapplication substance. `TransitionMode` therefore remains meaningful only at that boundary. Historical ADR prose remains frozen; current-state claims, templates, generated docs, changelog, and active pitfalls carry the forward correction.
