---
format: plan-v1
date: 2026-08-02
adrs:
  - task-scoped-plan-decision-context-and-phase-outcomes
status: Proposed
---
# Plan: Implement task-scoped plan context and phase outcomes

## Goal

Implement [ADR-task-scoped-plan-decision-context-and-phase-outcomes](../decisions/task-scoped-plan-decision-context-and-phase-outcomes.md): give new ADR Decision items stable slug identity, parse task-scoped Applying and Context references plus phase-owned outcomes in plan-v2, check their links and Proposed-only coverage in working and staged universes, and make `awf read plan` include the selected governing context without transferring phase ownership.

This plan does not retrofit historical ADR or plan bytes, restore ADR supersession or currentness inference, allow positional references into amendable ADRs, add a read flag, change task boundaries into transactions, or make assignment coverage blocking after implementation.

## Architecture summary

`internal/adr` remains the sole ADR parser and gains a V4 Decision-item model that retains exact source blocks and resolves stable slugs or frozen legacy ordinals. Schema generation 33 activates `current-state-v4` without rewriting existing ADRs. `internal/plan` remains the sole plan parser and renderer: plan-v2 extends the plan-v1 model with typed Decision references, slugged DoD items, and phase Advances/Completes assignments; a source-set seam lets filesystem and immutable snapshot adapters share one parser. The plan renderer consumes a plan-owned resolved-context representation so it neither loads ADRs nor reparses their Markdown.

`internal/project` is the composition boundary. The working check threads its one parsed ADR and plan corpus through hard reference checks and Proposed-only notes. The staged check independently parses HEAD and index plan sources from their immutable snapshot trees and joins the index plans only to the index ADR corpus; it never consults dirty working bytes. The read service resolves one plan selection, maps ADR-owned Decision blocks to plan-owned projection inputs, and returns `internal/plan`'s bytes. `cmd/awf` keeps the existing grammar and only routes arguments, output, and typed failures.

The ADR operations apply in declaration order. Phase 1 accepts the reviewed ADR without applying operations. Phase 2 enters Implementing and applies operations 1 through 10 for V4 identity and lifecycle, retaining item data privately until a real cross-package consumer arrives. Phase 3 refactors the existing plan-v1 source and projection paths behind immediately consumed shared seams without accepting a new format or applying claims. Phase 4 activates the complete plan-v2 parser, renderer, working/index checks, read composition, scaffold, and one pair-atomic batch for operations 11 through 18. Phase 5 applies operation 19 for safe phase ownership guidance and leaves operation 20, `rendering/workflow-skill-templates:plan-task-detail-modes`, as the final nonempty batch owned by terminal implementation review. That terminal transaction also freezes the ADR and this plan only after review establishes completion.

A bounded preparatory refactor is required in Phase 3 so the existing plan-v1 filesystem and read paths immediately consume shared source-set and projection seams before plan-v2 widens them. It changes no package boundary or output. Beyond that transaction, enabling work stays within cohesive model extensions; project code must not introduce parallel ADR or plan representations beyond the explicit projection translation.

## Phase 1: Accept the reviewed decision

**Execution mode: inline.**

### Task 1.1: Transition the pending ADR to Accepted
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Apply the `awf-adr-lifecycle` procedure to the reviewed pending ADR. Change `status:` from Proposed to Accepted and append the canonical Accepted history event with the current content digest; do not amend Decision or State changes content and do not apply any operation. Run `./x render` so the index and lock agree with the transition. Confirm `./awf context --show pending docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md` reports all 20 operations remaining and the ADR as Accepted and frozen only according to its V3 amend-until-terminal semantics.

### Phase close

Stage only the ADR lifecycle transaction and generated index/lock output. Require `./awf check staged` and `./x gate` to pass, then commit:

```commit
docs(adr): accept task-scoped plan context
```

## Phase 2: Add stable Decision identity and activate ADR V4

**Execution mode: subagent-driven.**

### Task 2.1: Parse exact Decision item source blocks and identities in the ADR model
Latitude: exact
Paths: ["internal/adr/adr.go", "internal/adr/decision.go", "internal/adr/format.go", "internal/adr/status.go", "internal/adr/adr_test.go", "internal/adr/format_test.go", "internal/adr/corpus_test.go", "internal/adr/pending_test.go"]

