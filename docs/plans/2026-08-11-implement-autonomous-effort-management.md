---
format: plan-v2
date: 2026-08-11
adrs: ["restore-autonomous-effort-management"]
status: Proposed
---
# Plan: Implement Autonomous Effort Management

## Goal

Restore autonomous effort creation and deliberate effort switching across every rendered target while preserving effort-free discovery, explicit immutable identity, safe topology handling, and the existing archive lifecycle; add no CLI behavior, schema, abandoned state, or runtime policy knob.

## Architecture summary

The guidance layer remains the sole changed boundary. One shared creation partial owns the transition from an independently fired continuity trigger to faithful outcome/title/slug selection, explicit-slug creation, identity reporting, and managed-worktree continuation. `effort-workflow` remains the only lifecycle owner and adds a reasoned switch branch: checkpoint a kept effort, or transfer necessary context and safely clean or explicitly discard a discontinued effort before ordinary archival finish. Canonical workflow documentation, adopter guidance, glossary data, and the three ADR-owned current-state claims project that same model. Deterministic cross-target tests prove the new positive ordering and reject obsolete confirmation language without changing the CLI, resident, worktree, or archive implementations.

## Phase 1: Replace confirmation with autonomous lifecycle judgment

**Execution mode: subagent-driven.**

Completes: ["autonomous-creation", "deliberate-switching", "contract-proof", "publication-sync"]

