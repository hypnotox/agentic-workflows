---
date: 2026-08-02
adrs:
  - 0209
  - 0210
  - parsed-plan-artifacts-and-executable-projections
status: Proposed
---
# Plan: Implement Plan Authoring Latitude and Parsed Projections

## Goal

Implement [ADR-0209](../decisions/0209-sanction-authoring-latitude-in-plans.md), [ADR-0210](../decisions/0210-corrective-re-application-of-an-applied-state-operation.md), and [ADR-parsed-plan-artifacts-and-executable-projections](../decisions/parsed-plan-artifacts-and-executable-projections.md): invert plan-detail guidance toward qualifying prose, add corrective ADR operation re-application, and make new plans parsed artifacts with executable phase and task projections.

This plan does not retrofit historical plans to `plan-v1`, add conditional tasks, allow re-applying removes or terminal ADRs, add fuzzy plan lookup, execute authored post-checks during validation, or change historical ADR bytes.

## Architecture summary

`internal/plan` becomes the single owner of a typed legacy-or-`plan-v1` model, structural validation, exact plan and selector resolution, and Markdown projection rendering. `internal/project` parses the plan directory once per check operation, converts typed validation failures to stable drift, resolves the configured plans directory for reads, and threads the same parsed set to commit-scope advisories. `internal/clispec` and `cmd/awf` add only the gated `read plan` command grammar, argument handling, output routing, and error-to-exit mapping.

`internal/adr` adds `Reapplied` to the shared V2/V3 history model and retains event kind plus occurrence identity in application batches while progress continues to count each declaration once. `internal/currentstate` consumes each corrective occurrence, validates corrective update and add mutations, and folds merge aggregates by their observable endpoint without inventing intermediate claim bytes.

Authored templates, convention parts, current-state claims, docs, and adopter examples move through `.awf/` sources and `./x render`; rendered files are never edited by hand. ADR-0209 applies before the parsed-plan ADR so their two updates to `plans-template-taxonomy` record the approved provenance order. Each ADR's State changes land with its matching production behavior and proof in one independently green transaction.

## File structure

- **Created:** `internal/plan/structure.go`, `internal/plan/structure_test.go`, `internal/plan/projection.go`, `internal/plan/projection_test.go`, `internal/project/plan_read.go`, `internal/project/plan_read_test.go`, `cmd/awf/read.go`, and `cmd/awf/read_test.go`.
- **Modified:** `internal/plan/{plan.go,plan_test.go}`; `internal/project/{check.go,check_test.go,golden_test.go,plan_detail_modes_test.go,phase_transaction_ownership_test.go,spine_test.go,target_test.go,output_plan_test.go}`; `internal/adr/{history.go,application.go,format.go,format_test.go,corpus_test.go}`; `internal/currentstate/{check.go,check_test.go,transition.go,transition_test.go,aggregate_test.go}`; `internal/clispec/{clispec.go,clispec_test.go}`; `cmd/awf/{check.go,check_test.go,dispatch.go,main_test.go,help_test.go,gate_test.go,new_test.go}`; `templates/{plans-template/template.md.tmpl,plans-readme/README.md.tmpl,skills/writing-plans/SKILL.md.tmpl,skills/reviewing-plan/SKILL.md.tmpl,skills/reviewing-plan-resync/SKILL.md.tmpl,skills/executing-plans/SKILL.md.tmpl,skills/subagent-driven-development/SKILL.md.tmpl,skills/adr-lifecycle/SKILL.md.tmpl,agents/plan-reviewer.md.tmpl,adr-readme/README.md.tmpl,adr-template/template.md.tmpl,docs/workflow.md.tmpl,docs/working-with-awf.md.tmpl}`; `.awf/agents/plan-reviewer.yaml`; `.awf/skills/parts/writing-plans/conventions-tasks.md`; `.awf/parts/{adr-readme/index.md,adr-template/frontmatter.md,working-with-awf/commands.md,working-with-awf/config-and-overrides.md,agents-doc/commands.md}`; `.awf/docs/pitfalls.yaml`; `.awf/docs/parts/architecture/{components,data-flow}.md`; `.awf/topics/parts/{adr-system/adr-lifecycle/current-state.md,adr-system/plan-artifacts/current-state.md,invariants/current-state-authority/current-state.md,rendering/workflow-skill-templates/current-state.md,rendering/pi-workflows/current-state.md,tooling/cli/current-state.md}`; the three linked ADRs; this plan; `README.md`; and `changelog/CHANGELOG.md`.
- **Rendered and example outputs:** `AGENTS.md`, `.pi/skills/**`, `.pi/agents/plan-reviewer.md`, `docs/{plans/README.md,plans/template.md,decisions/README.md,decisions/template.md,pitfalls.md,working-with-awf.md}`, the affected generated topic/domain docs, `docs/decisions/INDEX.md`, `.awf/awf.lock`, and the corresponding enabled-target and docs outputs plus lock under `examples/sundial/`. `./x render` determines the exact changed subset; stage every generated change caused by the authored inputs.
- **Deleted:** none. Historical plan files remain on the legacy path and historical ADRs remain unchanged.

## Exact current-state claim outcomes

These contract-bearing claim bodies are the required transaction endpoints. Numbering may mechanically replace the pending ADR identity with its assigned number; no other prose or metadata variation is permitted unless implementation discovers contradictory repository authority and stops for an ADR amendment.

**ADR-0209 update `rendering/workflow-skill-templates:plan-task-detail-modes`:**