Before dispatch, require `git status --short` to be empty and `go test ./internal/adr ./internal/migrate ./internal/project ./cmd/awf` plus `./x check` to pass from the Phase 1 commit.

Create `internal/adr/decision.go` as the single home of package-private Decision item state carrying ordinal, optional stable slug, and exact authored source block. Slice blocks from raw `ADR.Source` offsets between column-zero sequential item openers and the Decision section boundary; do not use `ADR.Sections` to reconstruct them, and preserve continuation paragraphs, nested lists, backtick and tilde fences, and final newlines byte-for-byte. Extend `ADR` only with retained source/model state needed by parsing and validation. Keep new indexes and selector helpers package-private in this phase, keep raw corpus access closed, and keep files outside `internal/adr` from reading `Sections` directly. Task 4.3 exposes the smallest documented semantic lookup/error seam with its first production consumer.

Add `CurrentStateV4` and exact marker `current-state-v4`. `ParseV4` reuses V3 record slug, filename, and heading validation plus V2/V3 lifecycle, digest, Applied, Reapplied, Amended, pairing, and freeze semantics, while requiring every Decision item to begin exactly `N. ` followed by `` `decision: <lowercase-kebab-slug>` `` and nonempty commitment prose. Refuse missing, empty, malformed, or duplicate slugs. Preserve V1 through V3 bytes and behavior. Validate and retain the V4 slug index during parsing without exporting an unconsumed lookup API.

Tests prove multiline and fenced source retention exactly, V4 uniqueness and grammar, V1-V3 compatibility, retained ordinal/source data for V1/V2/V3, pending retained-slug identity, and source immutability. Add `// invariant: adr-system/adr-lifecycle:decision-item-stable-identity (TestDecisionItemStableIdentity)` immediately above the test whose name occurs verbatim and proves the complete parser/identity contract. Every declaration exported for an existing outside-package consumer receives a leading Go doc comment; all other new declarations remain private.

### Task 2.2: Register schema generation 33 and scaffold V4 records
Latitude: exact
Paths: ["internal/adr/format.go", "internal/adr/format_test.go", "internal/migrate/migrate.go", "internal/migrate/decisionitemslugs.go", "internal/migrate/decisionitemslugs_test.go", "internal/migrate/forwardport_test.go", "internal/migrate/migrate_test.go", ".awf/parts/adr-template/frontmatter.md", ".awf/parts/adr-readme/index.md", "templates/adr-template/template.md.tmpl", "templates/adr-readme/README.md.tmpl", "internal/project/golden_test.go", "internal/project/target_test.go", "cmd/awf/new_test.go"]

Add generation 33 as the schema activation for V4. The migration is a silent byte-preserving tree migration for historical ADRs and ordinary config; the normal upgrade/sync machinery owns the schema and lock stamp. Register V4 after V3 in `formatActivations`, make it the only format accepted for newly pending records at generation 33, and prove `FormatAtGeneration` keeps V3 through generation 32. Update current-generation, forward-port, ahead-binary, stale lock, scaffolding, and zero-mutation upgrade tests so parser activation, migration `Current()`, lock schema, and `awf new adr` cannot disagree.

Change the authored ADR template and README sources so new records render `current-state-v4` and every example Decision item begins with a unique `decision:` slug. Explain that V4 slugs are stable navigation identities, not active authority or supersession anchors; older frozen formats use ordinals only. Preserve publication-safe generic prose and pending record scaffold behavior. Run `./x render` and let it determine root and Sundial outputs; never edit those outputs directly.

### Task 2.3: Apply the first ADR operation batch with exact current-state provenance
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", "docs/decisions/INDEX.md", ".awf/awf.lock", ".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", ".awf/domains/parts/adr-system/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md"]

Transition the Accepted ADR to Implementing and append the first Applied event containing exactly this declaration-ordered prefix:

1. update `adr-system/adr-lifecycle:intrinsic-format-routing`
2. update `adr-system/adr-lifecycle:adr-amendable-until-terminal`
3. update `adr-system/adr-lifecycle:adr-slug-frontmatter-mandatory`
4. update `adr-system/adr-lifecycle:corrective-reapplication`
5. update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
6. update `adr-system/adr-lifecycle:applied-history-events-append-only`
7. update `adr-system/adr-lifecycle:corpus-single-identity-key`
8. update `adr-system/adr-lifecycle:decision-items-enumerable`
9. update `adr-system/adr-lifecycle:pending-adr-slug-identity`
10. add `adr-system/adr-lifecycle:decision-item-stable-identity`