### Task 1.1: Rewrite the single-home creation and switching contract
Kind: batch
Applying: ["restore-autonomous-effort-management:create-efforts-autonomously", "restore-autonomous-effort-management:preserve-continuity-threshold", "restore-autonomous-effort-management:switch-deliberately", "restore-autonomous-effort-management:reuse-existing-lifecycle"]
Paths: ["templates/partials/outcome-confirmation.md", "templates/partials/effort-creation.md", "templates/partials/checkpoint-routine.md", "templates/skills/effort-workflow/SKILL.md.tmpl", "templates/skills/orienting/SKILL.md.tmpl", "templates/skills/roadmap-graduation/SKILL.md.tmpl", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/parts/workflow/chain.md", ".awf/skills/parts/retrospective/procedure.md"]
Representative: Rename the obsolete confirmation partial to an autonomous creation home and have `effort-workflow` include it once. The transition reads continuity judgment first, then faithful explicit identity selection, `awf effort new --slug`, post-creation identity reporting, and continuation in the managed worktree, with no ask, stop, wait, later-response, or reconfirmation step.
Edge: Discovery still creates nothing; a failed creation follows ordinary diagnosis and authority-preserving retry; an already-owned matching outcome resumes under fixed identity; a distinct active effort is never silently reused. A kept effort receives a resumable checkpoint. A discontinued effort transfers necessary context, uses ordinary safe removal where possible, and inspects repository identity and worktree state before explicitly discarding intentionally obsolete dirty or unmerged topology through existing native Git safety primitives and finishing the resident into the ordinary archive.
Post-check: Run `rg -n 'Mandatory first-creation confirmation|clear response in a later turn|confirm all three|reconfirm|already-confirmed|confirms and creates|title reconfirmation'` over every declared path that still exists and require zero obsolete active-policy matches. Read the renamed partial and rendered template inclusion together and verify one semantic owner, autonomous ordering, deliberate switching, and no CLI or abandoned-state claim.

Starting dependency: commits `9d6d0805a` and `405011d24` contain the Proposed, review-settled ADR; the approved design and user-provenance evidence remain in the owning effort memory.

Replace the confirmation boundary rather than merely deleting it. Preserve the explicit short-slug command, immutable identity, one-writer memory, report-only child, managed-worktree, checkpoint, integration, divergence-review, topology-removal, retrospective, and finish contracts. Remove confirmation-derived vocabulary from orienting, roadmap graduation, retrospective, checkpoints, and workflow summaries without letting those consumers create efforts independently. Keep switches deliberate and reasoned, including why the new outcome needs separate ownership and whether the former effort remains resumable or is intentionally discontinued.

### Task 1.2: Synchronize active documentation, adopter guidance, claims, and release notes
Kind: batch
Applying: ["restore-autonomous-effort-management:create-efforts-autonomously", "restore-autonomous-effort-management:preserve-continuity-threshold", "restore-autonomous-effort-management:switch-deliberately", "restore-autonomous-effort-management:reuse-existing-lifecycle"]
Paths: [".awf/parts/working-with-awf/commands.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/docs/glossary.yaml", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "changelog/CHANGELOG.md"]
Representative: The current `mandatory-approval-boundaries` claim names brainstorming outline approval as the remaining routine user stop and names autonomous effort creation as a non-approval boundary; `unified-effort-workflow-coverage` and `effort-workflow` name autonomous creation plus deliberate active-effort disposition while retaining sole lifecycle ownership.
Edge: Preserve each claim's existing `Origin`, prior `Revised-by` order, and `Backing: test`, appending `ADR-restore-autonomous-effort-management` only to the three declared claims. Full-replacement adopters must re-derive autonomous creation and deliberate switching because default projection tests cannot inspect replacement prose. Historical ADRs, plans, pitfalls, and released changelog entries remain untouched.
Post-check: Run `../../../awf topic rendering/workflow-skill-templates` and verify all three destination claims describe the pending semantics without changing unrelated claims. Run `rg -n 'clear later user confirmation|confirmation boundary|three-field confirmation|two hard stops|confirmed continuity-backed'` over the declared active sources and require zero obsolete matches. Confirm `[Unreleased]` contains one concise adopter-facing change and released history is byte-unchanged.

Update the command documentation to say workflow judgment, not conversational authorization, precedes the unchanged CLI command. Define the glossary approval boundary around pre-artifact outline approval rather than effort creation. Describe deliberate switching at the workflow altitude: checkpoint and keep, or transfer context, inspect, intentionally discard when necessary, remove topology, and archive through finish. Do not add an archive inventory, restore, abandoned status, or automatic destructive command.

### Task 1.3: Replace confirmation oracles with autonomous cross-target proof
Kind: batch
Applying: ["restore-autonomous-effort-management:create-efforts-autonomously", "restore-autonomous-effort-management:preserve-continuity-threshold", "restore-autonomous-effort-management:switch-deliberately", "restore-autonomous-effort-management:reuse-existing-lifecycle"]
Paths: ["internal/evals/independent_workflow_escalation_test.go", "internal/project/spine_test.go"]
Representative: `TestMandatoryApprovalBoundaries` retains ordered brainstorming approval assertions but proves that effort creation is autonomous and not an approval stop. `TestCheckpointDigestShape` reads the renamed creation partial and pins its positive transition. `TestActiveEffortCreationSignaturesStaySynchronized` preserves required explicit-slug signatures and rejects obsolete confirmation or reconfirmation prose across its existing active-source population.
Edge: Both Pi and Claude renderings must prove continuity-trigger independence, effort-free discovery, sole creation-command ownership, explicit identity reporting after creation, managed-worktree continuation, and deliberate keep/discontinue switching. Historical roots remain excluded from active-signature bans. Existing CLI, effort, worktree, and archive tests remain unchanged and green.
Post-check: Run `go test ./internal/evals -run 'Test(IndependentWorkflowEscalation|MandatoryApprovalBoundaries)$'` and `go test ./internal/project -run 'Test(CheckpointDigestShape|ActiveEffortCreationSignaturesStaySynchronized)$'`; all selected tests pass. Temporarily restore one obsolete later-response phrase in an in-memory or disposable fixture, confirm the relevant active-signature or projection oracle fails, restore only that falsification, and rerun to pass.

Assert semantic order rather than brittle counts: continuity materially helps; select faithful outcome, title, and canonical short slug; create with the unchanged explicit-slug command; report the allocated identity; continue in the managed worktree. Add negative assertions for asking the user to confirm, ending the turn for creation authorization, waiting for a later response, or reconfirming after context loss. Pin the switch alternatives and destructive-safety wording without requiring a new lifecycle state or force flag.

### Task 1.4: Apply authority, render every projection, and close green
Kind: batch
Applying: ["restore-autonomous-effort-management:create-efforts-autonomously", "restore-autonomous-effort-management:preserve-continuity-threshold", "restore-autonomous-effort-management:switch-deliberately", "restore-autonomous-effort-management:reuse-existing-lifecycle"]
Paths: ["docs/decisions/restore-autonomous-effort-management.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "docs/topics/rendering/workflow-skill-templates.md", "glob:.pi/skills/awf-*/SKILL.md", "glob:.claude/skills/awf-*/SKILL.md"]
Representative: Root Pi and Claude effort-workflow outputs describe the same autonomous creation and deliberate switch semantics with their target prefixes, while workflow and command docs retain intentional literal `<slug>` placeholders and contain no project-specific leakage.
Edge: Empty or unset interpolation remains coherent and token-free. Rendered files are changed only through authored sources. The ADR moves to `Implementing` and applies all three declared claim updates in this same transaction; it remains nonterminal, and the plan remains `Proposed` until terminal assurance and effort finalization.
Post-check: Run `./x render`, then `git diff --check`. Inspect `git diff --name-only` and require every change to be attributable to a declared authored source or generated family. Run the Task 1.1 obsolete-prose probe over active authored sources plus root generated workflow outputs and require zero findings. Read root Pi and Claude `effort-workflow`, workflow documentation, working-with-awf documentation, and glossary projections for contradictory fragments, concept-preserving paraphrase, intentional placeholders, and deliberate-switch safety. Run the focused tests from Task 1.3, `./x check`, and `./x gate`; all reach clean/pass terminal states.

Transition the ADR from Proposed to Implementing using the governed lifecycle command or placeholder-digest workflow, then append one Applied event listing exactly these operations in any order: update `rendering/workflow-skill-templates:mandatory-approval-boundaries`, update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`, and update `rendering/workflow-skill-templates:effort-workflow`. The matching claim mutations, ADR history, rendered topic, index, and lock travel in this transaction. Render all enabled targets and the example adopter; do not hand-edit generated outputs or broaden into CLI production code.

### Phase close

Return every implementation deviation and the focused semantic-rendering inspection evidence in the completed report. Leave plan frontmatter `status: Proposed` and the ADR `status: Implementing`; the parent supplies the completed report to report-only phase review and owns any focused Notes-and-findings settlement commit before checkpointing. Stage the complete authored, authority, test, changelog, ADR-event, lock, and rendered-output transaction explicitly, run `../../../awf check staged` and `./x gate`, and create the one phase-closing commit:

```commit
feat(rendering): restore autonomous efforts (applies ADR batch)
```

## Definition of done

- `dod: autonomous-creation` Every enabled target creates a faithfully named explicit-slug effort autonomously when durable continuity materially helps, reports its identity, and continues in its managed worktree without a creation approval stop.
- `dod: deliberate-switching` Switching away from an active effort is reasoned: the old effort is checkpointed for later continuation or necessary context is transferred before inspected safe cleanup or explicit intentional discard and ordinary archival finish.
- `dod: contract-proof` Deterministic tests reject obsolete confirmation and reconfirmation language while preserving effort-free discovery, sole lifecycle ownership, immutable explicit identity, lifecycle safety, and both rendered targets.
- `dod: publication-sync` The three declared claim updates are Applied with correct provenance; authored guidance, current documentation, glossary, adopter warning, Unreleased changelog, generated targets, example outputs, lock, drift check, and full 100%-coverage gate are synchronized and green.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Preserve the managed-worktree restriction only through pre-integration execution: eventual integration, ADR/plan terminal flips, topology removal, retrospective, and effort finish switch to the governed primary-checkout lifecycle owned by `effort-workflow`.

Phase-review settlement: report-only review found two mechanical proof gaps. The parent extended the active-source scanner and negative fixtures to reject creation-confirmation requests, turn-ending authorization, later-response authorization, and reconfirmation across the complete active path policy. It also replaced unordered switching keywords with ordered kept/discontinued/discard assertions and rejects switching-policy ownership outside `effort-workflow`. The corrections preserve the approved policy and add no runtime behavior.