```markdown
### `invariant: plan-task-detail-modes`

The rendered plan-authoring skill, plan reviewer, implementation-plans README, and plan template use qualifying implementation-ready instructions as the default task-content form; require `Latitude: exact` for machine-consumed configuration and manifests, contract-bearing declarations, fixtures, golden output, commands, mechanical replacements, required literal prose, and batch representative and edge transformations; and permit that marker voluntarily elsewhere. They define contiguous task fields for exactness, spikes, batches, affected paths, and deterministic post-checks; require `Paths:` whenever scope is ambiguous, always including a batch; require `Post-check:` for every batch and every glob or pathspec scope; preserve the no-placeholder boundary for implementation tasks; forbid conditional and optional tasks; require one coherent green transaction and an inline or subagent-driven owner per phase; and keep any helper partition exhaustive, path-disjoint, shared-file-safe, and command-confined. A spike is question-only, records its answer in Notes, cannot own a phase, and sequences dependent work into a later phase. Every surface renders coherently with empty variables.
Origin: ADR-0148
Revised-by: ADR-0157, ADR-0166, ADR-0209
Backing: test
```

**ADR-0209 interim update `adr-system/plan-artifacts:plans-template-taxonomy`:**

```markdown
### `invariant: plans-template-taxonomy`

The rendered plans template at docs/plans/template.md carries the date, adrs, and status frontmatter block and the plan section taxonomy: the # Plan: title, Goal and Architecture summary, at least one phase, optional Verification, and Notes that is required when any task is a spike and optional otherwise. File structure is not a plan-level section; affected paths belong to tasks.
Origin: ADR-0098
Revised-by: ADR-0209
Backing: test
```

**ADR-0210 add `adr-system/adr-lifecycle:corrective-reapplication`:**

```markdown
### `invariant: corrective-reapplication`

A current-state-v2 or current-state-v3 ADR in Implementing may append any number of `Reapplied; operations:` events for an add or update operation already named by an earlier Applied event. Each event is declaration-ordered, unique within the event, retained as its own ordered application occurrence, and reconciles one further material authored correction while operation progress continues to count the declaration once. A re-applied update preserves Origin and its existing canonical Revised-by entry; a re-applied add preserves its Origin naming the ADR and leaves Revised-by byte-identical. Remove operations, events before the first Applied occurrence, events outside Implementing, and events between the final Applied event and Implemented are refused.
Origin: ADR-0210
Backing: test
```

**ADR-0210 update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`:**

```markdown
### `invariant: adr-status-enum-and-matrix`

Every governed ADR is routed by its intrinsic declared format: V1 retains its four statuses and five legal edges, while V2 and V3 recognize Proposed, Accepted, Implementing, Implemented, and Abandoned, recognize status, Applied, Reapplied, and Amended history events, and accept only the format-specific status, history-event, digest-chain, application-cardinality, and corrective-reapplication transitions. A numberless record is valid only when it declares the running binary's current authoring format and satisfies that format's pending-identity rules.
Origin: ADR-0135
Revised-by: ADR-0143, ADR-0188, ADR-0202, ADR-0206, ADR-0210
Backing: test
```

**ADR-0210 update `invariants/current-state-authority:update-requires-substance`:**

```markdown
### `invariant: update-requires-substance`

An update preserves Origin and prior revision history, carries its ADR once at its canonical ascending position, and changes a canonical claim field other than formatting or provenance alone. The once rule is satisfied by adding the ADR for its first application and by preserving that existing entry for a corrective re-application. Across a merge, where intermediate claim states exist only in authored commits and not in either compared universe, substance is evaluated on the net operation-chain endpoint; every authored application and re-application proves its own materiality.
Origin: ADR-0135
Revised-by: ADR-0182, ADR-0191, ADR-0210
Backing: unbacked
Verify: Staged fixtures with Origin edits, revision deletion, duplication, or reordering, whitespace-only, provenance-only, first substantive update, and repeated substantive correction accept only the prefix-preserving materially changed cases with one canonical ADR entry.
```

**ADR-0210 update `invariants/current-state-authority:merge-transition-ordered-aggregate`:**

```markdown
### `invariant: merge-transition-ordered-aggregate`

A merge transition is validated as an ordered aggregate rather than one authoring step: application and re-application batches remain distinct in ascending ADR-identity and intra-ADR history order; a claim's operations across the pair form a legal ordered chain of at most one leading add, any number of updates, at most one remove, and after the remove any number of dominated updates; and appended Status history preserves the prior history as an exact prefix. Repeated updates from one ADR contribute that updater once and require a material endpoint; repeated adds by their originating ADR fold into the chain's first absent-to-present net add; a canceling update endpoint is refused. Per-occurrence materiality is proven by each authored commit, while aggregate validation checks the observable ordered net effect without inventing intermediate claim bytes. A non-merge transition keeps the stricter per-step contract of one new batch per ADR, one operation occurrence per claim, and the fixed status-event shape. A newly introduced ADR in an older intrinsic format is provisional at the staged boundary that lacks merge-parent and message evidence; every other derivable transition check remains blocking, and definitive admission requires exact incoming-parent qualification at commit-msg.
Origin: ADR-0182
Revised-by: ADR-0191, ADR-0206, ADR-0210
Backing: test
```

**Pending parsed-plan update `adr-system/plan-artifacts:plan-frontmatter-validated`:**

```markdown
### `invariant: plan-frontmatter-validated`

awf check fails on present-but-malformed plan frontmatter. Exact `format: plan-v1` selects structured parsing; marker absence selects legacy parsing; and an empty, unknown, duplicate, or malformed format is a frontmatter error rather than a legacy plan. Both paths retain the Proposed and Implemented status enum.
Origin: ADR-0098
Revised-by: ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan final update `adr-system/plan-artifacts:plans-template-taxonomy`:**