Update the nine existing claims to include V4 wherever they currently enumerate V2/V3 or V3 identity, without weakening older-format behavior. Add the stable-identity invariant stating the V4 marker grammar, exact source-block retention, unique per-record slug lookup, frozen pre-V4 `#N` compatibility, amendable-ordinal refusal, and absence of supersession/currentness inference. Set `Origin: ADR-task-scoped-plan-decision-context-and-phase-outcomes`, `Backing: test`, and the Task 2.1 proof marker. Preserve every existing Origin, Revised-by entry, Backing, and proof marker on updated claims, appending this pending ADR once to Revised-by in canonical order.

Update authored domain and architecture sources to describe V4 as the current scaffold format and the `internal/adr` Decision-item/source ownership. Render the domain, topic, architecture, template, README, index, workflow targets, Sundial outputs, and lock from `.awf/` sources.

### Phase close

Run `gofmt -w internal/adr/adr.go internal/adr/decision.go internal/adr/format.go internal/adr/status.go internal/adr/adr_test.go internal/adr/format_test.go internal/adr/corpus_test.go internal/adr/pending_test.go internal/migrate/migrate.go internal/migrate/decisionitemslugs.go internal/migrate/decisionitemslugs_test.go internal/migrate/forwardport_test.go internal/migrate/migrate_test.go internal/project/golden_test.go internal/project/target_test.go cmd/awf/new_test.go`, `go test ./internal/adr ./internal/migrate ./internal/project ./cmd/awf`, `./x render`, `./x check`, and `git diff --check`; all must pass. Stage the complete V4 behavior, first Applied batch, exact claims/proofs, docs, and generated outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(adr-system): add stable Decision identities
```

## Phase 3: Prepare shared plan source and projection seams

**Execution mode: subagent-driven.**

### Task 3.1: Route the existing plan-v1 parser through one source-set seam
Latitude: exact
Paths: ["internal/plan/plan.go", "internal/plan/structure.go", "internal/plan/source.go", "internal/plan/plan_test.go", "internal/plan/structure_test.go"]

Before dispatch, require a clean checkout and `go test ./internal/plan ./internal/adr` plus `./x check` to pass from Phase 2.

Create `internal/plan/source.go` with the smallest source value required to carry filename, display path, and bytes. Route filesystem `ParseDir` through one shared source-set parser while preserving symlink confinement in the filesystem adapter, deterministic diagnostic aggregation, the operation-owned one-parse contract, legacy skipping, and exact plan-v1 results. Do not accept `plan-v2`, add its task fields, or expose a source-set API to other packages yet. The existing filesystem production path is the first consumer in this transaction. Tests compare directory and source-set parsing internally across valid plan-v1, legacy, malformed-frontmatter, multi-diagnostic, and confined-symlink cases with byte-identical results.

Keep new declarations package-private unless an outside-package production consumer lands in this phase. Every declaration that must be exported for the existing project adapter receives a leading Go doc comment stating its semantic contract.

### Task 3.2: Refactor plan-v1 projection inputs without changing output
Latitude: exact
Paths: ["internal/plan/projection.go", "internal/plan/projection_test.go", "internal/project/plan_read.go", "internal/project/plan_read_test.go"]

Introduce only the neutral projection-input seam needed by the current `Project.ReadPlan` production path and route plan-v1 through it immediately. Keep exact resolution, selector errors, executable-closure ordering, and rendered bytes unchanged. Do not add resolved ADR values, plan-v2 categories, or an API with no production caller. Document every newly exported declaration and keep representation construction in `internal/plan`; `internal/project` supplies no Markdown model.

Exact-byte tests compare every existing plan-v1 phase/task projection and typed failure before and after the refactor, including source hashes and configured docs-directory behavior. The project service is the first outside-package consumer in this transaction, so no definition waits for Phase 4.

### Phase close

Run `gofmt -w internal/plan/plan.go internal/plan/structure.go internal/plan/source.go internal/plan/plan_test.go internal/plan/structure_test.go internal/plan/projection.go internal/plan/projection_test.go internal/project/plan_read.go internal/project/plan_read_test.go`, `go test ./internal/plan ./internal/project`, `./x check`, and `git diff --check`; each command must exit zero and the drift check must be clean. Stage only the behavior-preserving parser/projection seams and tests. This phase accepts no new authored format and applies no ADR operation. Require `./awf check staged` and `./x gate`, then commit:

```commit
refactor(plans): prepare shared projection seams
```

## Phase 4: Activate plan-v2 checks and composed projections

**Execution mode: subagent-driven.**

### Task 4.1: Parse plan-v2 references, phase assignments, and slugged DoD items
Latitude: exact
Paths: ["internal/plan/plan.go", "internal/plan/structure.go", "internal/plan/source.go", "internal/plan/plan_test.go", "internal/plan/structure_test.go"]

Before dispatch, require a clean checkout and `go test ./internal/plan ./internal/project ./cmd/awf` plus `./x check` to pass from Phase 3.

Add explicit `plan-v2` routing while leaving marker-absent and plan-v1 paths byte-compatible. Introduce a documented plan-owned `DecisionRef` carrying authored text, ADR identity, decision slug or legacy ordinal, and field kind; parse exact `<adr-number-or-retained-slug>:<decision-slug-or-#N>` syntax without resolving the ADR. Extend `TaskFields` with optional nonempty Applying and Context JSON string arrays. Extend `Phase` with optional nonempty Advances then Completes arrays directly after execution mode. Parse Definition-of-done bullets beginning with unique `` `dod: <slug>` `` markers and retain complete bullet blocks. Refuse empty arrays, duplicate entries, malformed identities/selectors, unknown or misplaced fields, missing DoD targets, duplicate Completes owners, and same-phase advance plus complete. Permit multiple advancing phases and phases with neither field.

