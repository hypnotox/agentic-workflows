---
date: 2026-07-26
adrs: [161]
status: Proposed
---
# Plan: Agent-adopted workflow state and derived detours

## Goal

Implement ADR-0161 so Pi can adopt explicit external checkpoints, continue an already-open
governed phase, isolate material deviations as derived detours, and return only bounded
agent-facing metrics and doctor summaries. This plan does not rewrite resident protocol-2.0 data,
add a TUI or pagination, or permit arbitrary phase jumps within one effort.

## Architecture summary

Six independently green commits preserve reader-before-writer publication and partition all twelve
ADR operations once, consecutively, in declaration order. The first commit publishes the complete
2.1 descriptor and compatible Go/TypeScript readers while all new-kind write entry points remain
disabled. The second enables lifecycle writes and entry/continuation routing. Adoption, detour
return, and compact queries then land as separate test-first slices. The final commit applies the
remaining Pi presentation and documentation operations, runs the pinned Pi 0.81.1 runtime fixture,
and freezes the ADR and plan.

## File structure

- **Created:** no production paths and no new test files; adoption and detour coverage remains in
  `tools/pi-extension-test/tests/workflow.test.ts`, compact-query coverage in
  `tools/pi-extension-test/tests/dashboard.test.ts`.
- **Authored code and template inputs modified:** production and test `.go` files under
  `internal/telemetry/` plus `internal/telemetry/protocol.json`; `cmd/awf/metrics.go` and
  `cmd/awf/metrics_test.go`; `internal/catalog/catalog.go`, `internal/catalog/workflow.go`, `internal/catalog/standard.go`,
  `internal/catalog/workflow_test.go`; `internal/project/render.go`,
  `internal/project/pi_workflow_render_test.go`, and `internal/project/target_test.go`;
  `templates/pi/awf-dashboard/index.ts.tmpl`, `templates/pi/awf-workflow/SKILL.md.tmpl`,
  `templates/docs/workflow.md.tmpl`; `tools/pi-extension-test/fixtures/fake-awf.mjs`;
  `.awf/parts/workflow/chain.md`,
  `.awf/parts/agents-doc/working-memory.md`, `.awf/parts/working-with-awf/commands.md`,
  `.awf/parts/working-with-awf/config-and-overrides.md`,
  `.awf/docs/parts/architecture/overview.md`, `.awf/docs/parts/architecture/data-flow.md`, and
  `.awf/docs/parts/testing/layout.md`; the exact `.awf/topics/parts/**` files named below.
- **Generated outputs modified only by `./x render`:** `.pi/extensions/awf-dashboard/index.ts`,
  `.pi/extensions/awf-dashboard/protocol.ts`, `.pi/skills/awf-workflow/SKILL.md`, hidden Pi
  workflow bodies, `docs/architecture.md`, `docs/testing.md`, `docs/workflow.md`, rendered topic
  and domain docs, `AGENTS.md`, `docs/decisions/INDEX.md`, `.awf/awf.lock`, and the three Sundial
  outputs `examples/sundial/.awf/awf.lock`,
  `examples/sundial/.pi/extensions/awf-dashboard/index.ts`, and
  `examples/sundial/.pi/extensions/awf-dashboard/protocol.ts`.
- **Deleted:** none.

## Phase 1: Publish protocol-2.1 readers with new writes disabled

- [ ] **Task 1.1: Write failing compatibility and closed-schema tests.** In
  `internal/telemetry/reader_test.go`, prove one unsupported required record suppresses the whole
  effort instead of leaving a partial lifecycle projection. In `aggregate_test.go`,
  `diagnostics_test.go`, and `retention_test.go`, prove the suppressed effort contributes no
  metrics, findings, cohort sample, or retention candidate and emits one bounded compatibility
  notice. In `protocol_test.go` and `protocol_typescript_test.go`, specify minor 1, all four exact
  event/request pairs, `adopted` creation mode, `detour` association origin, metadata fields,
  constraints, and Go/TypeScript parity. In `cmd/awf/metrics_test.go`, require the real
  `awf metrics protocol --json` handshake to report protocol 2.1. In
  ledger/lifecycle/projection/retention tests, build
  valid new-kind histories directly and assert alternative creation, adoption state,
  continuation phase-start preservation, detour lineage, post-terminal return marker, and
  pending-return exclusion. Before implementation, `go test ./internal/telemetry` must fail for
  partial projection and missing 2.1 shapes.
