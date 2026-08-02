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

The ADR operations apply in declaration order. Phase 1 accepts the reviewed ADR without applying operations. Phase 2 enters Implementing and applies operations 1 through 10 for V4 identity and lifecycle. Phase 3 lands the internal plan-v2 model as preparatory reachable parser/renderer behavior without applying a claim batch. Phase 4 activates plan-v2 authoring and applies operations 11 through 17 for plan structure, references, outcomes, assignment notes, and staged check routing. Phase 5 applies operation 18 for the composed read command. Phase 6 applies operation 19 for safe phase ownership guidance and leaves operation 20, `rendering/workflow-skill-templates:plan-task-detail-modes`, as the final nonempty batch owned by terminal implementation review. That terminal transaction also freezes the ADR and this plan only after review establishes completion.

No preparatory package refactor is needed. The enabling work is limited to cohesive model extensions and one source-set adapter seam in each model owner; project code must not introduce parallel ADR or plan representations beyond the explicit projection translation.

## Phase 1: Accept the reviewed decision

**Execution mode: inline.**

### Task 1.1: Transition the pending ADR to Accepted
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Apply the `awf-adr-lifecycle` procedure to the reviewed pending ADR. Change `status:` from Proposed to Accepted and append the canonical Accepted history event with the current content digest; do not amend Decision or State changes content and do not apply any operation. Run `./x render` so the index and lock agree with the transition. Confirm `./awf context docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md pending` reports all 20 operations remaining and the ADR as Accepted and frozen only according to its V3 amend-until-terminal semantics.

### Phase close

Stage only the ADR lifecycle transaction and generated index/lock output. Require `./awf check staged` and `./x gate` to pass, then commit:

```commit
docs(adr): accept task-scoped plan context
```

## Phase 2: Add stable Decision identity and activate ADR V4

**Execution mode: subagent-driven.**

### Task 2.1: Parse exact Decision item source blocks and selectors in the ADR model
Paths: ["internal/adr/adr.go", "internal/adr/decision.go", "internal/adr/format.go", "internal/adr/status.go", "internal/adr/adr_test.go", "internal/adr/format_test.go", "internal/adr/corpus_test.go", "internal/adr/pending_test.go"]

Before dispatch, require `git status --short` to be empty and `go test ./internal/adr ./internal/migrate ./internal/project ./cmd/awf` plus `./x check` to pass from the Phase 1 commit.

Create `internal/adr/decision.go` as the single home of a typed `DecisionItem` carrying ordinal, optional stable slug, and the exact authored source block. Slice blocks from raw `ADR.Source` offsets between column-zero sequential item openers and the Decision section boundary; do not use `ADR.Sections` to reconstruct them, and preserve continuation paragraphs, nested lists, backtick and tilde fences, and final newlines byte-for-byte. Extend `ADR` with the retained source/model state and semantic lookup methods for V4 slug selectors and frozen pre-V4 positive ordinals. Keep raw corpus access closed and keep files outside `internal/adr` from reading `Sections` directly.

Add `CurrentStateV4` and exact marker `current-state-v4`. `ParseV4` must reuse V3 record slug, filename, and heading validation plus V2/V3 lifecycle, digest, Applied, Reapplied, Amended, pairing, and freeze semantics, while requiring every Decision item to begin exactly `N. ` followed by `` `decision: <lowercase-kebab-slug>` `` and nonempty commitment prose. Refuse missing, empty, malformed, or duplicate slugs. Preserve V1 through V3 bytes and behavior. Expose a typed selector error that distinguishes an unknown slug, a noncanonical `#N`, a slug used against pre-V4, `#N` used against V4, and an ordinal target whose authored-format lifecycle remains amendable; include sorted available selectors.

Tests must prove multiline and fenced source retention exactly, V4 uniqueness and grammar, V1-V3 compatibility, frozen V1/V2/V3 ordinal lookup, refusal of amendable legacy ordinals, pending retained-slug identity, and source immutability. Add `// invariant: adr-system/adr-lifecycle:decision-item-stable-identity (TestDecisionItemStableIdentity)` immediately above the test whose name occurs verbatim and proves the complete contract.