Expose the Task 3.1 source-set seam only now because Tasks 4.3 and 4.4 are its first outside-package production consumers. Add leading Go doc comments to every exported type, function, constant, method, and error identity. Tests cover every grammar and relationship branch, source order, directory/source parity, and unchanged legacy/plan-v1 diagnostics and bytes. Add `// invariant: adr-system/plan-artifacts:plan-v2-decision-references (TestPlanV2DecisionReferences)` and `// invariant: adr-system/plan-artifacts:plan-v2-phase-outcomes (TestPlanV2PhaseOutcomes)` immediately above their named proving tests.

### Task 4.2: Render plan-v2 from resolved plan-owned inputs
Latitude: exact
Paths: ["internal/plan/projection.go", "internal/plan/projection_test.go"]

Define a documented plan-owned resolved Decision value containing resolved item key, ADR identity, title, status, and exact Markdown. Extend selection to expose the chosen task/phase references and Advances/Completes items without importing `internal/adr`. Render frontmatter/title, Goal, Architecture summary, Applying decisions, Context decisions, owning phase/execution mode, selected task(s), Phase close, advanced outcomes, completed outcomes, and Notes, omitting empty categories. Phase unions preserve first authored occurrence, deduplicate by resolved item key, and promote Context to Applying.

For `P.T`, emit exact generated scope prose before selected work: only that task is in scope; Phase close and phase outcomes remain phase-owner context; transaction ownership does not transfer; unselected tasks must not be performed merely to clear an outcome. Label Phase close and outcomes consistently. Phase selection has no warning. Preserve complete Decision/DoD Markdown and source bytes. Exact fixtures cover ordering, promotion, deduplication, empty categories, task qualification, fenced content, and source hashes. Task 4.6 supplies the first production provider of resolved inputs in this same transaction.

### Task 4.3: Resolve cross-corpus references and working-tree coverage
Latitude: exact
Paths: ["internal/adr/adr.go", "internal/adr/decision.go", "internal/project/check.go", "internal/project/plan_context.go", "internal/project/check_test.go", "internal/project/plan_detail_modes_test.go"]

