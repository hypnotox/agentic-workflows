---
format: current-state-v1
status: Proposed
date: 2026-07-21
---
# ADR-0143: Incremental ADR State Operation Application


## Context

ADR-0135 makes current-state impacts bidirectional and checks the Git pair in which an ADR becomes Implemented. That design keeps every surviving commit truthful and independently auditable, but it assigns one state sequence to the whole ADR and requires every declared operation to land in the same transaction. A decision that updates many claims must therefore become one large implementation commit or be split into artificial decisions.

Range audit already checks every parent-to-commit pair rather than trusting only the range endpoints. Weakening it to endpoint proof would allow stale intermediate authority and make partial cherry-picks unsafe. The missing capability is not range reconciliation but an append-only record of which frozen operations each individually checked commit applied.

Status alone cannot record partial progress, especially for a remove whose claim no longer exists. Reusing Accepted would also conflate a wholly pending decision with a partially incorporated one. Operation application therefore needs an explicit lifecycle state and exact history events while current-state claims remain the sole active authority.

The stored grammar must remain mechanically distinguishable. Existing `current-state-v1` ADRs are immutable history, so the new lifecycle uses a V2 format boundary rather than silently changing V1 or rewriting old records.

## Decision

1. awf introduces `format: current-state-v2`. The lock records `adrFormatV2From`, and every ADR at or above that number must use V2 while earlier V1 and legacy ADRs retain their existing validation. An empty first adoption at the V2 schema sets both `adrFormatV1From` and `adrFormatV2From` to 1. A brownfield first adoption sets both cutoffs to the highest existing ADR number plus one and retains every lower existing identity and gap in the closed legacy set. Upgrading an existing V1 adopter preserves `adrFormatV1From` and sets `adrFormatV2From` to the highest existing ADR number plus one. Upgrade stamps the new schema generation and V2 cutoff in the same atomic lock save. Both cutoffs are immutable afterward. No existing ADR, topic, or claim is rewritten.

2. V2 statuses are Proposed, Accepted, Implementing, Implemented, and Abandoned. Legal edges are Proposed to Accepted, Implementing, Implemented, or Abandoned; Accepted to Implementing, Implemented, or Abandoned; and Implementing to Implemented or Abandoned. Implemented and Abandoned remain terminal. Proposed to Implementing preserves optional acceptance, while Accepted remains wholly unapplied.

3. The canonical content digest continues to cover Context, Decision, State changes, Consequences, and Alternatives Considered. The first transition out of Proposed freezes that content. Accepted, Implementing, and terminal status entries carry the digest according to their transition, and every later status entry repeats the frozen value. Applied events carry no digest.

4. V2 Status history is a heterogeneous append-only event stream. A status event changes lifecycle state. An application event has the exact form `- YYYY-MM-DD: Applied; state-sequence: <positive integer>; operations: <operation-list>`, where each operation is a declared verb followed by its qualified ID in one inline code span, entries are separated by comma and space, and their relative order matches the frozen State changes list. The list is nonempty, contains only previously unapplied operations, and one Applied event is one application batch owned by that ADR.

5. The latest status event, ignoring intervening Applied events, must equal frontmatter status. Event dates never descend. Existing history is an exact prefix of later stable history: deleting or changing an event is not a valid checked transition. Branch-local amend, squash, and rebase remain possible because validation observes only the surviving snapshots; once a commit is retained in shared history, correction proceeds through a later ADR rather than a reverse commit.

6. Entering Implementing appends the Implementing status event and the first Applied event in that order, with the matching claim mutations in the same checked pair. A same-status Implementing pair appends exactly one Applied event. Reaching Implemented appends the final Applied event followed by the terminal status event. Reaching Abandoned appends only its terminal status event and applies no additional operation. An Implementing snapshot has at least one applied and one remaining operation; an Implemented snapshot has none remaining.