### Task 2.2: Register schema generation 33 and scaffold V4 records
Paths: ["internal/adr/format.go", "internal/adr/format_test.go", "internal/migrate/migrate.go", "internal/migrate/decisionitemslugs.go", "internal/migrate/decisionitemslugs_test.go", "internal/migrate/forwardport_test.go", "internal/migrate/migrate_test.go", ".awf/parts/adr-template/frontmatter.md", ".awf/parts/adr-readme/index.md", "templates/adr-template/template.md.tmpl", "templates/adr-readme/README.md.tmpl", "internal/project/golden_test.go", "internal/project/target_test.go", "cmd/awf/new_test.go"]

Add generation 33 as the schema activation for V4. The migration is a silent byte-preserving tree migration for historical ADRs and ordinary config; the normal upgrade/sync machinery owns the schema and lock stamp. Register V4 after V3 in `formatActivations`, make it the only format accepted for newly pending records at generation 33, and prove `FormatAtGeneration` keeps V3 through generation 32. Update current-generation, forward-port, ahead-binary, stale lock, scaffolding, and zero-mutation upgrade tests so parser activation, migration `Current()`, lock schema, and `awf new adr` cannot disagree.

Change the authored ADR template and README sources so new records render `current-state-v4` and every example Decision item begins with a unique `decision:` slug. Explain that V4 slugs are stable navigation identities, not active authority or supersession anchors; older frozen formats use ordinals only. Preserve publication-safe generic prose and pending record scaffold behavior. Run `./x render` and let it determine root and Sundial outputs; never edit those outputs directly.

### Task 2.3: Apply the first ADR operation batch with exact current-state provenance
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", ".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", ".awf/domains/parts/adr-system/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md"]

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

Run `gofmt -w` on changed Go files, `go test ./internal/adr ./internal/migrate ./internal/project ./cmd/awf`, `./x render`, `./x check`, and `git diff --check`; all must pass. Stage the complete V4 behavior, first Applied batch, exact claims/proofs, docs, and generated outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(adr-system): add stable Decision identities
```

## Phase 3: Build the typed plan-v2 model and bounded renderer

**Execution mode: subagent-driven.**

### Task 3.1: Parse plan-v2 task references, phase assignments, and slugged DoD items
Paths: ["internal/plan/plan.go", "internal/plan/structure.go", "internal/plan/source.go", "internal/plan/plan_test.go", "internal/plan/structure_test.go"]

Before dispatch, require a clean checkout and `go test ./internal/plan ./internal/adr` plus `./x check` to pass from Phase 2.

Add explicit `plan-v2` routing while leaving marker-absent and plan-v1 paths unchanged. Introduce a plan-owned `DecisionRef` with authored text, ADR identity, decision slug or legacy ordinal, and field kind; parse exact `<adr-number-or-retained-slug>:<decision-slug-or-#N>` syntax without resolving the ADR. Extend `TaskFields` with Applying and Context, each an optional nonempty one-line JSON array of unique strings. Extend `Phase` with optional nonempty Advances then Completes JSON arrays directly after execution mode. Parse plan-v2 Definition-of-done plain bullets beginning with unique `` `dod: <slug>` `` markers and retain each complete bullet block. Refuse empty arrays, duplicate entries, malformed identities/selectors, unknown or misplaced fields, missing DoD targets, duplicate Completes owners, and one phase both advancing and completing an item. Permit multiple advancing phases and phases with neither field.

Create a source-set parsing seam in `internal/plan/source.go` whose input carries filename, display path, and bytes. Make filesystem `ParseDir` adapt its confined files into that seam so working, HEAD, and index consumers share the same frontmatter and structural parser. Preserve symlink confinement in the filesystem adapter, deterministic diagnostics, the one-parse-per-operation contract, plan-v1 exact results, and legacy skipping. Tests cover valid mixed references and outcomes, every relationship failure, source order, source-set versus directory parity, and no plan-v1 diagnostic or byte change.

### Task 3.2: Render plan-v2 from resolved plan-owned projection inputs
Paths: ["internal/plan/projection.go", "internal/plan/projection_test.go"]