Expose the smallest documented `internal/adr` Decision lookup and typed selector-error API now, in the same task as its first production consumer. It resolves V4 slugs and frozen pre-V4 canonical `#N`, refuses incompatible or amendable targets, and lists sorted available selectors; every exported declaration has a leading Go doc comment. Create one project-owned plan-artifact report seam that consumes already-parsed `[]plan.Plan` and `adr.Corpus` and returns blocking `manifest.Drift` plus non-failing notes. Resolve both ADR number and retained slug through `Corpus.ByIdentity`, compare Applying membership by resolved record rather than authored spelling, and delegate item lookup and freeze/format compatibility to `internal/adr`. Hard findings name plan, phase/task, field, authored reference, and available selectors. They cover unresolved ADR/item, incompatible selector, amendable legacy ordinal, Applying outside plan-level `adrs:`, and every structural plan-v2 diagnostic at both plan statuses.

For Proposed plan-v2 only, emit stable sorted notes for each non-spike task in a nonempty-ADRs plan that omits Applying, each plan-level ADR Decision item with no Applying assignment anywhere in the plan, each DoD item with no Advances or Completes assignment, and each advanced-only DoD item with no Completes owner. Context does not cover a Decision. Multiple Applying tasks and multiple Advances are clean. Empty-ADRs plans and spikes are exempt. Implemented plans emit none of these coverage notes. Thread the same one parsed plan set from `CheckReport` to hard and advisory consumers; do not call `plan.ParseDir` again.

### Task 4.4: Evaluate staged plan-v2 bytes without working-tree contamination
Latitude: exact
Paths: ["internal/project/currentstate.go", "internal/project/staged_plan.go", "internal/project/currentstate_test.go", "internal/project/staged_test.go", "internal/snapshot/snapshot.go", "cmd/awf/checkstaged.go", "cmd/awf/check_test.go"]

Use the existing HEAD/index trees and their own config, lock, docs directory, and ADR corpus to enumerate plan sources and call the Task 4.1 source-set parser. Extend the project-owned staged report so index plan hard findings join `CurrentStateReport.Findings()` and index Proposed coverage joins `Notes()` without changing exit status. Parse HEAD plans only where pair context is required; do not read filesystem plans or call ordinary `CheckReport`. A dirty working plan or ADR must not affect staged output, while staging either side must change output deterministically. Preserve current-state static, provisional older-format, coverage/fan-out, merge aggregate, missing-config, and no-HEAD behavior.

Tests construct divergent HEAD, index, and working bytes and prove staged reference resolution and notes use only index plans plus index ADRs, ordinary repo checks use working bytes, partial staging cannot borrow a working fix, blocking staged link failures fail the command, and assignment notes print without changing success. Add `// invariant: adr-system/plan-artifacts:plan-v2-assignment-advisories (TestPlanV2AssignmentAdvisories)` above the cross-universe proof and keep the proving name on a non-marker line. Update the check-universe command proof to cover plan-artifact findings and notes in the staged aggregate.

