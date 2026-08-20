---
format: plan-v2
date: 2026-08-20
adrs: [move-pi-session-handoff-authority-to-pi-runtime]
status: Proposed
---
# Plan: Move Pi Session Handoff Authority to Pi Runtime

## Goal

Move all Pi-specific session-replacement protocol to the `rendering/pi-runtime` owner while generic
workflow authority retains capability-neutral continuity rules and every Pi executable behavior
remains available through target-owned guidance. Do not reorganize daily-use, recovery, migration,
or configuration information assigned to AF-011.

## Architecture summary

Keep checkpoint persistence, safe-resumability eligibility, reorientation, truthful fresh-boundary
logging, cancellation and failure disposition, retained-context judgment, and target-native
continuation in shared workflow and checkpoint sources. Move Pi's `[session context]` evidence,
`handoff_session` invocation, exact effort kickoff, association projection, and target-specific
failure instructions into the Pi-only `using-effort` skill, with `rendering/pi-runtime` as the active
claim owner and `rendering/pi-workflows` retaining only skill-projection responsibility. Move the
proof boundary with the ownership change, update precise topic selectors, then apply all ADR
operations and render the governed outputs in one coherent green transaction. Preserve the
independently installed `pi-tools` implementation boundary and add no awf handoff runtime.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.


## Phase 1: Relocate and prove Pi handoff protocol ownership

**Execution mode: subagent-driven.**

Completes: ["generic-continuity-neutral", "pi-protocol-preserved", "authority-moved"]

### Task 1.1: Establish the generic and Pi-specific proof boundary
Kind: batch
Applying: ["move-pi-session-handoff-authority-to-pi-runtime:target-owned-pi-session-handoff", "move-pi-session-handoff-authority-to-pi-runtime:capability-neutral-continuity-authority"]
Paths: ["internal/evals/independent_workflow_escalation_test.go", "internal/project/spine_test.go", "internal/project/target_test.go", "internal/contextq/adapter_outputs_test.go"]
Representative: "A shared checkpoint persists the effort and chooses a target-native successor without naming Pi, while the Pi-only effort skill supplies the session-context evidence, exact handoff call, and bounded kickoff."
Edge: "A replacement fails or is cancelled, so generic authority permits no fresh-boundary log and the Pi projection gives no conflicting success instruction."
Post-check: "Before authored protocol sources change, `go test ./internal/project ./internal/evals ./internal/contextq -run 'TestCheckpointDigestShape|TestWorkingMemorySingleHomeSurfaces|TestPiRuntimeTargetRender|TestIndependentWorkflowEscalation|TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion'` exits nonzero because the new ownership marker, generic-token exclusions, Pi-only executable projection, or selector ownership is absent; after Task 1.2 the same command exits zero."

Move the `pi-session-handoff-workflow` proof marker from `rendering/pi-workflows` to
`rendering/pi-runtime`. Split existing checkpoint assertions so generic tests prove persistence,
safe-resumability eligibility, reorientation, truthful logging, cancellation and failure
semantics, repository precedence, one-writer memory, and target-native continuation without
`handoff_session`, `[session context]`, or the exact Pi kickoff. Keep a focused Pi target test that
proves the complete target projection, including no fixed threshold, sole handoff call, exact
`Continue with effort <slug>.` kickoff, association and reorientation instructions, managed-worktree
integration boundary, actual-boundary logging, and no log after continuation, cancellation, or
failure. Preserve non-Pi absence and empty-data coherence.

Update context-ownership evidence so the precise Pi-runtime selectors reach the authored protocol
surfaces without reclassifying generated adapter runtime or weakening coverage exclusion. Do not
behavior-test the separately installed `pi-tools` handoff runtime or change the pinned runtime
smoke.

### Task 1.2: Move protocol sources and active claims without duplication
Kind: batch
Applying: ["move-pi-session-handoff-authority-to-pi-runtime:target-owned-pi-session-handoff", "move-pi-session-handoff-authority-to-pi-runtime:capability-neutral-continuity-authority"]
Paths: ["templates/partials/checkpoint-routine.md", "templates/partials/checkpoint-approval.md", "templates/skills/using-effort/SKILL.md.tmpl", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", ".awf/docs/glossary.yaml", ".awf/topics/metadata/rendering/pi-runtime.yaml", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md"]
Representative: "The generic routine checkpoint ends with capability-neutral successor guidance and a pointer to target-owned protocol, while the Pi-only skill contains the complete executable handoff sequence once."
Edge: "Core or a target without session replacement still renders coherent continuation guidance with no Pi runtime fact, tool token, unsupported replacement claim, or dangling reference."
Post-check: "The focused tests from Task 1.1 exit zero; repository search over shared checkpoint, workflow, guide, and glossary sources finds no `[session context]`, `handoff_session`, exact Pi effort kickoff, or Pi-only handoff term outside the Pi-owned skill and current-state claim, while the Pi-owned sources contain every preserved protocol clause and `./awf topic rendering/pi-runtime --coverage` reports coverage for them."

