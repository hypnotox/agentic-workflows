---
format: current-state-v3
slug: task-scoped-plan-decision-context-and-phase-outcomes
status: Accepted
date: 2026-08-02
---
# ADR-task-scoped-plan-decision-context-and-phase-outcomes: Task-scoped plan decision context and phase outcomes

## Context

ADR-0213 made a structured plan projection an executable closure over one selected phase or
task. That closure deliberately retains the plan frontmatter, Goal, Architecture summary,
phase ownership, Phase close, whole-plan Definition of done, and Notes. It does not resolve
the `adrs:` frontmatter links or include the decisions behind them.

The omission conflicts with the authoring boundary the projection is meant to support. Plans
link to ADRs instead of duplicating their design rationale, and the execution workflows tell a
fresh phase or task owner to preserve settled structural choices. A projection therefore names
the governing records without carrying the particular commitments relevant to the selected
work. Including every Decision item from every linked ADR would recover the missing context at
the cost of making every task projection as broad as the whole design.

The whole-plan Definition of done has the same granularity problem. Every task projection sees
every final outcome even though a phase is the transaction boundary, an early phase may only
advance an outcome, and a later phase may complete it. A task-level projection handed to a
helper can consequently look like authority to perform unselected work, run the phase close,
or clear a whole-phase outcome.

The repository now has stable pending ADR identity at record granularity: an off-integration
record uses its retained slug before and after numbering. It does not have stable identity for
an individual Decision item. Current-format items are sequential numbered prose, and ADR-0135
explicitly retired their ordinals as active supersession anchors. Historical records cannot be
retrofitted merely to serve plan navigation, while current authoring needs a selector that
survives amendment and numbering.

The existing ownership boundaries are suitable. `internal/adr` owns the parsed ADR corpus and
must retain authored Decision item bytes because its generic section view intentionally does
not preserve fenced Markdown. `internal/plan` owns the typed plan model, selectors, and
projection rendering. `internal/project` already composes the two corpora for plan link checks.
The command layer should continue to own only arguments, output routing, and typed failure
mapping. No preparatory package refactor is required.

## Decision

1. A new governed ADR authored format, `current-state-v4`, requires every column-zero numbered
   item in `## Decision` to begin with one inline `` `decision: <slug>` `` marker. The slug uses
   the repository's lowercase kebab grammar and is unique within the record. V4 retains V3's
   mandatory record slug and pending-to-numbered identity, and retains the V2/V3 status matrix,
   application and re-application events, amendment chain, content digest, transition pairing,
   and terminal freeze semantics. Its activation is registered by schema generation rather
   than by ADR number. Older formats and their bytes remain unchanged.

2. `internal/adr` parses each Decision item once into its ordinal, optional stable slug, and
   complete authored Markdown block from the item opener through the boundary before the next
   item or section. Source offsets, rather than the generic section body projection, preserve
   continuation paragraphs, lists, and fenced Markdown byte-for-byte. V4 parsing refuses a
   missing, malformed, or duplicate item slug. Older formats retain sequential ordinal
   validation and expose their items by ordinal without gaining authored slugs.

3. A plan Decision reference has exact grammar
   `<adr-identity>:<decision-selector>`. `<adr-identity>` is a four-digit ADR number or a
   retained ADR slug. A V4 selector is a Decision slug. A pre-V4 selector is `#N`, where `N`
   is a canonical positive item ordinal. A positional selector may target only a record whose
   Decision content is already frozen under that record's authored-format lifecycle; an
   amendable older record is refused because its ordinal meaning can still move. `#N` is a
   compatibility locator for frozen prose, not revived supersession, currentness, or active
   authority inference. Authors use the retained ADR slug for pending V4 records so numbering
   requires no plan rewrite.

4. A new intrinsic plan format, `plan-v2`, retains plan-v1's title, Goal, Architecture summary,
   sequential phases and tasks, task kinds and latitude, Phase close, Notes, and two-state
   lifecycle. It adds typed task Decision references, phase outcome assignments, and slugged
   Definition-of-done items. Marker absence and `plan-v1` retain their existing validation and
   projection behavior; no historical plan is retrofitted or selected by date.

5. A plan-v2 task accepts two additional contiguous fields directly beneath its heading:
   `Applying: ["<reference>", ...]` names Decision commitments the task directly implements,
   and `Context: ["<reference>", ...]` names historical or design constraints the task must
   understand but does not implement or mutate. Both values are one-line JSON arrays of unique
   nonempty strings, following the `Paths:` representation precedent. An authored array must be
   nonempty; a task with no assignment omits the field rather than writing `[]`. `Applying`
   targets must resolve to records in the plan-level `adrs:` set; membership compares resolved
   records so a number and retained-slug spelling cannot disagree. `Context` may target any
   resolvable frozen record. A Context assignment never satisfies implementation coverage, and
   an omitted Applying field is the absence the Proposed-plan coverage note evaluates.