```markdown
### `invariant: plans-template-taxonomy`

The rendered plans template emits `format: plan-v1`, date, adrs, and status frontmatter; `# Plan:`; nonempty Goal and Architecture summary; sequential heading-identified phases and tasks with one execution mode and one final Phase close per phase; required Definition of done plain bullets; and optional Notes. File structure, Verification, and task checkboxes are not plan-v1 sections or task declarations. Marker-absent historical plans remain on the legacy taxonomy.
Origin: ADR-0098
Revised-by: ADR-0209, ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan add `adr-system/plan-artifacts:plan-v1-structure-validated`:**

```markdown
### `invariant: plan-v1-structure-validated`

A `plan-v1` document has nonempty Goal and Architecture summary sections, one or more sequential `## Phase P:` headings, one exact execution-mode declaration per phase, one or more sequential `### Task P.T:` headings followed by exactly one final `### Phase close`, a required Definition of done with one or more plain bullets, and optional Notes. Task fields are contiguous recognized `<Field>: <value>` lines directly below the heading; duplicate or unknown fields fail. Spike and batch relationships, JSON-array Paths entries, literal/glob/pathspec confinement, post-check triggers, phase-close placement, and its single commit fence are mechanically enforced. Ambiguous scope, contract-bearing exactness, baseline substance, and post-check execution remain reviewer or executor judgments. Marker-absent plans skip these structural rules.
Origin: ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan add `adr-system/plan-artifacts:plan-executable-projection`:**

```markdown
### `invariant: plan-executable-projection`

internal/plan owns exact filename-or-stem resolution, canonical positive `P` and `P.T` selectors, and projection rendering from the typed plan model. A phase projection contains every task in that phase; a task projection contains only that task; and both contain frontmatter, title, Goal, Architecture summary, owning phase and execution mode, Phase close, Definition of done, and Notes when present in source order. Errors list available exact names or selectors as appropriate. Projection includes no other phase, reparses no Markdown outside the model owner, and never mutates source bytes.
Origin: ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan add `tooling/cli:plan-read-command`:**

```markdown
### `invariant: plan-read-command`

The gated `awf read plan <plan> <P[.T]>` command resolves only an exact plan filename or stem under the configured plans directory and only canonical positive numeric phase or task selectors. Failures list available exact values. Success writes the internal/plan-rendered executable closure unchanged: frontmatter, title, Goal, Architecture summary, owning phase and execution mode, selected phase or task, Phase close, Definition of done, and Notes when present; it neither includes other phases nor mutates the source.
Origin: ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan update `rendering/workflow-skill-templates:phase-transaction-ownership`:**

```markdown
### `invariant: phase-transaction-ownership`

A rendered plan phase is one independently green coherent implementation transaction with an explicit per-phase inline or subagent-driven owner; heading-identified tasks are ordered steps rather than completion state or default dispatch, review, checkpoint, or commit boundaries. A fresh phase or task owner may consume `awf read plan`'s executable closure without changing ownership. One commit-capable implementer owns a complete subagent-driven phase from a known green baseline through staged check, gate, and Phase close commit, while the parent owns inline integration, sequential commit-disabled batch helpers, report-only review settlement, phase checkpointing, and explicit dirty-state recovery without blind task-level succession.
Origin: ADR-0166
Revised-by: ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`:**

```markdown
### `invariant: memory-checkpoint-chain-coverage`

Checkpoint guidance renders the four-step digest: it creates no effort for a minimal simple fix or merely because a boundary was reached, and once the outcome is concrete and non-minimal it validates exactly one immutable slug and `.awf/efforts/<slug>/memory.md`, confirms `Effort: <slug>`, carries continuation in the effort's managed worktree when one exists with the owned path spelled primary-root-relative, updates phase, next action, time, and handoff log, and appends any unrecorded settled decision and observation since the last boundary, in one writer-owned batch, and points at the workflow doc's working-memory section for authority precedence, the one-writer contract, the skeleton, and the full protocol. Routine implementation checkpoints remain after the phase-closing commit and settled report-only review, never after heading-identified tasks or helper returns; an executable plan projection does not create a checkpoint boundary; Pi handoff carries the exact owned path alone.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0186, ADR-0189, ADR-0197, ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

**Pending parsed-plan update `rendering/pi-workflows:pi-session-handoff-workflow`:**

```markdown
### `invariant: pi-session-handoff-workflow`