Reduce both checkpoint partials and the workflow document to capability-neutral continuation
semantics. Keep truthful replacement logging and reorientation generic, but express them without a
Pi tool or evidence source. Remove Pi-only glossary entries rather than leaving a second authority;
retain the generic safe-resumable-point term. Keep audience-specific installation summaries terse
and link the Pi runtime topic instead of restating its protocol.

Expand the Pi-only `using-effort` skill from association guidance into the executable projection of
the canonical runtime contract. It retains its existing attach, detach, fixed-path, advisory
activity, memory-tool, and display-suffix behavior, and adds the moved session-replacement protocol
without duplicating the generic continuity rules. Remove `pi-session-handoff-workflow` from Pi
workflows and add it to Pi runtime with lossless Pi detail and test backing. Narrow
`working-memory-single-home` and `memory-checkpoint-chain-coverage` to their generic authority, and
update `using-effort-skill` to name the target projection. Preserve claim provenance with the pending
ADR slug and add only precise Pi-runtime topic selectors needed for the owned source surfaces.

### Task 1.3: Apply authority, render outputs, and record adopter behavior
Kind: batch
Applying: ["move-pi-session-handoff-authority-to-pi-runtime:target-owned-pi-session-handoff", "move-pi-session-handoff-authority-to-pi-runtime:capability-neutral-continuity-authority"]
Paths: ["docs/decisions/move-pi-session-handoff-authority-to-pi-runtime.md", "docs/decisions/INDEX.md", "changelog/CHANGELOG.md", ".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "docs/topics/rendering/pi-runtime.md", "docs/topics/rendering/pi-workflows.md", "docs/topics/rendering/guide-and-doc-templates.md", "docs/topics/rendering/workflow-skill-templates.md", "pathspec:.pi/skills/awf-*/SKILL.md", "pathspec:.claude/skills/awf-*/SKILL.md"]
Post-check: "After `./x render`, `./x check` and the focused tests exit zero; `./awf context --show pending docs/decisions/move-pi-session-handoff-authority-to-pi-runtime.md` reports no Remaining operation; generated Pi output contains the executable protocol once, generated Claude and capability-neutral surfaces contain none of its Pi tokens, `git grep -n '<no value>' -- . ':!docs/plans/*'` returns no output, and focused semantic review finds no contradictory generic/Pi ownership or lost runtime behavior."

Use `awf-adr-lifecycle` to move the reviewed ADR to Implementing and append one Applied event naming
all five declared operations. Apply the matching claim mutations in the same transaction. Add an
Unreleased feature entry describing target-owned Pi protocol and capability-neutral generic
continuity without claiming an awf-owned handoff implementation.

Run `./x render` so current-state topics, workflow documentation, the agent guide, glossary, native
skill trees, index, and lock follow their authored sources. Inspect representative Full and Core Pi
and Claude outputs. Record that the Pi skill retains every executable clause, generic outputs read
coherently without Pi knowledge, intentional target differences are clear, literal placeholders are
intentional, and no contradictory fragment assigns the protocol to both owners. Run focused tests,
`./awf check staged`, and the full gate before phase close.

### Phase close

Land moved proof markers, generic and target-specific protocol sources, current-state mutations,
precise selectors, ADR application history, rendered outputs, and changelog as one independently
green transaction.

```commit
feat(rendering): move Pi handoff protocol authority (applies ADR batch)
```

## Definition of done

- `dod: generic-continuity-neutral` Shared workflow, checkpoint, guide, and glossary authority retains complete capability-neutral continuity semantics without Pi evidence, tool calls, kickoff text, or association mechanics.
- `dod: pi-protocol-preserved` Pi-owned guidance contains one complete executable session-replacement projection with every prior eligibility, evidence, invocation, exact kickoff, association, reorientation, logging, cancellation, failure, managed-worktree, and no-threshold behavior preserved.
- `dod: authority-moved` Current-state claims, topic selectors, proof markers, generated outputs, and changelog consistently assign the canonical protocol to `rendering/pi-runtime`, retain only skill projection under Pi workflows, pass focused tests and the full gate, and leave AF-011 information architecture untouched.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material
cross-owner revisions rather than editing the plan; the parent supplies the report to phase review
and reconciles required plan changes with findings in one focused post-review settlement commit
before checkpointing or later execution.

After implementation assurance settles, `awf-effort-workflow` appends only the ADR's Implemented
event, changes this plan to `status: Implemented`, renders the index and lock, and lands that deferred
lifecycle transaction before returning integration-ready control to the audit orchestrator. It does
not integrate main or perform AF-011.

- Phase review found that the Pi-runtime proof did not independently enforce all three eligible
  checkpoint forms or single-projection uniqueness. The settlement adds clause-specific and
  exact-one assertions without changing protocol semantics.