- [ ] **Task 1.2: Encode the exact protocol-2.1 contract.** In
  `internal/telemetry/protocol.json`, set minor 1 and encode ADR-0161 Decision 1 exactly:
  `effort_adopted/adopt`, `phase_continued/continue-phase`,
  `detour_started/start-detour`, and `detour_returned/mark-detour-returned`; creation mode
  `adopted`; association origin `detour`; fixed/optional fields; omission-clears-attribution,
  alternative-creation, metadata-match, fixed-brainstorming, matching-terminal-outcome, and
  post-terminal constraints. In `types.go`, add the four payloads and request types, adopted and
  detour-return metadata, current workflow/activity/mode attribution, and canonical adoption and
  return projections. Extend descriptor decoding/generation in `protocol.go` and
  `protocol_typescript.go`. Existing minor-0 events remain valid. Add descriptor constraint
  `{ "kind": "field-const", "field": "<field>", "value": "<value>" }` and implement its
  descriptor self-check plus Go/TypeScript value checks. The exact payload fields are:

  ```json
  {
    "effort_adopted":{"creationMode":{"type":"string","required":true,"vocabulary":"creationModes"},"route":{"type":"string","required":false,"vocabulary":"routes"},"phase":{"type":"string","required":true,"vocabulary":"phases"},"workflow":{"type":"string","required":true,"format":"category"},"trajectoryId":{"type":"string","required":true,"format":"identifier"},"anchorId":{"type":"string","required":true,"format":"identifier"},"associationOrigin":{"type":"string","required":true,"vocabulary":"associationOrigins"}},
    "phase_continued":{"phase":{"type":"string","required":true,"vocabulary":"phases"},"startEventId":{"type":"string","required":true,"format":"identifier"},"workflow":{"type":"string","required":true,"format":"category"},"activity":{"type":"string","required":false,"vocabulary":"activities"},"implementationMode":{"type":"string","required":false,"format":"category"}},
    "detour_started":{"creationMode":{"type":"string","required":true,"vocabulary":"creationModes"},"origin":{"type":"origin","required":true},"returnPhase":{"type":"string","required":true,"vocabulary":"phases"},"returnPhaseStartEventId":{"type":"string","required":true,"format":"identifier"},"trajectoryId":{"type":"string","required":true,"format":"identifier"},"anchorId":{"type":"string","required":true,"format":"identifier"},"workflow":{"type":"string","required":true,"format":"category"},"associationOrigin":{"type":"string","required":true,"vocabulary":"associationOrigins"}},
    "detour_returned":{"terminalOutcome":{"type":"string","required":true,"vocabulary":"terminalOutcomes"},"parentAssociationEventId":{"type":"string","required":true,"format":"identifier"}}
  }
  ```

  Every new payload is lifecycle-class, `additionalProperties:false`, and
  `privacyPolicy:"long-lived-minimal"`; the first three are repairable and return is not. Field
  constants are adopt creationMode=adopted and associationOrigin=manual, and detour-start
  creationMode=derived, workflow=brainstorming, associationOrigin=detour. Each lifecycle request
  repeats the existing request-base fields in existing order, followed by its matching payload
  fields except that adopt omits associationOrigin and construction supplies manual, while detour
  start omits associationOrigin and construction supplies detour. The exact Go declarations are:

  ```go
  type EffortAdoptedPayload struct { CreationMode CreationMode `json:"creationMode"`; Route Route `json:"route,omitempty"`; Phase Phase `json:"phase"`; Workflow BoundedCategory `json:"workflow"`; TrajectoryID string `json:"trajectoryId"`; AnchorID string `json:"anchorId"`; AssociationOrigin AssociationOrigin `json:"associationOrigin"` }
  type PhaseContinuedPayload struct { Phase Phase `json:"phase"`; StartEventID string `json:"startEventId"`; Workflow BoundedCategory `json:"workflow"`; Activity Activity `json:"activity,omitempty"`; ImplementationMode BoundedCategory `json:"implementationMode,omitempty"` }
  type DetourStartedPayload struct { CreationMode CreationMode `json:"creationMode"`; Origin OriginMetadata `json:"origin"`; ReturnPhase Phase `json:"returnPhase"`; ReturnPhaseStartEventID string `json:"returnPhaseStartEventId"`; TrajectoryID string `json:"trajectoryId"`; AnchorID string `json:"anchorId"`; Workflow BoundedCategory `json:"workflow"`; AssociationOrigin AssociationOrigin `json:"associationOrigin"` }
  type DetourReturnedPayload struct { TerminalOutcome TerminalOutcome `json:"terminalOutcome"`; ParentAssociationEventID string `json:"parentAssociationEventId"` }
  type DetourReturnMetadata struct { SessionID string `json:"sessionId"`; Phase Phase `json:"phase"`; PhaseStartEventID string `json:"phaseStartEventId"` }
  type AdoptLifecycleRequest struct { LifecycleRequestBase; Route Route `json:"route,omitempty"`; Phase Phase `json:"phase"`; Workflow BoundedCategory `json:"workflow"`; TrajectoryID string `json:"trajectoryId"`; AnchorID string `json:"anchorId"` }
  type ContinuePhaseLifecycleRequest struct { LifecycleRequestBase; Phase Phase `json:"phase"`; StartEventID string `json:"startEventId"`; Workflow BoundedCategory `json:"workflow"`; Activity Activity `json:"activity,omitempty"`; ImplementationMode BoundedCategory `json:"implementationMode,omitempty"` }
  type StartDetourLifecycleRequest struct { LifecycleRequestBase; CreationMode CreationMode `json:"creationMode"`; Origin OriginMetadata `json:"origin"`; ReturnPhase Phase `json:"returnPhase"`; ReturnPhaseStartEventID string `json:"returnPhaseStartEventId"`; TrajectoryID string `json:"trajectoryId"`; AnchorID string `json:"anchorId"`; Workflow BoundedCategory `json:"workflow"` }
  type MarkDetourReturnedLifecycleRequest struct { LifecycleRequestBase; TerminalOutcome TerminalOutcome `json:"terminalOutcome"`; ParentAssociationEventID string `json:"parentAssociationEventId"` }
  ```

  `EffortMetadata` adds `DetourReturn *DetourReturnMetadata json:"detourReturn,omitempty"` after
  Origin. Adopted metadata has mode adopted and nil Origin/DetourReturn. Detour metadata has mode
  derived, nonnil Origin, and nonnil DetourReturn whose SessionID is request-base SessionID.
  In `protocol.json`, add object `DetourReturnMetadata` with `goType:"DetourReturnMetadata"`,
  `additionalProperties:false`, and ordered fields `sessionId` required identifier, `phase` required
  phases vocabulary, and `phaseStartEventId` required identifier. Add ordered optional
  `detourReturn:{"type":"detourReturn","required":false}` after `origin` in `EffortMetadata`.
  Add metadata constraints: independent and adopted forbid origin and detourReturn; adopted also
  requires neither; derived requires origin while detourReturn remains optional so ordinary derived
  efforts stay valid. Atomic `detour_started` validation additionally requires nonnil detourReturn.

  Each request descriptor has `additionalProperties:false` and begins with these exact ordered
  fields: `action` required string; `idempotencyKey` required identifier string; `eventId` required
  identifier string; `effortId` required identifier string; `sessionId` required identifier string;
  `timestamp` required timestamp string; `predecessors` required unique identifier-string array with
  `minItems:0`. The ordered tails are exact: adopt = optional route/routes, required phase/phases,
  required workflow/category, required trajectoryId/identifier, required anchorId/identifier;
  continue-phase = required phase/phases, required startEventId/identifier, required
  workflow/category, optional activity/activities, optional implementationMode/category;
  start-detour = required creationMode/creationModes, required origin/origin, required
  returnPhase/phases, required returnPhaseStartEventId/identifier, required trajectoryId/identifier,
  required anchorId/identifier, required workflow/category; mark-detour-returned = required
  terminalOutcome/terminalOutcomes and required parentAssociationEventId/identifier. Only
  start-detour request has field-const constraints, creationMode=derived and
  workflow=brainstorming. Adopt request deliberately has neither creationMode nor
  associationOrigin; continue and mark-return have no field-const; payload constraints remain
  exactly those listed above.