Pi checkpoint guidance invokes handoff alone after persistence at a settled phase boundary, carrying the same effort slug and exact owned memory path for non-minimal work. A fresh phase or task owner may consume `awf read plan`'s executable closure, but projection never creates a handoff boundary. Pi never creates standalone memory, requires selection or telemetry lifecycle state, adopts a checkpoint, or treats heading-identified tasks and helper returns as routine handoff boundaries; repository authority remains primary and report-only children do not edit memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-parsed-plan-artifacts-and-executable-projections
Backing: test
```

## Pre-implementation ADR acceptance transaction

After this plan and its resync review settle, transition ADR-0209, ADR-0210, and the pending parsed-plan ADR from Proposed to Accepted in one docs-only transaction, appending each stamped Accepted status event without applying an operation or changing a claim. Run `./x render`; stage the three lifecycle changes and every generated output caused by them, including `docs/decisions/INDEX.md`, `.awf/awf.lock`, and any adopter lock/output selected by render. Require `git diff --name-only` to contain only those lifecycle and derived render paths, then require `./awf check --staged` and `./x gate`, and commit:

```commit
docs(adr): accept plan authoring slate
```

Each implementation phase below then uses the intentional direct `Accepted -> Implemented` path. Its terminal status event, production behavior, claim mutations, and proof changes form one atomic transaction; the direct path implicitly applies every declared operation and carries no Applied event. This avoids an illegal Implementing state with no Remaining operation.

## Phase 1: Sanction plan-authoring latitude

**Execution mode: inline.** This phase applies ADR-0209's two State changes in declaration order and leaves the current legacy plan parser valid. It deliberately precedes the parsed-plan transaction so `plans-template-taxonomy` records ADR-0209 before the pending parsed-plan ADR.

- [ ] **Task 1.1: Rewrite and strengthen the authoring-surface parity tests first.** In `internal/project/plan_detail_modes_test.go`, retain the existing proof marker for `plan-task-detail-modes` and make its clause table require, across the standard template and this repository's rendered writing skill, plan reviewer, plans README, and plan template: qualifying instructions as the default; the unchanged closed exactness categories; `Latitude: exact`; `Kind: spike` with `Question:` and no implementation body; Notes as the spike answer target; `Kind: batch`; `Paths:` whenever the affected set is ambiguous, with every batch as the always-true case; `Post-check:` for every batch and every glob/pathspec scope; the unchanged no-placeholder rule for implementation tasks; the no-conditional-task rule; field placement immediately under a task declaration; and coherent missingkey=zero output with no no-value token. In `internal/project/golden_test.go`, replace the assertion for three narrative headings with Goal and Architecture summary and assert `File structure` is absent while the legacy optional Verification and conditional Notes tails remain. Run `go test ./internal/project -run 'TestPlanTaskDetailModes|TestGolden'`; the new assertions must fail before source edits and pass afterward.

- [ ] **Task 1.2: Invert detail guidance and add the approved field vocabulary at every authored source.** Update `templates/skills/writing-plans/SKILL.md.tmpl`, `.awf/skills/parts/writing-plans/conventions-tasks.md`, `templates/agents/plan-reviewer.md.tmpl`, `.awf/agents/plan-reviewer.yaml`, `templates/plans-readme/README.md.tmpl`, and `templates/plans-template/template.md.tmpl`. State qualifying implementation-ready pseudocode as the normal task form, then state the closed exact categories and require `Latitude: exact` for them. Define the exact `Kind`, `Latitude`, `Question`, `Paths`, `Representative`, `Edge`, and `Post-check` spellings and their placement directly below the task declaration. Require `Paths:` whenever the affected set is ambiguous, including ambiguous non-batch tasks, and state that every batch is necessarily ambiguous. Define a spike as question-only, Notes-recorded, non-phase-owning investigation whose dependants start in a later phase. Replace the batch affected-site-set wording with `Paths:` and require `Post-check:` for a batch or any `glob:` or `pathspec:` entry. Keep conditional and optional tasks forbidden and preserve publication-safe generic prose.

- [ ] **Task 1.3: Retire the whole-plan File structure section across its exhaustive authored surfaces.** Remove the section from `templates/plans-template/template.md.tmpl`; change the canonical-header text and plan-writing procedure in `templates/skills/writing-plans/SKILL.md.tmpl`; change the plans structure prose in `templates/plans-readme/README.md.tmpl`; and change the plan reviewer's `section-taxonomy` focus in both `templates/agents/plan-reviewer.md.tmpl` and `.awf/agents/plan-reviewer.yaml`. In `templates/skills/writing-plans/SKILL.md.tmpl`, `templates/skills/reviewing-plan/SKILL.md.tmpl`, and `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, replace collection from the retired header with collection of every task-level `Paths:` entry plus exact repository paths named in task titles and bodies; deduplicate the resolved set before `awf context`, and do not infer a path from vague prose. Add parity assertions to `internal/project/plan_detail_modes_test.go` for all three collection instructions. Do not change the catalog `Sections` list or `adr-singleton-section-parity`, because File structure is inside the existing header marker rather than a singleton section. Run `rg -n 'three canonical|Goal, Architecture summary, and File structure|## File structure|file-structure header' templates/{plans-template,plans-readme,skills/writing-plans,skills/reviewing-plan,skills/reviewing-plan-resync,agents/plan-reviewer} .awf/agents/plan-reviewer.yaml` and require no stale structural mandate or collection instruction.

- [ ] **Task 1.4: Complete ADR-0209 through the direct path and land its exact claims.** Transition accepted ADR-0209 directly to Implemented with the current canonical stamp; append no Applied event because this direct transaction implicitly applies both declared operations. Land the two verbatim ADR-0209 claim outcomes above in their owning `.awf/topics/parts/**/current-state.md` files and retain each existing proof marker.

- [ ] **Task 1.5: Render and verify the complete alpha sweep.** Add an Unreleased changelog entry for the authoring forms. Run `./x render && ./x check`; both must finish clean. Inspect `git diff --name-only` and require every generated change to derive from a listed authored source, including all enabled target renders and sundial outputs. Run `go test ./internal/project`, `git diff --check`, and `rg -n '<no value>|TBD|implement later' docs/plans/template.md .pi/skills/awf-writing-plans/SKILL.md .pi/agents/plan-reviewer.md`; the first two commands must pass and the grep may show only the intentional no-placeholder prohibition, never unresolved template output.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete ADR-0209 implementation transaction. Require `./awf check --staged` and `./x gate` to pass, then commit:

```commit
feat(plans): sanction task authoring latitude
```

## Phase 2: Add corrective re-application semantics

**Execution mode: inline.** This phase implements ADR-0210 as one coherent feature because its first added claim describes the complete author-visible route; splitting parser admission from mutation reconciliation would make that claim or the user guidance temporarily false. It applies all four ADR-0210 State changes in declaration order.