7. Proposed to Implemented and Accepted to Implemented remain direct one-transaction paths. Their terminal `state-sequence` represents one implicit batch containing every declared operation, preserving the V1 shorthand. `None.` ADRs and one-operation ADRs use a direct terminal transition rather than Implementing. A V2 ADR cannot mix implicit terminal sequencing with explicit Applied events.

8. V1 implicit batches and V2 implicit or explicit batches share one globally unique, contiguous state-sequence namespace. Each applied operation inherits its batch sequence. Claim histories and Revised-by ordering use that operation sequence rather than one sequence for the owning ADR, so operations from different Implementing ADRs may interleave without a project-wide implementation lock.

9. One checked pair may append at most one batch per ADR and may apply batches from several ADRs when all newly applied operations target distinct qualified claim IDs across the pair. Each batch receives its own consecutive sequence. Multiple batches for one ADR must be combined, and two new batches cannot target the same claim because the pair contains no intermediate claim snapshot to prove their ordering.

10. Static authority is operation-specific. An explicit V2 Applied event, or a valid implicit terminal batch, makes that exact operation applied. Applied operations from Implementing, Implemented, or partially Abandoned ADRs support Origin, Revised-by, removed-identity history, and inverse validation. Remaining operations are pending only; operations left when an ADR becomes Abandoned are canceled and provide no authority.

11. An Implementing ADR may become Abandoned after partial application. Its applied operations remain historical facts and continue to support any resulting current provenance or absence; its remaining operations are canceled. The required rationale explains why execution stopped. Abandonment never claims to undo an applied effect, and a desired reversal uses a successor ADR under the ordinary add, update, remove, and removed-ID rules.

12. Static checking projects every V2 ADR into declared, applied, remaining, and canceled operations. It rejects an empty batch, an undeclared or duplicate application, mixed implicit and explicit sequencing, sequence gaps or duplicates, invalid lifecycle cardinality, an application combined with abandonment, unsupported provenance, and reuse after an applied remove. The ADR corpus exposes operation-level batch membership and sequence rather than requiring consumers to infer application from terminal status.

13. Pair checking derives operations from newly appended application records rather than from ADRs newly reaching Implemented. Every newly applied operation has exactly its matching claim mutation, and every claim mutation belongs to exactly one newly appended batch. Existing add, remove, substantive-update, Origin-preservation, and Revised-by-prefix rules remain. Diagnostics identify the ADR, operation, expected sequence or event shape, and mismatched mutation where possible.

14. `awf check --staged` and range audit continue to call the same pair validator. Audit continues to evaluate every included commit against its first parent; endpoint comparison remains reporting only and never proves application.

15. Normal path context shows every Accepted operation as pending. For Implementing it shows only remaining operations plus concise applied-to-total progress; already applied effects appear through current claims rather than as repeated historical guidance. Implemented and Abandoned ADRs produce no pending notice. Topic history includes an operation as soon as its batch is applied.

16. The ADR corpus exposes enough progress data for explicit ADR-path context to partition Applied, Remaining, and, for Abandoned ADRs, Canceled operations with batch sequences while labeling decision material as non-current authority. The separate context-artifact UX decision owns that presentation; this decision establishes only the lifecycle contract and API seam.

17. The generated decision index lists Proposed, Accepted, and Implementing ADRs under In flight and keeps Implemented and Abandoned records in compact History. ADR authoring, lifecycle, planning, execution, review, audit, and retrospective guidance describes application batches and no longer requires all operations in one final commit.

18. Lock ownership remains separated by package. `internal/manifest` parses and canonically serializes both cutoffs; `internal/migrate` and the upgrade command initialize the V2 cutoff and schema generation atomically; `internal/project` preserves the lock fields, validates snapshot transitions, selects the scaffold format, and renders the resulting workflow surfaces. `internal/config` does not own either cutoff because they are permanent project metadata rather than authored configuration. Mixed legacy, V1, and V2 corpora remain readable according to the two immutable boundaries.