- [ ] **Task 1.3: Implement alternative creation and projection as reader capability.** In
  `internal/telemetry/ledger.go`, add one closed helper returning true only for
  `effort_created`, `effort_adopted`, and `detour_started`. `CreateEffort` accepts metadata plus
  exactly one matching first event; `Append` rejects every creation kind outside atomic creation;
  `validateCreation` matches effort ID, timestamp, creation mode, origin, and detour-return
  metadata by kind; `identicalCreation` succeeds only when metadata and canonical first-event bytes
  match and otherwise returns the existing visible creation conflict. In `reader.go`, count exactly
  one valid first-stream creation kind and suppress the whole effort after any unsupported required
  record. In `lifecycle.go` and `projection.go`, teach readers all four effects without exposing a
  successful new-kind mutation path. In `retention.go`, exclude pending-return terminal children.
- [ ] **Task 1.4: Publish the generated local reader but disable every new writer entry.** In
  `templates/pi/awf-dashboard/index.ts.tmpl`, update protocol validation, metadata scanning,
  creation-kind validation, and local projection to read all 2.1 kinds. Add an explicit
  reader-before-writer guard: `awf_lifecycle` rejects the four new actions with a bounded
  `protocol 2.1 writer is not enabled` error; no adoption or detour tool is registered; workflow
  mappings still emit only 2.0 kinds. Rendered TypeScript therefore understands resident 2.1
  records before any Pi route can append one. Advance the real handshake in `cmd/awf/metrics.go`
  to 2.1 so registration uses the compatible binary reader rather than only a fixture. Update
  `tools/pi-extension-test/fixtures/fake-awf.mjs` to return the required 2.1 protocol handshake,
  because the generated dashboard must reject its old 2.0 response. Test this guard in
  `protocol.test.ts` and `dashboard.test.ts`.
- [ ] **Task 1.5: Enter Implementing, apply operation 1, and commit green.** Update only
  `.awf/topics/parts/tooling/workflow-telemetry/current-state.md` claim
  `event-protocol-and-ledger` to its complete ADR-0161 wording and append ADR-0161 to
  `Revised-by`. Append `Implementing` with the frozen digest and the first Applied event containing
  exactly operation 1, `update tooling/workflow-telemetry:event-protocol-and-ledger`, at the next
  global state sequence. Update `.awf/docs/parts/architecture/data-flow.md` and
  `.awf/docs/parts/testing/layout.md` for reader suppression and reader-first publication. Run
  `./x render`, `./x check`, `go test ./internal/telemetry`, and `./x pi-test run`, all with
  zero exit. First add this modified plan explicitly with:

  `git add -- docs/plans/2026-07-26-agent-adopted-workflow-state-and-derived-detours.md`

  Then stage the remaining tracked transaction with:

  `git add -u -- internal/telemetry cmd/awf/metrics.go cmd/awf/metrics_test.go templates/pi/awf-dashboard/index.ts.tmpl tools/pi-extension-test/fixtures/fake-awf.mjs tools/pi-extension-test/tests/protocol.test.ts tools/pi-extension-test/tests/dashboard.test.ts .awf/topics/parts/tooling/workflow-telemetry/current-state.md .awf/docs/parts/architecture/data-flow.md .awf/docs/parts/testing/layout.md .pi/extensions/awf-dashboard docs/architecture.md docs/testing.md docs/topics/tooling/workflow-telemetry.md docs/domains/tooling.md docs/decisions/0161-agent-adopted-workflow-state-and-derived-detours.md docs/decisions/INDEX.md .awf/awf.lock examples/sundial/.awf/awf.lock examples/sundial/.pi/extensions/awf-dashboard/index.ts examples/sundial/.pi/extensions/awf-dashboard/protocol.ts`

  Run `./awf check --staged` and `./x gate`, both with zero exit, then commit:

```commit
feat(tooling): publish protocol 2.1 readers (applies 0161 batch)
```

## Phase 2: Enable lifecycle writes and entry/continuation routing

- [ ] **Task 2.1: Add failing writer and router regressions.** In
  `internal/telemetry/ledger_test.go`, `ledger_branches_test.go`,
  `ledger_fault_branches_test.go`, `lifecycle_test.go`, `lifecycle_branches_test.go`, and
  `faults_test.go`, invoke all four lifecycle requests and cover request-to-envelope construction,
  immutable creation retry, mismatched retry, current-frontier requirements, only-return
  post-terminal legality, and deterministic conflict keys. In
  `internal/catalog/workflow_test.go`, encode ADR-0161's complete mapping table and reject missing,
  duplicate, unordered, or incompatible entry/continuation phases. In
  `tools/pi-extension-test/tests/workflow.test.ts`, reproduce investigation-to-brainstorming and
  already-open target failures; assert continuation preserves the original start, replaces or
  clears attribution, and emits no same-phase transition. Focused Go/Pi tests must fail before the
  write guard and old `PhaseEffect` model are removed.
- [ ] **Task 2.2: Enable the canonical lifecycle request union.** In
  `internal/telemetry/lifecycle.go`, make request decomposition return exact metadata/event pairs:
  adopt uses absent effort, empty predecessors, adopted metadata, optional route, and its event as
  phase start; continue requires the visible named start and whole current frontier; start-detour
  uses the request effort ID as child, empty child predecessors, derived origin and return
  metadata; mark-return requires terminal child frontier, matching outcome, and durable parent
  association ID. `ApplyLifecycle` routes both new creation actions through `CreateEffort` and both
  append actions through ordinary causal append. Remove the Phase-1 write-disabled rejection in
  Go only after these tests pass.
- [ ] **Task 2.3: Replace catalog `PhaseEffect` with a three-effect planner.** In
  `internal/catalog/catalog.go`, remove `PhaseEffect` and declare exactly:

  ```go
  type WorkflowMapping struct {
      Kind WorkflowKind
      EntryPhase string
      AllowEntryWithoutPhase bool
      EntryPredecessors []string
      ContinuationPhases []string
      Activity string
      ImplementationMode string
      RouteEffect RouteEffect
      TerminalEffect TerminalEffect
  }
  ```

  In `standard.go`, define `allWorkflowPhases` exactly as sorted
  `[]string{"adr-authoring","adr-plan-resync","adr-review","brainstorming","implementation","implementation-review","investigation","plan-review","planning","retrospective"}`. For each existing SkillSpec key below, replace only its `Workflow:` initializer with the exact RHS shown; `all()` means the exact expression `append([]string{}, allWorkflowPhases...)`:

  ```go
  "brainstorming" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "brainstorming", AllowEntryWithoutPhase: true, EntryPredecessors: []string{"investigation"}, ContinuationPhases: []string{"brainstorming"}}
  "bugfix" -> &WorkflowMapping{Kind: WorkflowTask, EntryPhase: "brainstorming", AllowEntryWithoutPhase: true, EntryPredecessors: []string{}, ContinuationPhases: []string{"brainstorming"}, RouteEffect: RouteSelectBugfix}
  "debugging" -> &WorkflowMapping{Kind: WorkflowTask, EntryPhase: "investigation", AllowEntryWithoutPhase: true, EntryPredecessors: []string{}, ContinuationPhases: all(), Activity: "debugging"}
  "exploring" -> &WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: all(), Activity: "exploration"}
  "tdd" -> &WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: []string{"implementation"}, Activity: "tdd"}
  "refactor-coupling-audit" -> &WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: []string{"brainstorming"}, Activity: "refactor-coupling-audit"}
  "adr-lifecycle" -> &WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: all(), Activity: "adr-lifecycle"}
  "roadmap-graduation" -> &WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: all(), Activity: "roadmap-graduation"}
  "proposing-adr" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "adr-authoring", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"adr-authoring"}, RouteEffect: RouteSelectADR}
  "writing-plans" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "planning", EntryPredecessors: []string{"adr-review","brainstorming"}, ContinuationPhases: []string{"planning"}, RouteEffect: RoutePromoteADRPlan}
  "reviewing-adr" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "adr-review", EntryPredecessors: []string{"adr-authoring"}, ContinuationPhases: []string{"adr-review"}}
  "reviewing-plan" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "plan-review", EntryPredecessors: []string{"planning"}, ContinuationPhases: []string{"plan-review"}}
  "reviewing-plan-resync" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "adr-plan-resync", EntryPredecessors: []string{"plan-review"}, ContinuationPhases: []string{"adr-plan-resync"}}
  "executing-direct" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"implementation"}, ImplementationMode: "inline-execution", RouteEffect: RouteSelectDirect}
  "executing-plans" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"adr-plan-resync","plan-review"}, ContinuationPhases: []string{"implementation"}, ImplementationMode: "inline-execution"}
  "subagent-driven-development" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"adr-plan-resync","plan-review"}, ContinuationPhases: []string{"implementation"}, ImplementationMode: "subagent-driven-development"}
  "reviewing-impl" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation-review", EntryPredecessors: []string{"implementation"}, ContinuationPhases: []string{"implementation-review"}}
  "retrospective" -> &WorkflowMapping{Kind: WorkflowChain, EntryPhase: "retrospective", EntryPredecessors: []string{"implementation-review","investigation"}, ContinuationPhases: []string{"retrospective"}, RouteEffect: RouteSelectInvestigationIfUnrouted, TerminalEffect: TerminalArmCompletion}
  ```

  Rewrite validation in `workflow.go`. Replace `workflowLoaderEntry` in
  `internal/project/render.go` with JSON properties exactly `kind`,
  `entryPhase,omitempty`, `allowEntryWithoutPhase`, `entryPredecessors`, `continuationPhases`,
  `activity,omitempty`, `implementationMode,omitempty`, `routeEffect,omitempty`, and
  `terminalEffect,omitempty`; both slice properties are nonnil sorted arrays. Update
  `pi_workflow_render_test.go` to pin that shape. In the dashboard
  template, plan start with no phase, transition from a legal predecessor, or `continue-phase` at a
  legal current phase. Debugging may enter investigation or continue any phase; brainstorming may
  transition from investigation; TDD remains implementation-only; chain skills continue only at
  their target. Remove all same-phase `phase_transitioned` writes.
