---
format: current-state-v1
status: Implemented
date: 2026-07-21
---
# ADR-0142: Allow Implementation-Ready Pseudocode in Plans

## Context

The plan convention currently treats verbatim form as the general route to executability. The writing skill requires exact content for new files or exact diffs for modifications, its no-placeholder rule requires every changed file to be shown verbatim, and the plan reviewer rejects code changes shown as prose. ADR-0095 added a structured batch-task exception for repeated transformations, while ADR-0097 deliberately retained the exact-content and exact-diff rule for other work.

That proxy has become more restrictive than the guarantee it protects. A plan can identify exact files and symbols, enumerate every required branch and ordering constraint, prohibit unsafe alternatives, specify acceptance assertions, and provide deterministic verification while leaving syntax-level choices to the implementer. Requiring a literal patch as well adds authoring and review cost, couples the plan to a transient source layout, and encourages premature implementation during planning without necessarily removing a design decision.

The repository has now exercised that distinction directly. The Pi subagent execution plan contained detailed TypeScript, Go, and documentation instructions whose pseudocode was applicable without a new design choice, yet repeated plan review pushed it toward exhaustive literal diffs. The user had to approve an explicit pseudocode exception in the plan. This was not a one-off mechanical batch of the form ADR-0095 sanctions; it exposed a missing general task-detail form.

The loosened rule must not turn plans into high-level intent documents. An executor without the preceding conversation must still be able to implement the task, and exact representation remains cheap and valuable where bytes or structural shape are themselves the contract.

## Decision

1. **Plans admit two general implementation-ready detail forms.** A task or task portion may use either exact content/diff or qualifying pseudocode. Both are peers for executability; exact syntax is not the default merely because the work is single-site. ADR-0095's batch task remains a specialized compression mode for a repeated exact transformation, not a third general form: its representative and edge sites are exact, while its affected-site set and deterministic post-check govern the unshown sites. Those unshown sites do not become pseudocode and need not repeat the pseudocode application contract.

2. **Qualifying pseudocode carries a closed application contract.** It names the exact file paths and relevant symbols; required behavior, control-flow branches, ordering, and failure behavior; constraints and explicitly forbidden behavior; tests and acceptance assertions; and deterministic verification commands with their expected terminal state. It must leave no design decision to the executor and must be executable by an agent with no prior conversation context.

3. **Exact form remains mandatory where representation is contract-bearing.** Machine-consumed declarative content such as configuration and manifests, contract-bearing API/type/schema declarations, fixtures, golden output, commands, mechanical replacements, and the representative and edge transformations of a batch task are shown exactly. Exact commands retain observable terminal-state expectations rather than fragile corpus counts. Prose documentation is not declarative content under this rule: it may use qualifying instructions when its precise wording is not contractual, while any required literal wording is shown exactly.

4. **Mixed tasks may combine the forms.** Exact form wins only for the contract-bearing portion. A task may show a schema declaration, fixture, or required documentation sentence exactly while specifying its surrounding implementation, test control flow, or non-contractual prose edit as qualifying pseudocode. The presence of one exact-required fragment does not force a literal diff for the whole task.

5. **Pseudocode creates no placeholder latitude.** `TBD`, `implement later`, `as needed`, vague prose summaries, references such as `similar to Task N`, hidden choices, and instructions that merely restate desired outcomes remain invalid. The reviewer judges whether the application contract is complete, not whether a block is labeled pseudocode.

6. **The author and reviewer surfaces move together.** The writing-plans skill, implementation-plans README, agent-guide plan-convention summary, plan-reviewer executability lens, catalog-default reviewer focus data, and repository reviewer override describe the same two forms and boundary. Repository convention parts that replace default writing guidance carry the same contract. An adopter wholesale override remains adopter-owned, as with the batch-task refinement.

7. **The rendered agreement is deterministically backed and publication-safe.** The Implemented transaction adds `rendering/templates:plan-task-detail-modes` with `Origin: ADR-0142` and `Backing: test` together with its substantive invariant proof marker. The proof establishes that the rendered authoring guidance, reviewer guidance, plan documentation, and agent-guide summary sanction qualifying pseudocode, retain exact-required categories, and preserve the no-placeholder boundary. Every modified template must still render coherent generic prose with empty variables and emit no unresolved-value token. This changes the prose convention established by ADR-0095 and retained by ADR-0097 without editing those frozen records.

8. **Lifecycle transactions regenerate derived authority.** Every later Accepted or Implemented status transaction runs `./x sync` and commits the regenerated decision index and lock changes with the ADR transaction.

## State changes

- add `rendering/templates:plan-task-detail-modes`

## Consequences

- Plans can describe complex implementation logic at the altitude needed for correct execution without embedding an entire transient patch.
- Planning and review effort shifts from byte-for-byte transcription toward checking behavior, boundaries, and acceptance criteria.
- The reviewer must exercise judgment about whether pseudocode leaves a hidden design choice. The closed application contract and deterministic backing keep that judgment bounded.
- Exact content remains required for representation-sensitive work, so declarative formats, public shapes, fixtures, golden output, and mechanical edits do not become interpretive.
- Mixed tasks avoid an all-or-nothing penalty: one exact contract fragment does not expand into a full literal implementation diff.
- The batch-task form remains useful and unchanged for repeated transformations; its representative and edge diffs stay exact.
- Catalog-default adopters receive the new convention on re-render. Projects that replace reviewer focus data or writing sections wholesale must reconcile their overrides, including this repository's own sidecar and convention part.
- The convention remains prose-reviewed rather than parsed from plan bodies. The invariant backs agreement among the rendered instructions, not automatic classification of every authored plan task.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Allow pseudocode for every kind of change | Declarative content, schemas, fixtures, commands, and mechanical replacements are safer and usually cheaper to state exactly; interpretation there adds risk without useful flexibility. |
| Keep exact diffs as the default and require an explicit exception | Preserves the recurring review friction and treats a complete, applicable implementation contract as a deviation rather than a supported form. |
| Retain only ADR-0095 batch tasks as the non-literal route | Batch tasks solve repetition, not single-site or distinct complex logic; forcing unrelated logic into batch form misstates the work. |
| Mechanically parse and validate pseudocode completeness | Completeness depends on code and design context, while awf does not parse plan bodies; the rendered reviewer contract is the appropriate present enforcement layer. |
| Retain exact diffs for mechanically uniform review | Exact diffs make reviewer classification simpler, but repeatedly impose transcription cost without improving an already-complete application contract; bounded judgment is accepted and constrained by the closed pseudocode requirements. |

## Status history

- 2026-07-21: Proposed
- 2026-07-21: Implemented; content-sha256: 34d29d2169d86ce6717e50c9080865751283ab400c2c791754860c87676948a9; state-sequence: 6