- [ ] **Task 2.1: Add failing history and progress tests for Reapplied.** In `internal/adr/format_test.go` and `internal/adr/corpus_test.go`, cover the exact `Reapplied; operations:` grammar for V2 and V3, declaration order and within-event uniqueness, prior-Applied requirement, repeated Reapplied events for one add or update, refusal of remove, refusal outside Implementing, refusal before the first Applied event, and refusal between the final Applied event and Implemented. Prove a second Applied remains an error, every Reapplied occurrence keeps its own event/batch identity, and `OperationProgress` reports one applied declaration with no duplicate progress. Add `// invariant: adr-system/adr-lifecycle:corrective-reapplication (TestCorrectiveReapplication)` immediately above `TestCorrectiveReapplication`. Run `go test ./internal/adr -run 'TestCorrectiveReapplication|TestOperationProgressReapplied'`; it must fail before production edits and pass afterward.

- [ ] **Task 2.2: Extend the shared V2/V3 event and batch model without changing declarations.** In `internal/adr/history.go`, add `HistoryReapplied`, parse the exact new event grammar through the same qualified-operation parser as Applied, and retain event kind. In `internal/adr/application.go`, extend `ApplicationBatch` with event kind and stable occurrence identity (history index), and extend `AppliedOperation` only as needed for consumers to distinguish the declaration's first Applied occurrence from corrective occurrences. `ApplicationBatches` must emit both kinds in history order. `OperationProgress` must key declaration identity independently from occurrence identity: exactly one Applied occurrence moves the declaration to Applied, any number of later Reapplied occurrences contribute no progress, and a second Applied remains a duplicate-application error. Do not permit a second declaration in `internal/adr/operations.go`.

- [ ] **Task 2.3: Enforce the governed history window and cardinality.** In `internal/adr/format.go`, update `HistoryTransitionValid`, `historiesEqual`, and `validateV2History` so one same-status Reapplied event is an authored Implementing transition; every Reapplied operation has an earlier Applied occurrence; add and update only are legal; repetition is legal; and Implemented adjacency remains final Applied followed immediately by the status event. Keep V1 and legacy behavior unchanged. Error text for an already-applied duplicate declaration must point to a Reapplied correction when the target was previously applied, while still naming follow-up ADR/amendment routes where re-application is unavailable.

- [ ] **Task 2.4: Add failing authored and aggregate mutation tests.** In `internal/currentstate/transition_test.go` and `aggregate_test.go`, cover corrective update with unchanged canonical Revised-by, corrective add with unchanged Origin naming the ADR and byte-identical Revised-by, material and non-material corrections, wrong Origin/provenance, remove refusal, two ordered corrections to one operation, authored one-batch atomicity, and merge aggregates containing repeated updates or adds from one ADR. Prove aggregate updates use one updater identity and require a material endpoint, canceling updates fail, and aggregate corrective adds fold to one absent-to-present net add without comparing unavailable intermediate bytes. Attach the existing `merge-transition-ordered-aggregate` proof marker to `TestAggregateCorrectiveReapplication`, which includes repeated corrective occurrences, and retain all other backing markers. Run `go test ./internal/currentstate -run 'TestAuthoredCorrectiveReapplication|TestAggregateCorrectiveReapplication'`; it must fail before Task 2.5 and pass afterward.

- [ ] **Task 2.5: Reconcile each corrective occurrence in current state.** In `internal/currentstate/check.go` and `transition.go`, thread event kind and occurrence identity from `ApplicationBatches` through `pairOps` and the ordered chain. In authored mode accept one new Reapplied batch and reconcile it against the immediate before/after claims. Route a re-applied update through the existing materiality and Origin checks while treating the existing canonical ADR entry in Revised-by as the required once-only provenance. Add a corrective-add branch requiring the claim on both sides, unchanged Origin naming the owning ADR, byte-identical Revised-by, and a material canonical change. In merge mode let repeated updates from the same ADR contribute one updater identity and fold repeated adds into the first net add; retain per-authored-commit materiality and refuse a non-material endpoint update. Keep remove absorption, dominated updates, numbering substitutions, and ordinary duplicate-target errors unchanged.