- [ ] **Task 2.4: Enable the generated Pi writer safely.** Remove the Phase-1 TypeScript write
  guard only for `continue-phase`; retain adopt and start-detour behind their dedicated later tools,
  while `mark-detour-returned` remains callable only by return settlement. Update
  `awf_lifecycle` validation so direct public calls cannot bypass adoption memory validation or
  detour parent validation. Render and test the compatible reader is already present in the prior
  commit and every enabled path acknowledges durability before returning a body.
- [ ] **Task 2.5: Apply operations 2-3 and commit.** Update once, to complete ADR-0161 wording,
  `tooling/workflow-telemetry:effort-lifecycle-and-routes` and
  `tooling/workflow-telemetry:trajectory-and-derived-effort-model`; append one Applied event with
  exactly those declaration-consecutive operations. Update `.awf/parts/workflow/chain.md`,
  `templates/pi/awf-workflow/SKILL.md.tmpl`,
  `.awf/docs/parts/architecture/overview.md`, and
  `.awf/docs/parts/architecture/data-flow.md`. Run `./x render`, `./x check`,
  `go test ./internal/telemetry ./internal/catalog ./internal/project`, and `./x pi-test run`, each
  with zero exit. Stage the named code, tests, templates, authored parts, claim source,
  ADR/index/lock, and rendered outputs using
  `git add -u -- internal/telemetry internal/catalog internal/project/render.go internal/project/pi_workflow_render_test.go templates/pi/awf-dashboard/index.ts.tmpl templates/pi/awf-workflow/SKILL.md.tmpl tools/pi-extension-test/tests/workflow.test.ts .awf/parts/workflow/chain.md .awf/docs/parts/architecture/overview.md .awf/docs/parts/architecture/data-flow.md .awf/topics/parts/tooling/workflow-telemetry/current-state.md .pi docs AGENTS.md .awf/awf.lock`.
  Then run `./awf check --staged` and `./x gate`, both with zero exit; commit:

```commit
feat(rendering): continue workflow phases (applies 0161 batch)
```

## Phase 3: Adopt normalized external checkpoints

- [ ] **Task 3.1: Add the failing adoption proof suite.** In
  `tools/pi-extension-test/tests/workflow.test.ts`, add
  `// invariant: tooling/workflow-telemetry:external-adoption-boundary` on a suite covering:
  confined regular UTF-8 file; 1-MiB file, first-16-KiB header, and 512-byte line bounds; LF/CRLF;
  exact ordered Effort/Route/Phase/Workflow/Next fields before first H2; duplicate/reordered/bad
  fields; 256-file sibling cap; symlink/unsafe siblings; duplicate memory IDs; every resident effort
  and tombstone collision; request/header mismatch; unselected-route translation; catalog
  compatibility; exclusive batches; precommit failure; postcommit association failure; identical
  retry; and absence of path, filename, Next text, or content from metadata/events.
- [ ] **Task 3.2: Implement bounded memory validation.** In the dashboard template, add helpers that
  inspect from the project root without symlink traversal, require `.awf/memory/` containment and a
  regular file, perform bounded byte reads followed by strict UTF-8 decode, parse the exact header
  prefix, and scan at most 256 confined regular `.md` siblings for Effort collisions. Reject unsafe
  entries and exceeded bounds; ignore an unrelated bounded sibling only when it has no Effort
  field. Check effort directories and tombstones. Never infer from filenames or prose and never
  import another generated extension.
- [ ] **Task 3.3: Register exclusive `awf_adopt_effort`.** Add closed input
  `{memoryPath, effortId, route, phase, workflow}` and generalized single-tool preflight. Re-read
  and match all normalized fields, translate route `unselected` to omitted lifecycle route,
  validate mapping compatibility, derive deterministic request/event identity from effort,
  session, workflow, and normalized state, generate trajectory/anchor IDs once per recoverable
  request, and invoke atomic adopt. Persist association after durability and then return the fixed
  body. Before commit create nothing; after commit recover by explicit effort ID and identical
  event even when custom persistence previously failed.