Define a narrow plan-owned resolved Decision projection value containing the resolved item key, ADR identity, title, status, and exact item Markdown. Extend projection selection to expose a selected task/phase reference set and assigned Advances/Completes DoD items without importing `internal/adr`. Render plan-v2 in this order: frontmatter/title, Goal, Architecture summary, nonempty Applying decisions, nonempty Context decisions, owning phase/execution mode, selected task(s), Phase close, advanced outcomes, completed outcomes, and Notes. A phase union preserves first authored occurrence, deduplicates by resolved item key, and promotes an item from Context to Applying.

For `P.T`, insert exact generated scope prose before selected work stating that only the named task is in scope, Phase close and phase outcomes remain phase-owner context, transaction ownership does not transfer, and unselected tasks must not be performed merely to clear an outcome. Label Phase close and both outcome categories consistently with that notice. A `P` projection has no warning. Empty categories are omitted. Preserve complete Decision and DoD Markdown and source files byte-for-byte. Retain current plan-v1 projection bytes and errors exactly. Exact-byte fixtures must cover task/phase order, promotion, deduplication, no-reference/no-outcome cases, scope qualification, fenced ADR content, and source hashes before and after reads.

### Phase close

Run `gofmt -w internal/plan`, `go test ./internal/plan`, `./x check`, and `git diff --check`. This is preparatory but reachable parser/renderer behavior exercised entirely through package APIs; it changes no scaffold or current-state claim and applies no ADR operation. Stage only the cohesive plan model, renderer, and tests. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(plans): model task-scoped plan projections
```

## Phase 4: Activate plan-v2 checks in working and staged universes

**Execution mode: subagent-driven.**

### Task 4.1: Resolve cross-corpus references and working-tree coverage
Paths: ["internal/project/check.go", "internal/project/plan_context.go", "internal/project/check_test.go", "internal/project/plan_detail_modes_test.go"]

Before dispatch, require a clean checkout and `go test ./internal/plan ./internal/project ./cmd/awf` plus `./x check` to pass from Phase 3.

Create one project-owned plan-artifact report seam that consumes already-parsed `[]plan.Plan` and `adr.Corpus` and returns blocking `manifest.Drift` plus non-failing notes. Resolve both ADR number and retained slug through `Corpus.ByIdentity`, compare Applying membership by resolved record rather than authored spelling, and delegate item lookup and freeze/format compatibility to `internal/adr`. Hard findings name plan, phase/task, field, authored reference, and available selectors. They cover unresolved ADR/item, incompatible selector, amendable legacy ordinal, Applying outside plan-level `adrs:`, and every structural plan-v2 diagnostic at both plan statuses.

For Proposed plan-v2 only, emit stable sorted notes for each non-spike task in a nonempty-ADRs plan that omits Applying, each plan-level ADR Decision item with no Applying assignment anywhere in the plan, each DoD item with no Advances or Completes assignment, and each advanced-only DoD item with no Completes owner. Context does not cover a Decision. Multiple Applying tasks and multiple Advances are clean. Empty-ADRs plans and spikes are exempt. Implemented plans emit none of these coverage notes. Thread the same one parsed plan set from `CheckReport` to hard and advisory consumers; do not call `plan.ParseDir` again.

### Task 4.2: Evaluate staged plan-v2 bytes without working-tree contamination
Paths: ["internal/project/currentstate.go", "internal/project/staged_plan.go", "internal/project/currentstate_test.go", "internal/project/staged_test.go", "internal/snapshot/snapshot.go", "cmd/awf/checkstaged.go", "cmd/awf/check_test.go"]

Use the existing HEAD/index trees and their own config, lock, docs directory, and ADR corpus to enumerate plan sources and call the Task 3.1 source-set parser. Extend the project-owned staged report so index plan hard findings join `CurrentStateReport.Findings()` and index Proposed coverage joins `Notes()` without changing exit status. Parse HEAD plans only where pair context is required; do not read filesystem plans or call ordinary `CheckReport`. A dirty working plan or ADR must not affect staged output, while staging either side must change output deterministically. Preserve current-state static, provisional older-format, coverage/fan-out, merge aggregate, missing-config, and no-HEAD behavior.

Tests construct divergent HEAD, index, and working bytes and prove staged reference resolution and notes use only index plans plus index ADRs, ordinary repo checks use working bytes, partial staging cannot borrow a working fix, blocking staged link failures fail the command, and assignment notes print without changing success. Add `// invariant: adr-system/plan-artifacts:plan-v2-assignment-advisories (TestPlanV2AssignmentAdvisories)` above the cross-universe proof and keep the proving name on a non-marker line. Update the check-universe command proof to cover plan-artifact findings and notes in the staged aggregate.