### Task 4.5: Activate plan-v2 scaffolding and prepare the plan/check claim endpoints
Latitude: exact
Paths: ["templates/plans-template/template.md.tmpl", "templates/plans-readme/README.md.tmpl", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", "internal/project/golden_test.go", "internal/project/target_test.go", "internal/project/output_plan_test.go", "cmd/awf/new_test.go", "docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md"]

Change future plan scaffolds to `format: plan-v2`. Because its default `adrs:` is empty, the canonical scaffold emits no Applying or Context field; its task prose shows unique reference examples only as inert inline code and tells the author to add nonempty fields when an ADR applies. It emits one unique `dod:` bullet and a matching phase Completes assignment so the untouched scaffold parses without coverage gaps; the prose explains Advances for earlier partial contribution. Update the plan README with the exact grammar, omission-not-empty rule, hard links, Proposed-only coverage, phase outcome semantics, and plan-v1/legacy boundary. Do not yet change execution or reviewer skills; Phase 5 owns those workflow surfaces. Render root and Sundial plan artifacts from authored sources and prove `awf new plan` immediately parses as plan-v2.

Prepare exact claim endpoints for operations 11 through 17, but do not append a history event yet; Task 4.8 owns one pair-atomic Applied event for the complete 11-through-18 batch:

11. update `adr-system/plan-artifacts:plan-frontmatter-validated`
12. update `adr-system/plan-artifacts:plans-template-taxonomy`
13. update `adr-system/plan-artifacts:plan-executable-projection`
14. add `adr-system/plan-artifacts:plan-v2-decision-references`
15. add `adr-system/plan-artifacts:plan-v2-phase-outcomes`
16. add `adr-system/plan-artifacts:plan-v2-assignment-advisories`
17. update `tooling/cli:check-universe-groups`

Update claim prose to preserve legacy and plan-v1 contracts while adding plan-v2 grammar, hard resolution, Proposed-only working/index notes, renderer ordering and scope safety, and staged-universe isolation. Add proof markers immediately above `TestPlanV2DecisionReferences`, `TestPlanV2PhaseOutcomes`, and `TestPlanV2AssignmentAdvisories`; update existing proof markers without renaming their proving units unless the plan changes the unit and marker together. Each new invariant uses this ADR as Origin and `Backing: test`; updated claims preserve Origin and append this ADR once to Revised-by.

### Task 4.6: Resolve selected Decision items in the project read service
Latitude: exact
Paths: ["internal/project/plan_read.go", "internal/project/plan_read_test.go", "internal/plan/projection.go"]

Extend `Project.ReadPlan` without changing its arguments. Resolve the configured plan directory once, select the exact plan and `P`/`P.T`, and preserve the plan-v1 fast path and bytes. For plan-v2, load the operation-owned ADR corpus once, resolve only the selected Applying/Context references, map ADR-owned exact Decision blocks into the plan-owned projection values, and pass those plus selected phase outcomes to `internal/plan`. Do not parse ADR Markdown, read ADR paths directly, or render in project. Typed errors retain plan/task/field/reference context and available ADR Decision selectors. Tests cover configured docs directories, pending retained-slug lookup before and after a simulated numbering rename, numeric legacy lookup, frozen-status enforcement, Applying promotion, and typed-error preservation.

### Task 4.7: Preserve the CLI grammar and prove end-to-end projection safety
Latitude: exact
Paths: ["cmd/awf/read.go", "cmd/awf/read_test.go", "cmd/awf/help_test.go", "cmd/awf/gate_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go"]

Keep exact grammar `awf read plan <plan> <P[.T]>` and add no flag. The command continues to validate arity, open through the ordinary gated project path, call one project service, write returned bytes unchanged, and map typed failures. Update help prose to say plan-v2 always includes task-scoped Decisions and phase outcomes, while plan-v1 retains its original closure. End-to-end tests use a copied plan/ADR corpus, compare phase and task output exactly, assert the task scope notice and category ordering, assert no unselected task or Decision leaks, hash every source before and after, and cover ahead-binary refusal and clean stdout/stderr separation.

### Task 4.8: Apply the combined plan/read operation batch and document the closure
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/parts/agents-doc/commands.md", "templates/docs/working-with-awf.md.tmpl", ".awf/docs/parts/architecture/data-flow.md", "README.md", "changelog/CHANGELOG.md"]

Append one Applied event containing exactly the declaration-ordered operations 11 through 18 prepared by Tasks 4.5 and 4.8; one authored commit must not append two batches. Operation 18 updates `tooling/cli:plan-read-command`. Update its claim to retain exact plan/selector resolution and plan-v1 bytes while stating the plan-v2 Decision/outcome ordering, first-authored deduplication, Applying precedence, task scope notice, source preservation, and absence of a flag. Preserve its existing Origin and proof unit `TestReadPlanCommand`, append this ADR once to Revised-by, and strengthen that test rather than adding a competing proof marker.

Update authored command/docs sources, README, architecture data flow where the read composition changed, and Unreleased changelog. Render every managed root and Sundial output. Do not describe ADR Decision prose as active current authority; Applying is accepted implementation intent and Context is frozen history/design input.

### Phase close

Run `gofmt -w internal/plan/plan.go internal/plan/structure.go internal/plan/source.go internal/plan/plan_test.go internal/plan/structure_test.go internal/plan/projection.go internal/plan/projection_test.go internal/project/check.go internal/project/plan_context.go internal/project/check_test.go internal/project/plan_detail_modes_test.go internal/project/currentstate.go internal/project/staged_plan.go internal/project/currentstate_test.go internal/project/staged_test.go internal/project/plan_read.go internal/project/plan_read_test.go cmd/awf/checkstaged.go cmd/awf/check_test.go cmd/awf/read.go cmd/awf/read_test.go cmd/awf/help_test.go cmd/awf/gate_test.go cmd/awf/new_test.go internal/clispec/clispec.go internal/clispec/clispec_test.go`, `go test ./internal/adr ./internal/plan ./internal/project ./internal/clispec ./cmd/awf`, `go test ./internal/project ./cmd/awf -run 'TestPlanV2AssignmentAdvisories|TestCheckUniverseGroups|TestReadPlanCommand' -count=1`, `./x render`, `./x check`, and `git diff --check`; every command must exit zero, the focused staged fixtures must prove no working-tree contamination, and render/check must be clean. Stage the complete plan-v2 parser, renderer, working/index validation, read composition, one Applied batch for operations 11 through 18, claims/proofs/docs, and generated outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(plans): activate task-scoped plan context
```

## Phase 5: Roll the ownership contract through workflow surfaces

**Execution mode: subagent-driven.**

### Task 5.1: Update plan authoring, review, and execution contracts from `.awf/` sources
Latitude: exact
Kind: batch
Paths: ["glob:.awf/skills/parts/writing-plans/*.md", ".awf/agents/plan-reviewer.yaml", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/adr-lifecycle/SKILL.md.tmpl", "templates/agents/plan-reviewer.md.tmpl"]
Representative: Update the writing-plans field vocabulary to require decision-slugged V4 items, plan-v2 Applying/Context omission semantics, slugged DoD bullets, and phase Advances/Completes ownership, while preserving qualifying prose and exactness rules.
Edge: Update task-query execution guidance to treat the generated scope notice, Phase close, and advanced/completed outcomes as phase-owner context only; reviewers must flag Context used to evade Applying, false completion ownership, and references that confuse historical ADR prose with current authority.
Post-check: Run `rg -n 'plan-v1|Applying|Context|Advances|Completes|decision:|dod:|phase-owner context' .awf/skills .awf/agents templates/skills templates/agents` and inspect every plan authoring/review/execution consumer; no current scaffold mandate may remain plan-v1, and every task-projection consumer must preserve phase ownership.

Before dispatch, require a clean checkout and `go test ./...` plus `./x check` to pass from Phase 4.

Update only authored sources. Writing plans must tell authors to use retained pending ADR slugs, stable Decision slugs for V4, and frozen `#N` only for pre-V4. Review must check every plan-level Decision assignment and DoD final owner substantively while recognizing the checks are advisory during Proposed. Execution must not infer that a task helper owns the phase close or outcomes. ADR lifecycle guidance must explain V4 markers and legacy ordinal freeze rules. Preserve chain checkpoints, report-only review, managed-memory, and model-selection contracts unchanged.

### Task 5.2: Update architecture, domain, workflow, and publication docs
Latitude: exact
Kind: batch
Paths: [".awf/domains/parts/adr-system/current-state.md", ".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/parts/agents-doc/commands.md", ".awf/parts/working-with-awf/commands.md"]
Representative: Document `internal/adr` source-block identity ownership, `internal/plan` source-set parsing and rendering ownership, and `internal/project` working/index/read composition without duplicating parser policy in command code.
Edge: Explain that task projections include phase-owned outcomes only as constraints, that assignment notes stop after plan implementation, and that historical Decision context never replaces current-state claim authority.
Post-check: Run `./x render && ./x check`, then inspect `docs/architecture.md`, `docs/domains/{adr-system,tooling}.md`, `docs/workflow.md`, `docs/working-with-awf.md`, `docs/pitfalls.md`, root native targets, and corresponding Sundial outputs; every generated file must derive from an authored source and no unresolved value token may appear.

Update documentation that explicitly describes the changed formats and package model. Do not add a pitfall: the generated projection notice plus authoring, review, and execution contracts deterministically enforce the ownership boundary, and no recurring operational trap remains outside those homes. Keep command grammar unchanged and document plan-v1 as compatibility behavior, not the current scaffold.

### Task 5.3: Apply the phase-ownership operation and complete rendered parity
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "internal/project/phase_transaction_ownership_test.go", "internal/project/plan_detail_modes_test.go", "internal/project/spine_test.go", "internal/project/target_test.go", "internal/project/output_plan_test.go"]