- [ ] **Task 3.4: Apply operation 4 and commit.** Add
  `tooling/workflow-telemetry:external-adoption-boundary` with `Origin: ADR-0161` and
  `Backing: test`; apply exactly operation 4. Update `templates/docs/workflow.md.tmpl`,
  `.awf/parts/agents-doc/working-memory.md`, `.awf/parts/workflow/chain.md`,
  `.awf/parts/working-with-awf/commands.md`, and
  `.awf/parts/working-with-awf/config-and-overrides.md` with the agent-driven normalized-header
  route. Do not edit rendered docs or AGENTS directly. Run `./x render`, `./x check`,
  `go test ./internal/telemetry ./internal/catalog ./internal/project`, and `./x pi-test run`, each
  with zero exit. Stage with
  `git add -u -- templates/pi/awf-dashboard/index.ts.tmpl templates/docs/workflow.md.tmpl tools/pi-extension-test/tests/workflow.test.ts internal/project/pi_workflow_render_test.go .awf/topics/parts/tooling/workflow-telemetry/current-state.md .awf/parts/agents-doc/working-memory.md .awf/parts/workflow/chain.md .awf/parts/working-with-awf/commands.md .awf/parts/working-with-awf/config-and-overrides.md .pi docs AGENTS.md .awf/awf.lock`.
  Then run `./awf check --staged` and `./x gate`, both with zero exit; commit:

```commit
feat(rendering): adopt external checkpoints (applies 0161 batch)
```

## Phase 4: Add derived detours and deterministic parent return

- [ ] **Task 4.1: Add the failing detour proof and fault matrix.** In
  `tools/pi-extension-test/tests/workflow.test.ts`, add
  `// invariant: tooling/workflow-telemetry:derived-detour-return` on tests for explicit child ID,
  effort/tombstone/pending collision, exact active parent/phase/start/session/trajectory/anchor,
  atomic retry, untouched parent phase, nested detours, completion, abandonment, frontier race,
  invalid parent, repeated settlement, and startup recovery. For both outcomes inject failure
  immediately before and after terminal child durability, parent association durability, child
  return marker durability, and custom association persistence. Assert selection changes only after
  success, one logical parent association, and pending children remain non-retainable.
- [ ] **Task 4.2: Register `awf_detour`.** Add exclusive closed input
  `{childEffortId, workflow}` with workflow enum initially exactly brainstorming. Validate parent
  projection and every collision. Derive child lifecycle idempotency, event, trajectory, and anchor
  IDs from child ID plus parent effort/phase start/session so retry reconstructs identical bytes.
  Invoke atomic `start-detour`, persist child association after durability, and return brainstorming
  without any parent mutation.
- [ ] **Task 4.3: Implement the exact return state machine.** Add named template helpers
  `detourReturnIdentity`, `findParentReturnAssociation`, and `settleDetourReturn`. Discover pending
  work by projecting the selected terminal child at `session_start`, after retrospective completion,
  and after successful active-child abandonment. The terminal child event is the commit boundary.
  Derive parent association event ID/idempotency key from child ID and terminal epoch and use the
  child terminal timestamp as its stable timestamp. Revalidate parent phase/start/session and active
  trajectory, read the whole current parent frontier, and attempt association with detour origin. A
  stale-frontier failure with no durable identity reprojects and retries; once any matching
  idempotency event exists, reuse its exact event ID regardless of later frontier movement. Append
  child `detour_returned` against the current terminal child frontier with matching outcome and that
  association ID, persist the parent custom entry, then and only then switch in-memory association.
  A failure leaves the terminal child selected and recoverable; never append trajectory resume or
  restart the parent phase.
- [ ] **Task 4.4: Apply operation 5 and commit.** Add
  `tooling/workflow-telemetry:derived-detour-return` with `Origin: ADR-0161` and `Backing: test`;
  apply exactly operation 5. Update `.awf/docs/parts/architecture/overview.md`,
  `.awf/docs/parts/architecture/data-flow.md`, `.awf/parts/workflow/chain.md`, and the authored
  agent-guide rule for derived blockers. Run `./x render`, `./x check`,
  `go test ./internal/telemetry`, and `./x pi-test run`, each with zero exit. Stage with
  `git add -u -- internal/telemetry templates/pi/awf-dashboard/index.ts.tmpl tools/pi-extension-test/tests/workflow.test.ts .awf/topics/parts/tooling/workflow-telemetry/current-state.md .awf/docs/parts/architecture/overview.md .awf/docs/parts/architecture/data-flow.md .awf/parts/workflow/chain.md .awf/parts/agents-doc/working-memory.md .pi docs AGENTS.md .awf/awf.lock`.
  Then run `./awf check --staged` and `./x gate`, both with zero exit; commit:

```commit
feat(rendering): add derived detour return recovery (applies 0161 batch)
```

## Phase 5: Bound agent-facing query results