- [ ] **Task 2.6: Complete ADR-0210 and document the route.** Transition accepted ADR-0210 directly to Implemented with the current canonical stamp; append no Applied event because the transaction implicitly applies all four operations. Land the four exact ADR-0210 claim outcomes above. Update `templates/adr-readme/README.md.tmpl`, `.awf/parts/adr-readme/index.md`, `templates/adr-template/template.md.tmpl`, `.awf/parts/adr-template/frontmatter.md`, `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `.awf/parts/working-with-awf/commands.md`, and `.awf/docs/pitfalls.yaml` with route selection: amend an unapplied operation, Reapply an already-applied add/update while operations remain, otherwise use a follow-up ADR or remove-plus-add. Preserve generic rendering with empty variables.

- [ ] **Task 2.7: Render and verify beta end to end.** Add the behavior to the Unreleased changelog. Run `gofmt -w internal/adr/history.go internal/adr/application.go internal/adr/format.go internal/adr/format_test.go internal/adr/corpus_test.go internal/currentstate/check.go internal/currentstate/check_test.go internal/currentstate/transition.go internal/currentstate/transition_test.go internal/currentstate/aggregate_test.go`, `go test ./internal/adr ./internal/currentstate ./internal/project`, `./x render`, `./x check`, and `git diff --check`; all must pass. Verify `rg -n 'Reapplied' internal templates .awf docs/decisions/{README.md,template.md} docs/working-with-awf.md docs/pitfalls.md` shows parser, validators, authored sources, rendered guidance, and tests, and no production consumer reconstructs event kind from prose.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete ADR-0210 implementation and claim transaction. Require `./awf check --staged` and `./x gate` to pass, then commit:

```commit
feat(adr-system): permit corrective operation reapplication
```

## Phase 3: Parse, validate, and project plan-v1 artifacts

**Execution mode: inline.** This phase is one coherent activation transaction for the pending parsed-plan ADR. Parser, project consumer, projection, CLI command, scaffolded format, authoring surfaces, and all eight State changes land together so no dead production API, advertised-but-unusable format, or temporarily false claim crosses a commit boundary.

- [ ] **Task 3.1: Specify the typed plan-v1 grammar with failing package tests.** Create `internal/plan/structure_test.go` with table-driven fixtures for legacy marker absence and valid `format: plan-v1` documents, plus exact failures for empty/unknown/duplicate/malformed format; top-level order; nonempty Goal and Architecture summary; one-or-more sequential phases and tasks; execution-mode spelling; contiguous recognized fields; duplicate/unknown fields; spike and batch relationships; JSON `Paths`; literal, `glob:`, and `pathspec:` validation; phase-close placement and unique commit fence; required nonempty Definition of done bullets; optional Notes and spike-required Notes; and prohibited File structure, Verification, checkboxes, conditional/optional task spellings. Tests must assert typed error identity and stable category/path/detail fields rather than only substrings. Legacy fixtures must retain today's frontmatter, link, status, and commit-subject results without new structural errors.

- [ ] **Task 3.2: Build one typed parse in internal/plan.** Extend `internal/plan/plan.go` frontmatter with `Format` and retain exact source bytes. Create `structure.go` with documented typed values for sections, phases, tasks, fields, path entries, phase close, definition-of-done requirements, Notes, and structural diagnostics. Marker absence routes to the current legacy extraction; only exact `plan-v1` routes to the structural parser. Parse Markdown by line structure in one pass without adding a second Markdown dependency. Enforce positive sequential numeric identities, exact headings and field placement, the `internal/pathglob` grammar after `glob:`, and duplicate detection after authored spelling normalization. For every entry, reject an absolute path portion or any path component exactly `..`. For a pathspec payload, lex only its optional Git magic-prefix boundary: recognize Git's long `:(...)` form and short signature form, reject a missing terminator or an unrecognized/malformed prefix, and apply confinement to the suffix path portion without interpreting, normalizing, or executing the magic. Retain and pass the original complete pathspec payload byte-for-byte after validation. Keep ambiguity of Paths, exactness-category inference, baseline substance, and post-check execution as reviewer concerns.

- [ ] **Task 3.3: Parse once per project check and map typed diagnostics.** In `internal/project/check.go`, add documented `type CheckReport struct { Drift []manifest.Drift; Notes []string }` and `func (p *Project) CheckReport(ctx context.Context) (CheckReport, error)`. It creates one operation-scoped parsed plan set after project state opens, passes it to private blocking `checkPlans` and advisory `planCommitScopeNotes` helpers, and assembles both outputs without a cache or package global. Keep `Check` and `AdvisoryNotes` as compatibility projections over shared private helpers, but change `cmd/awf/check.go` to call `CheckReport` once for an ordinary `awf check`; add command regression coverage in `cmd/awf/check_test.go`. Map plan diagnostics to stable `manifest.Drift` findings while preserving legacy status, ADR-link, commit-subject, and scope-note behavior. Extend `internal/project/check_test.go` with frontmatter format coverage and add `// invariant: adr-system/plan-artifacts:plan-v1-structure-validated (TestPlanV1StructureValidated)` immediately above `TestPlanV1StructureValidated`, proving valid structure and representative failures. Add a source-structure assertion that `CheckReport` has one `plan.ParseDir` call and neither consumer reparses.

- [ ] **Task 3.4: Specify exact resolution and executable closure rendering.** Create `internal/plan/projection_test.go`. Cover exact `.md` filename and exact stem resolution beneath one supplied plans directory, refusal of traversal, absolute/outside paths, fuzzy/partial/title matches, ambiguous stem/filename collisions, and errors listing sorted available exact values. Cover `P` and `P.T`, positive canonical selectors, missing selectors listing available values, source-order phase and task rendering, and omission of every unselected phase/task. Assert every projection contains frontmatter, title, Goal, Architecture summary, owning phase and execution mode, selected tasks, Phase close, Definition of done, and Notes when present, with no source mutation. Use typed not-found, ambiguous, and invalid-selector errors and compare rendered bytes exactly in fixtures.

- [ ] **Task 3.5: Implement projection in the model owner and expose a narrow project seam.** Create `internal/plan/projection.go` with exact plan selection, selector parsing, and Markdown rendering from the typed model. Do not re-scan source headings during projection. Create `internal/project/plan_read.go` with documented `func (p *Project) ReadPlan(name, selector string) ([]byte, error)`: resolve `filepath.Join(p.Root, p.Cfg.DocsDir, "plans")`, call the internal/plan resolver and renderer, and return its typed errors unchanged. `cmd/awf/read.go` is its only production consumer; the project package owns no Markdown representation. Create `internal/project/plan_read_test.go` for configured docs-directory resolution and typed-error preservation. Add `// invariant: adr-system/plan-artifacts:plan-executable-projection (TestPlanExecutableProjection)` immediately above `TestPlanExecutableProjection`, whose exact-byte assertions include every executable-closure member.