Append one Applied event for operation 19 only: update `rendering/workflow-skill-templates:phase-transaction-ownership`. Update the claim so a fresh task owner may consume the bounded plan-v2 closure but the generated scope notice, Phase close, Advances, and Completes remain phase-owner context and never transfer commit, review, checkpoint, handoff, or helper authority. Preserve its Origin and existing proof unit, append this ADR once to Revised-by, and strengthen the existing parity test.

Land the production workflow/template changes relevant to operation 20 as reviewed behavior, but leave `rendering/workflow-skill-templates:plan-task-detail-modes` unchanged and Remaining in the ADR for the terminal-review-owned final batch. The terminal transaction will update that claim and its existing proof marker after review confirms the rendered vocabulary. Run `./x render` and inspect root and Sundial target parity, publication safety with empty variables, generated ADR/plan templates, and absence of hand-edited outputs.

### Phase close

Run `gofmt -w internal/project/phase_transaction_ownership_test.go internal/project/plan_detail_modes_test.go internal/project/spine_test.go internal/project/target_test.go internal/project/output_plan_test.go`, `go test ./internal/project -run 'TestPhaseTransactionOwnershipAcrossWorkflowSurfaces|TestPlanTaskDetailModesStayAligned|TestWritingPlansTemplate|TestExecutingPlansTemplate|TestSubagentDrivenDevelopmentTemplate|TestAdrLifecycleTemplate' -count=1`, `go test ./...`, `./x render`, `./x check`, and `git diff --check`; every command must exit zero, focused parity tests must pass, and render/check must be clean. Stage authored sources, operation 19, proof changes, docs, changelog, and every generated output selected by render. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(rendering): roll out task-scoped plan context
```

## Definition of done

- New ADR scaffolds use schema-activated `current-state-v4`; each Decision item has a unique stable slug, pending record identity survives numbering, and pre-V4 frozen items remain addressable only by canonical ordinal without changing historical bytes.
- New plan scaffolds use `plan-v2`; Applying, Context, slugged DoD items, Advances, and Completes parse into one typed model, while legacy and plan-v1 behavior remains compatible.
- Working and staged checks hard-fail broken references from their own selected universes and emit Proposed-only assignment notes without changing success; dirty working bytes cannot contaminate staged results.
- `awf read plan` keeps its existing grammar and automatically includes only the selected task/phase Decisions and outcomes in deterministic order, with exact source preservation and a task scope notice that prevents phase-owner overreach.
- Current-state claims, proof markers, templates, skills, reviewers, architecture/domain/command docs, generated targets, Sundial outputs, locks, README, and changelog agree with production behavior and remain publication-safe.
- Every implementation phase closes with `./awf check staged` and `./x gate` green; terminal implementation review settles before the final operation, ADR Implemented event, and plan freeze land together.

## Notes

The terminal workflow is not another implementation phase. After Phase 5, invoke `awf-reviewing-impl` over every commit after the settled plan-review baseline. Resolve mechanical and reasoned findings in new green commits and stop for any user-decision finding. Once terminal review establishes every Definition-of-done outcome, apply the final declaration-ordered operation `rendering/workflow-skill-templates:plan-task-detail-modes`: update its claim to enumerate plan-v2 Applying/Context, slugged DoD, Advances/Completes, omission-not-empty arrays, reviewer coverage, and task scope safety; preserve its Origin and existing proof marker and append this ADR once to Revised-by. In the same terminal transaction append the final Applied event and Implemented status event with the canonical digest, change this plan to `status: Implemented`, record deviations here, run `./x render`, and require `./awf check staged` and `./x gate` before committing `docs(plans): complete task-scoped plan context`.

Then follow the governed managed-worktree integration path: merge current `main` into this worktree, resolve or abort conflicts, run `./awf adr number task-scoped-plan-decision-context-and-phase-outcomes`, render and gate the deterministic numbering transaction, integrate through the effort command, renew implementation review if integration introduces divergence, and remove managed topology only after the integrated history is settled. Run `awf-retrospective` last. Numbering must retain this plan's slug link.

Generation 33 is intentionally byte-preserving for historical ADRs. It activates only the V4 parser/scaffold contract and lock schema; it must not retrofit Decision markers into V1-V3 records. Likewise, plan-v2 is selected only by its intrinsic marker and future scaffold, never by date or filename.