19. The Implemented transaction applies every operation in this ADR's State changes list. Added claims name ADR-0143 as Origin and use `Backing: test` with matching proof markers. Updated claims preserve Origin and the complete prior Revised-by prefix, append ADR-0143, change at least one canonical field substantively, and retain or strengthen their required backing. The claim transaction and proof markers land with the behavior they describe.

20. Every Accepted, Implementing, Implemented, or Abandoned status transition runs `./x sync` and commits the regenerated `docs/decisions/INDEX.md` and lock output in the same transaction. An Applied-only same-status transaction also syncs and commits any generated output that changes.

21. The implementation updates the authored ADR guide and template, lifecycle and workflow skills, reviewer instructions, architecture, glossary, configuration reference, current-state sources, `templates/agents-doc/AGENTS.md.tmpl`, any applicable project convention parts under `.awf/parts/agents-doc/`, and rendered `AGENTS.md` in the behavior-changing commits. Generated target copies travel with their authored template and `.awf/` inputs.

22. Every affected template preserves missing-key-zero rendering and coherent generic prose when its variables or data are unset. No empty interpolation or unresolved-value token is permitted in adopter output.

## State changes

- update `adr-system/adr-lifecycle:fresh-adoption-v1-cutoff`
- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- add `adr-system/adr-lifecycle:applied-history-events-append-only`
- add `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`
- update `invariants/current-state-authority:abandoned-remove-pair-attributed`
- update `invariants/current-state-authority:implemented-impact-bidirectional`
- update `invariants/current-state-authority:removed-claim-id-not-reused`
- update `invariants/current-state-authority:state-impact-transition-atomic`
- add `invariants/current-state-authority:application-batch-sequence-order`

## Consequences

Large decisions can apply their current-state effects over a plan's natural commit boundaries without sacrificing truth at any boundary. Each operation-bearing commit remains independently staged-checkable, auditable, bisectable, and safe to cherry-pick together with its application record.

The lifecycle and history parser become more complex. Status history is no longer a status-only list, authority can no longer be decided from terminal status alone, and global ordering moves from ADRs to batches. Central operation projections and shared pair validation contain that complexity instead of spreading status checks across consumers.

Partially Abandoned decisions become honest history rather than requiring already truthful effects to be erased. This means Abandoned no longer implies that nothing happened; callers must distinguish applied and canceled operations. Current claims still decide active truth, while the operation record explains how that truth arose.

Stable history deliberately loses simple `git revert` as a compliant correction mechanism. Before publication, branches may rewrite their own commits. After publication, a successor ADR records the forward correction, preserving sequence continuity, provenance, and removed identities.

Adopters perform a schema upgrade, but it changes only lock metadata and rendered/scaffold behavior. Historical ADR content remains untouched. Older binaries are excluded by the existing schema and binary-version gates before they can misinterpret V2 records.

The later context/topic UX effort can use incremental batches for its larger claim migration and can render explicit ADR progress from the corpus seam established here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Validate one ADR over audit range endpoints | Intermediate commits could carry stale authority, and staged checking, bisectability, cherry-pick safety, and mutation attribution would weaken. |
| Keep four statuses and apply operations while Accepted | Accepted would mean both wholly pending and partially incorporated, obscuring the implementation boundary. |
| Store application state in a separate ledger | ADR progress would be split across authored files and require another drift-prone authority schema. |
| Split one multi-operation decision into several ADRs | It fragments one load-bearing rationale into artificial decisions solely to satisfy commit sizing. |
| Keep one sequence per ADR and forbid interleaving | A partially implemented ADR would impose a project-wide mutation lock and block otherwise independent plans. |
| Add reversible application events | Provenance deletion, sequence tombstones, and restoration of removed identities would substantially enlarge the state machine; forward successor decisions preserve clearer history. |
| Extend current-state-v1 in place | Old and new records would share a marker despite different status and history grammars, making stored-version reasoning harder. |
| Rewrite all existing ADRs to V2 | Historical content needs no change; an immutable cutoff gives deterministic mixed-version validation without churn. |

## Status history

- 2026-07-21: Proposed