6. Malformed JSON, an invalid or duplicate reference, a missing ADR or item, a selector
   incompatible with the target's format or freeze state, and an Applying target outside the
   plan-level ADR set are blocking plan findings. Resolution errors name the plan, phase and
   task, field, authored reference, and available selectors where applicable. Structural and
   referential checks remain blocking after the plan is Implemented.

7. While a plan-v2 record is `Proposed`, awf emits non-failing coverage notes for each non-spike
   task in a plan with nonempty `adrs:` that has no Applying assignment and for each Decision
   item of every plan-level ADR assigned to no task. Multiple tasks may apply one item. Spike
   tasks and plans with an empty ADR set are exempt. The working-tree check and the staged
   HEAD-to-index check each derive notes from their own selected universe, so a pre-commit
   report never substitutes unstaged plan or ADR bytes. Coverage notes stop when the plan is
   `Implemented`; a frozen plan with a historical assignment gap is valid and silent.

8. Each plain bullet in a plan-v2 `## Definition of done` begins with a unique inline
   `` `dod: <slug>` `` marker. Directly after its execution-mode declaration, a phase may carry
   `Advances: ["<dod-slug>", ...]`, `Completes: ["<dod-slug>", ...]`, both in that order, or
   neither. These are one-line JSON arrays of unique nonempty strings. An authored array must be
   nonempty; a phase with no assignment omits that field rather than writing `[]`. Any number
   of phases may advance one outcome; at most one phase may complete it; one phase cannot both
   advance and complete the same item. Missing DoD items, duplicate completion ownership, and
   conflicting same-phase assignment are blocking. While Proposed, an item with neither
   assignment emits a coverage note, while an item that is only advanced emits a distinct note
   that final completion ownership is missing. Those coverage notes stop at Implemented. A
   phase is free to own no DoD outcome.

9. `awf read plan <plan> <P[.T]>` always resolves plan-v2 Decision references; there is no
   opt-in flag. After frontmatter, title, Goal, and Architecture summary, the projection emits
   nonempty Applying decisions and then nonempty Context decisions before the owning phase.
   Each item is framed by its ADR identity, title, and status and otherwise retains its complete
   authored Markdown. A phase selection takes first-authored-order unions across its tasks and
   deduplicates by resolved ADR item. Applying wins when one item appears in both categories.
   Plan-v1 output remains byte-for-byte unchanged.

10. A plan-v2 phase projection emits its selected tasks, Phase close, the DoD items it Advances,
    the DoD items it Completes, and Notes. The two outcome categories remain distinct and omit
    themselves when empty. Global constraints every phase must preserve belong in Goal,
    Architecture summary, or phase-close instructions rather than being disguised as a DoD
    assignment.

11. A plan-v2 task projection includes the owning phase's advanced and completed DoD items and
    Phase close as phase-owner context, but inserts a generated scope notice before the selected
    work. The notice says that only the selected task is in scope, that Phase close and
    phase-owned outcomes do not transfer transaction ownership, and that the consumer must not
    perform unselected tasks or broaden the task merely to clear them. A phase projection needs
    no such notice. Projection never changes commit, review, checkpoint, or handoff boundaries.

12. `internal/adr` owns Decision item parsing, identity, complete source blocks, and semantic
    lookup. `internal/plan` owns plan-v2 parsing, typed references and phase outcomes, selector
    extraction, and final projection rendering from resolved typed inputs. `internal/project`
    composes the already-parsed plan and ADR corpora for cross-corpus findings, staged and
    working-tree notes, and selected read resolution. `cmd/awf` and `internal/clispec` retain
    only the unchanged command grammar, help, output, and error mapping. Ordinary checks parse
    each corpus once per selected universe; no consumer reparses Markdown or reads ADR paths
    directly.

13. Authoring and execution surfaces move with the formats: ADR and plan templates, decision
    and plan READMEs, the ADR-system domain document, architecture documentation, ADR lifecycle,
    plan writing, plan review and resync, inline and subagent-driven execution, reviewer
    contracts, command documentation, current-state claims, generated targets, and the example
    adopter. Their authored `.awf/` sources change first; `./x render` regenerates every managed
    output, including `docs/decisions/INDEX.md` in each transaction that changes the ADR's
    lifecycle status. The plan reviewer checks assignment substance, detects Context used to
    evade Applying, and keeps historical context distinct from current authority. Task execution
    guidance preserves the generated scope notice and phase ownership. Every surface remains
    publication-safe with unset variables.