- [ ] **Task 5.1: Add the failing raw-query and byte-bound suite.** In
  `tools/pi-extension-test/tests/dashboard.test.ts`, feed metrics and doctor fixtures with many
  efforts, event IDs, evidence, eventless integrity, controls, POSIX/Windows/project-relative paths,
  multiline text, long Unicode, malformed canonical output, and raw stdout/stderr. Assert literal
  line formats and ordering. Metrics emits, in order:

  1. `awf metrics: efforts=%d shown=%d truncated=%t`;
  2. only when the active associated effort occurs in the result,
     `current effort=%s state=%s route=%s phase=%s`, using `unselected` and `none` sentinels;
  3. `totals input=%d output=%d cache-read=%d cost-usd=%.6f tool-failures=%d gate-failures=%d`;
  4. zero to eight effort-ID-sorted rows,
     `effort id=%s state=%s route=%s phase=%s input=%d output=%d`;
  5. exactly `details: /awf-dashboard` when any canonical field or row is undisplayed, otherwise no
     footer.

  Doctor emits, in order:

  1. `awf doctor: findings=%d shown=%d integrity=%d truncated=%t`;
  2. nonzero severity rows sorted by severity, `severity name=%s count=%d`;
  3. nonzero rule rows sorted by code, `rule code=%s count=%d`;
  4. nonzero integrity rows sorted by code, `integrity code=%s count=%d`;
  5. zero to five findings sorted by severity, code, effort, each as three lines:
     `finding effort=%s code=%s severity=%s`, `explanation %s`, and `next %s`;
  6. the same exact optional footer `details: /awf-dashboard`.

  Join lines with one ASCII LF and emit no terminal newline. Assert content is at most 4096 UTF-8
  bytes. Details use `JSON.stringify` insertion order and
  serialize without spaces exactly as
  `{"format":"awf-compact-v1","selector":"%s","truncated":%t,"displayed":{"efforts":%d,"findings":%d,"integrityCodes":%d}}`,
  at most 1024 bytes. Raw JSON, IDs/evidence, paths, stdout, and stderr must be absent while the
  overlay still retains the full private parsed object.
- [ ] **Task 5.2: Implement allowlist-only serializers exactly.** In the dashboard template add
  `sanitizeCompactText`, `fitUTF8`, `compactSelector`, `compactMetricsResult`,
  `compactDoctorResult`, and `compactQueryDetails`. Normalize CR/LF/tab and Unicode categories Cc
  and Cf to spaces and collapse `[ \\f\\v]+` to one space. Replace path tokens before sorting. A
  token is maximal non-whitespace text matching `^/[^ ]+`, `^[A-Za-z]:[\\\\/][^ ]+`, or
  `^(?:\\.awf|\\.pi|docs|templates|internal|cmd|tools)[\\\\/][^ ]+`; strip one leading and
  trailing ASCII quote, parenthesis, bracket, comma, colon, or semicolon only for matching, then
  restore that punctuation around literal `<path>`. Cap selector at 256 bytes and each
  explanation/next field at 512 bytes. Format cost with six decimal places and integers base 10.
  Assemble already field-capped lines, then fit deterministically. Metrics preserves header,
  totals, current, and footer in that priority; remove effort rows from the lexicographically last
  ID backward until the UTF-8 encoding fits. Doctor preserves header and footer, then removes
  findings from the reverse of their sort order, integrity rows from reverse code order, rule rows
  from reverse code order, and severity rows from reverse name order until it fits. If a mandatory
  set still exceeds the limit, shorten selector/details only (not content), because protocol-bounded
  identifiers and fixed headers make content mandatory sets fit. Set `truncated=true` and include
  the single exact footer `details: /awf-dashboard` whenever a row or field is removed, a field is
  byte-shortened, sanitization replaces a path/control run, or canonical information is not in the
  allowlist. A canonical field is undisplayed only when it is nonempty and is one of sessions,
  phases, trajectories, event IDs, evidence, per-scope integrity, retention candidates, or extra
  finding fields; absent and numeric-zero fields omitted by the grammar do not trigger the footer.
  Recompute in this exact order: sanitize and per-field fit; build candidate lines; set initial
  truncated from undisplayed/sanitized/fitted data or cardinality caps; add footer when truncated;
  drop optional rows in the stated order; recompute shown counts and headers; if a drop changed
  truncation/footer, rebuild once and repeat dropping until at most 4096 bytes; derive details from
  final shown counts and truncated. Build details in the stated insertion order; shorten selector
  on a Unicode boundary until its JSON encoding is at most 1024 bytes. Never call
  generic `toolResult` with parsed canonical data and never include
  process output in degraded errors; degraded content is
  `awf <metrics|doctor>: unavailable (<bounded-category>)`.
- [ ] **Task 5.3: Apply operations 6-7 and commit.** Update once and apply exactly consecutive
  operations 6 and 7:
  `tooling/workflow-telemetry:privacy-integrity-and-retention` and
  `tooling/workflow-telemetry:canonical-projections-and-diagnostics`. Update
  `.awf/docs/parts/architecture/data-flow.md` and `.awf/docs/parts/testing/layout.md`. Run
  `./x render`, `./x check`, and `./x pi-test run`, each with zero exit. Stage with
  `git add -u -- templates/pi/awf-dashboard/index.ts.tmpl tools/pi-extension-test/tests/dashboard.test.ts .awf/topics/parts/tooling/workflow-telemetry/current-state.md .awf/docs/parts/architecture/data-flow.md .awf/docs/parts/testing/layout.md .pi docs .awf/awf.lock`.
  Then run `./awf check --staged` and `./x gate`, both with zero exit; commit:

```commit
fix(rendering): bound agent-facing workflow queries (applies 0161 batch)
```

## Phase 6: Apply Pi authority, run acceptance, and freeze