- [ ] **Task 3.6: Add the gated `awf read plan` command.** In `internal/clispec/clispec.go`, add gated parent `read` with child `plan`, two required positionals, exact usage `awf read plan <plan> <P[.T]>`, and help describing exact filename/stem and numeric selectors. Update clispec tests and generated gated-command expectations. Create `cmd/awf/read.go`; validate arity, open through the ordinary gated project path, call the project seam, write returned bytes unchanged, and map typed errors without parsing Markdown or rendering plan content. Register the handler in `cmd/awf/dispatch.go`. Add `cmd/awf/read_test.go` plus help/registry/gate coverage for success, failures, stdout/stderr separation, and ahead-binary refusal. Add `// invariant: tooling/cli:plan-read-command (TestReadPlanCommand)` immediately above the end-to-end `TestReadPlanCommand` and retain single-source command-table proofs.

- [ ] **Task 3.7: Convert future scaffolds and all authoring surfaces to plan-v1.** Replace the default body of `templates/plans-template/template.md.tmpl` with the exact scaffold below, retaining the existing `awf:edit` marker comments immediately before their owning sections. The field-vocabulary prose inside Task 1.1 must spell `Kind`, `Latitude`, `Question`, `Paths`, `Representative`, `Edge`, and `Post-check` exactly and state their ADR-0209 relationships; no field line is emitted for the ordinary qualifying example task.

````markdown
---
format: plan-v1
date: YYYY-MM-DD
adrs: []
status: Proposed
---
# Plan: Title

## Goal

State the outcome and, in one line, its non-goals.

## Architecture summary

State the execution structure and dependency direction without repeating ADR rationale.

## Phase 1: <name>

**Execution mode: inline.**

### Task 1.1: <what>

Supply qualifying implementation-ready instructions. Immediately below a task heading, the recognized fields are `Kind`, `Latitude`, `Question`, `Paths`, `Representative`, `Edge`, and `Post-check`. Use `Latitude: exact` for a contract-bearing task; `Kind: spike` requires `Question`, no body, and an answer in Notes; `Kind: batch` requires JSON-array `Paths`, `Representative`, `Edge`, and `Post-check`; ambiguous scope requires `Paths`; and any batch, glob, or pathspec scope requires `Post-check`. Omit fields whose contracts do not apply.

### Phase close

Stage the complete transaction and create its one closing commit after the staged check and gate pass.

```commit
feat(scope): describe phase outcome
```

## Definition of done

- State at least one concrete observable whole-plan end condition.

## Notes

Record deviations, spike answers, follow-ups, and findings surfaced during implementation.
````

The outer plan must encode the inner `commit` fence without changing its bytes; use the repository's established fenced-example escaping in the authored template. Remove legacy Verification and checkbox forms from the new template. Update `templates/plans-readme/README.md.tmpl`, both writing-plan sources, both plan-reviewer sources, the relevant execution skill templates, `templates/docs/workflow.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`, and `.awf/parts/working-with-awf/config-and-overrides.md` to use the same grammar and treat headings as ordered instructions rather than completion state. Rendered root and sundial workflow and working-guide docs must carry no checkbox-task mandate. Update `internal/plan/plan_test.go` and `cmd/awf/new_test.go` so `awf new plan` proves the generated file is `plan-v1` and immediately parses cleanly. Strengthen `internal/project/{golden_test.go,plan_detail_modes_test.go,phase_transaction_ownership_test.go,spine_test.go,target_test.go,output_plan_test.go}` to pin the grammar, phase-close fence, required Definition of done, checkpoint-after-phase-close ordering, projection use for fresh phase/task owners, missingkey=zero coherence, and sundial parity.

- [ ] **Task 3.8: Document the command, legacy boundary, and package ownership through authored sources.** Update `.awf/parts/working-with-awf/commands.md`, `templates/docs/working-with-awf.md.tmpl`, `.awf/parts/agents-doc/commands.md`, `README.md`, and the Unreleased changelog with `awf read plan`, exact name/selector grammar, executable-closure membership, and marker-absent legacy behavior; render both root and sundial working guides. Update writing/execution/subagent-driven guidance so a fresh owner uses the projection when assigned a plan phase or task and still follows Notes and phase-close transaction semantics. Update `.awf/docs/parts/architecture/components.md` and `data-flow.md`: `internal/plan` owns typed parsing, validation, resolution, and projection rendering; `internal/project` orchestrates one operation-scoped parse and configured plans-directory access; `cmd/awf` owns arguments, output routing, and exit mapping only. Render and inspect `docs/architecture.md`. Do not prescribe projection as a substitute for reading the full plan during plan review or plan resync.

- [ ] **Task 3.9: Complete the parsed-plan ADR and land exact claim provenance.** Transition the accepted pending ADR directly to Implemented with the current canonical stamp; append no Applied event because this transaction implicitly applies all eight declared operations. Land all eight pending parsed-plan claim outcomes above verbatim in their owning current-state sources. Preserve existing proof markers and add exactly `TestPlanV1StructureValidated`, `TestPlanExecutableProjection`, and `TestReadPlanCommand` as the new proof names.

- [ ] **Task 3.10: Render and verify the complete gamma activation.** Run `gofmt -w internal/plan/plan.go internal/plan/plan_test.go internal/plan/structure.go internal/plan/structure_test.go internal/plan/projection.go internal/plan/projection_test.go internal/project/check.go internal/project/check_test.go internal/project/plan_read.go internal/project/plan_read_test.go internal/clispec/clispec.go internal/clispec/clispec_test.go cmd/awf/check.go cmd/awf/check_test.go cmd/awf/read.go cmd/awf/read_test.go cmd/awf/dispatch.go cmd/awf/main_test.go cmd/awf/help_test.go cmd/awf/gate_test.go cmd/awf/new_test.go`, `go test ./internal/plan ./internal/project ./internal/clispec ./cmd/awf`, `./x render`, `./x check`, and `git diff --check`; all must pass. Run `go test ./internal/plan -run 'TestPlanV1StructureValidated|TestPlanExecutableProjection' -count=1` and `go test ./cmd/awf -run TestReadPlanCommand -count=1`; the named fixture tests must scaffold through a copied plans directory, compare phase 1 and task 1.1 projections byte-for-byte, compare the source SHA-256 before and after both reads, and remove their `t.TempDir` fixtures automatically. Run `rg -n 'checkbox tasks|\- \[ \] \*\*Task|## Verification|## File structure' templates/{plans-template,plans-readme,skills/writing-plans,agents/plan-reviewer} .awf/skills/parts/writing-plans .awf/agents/plan-reviewer.yaml`; it must return no new-format mandate, though historical plans remain untouched.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete parsed-plan implementation and eight-operation claim transaction. Require `./awf check --staged` and `./x gate` to pass, then commit:

```commit
feat(plans): parse and project executable plan sections
```

## Governed review and integration tail

This tail is workflow, not an implementation phase or transaction. After Phase 3 closes green, merge current `main` into the managed worktree and resolve or abort any conflict. Run `./awf adr number parsed-plan-artifacts-and-executable-projections`, then `./x render`; stage only the deterministic numbering and generated index/output changes, require `./awf check --staged` and `./x gate`, and commit `docs(adr): number parsed plan artifacts` with the printed slug-to-number mapping in the body. Numbering must not rewrite this plan's slug link.

Invoke `awf-reviewing-impl` over every implementation and numbering commit after this plan's review-settled baseline. Resolve mechanical and reasoned findings in new green commits and stop on a user-decision finding. In the clean primary checkout, integrate the effort with `./awf effort integrate sanction-authoring-latitude-and-gate-plan-structure`; accept only its documented terminal outcomes. If integration creates a divergent staged merge, run the staged check and gate, commit it, and renew implementation review over the combined target history. Remove managed topology only after terminal review and integration settle.

## Phase 4: Freeze the plan after completion is established

**Execution mode: inline.** This deferred phase begins only after terminal implementation review and integration explicitly deem every planned outcome complete. If review finds incomplete work, this phase does not begin: the plan stays Proposed and mutable while the missing work is added, reviewed, and settled. The three ADRs are already Implemented by their direct implementation transactions.

- [ ] **Task 4.1: Record established completion without over-restricting the plan.** Confirm every Definition of done and Verification item against repository truth and record actual deviations or follow-ups under Notes. Only after that positive completion determination, change this plan's `status:` to `Implemented`. If any item is not complete, leave status Proposed and return to implementation rather than freezing an incomplete execution record. Run `./x render`; `docs/decisions/INDEX.md` must already retain all three terminal ADR identities.

- [ ] **Phase-close: stage, check, gate, and commit.** Run `git diff --check`, `./x render`, and `./x check`; stage only the plan status/Notes and generated output. Require `./awf check --staged` and `./x gate`, then commit:

```commit
docs(plans): complete parsed plan rollout
```

After the commit, require the effort worktree and branch to be absent through the governed effort removal command before running `awf-retrospective` as the final workflow step.

## Verification

- `go test ./...`, `./x render`, `./x check`, and `./x gate` finish successfully; statement coverage, dead-code, lint, cross-compile, generated Pi, prose, memory, and supply-chain checks remain green.
- Every direct implementation transaction passes `./awf check --staged` only when its Implemented event, implicit operation effects, exact current-state mutations, proof markers, production behavior, docs, and rendered outputs are staged together.
- New scaffolds carry `format: plan-v1` and parse into one typed model; marker-absent historical plans retain their prior check results and are not rewritten.
- Structural validation rejects malformed numbering, fields, Paths, spike/batch relationships, phase closes, and Definition of done, while leaving ambiguity, exactness-category judgment, baseline substance, and post-check execution to review or execution.
- `awf read plan` accepts only exact filename/stem and canonical numeric selectors, returns the full executable closure in source order, lists available exact values on failure, and never mutates the plan.
- A V2 or V3 Implementing ADR can Reapply an already-Applied add or update any number of times while an operation remains; progress counts the declaration once, provenance is unchanged after its first write, each authored correction is material and atomic, removes and terminal corrections remain refused, and aggregate validation checks the ordered net endpoint.
- `rg -n '## File structure|## Verification|\- \[ \] \*\*Task' docs/plans/template.md` returns no output, while the same search under historical dated plans is permitted.
- `git diff --exit-code` is clean after the final commit, all three ADRs are Implemented, the plan is Implemented only after terminal review has deemed every planned outcome complete, the pending ADR has been numbered without rewriting the plan link, managed topology is absent, and retrospective is last.

## Notes

- Phase 2 preserves ADR-0206 in the `adr-status-enum-and-matrix` Revised-by endpoint. The reviewed endpoint omitted that already-applied provenance entry; repository authority and the user-approved execution correction require retaining it before appending ADR-0210.
- Phase 2 needed no byte change in `internal/currentstate/check.go`: that static authority path deliberately projects only the first Applied occurrence through `OperationProgress`, while corrective event kind and occurrence identity belong to the pair-transition boundary in `transition.go`. Counting Reapplied there would violate the progress-once contract.
- Phase 2 needed no byte change in `.awf/parts/adr-readme/index.md` or `.awf/parts/adr-template/frontmatter.md`. Those overrides own only INDEX guidance and scaffold frontmatter, neither of which carries operation-route prose; the applicable lifecycle and template-body sources changed and rendered instead. Adding Reapplied prose to either override would pollute an unrelated section.

The gamma activation is intentionally one large transaction: its eight operations interlock parser admission, scaffold output, projection reachability, CLI documentation, and current-state truth, so an apparently smaller split would cross a commit with either dead code, an advertised unusable format, or an inaccurate claim.