14. Tests mechanically cover V4 activation and older-format compatibility; pending slug
    numbering; slugged, multiline, and fenced Decision item retention; legacy frozen ordinal
    lookup; plan-v2 task and phase grammar; hard cross-corpus references; Proposed-only working
    and staged assignment notes; phase outcome ownership; task and phase projection ordering,
    deduplication, scope qualification, and source immutability; unchanged plan-v1 bytes;
    command behavior; rendered-surface parity; and publication safety. Every new or updated
    invariant receives its matching proof in the transaction that applies it.

## State changes

- update `adr-system/adr-lifecycle:intrinsic-format-routing`
- update `adr-system/adr-lifecycle:adr-amendable-until-terminal`
- update `adr-system/adr-lifecycle:adr-slug-frontmatter-mandatory`
- update `adr-system/adr-lifecycle:corrective-reapplication`
- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- update `adr-system/adr-lifecycle:applied-history-events-append-only`
- update `adr-system/adr-lifecycle:corpus-single-identity-key`
- update `adr-system/adr-lifecycle:decision-items-enumerable`
- update `adr-system/adr-lifecycle:pending-adr-slug-identity`
- add `adr-system/adr-lifecycle:decision-item-stable-identity`
- update `adr-system/plan-artifacts:plan-frontmatter-validated`
- update `adr-system/plan-artifacts:plans-template-taxonomy`
- update `adr-system/plan-artifacts:plan-executable-projection`
- add `adr-system/plan-artifacts:plan-v2-decision-references`
- add `adr-system/plan-artifacts:plan-v2-phase-outcomes`
- add `adr-system/plan-artifacts:plan-v2-assignment-advisories`
- update `tooling/cli:check-universe-groups`
- update `tooling/cli:plan-read-command`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`

## Consequences

A phase or task owner receives the particular commitments and outcomes that constrain the
selected work without reopening every linked ADR. Pending ADR references survive numbering,
legacy rationale remains addressable without rewriting frozen history, and the projection
itself makes task versus phase ownership explicit.

Plan and ADR authoring gain ceremony. Every new Decision and DoD item needs a stable slug, and
plan authors must classify Applying versus Context and advancing versus completing outcomes.
The non-failing Proposed-plan notes make omissions visible while they are actionable without
leaving permanent noise after implementation. Review remains responsible for semantic fidelity:
a resolved link proves identity, not that the chosen task truly implements the commitment.

Phase projections may still be larger than plan-v1 projections, but their growth is controlled
by authored relevance rather than total linked-ADR size. A task sees phase outcomes as context,
not as delegated scope. Older in-flight amendable ADRs cannot be safely targeted by ordinal and
must finish under their existing workflow or freeze before a plan-v2 positional reference can
bind them.

The implementation changes two authored formats and both working-tree and staged checking. It
therefore requires a successor implementation plan with explicit schema activation, staged
snapshot, generated-source, adopter-parity, and invariant-backing transactions. Existing package
boundaries support the work without an enabling refactor.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep plan-v1 and require executors to open linked ADRs separately | The advertised executable closure would still omit the design source plans intentionally avoid duplicating. |
| Add `--decisions` to `awf read plan` | Task-scoped references already bound the result, and an opt-in would let the normal execution path silently miss governing commitments. |
| Include every Decision from every plan-level ADR | It restores context by discarding the bounded task and phase projection that motivated ADR-0213. |
| Extend `current-state-v3` with Decision slugs | Intrinsic authored formats have frozen parser semantics; schema-activated behavior belongs in a new format with explicit compatibility proofs. |
| Use new Decision ordinals as durable selectors | Amendments can change positional meaning, and it would revive the anchor semantics ADR-0135 retired. |
| Retrofit stable slugs into historical ADRs | Frozen historical bytes do not need mutation when their already-fixed ordinals provide adequate navigation. |
| Encode task references or phase outcomes as nested Markdown lists | It breaks the contiguous field grammar and adds a second representation where one-line JSON string arrays already exist. |
| Assign DoD items directly to tasks | Tasks are ordered steps, not transaction owners; the phase owns the green close and observable outcome. |
| Give each DoD item only one phase field | It cannot distinguish an early phase's partial contribution from the later phase that makes the condition fully true. |
| Omit DoD from task projections | A task could miss an outcome constraint that shapes correct implementation; an explicit ownership notice prevents scope transfer without hiding context. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Accepted; content-sha256: cebe1b41b700ffd88c6c3c5aafdc82d3e6b0afdb5eb73ffba9642394a71facdf