- [ ] **Task 6.1: Complete exact authored Pi and documentation surfaces.** Update each remaining
  claim once to its complete ADR-0161 wording: in
  `.awf/topics/parts/rendering/pi-workflows/current-state.md`, router then dashboard claims; in
  `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, working-memory single
  home; in `.awf/topics/parts/rendering/adapter-outputs/current-state.md`, dashboard runtime; in
  `.awf/topics/parts/rendering/pi-runtime/current-state.md`, real-runtime smoke. Update
  `templates/pi/awf-workflow/SKILL.md.tmpl`, `templates/docs/workflow.md.tmpl`,
  `.awf/parts/workflow/chain.md`, `.awf/parts/agents-doc/working-memory.md`,
  `.awf/parts/working-with-awf/commands.md`,
  `.awf/parts/working-with-awf/config-and-overrides.md`,
  `.awf/docs/parts/architecture/overview.md`, `.awf/docs/parts/architecture/data-flow.md`, and
  `.awf/docs/parts/testing/layout.md`. Render all outputs; do not edit `.pi/**`, rendered `docs/**`,
  `AGENTS.md`, index, or lock directly.
- [ ] **Task 6.2: Run final acceptance without open-ended repair.** Run `./x render`, `./x check`,
  `go test ./internal/telemetry ./internal/catalog ./internal/project`, `./x pi-test run`,
  `go test ./...`, and `./x gate full`, each with expected zero exit. This is the pinned Pi 0.81.1
  fixture lane, not the release-only manual interactive Pi smoke. Verify adoption into every
  legal mapping, investigation-to-brainstorming, start/transition/continue, nested completion and
  abandonment return at every fault boundary, pending-return retention exclusion, exact compact
  output bytes/details, dashboard full detail, protocol TypeScript parity, generated ownership,
  and 100 percent Go and Pi coverage. Any nonzero result stops execution and requires this Proposed
  plan to be amended before further implementation; Task 6.3 begins only when every command exits
  zero without an additional unplanned commit.
- [ ] **Task 6.3: Apply exact operations 8-12 and freeze records.** Append one final Applied event
  with exactly, in declaration order: update
  `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`; update
  `rendering/pi-workflows:pi-workflow-dashboard-public-contract`; update
  `rendering/guide-and-doc-templates:working-memory-single-home`; update
  `rendering/adapter-outputs:pi-workflow-dashboard-runtime`; update
  `rendering/pi-runtime:pi-real-runtime-smoke`. Then append Implemented with the frozen digest and
  the same terminal state sequence. Check every plan task, add actual commit IDs and final command
  results to Notes, and flip this plan to Implemented. Run `./x render`, explicitly stage ADR,
  plan, index, lock, the five claim sources, authored docs/templates, tests, and generated outputs.
  Stage exactly with
  `git add -- docs/plans/2026-07-26-agent-adopted-workflow-state-and-derived-detours.md`, then
  `git add -u -- docs/decisions/0161-agent-adopted-workflow-state-and-derived-detours.md docs/decisions/INDEX.md .awf/awf.lock .awf/topics/parts/rendering/pi-workflows/current-state.md .awf/topics/parts/rendering/guide-and-doc-templates/current-state.md .awf/topics/parts/rendering/adapter-outputs/current-state.md .awf/topics/parts/rendering/pi-runtime/current-state.md templates/pi/awf-workflow/SKILL.md.tmpl templates/docs/workflow.md.tmpl .awf/parts/workflow/chain.md .awf/parts/agents-doc/working-memory.md .awf/parts/working-with-awf/commands.md .awf/parts/working-with-awf/config-and-overrides.md .awf/docs/parts/architecture/overview.md .awf/docs/parts/architecture/data-flow.md .awf/docs/parts/testing/layout.md .pi/skills/awf-workflow/SKILL.md docs/workflow.md docs/architecture.md docs/testing.md docs/topics/rendering/pi-workflows.md docs/topics/rendering/guide-and-doc-templates.md docs/topics/rendering/adapter-outputs.md docs/topics/rendering/pi-runtime.md docs/domains/rendering.md AGENTS.md examples/sundial/.awf/awf.lock examples/sundial/docs/workflow.md examples/sundial/.pi/skills/awf-workflow/SKILL.md`.
  If `git diff --name-only` names any other path after the final render, stop and amend this Proposed
  plan rather than broadening the staging command at execution time.
  Run `./awf check --staged` and `./x gate`, both with zero exit; commit:

```commit
feat(awf): complete workflow adoption and detours (implements 0161)
```

## Verification

- A normalized memory-only effort adopts into every legal mapped phase and loads its fixed body
  without fabricated history or a TUI.
- Debugging transitions to brainstorming; legal current-phase loads preserve their original start
  and record continuation rather than same-phase transition.
- Nested derived detours return to the exact parent after completion or abandonment and recover at
  every durable boundary with one parent association.
- Pending-return children never become retention candidates.
- Metrics/doctor model content is non-JSON and at most 4096 UTF-8 bytes; details are allowlisted and
  at most 1024 bytes; dashboard state retains full canonical detail.
- Minor-0 history remains readable and any unsupported required record suppresses its complete
  effort from lifecycle, metrics, diagnostics, cohorts, and retention.
- Every commit passes staged check and gate; the final pinned Pi 0.81.1 fixture lane and 100 percent
  coverage pass. The release-only manual interactive Pi smoke remains governed by its current-state
  claim and is not asserted as executed by this plan.

## Notes

- The current session reproduced two routing failures: debugging could not hand off to
  brainstorming, and accepted ADR work could not enter planning while the ledger remained in
  investigation. They are Phase-2 regressions.
- Protocol 2.1 prioritizes stabilization over making an old binary safely read newly written event
  kinds. The committed reader-before-writer split prevents the pinned Pi runtime from creating that
  mixed deployment.