### Task 4.3: Activate plan-v2 scaffolding and apply the plan/check operation batch
Latitude: exact
Paths: ["templates/plans-template/template.md.tmpl", "templates/plans-readme/README.md.tmpl", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", "internal/project/golden_test.go", "internal/project/target_test.go", "internal/project/output_plan_test.go", "cmd/awf/new_test.go", "docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md"]

Change future plan scaffolds to `format: plan-v2`. Because its default `adrs:` is empty, the canonical scaffold emits no Applying or Context field; its task prose shows unique reference examples only as inert inline code and tells the author to add nonempty fields when an ADR applies. It emits one unique `dod:` bullet and a matching phase Completes assignment so the untouched scaffold parses without coverage gaps; the prose explains Advances for earlier partial contribution. Update the plan README with the exact grammar, omission-not-empty rule, hard links, Proposed-only coverage, phase outcome semantics, and plan-v1/legacy boundary. Do not yet change execution or reviewer skills; Phase 6 owns those workflow surfaces. Render root and Sundial plan artifacts from authored sources and prove `awf new plan` immediately parses as plan-v2.

Append one Applied event containing exactly operations 11 through 17:

11. update `adr-system/plan-artifacts:plan-frontmatter-validated`
12. update `adr-system/plan-artifacts:plans-template-taxonomy`
13. update `adr-system/plan-artifacts:plan-executable-projection`
14. add `adr-system/plan-artifacts:plan-v2-decision-references`
15. add `adr-system/plan-artifacts:plan-v2-phase-outcomes`
16. add `adr-system/plan-artifacts:plan-v2-assignment-advisories`
17. update `tooling/cli:check-universe-groups`

Update claim prose to preserve legacy and plan-v1 contracts while adding plan-v2 grammar, hard resolution, Proposed-only working/index notes, renderer ordering and scope safety, and staged-universe isolation. Add proof markers immediately above `TestPlanV2DecisionReferences`, `TestPlanV2PhaseOutcomes`, and `TestPlanV2AssignmentAdvisories`; update existing proof markers without renaming their proving units unless the plan changes the unit and marker together. Each new invariant uses this ADR as Origin and `Backing: test`; updated claims preserve Origin and append this ADR once to Revised-by.

### Phase close

Run `gofmt -w` on changed Go files, `go test ./internal/plan ./internal/project ./cmd/awf`, `./x render`, `./x check`, and `git diff --check`. Run focused staged fixtures with deliberately divergent working bytes and require zero contamination. Stage the complete working/index validation, plan-v2 scaffold activation, operations 11 through 17, proofs, and generated outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(plans): check task-scoped plan context
```

## Phase 5: Compose ADR Decisions into read-plan output

**Execution mode: subagent-driven.**

### Task 5.1: Resolve selected Decision items in the project read service
Paths: ["internal/project/plan_read.go", "internal/project/plan_read_test.go", "internal/plan/projection.go"]

Before dispatch, require a clean checkout and `go test ./internal/adr ./internal/plan ./internal/project ./cmd/awf` plus `./x check` to pass from Phase 4.

Extend `Project.ReadPlan` without changing its arguments. Resolve the configured plan directory once, select the exact plan and `P`/`P.T`, and preserve the plan-v1 fast path and bytes. For plan-v2, load the operation-owned ADR corpus once, resolve only the selected Applying/Context references, map ADR-owned exact Decision blocks into the plan-owned projection values, and pass those plus selected phase outcomes to `internal/plan`. Do not parse ADR Markdown, read ADR paths directly, or render in project. Typed errors retain plan/task/field/reference context and available ADR Decision selectors. Tests cover configured docs directories, pending retained-slug lookup before and after a simulated numbering rename, numeric legacy lookup, frozen-status enforcement, Applying promotion, and typed-error preservation.

### Task 5.2: Preserve the CLI grammar and prove end-to-end projection safety
Paths: ["cmd/awf/read.go", "cmd/awf/read_test.go", "cmd/awf/help_test.go", "cmd/awf/gate_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go"]

Keep exact grammar `awf read plan <plan> <P[.T]>` and add no flag. The command continues to validate arity, open through the ordinary gated project path, call one project service, write returned bytes unchanged, and map typed failures. Update help prose to say plan-v2 always includes task-scoped Decisions and phase outcomes, while plan-v1 retains its original closure. End-to-end tests use a copied plan/ADR corpus, compare phase and task output exactly, assert the task scope notice and category ordering, assert no unselected task or Decision leaks, hash every source before and after, and cover ahead-binary refusal and clean stdout/stderr separation.

### Task 5.3: Apply the read-command operation and document the closure
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/parts/agents-doc/commands.md", "templates/docs/working-with-awf.md.tmpl", "README.md", "changelog/CHANGELOG.md"]

Append one Applied event for operation 18 only: update `tooling/cli:plan-read-command`. Update its claim to retain exact plan/selector resolution and plan-v1 bytes while stating the plan-v2 Decision/outcome ordering, first-authored deduplication, Applying precedence, task scope notice, source preservation, and absence of a flag. Preserve its existing Origin and proof unit `TestReadPlanCommand`, append this ADR once to Revised-by, and strengthen that test rather than adding a competing proof marker.

Update authored command/docs sources, README, architecture data flow where the read composition changed, and Unreleased changelog. Render every managed root and Sundial output. Do not describe ADR Decision prose as active current authority; Applying is accepted implementation intent and Context is frozen history/design input.

### Phase close

Run `gofmt -w` on changed Go files, `go test ./internal/adr ./internal/plan ./internal/project ./internal/clispec ./cmd/awf`, `./x render`, `./x check`, and `git diff --check`. Stage read composition, operation 18, proof/docs, and generated outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(plans): project task-scoped ADR decisions
```

## Phase 6: Roll the ownership contract through workflow surfaces

**Execution mode: subagent-driven.**

### Task 6.1: Update plan authoring, review, and execution contracts from `.awf/` sources
Kind: batch
Paths: ["glob:.awf/skills/parts/writing-plans/*.md", ".awf/agents/plan-reviewer.yaml", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/adr-lifecycle/SKILL.md.tmpl", "templates/agents/plan-reviewer.md.tmpl"]
Representative: Update the writing-plans field vocabulary to require decision-slugged V4 items, plan-v2 Applying/Context omission semantics, slugged DoD bullets, and phase Advances/Completes ownership, while preserving qualifying prose and exactness rules.
Edge: Update task-query execution guidance to treat the generated scope notice, Phase close, and advanced/completed outcomes as phase-owner context only; reviewers must flag Context used to evade Applying, false completion ownership, and references that confuse historical ADR prose with current authority.
Post-check: Run `rg -n 'plan-v1|Applying|Context|Advances|Completes|decision:|dod:|phase-owner context' .awf/skills .awf/agents templates/skills templates/agents` and inspect every plan authoring/review/execution consumer; no current scaffold mandate may remain plan-v1, and every task-projection consumer must preserve phase ownership.

Before dispatch, require a clean checkout and `go test ./...` plus `./x check` to pass from Phase 5.

Update only authored sources. Writing plans must tell authors to use retained pending ADR slugs, stable Decision slugs for V4, and frozen `#N` only for pre-V4. Review must check every plan-level Decision assignment and DoD final owner substantively while recognizing the checks are advisory during Proposed. Execution must not infer that a task helper owns the phase close or outcomes. ADR lifecycle guidance must explain V4 markers and legacy ordinal freeze rules. Preserve chain checkpoints, report-only review, managed-memory, and model-selection contracts unchanged.

### Task 6.2: Update architecture, domain, workflow, and publication docs
Kind: batch
Paths: [".awf/domains/parts/adr-system/current-state.md", ".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/pitfalls.yaml", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/parts/agents-doc/commands.md", ".awf/parts/working-with-awf/commands.md"]
Representative: Document `internal/adr` source-block identity ownership, `internal/plan` source-set parsing and rendering ownership, and `internal/project` working/index/read composition without duplicating parser policy in command code.
Edge: Explain that task projections include phase-owned outcomes only as constraints, that assignment notes stop after plan implementation, and that historical Decision context never replaces current-state claim authority.
Post-check: Run `./x render && ./x check`, then inspect `docs/architecture.md`, `docs/domains/{adr-system,tooling}.md`, `docs/workflow.md`, `docs/working-with-awf.md`, `docs/pitfalls.md`, root native targets, and corresponding Sundial outputs; every generated file must derive from an authored source and no unresolved value token may appear.

Update documentation that explicitly describes the changed formats and package model. Add a pitfall only if needed to make the task-versus-phase ownership warning durable; do not manufacture one when the workflow and projection prose suffice. Keep command grammar unchanged and document plan-v1 as compatibility behavior, not the current scaffold.

### Task 6.3: Apply the phase-ownership operation and complete rendered parity
Latitude: exact
Paths: ["docs/decisions/task-scoped-plan-decision-context-and-phase-outcomes.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "internal/project/phase_transaction_ownership_test.go", "internal/project/plan_detail_modes_test.go", "internal/project/spine_test.go", "internal/project/target_test.go", "internal/project/output_plan_test.go"]

Append one Applied event for operation 19 only: update `rendering/workflow-skill-templates:phase-transaction-ownership`. Update the claim so a fresh task owner may consume the bounded plan-v2 closure but the generated scope notice, Phase close, Advances, and Completes remain phase-owner context and never transfer commit, review, checkpoint, handoff, or helper authority. Preserve its Origin and existing proof unit, append this ADR once to Revised-by, and strengthen the existing parity test.

Land the production workflow/template changes relevant to operation 20 as reviewed behavior, but leave `rendering/workflow-skill-templates:plan-task-detail-modes` unchanged and Remaining in the ADR for the terminal-review-owned final batch. The terminal transaction will update that claim and its existing proof marker after review confirms the rendered vocabulary. Run `./x render` and inspect root and Sundial target parity, publication safety with empty variables, generated ADR/plan templates, and absence of hand-edited outputs.

### Phase close

Run `gofmt -w` on any changed Go tests, `go test ./...`, `./x render`, `./x check`, `git diff --check`, and the focused template/parity tests named by the existing proof markers. Stage authored sources, operation 19, proof changes, docs, changelog, and every generated output selected by render. Require `./awf check staged` and `./x gate`, then commit:

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

The terminal workflow is not another implementation phase. After Phase 6, invoke `awf-reviewing-impl` over every commit after the settled plan-review baseline. Resolve mechanical and reasoned findings in new green commits and stop for any user-decision finding. Once terminal review establishes every Definition-of-done outcome, apply the final declaration-ordered operation `rendering/workflow-skill-templates:plan-task-detail-modes`: update its claim to enumerate plan-v2 Applying/Context, slugged DoD, Advances/Completes, omission-not-empty arrays, reviewer coverage, and task scope safety; preserve its Origin and existing proof marker and append this ADR once to Revised-by. In the same terminal transaction append the final Applied event and Implemented status event with the canonical digest, change this plan to `status: Implemented`, record deviations here, run `./x render`, and require `./awf check staged` and `./x gate` before committing `docs(plans): complete task-scoped plan context`.

Then follow the governed managed-worktree integration path: merge current `main` into this worktree, resolve or abort conflicts, run `./awf adr number task-scoped-plan-decision-context-and-phase-outcomes`, render and gate the deterministic numbering transaction, integrate through the effort command, renew implementation review if integration introduces divergence, and remove managed topology only after the integrated history is settled. Run `awf-retrospective` last. Numbering must retain this plan's slug link.

Generation 33 is intentionally byte-preserving for historical ADRs. It activates only the V4 parser/scaffold contract and lock schema; it must not retrofit Decision markers into V1-V3 records. Likewise, plan-v2 is selected only by its intrinsic marker and future scaffold, never by date or filename.
