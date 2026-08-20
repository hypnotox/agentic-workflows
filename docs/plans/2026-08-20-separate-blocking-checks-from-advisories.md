---
format: plan-v2
date: 2026-08-20
adrs: [separate-blocking-checks-from-advisories]
status: Proposed
---
# Plan: Separate Blocking Checks From Advisories

## Goal

Make every current check and repository advisory visibly Error, Warning, or unranked Information according to its protected property, with nonzero exit only for Error, while preserving serious failures and the existing two-rank model. Do not add configurable severity or perform RF-004 checker decomposition.

## Architecture summary

Keep the existing check producers and aggregation. Preserve `internal/severity` as the two-member ranked finding model, retain provenance for Warning versus unranked Information through the existing project and command result boundaries, and translate those meanings into separate readable presentation categories. Split the repository's lint execution into blocking defect rules and a visible zero-exit advisory lane using the classifications fixed by ADR-separate-blocking-checks-from-advisories. Update the active claims and generated documentation in the same transaction as the behavior they describe; no new checker framework, plugin surface, or policy configuration is introduced.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what this plan may not change. Tasks below record the best known route; a commit-capable owner may revise local representation and test arrangement while preserving the linked ADR, serious-failure behavior, generated-source ownership, and verification strength.

## Phase 1: Classify checks and repository advisories

**Execution mode: inline.**

Completes: ["check-labels-and-exits", "serious-blockers-preserved", "advisory-lint-visible", "authority-and-docs-current"]

### Task 1.1: Establish exit and presentation oracles
Applying: ["separate-blocking-checks-from-advisories:errors-protect-validity", "separate-blocking-checks-from-advisories:judgement-warns", "separate-blocking-checks-from-advisories:optional-notes-inform", "separate-blocking-checks-from-advisories:aggregate-remains-actionable"]
Paths: ["cmd/awf/check_presentation_test.go", "cmd/awf/checkrepo_test.go", "cmd/awf/check_test.go", "cmd/awf/prosegate_test.go", "internal/project/check_test.go", "internal/project/unused_test.go", "internal/project/gate_runner_test.go"]

Add focused tests that initially expose the current defects and then remain as durable behavior oracles. Cover direct and aggregate Error-only, Warning-only, Information-only, and mixed output; prove nonzero exit exactly when an Error is present; prove prose findings warn while prose preparation failures still block; prove unused variables and data inform; prove glossary, tag, plan-detail, fan-out, and guide-size findings warn; and prove unset, stub, marker, tracking, compatibility, and provisional notes inform. Preserve tests for serious drift, current-state, memory, commit, version, verification, and operational failures.

For the runner, extend the existing gate wiring tests to prove the defect lint lane remains blocking, the advisory lane is executed and visible without failing the gate for findings, execution failures remain blocking, and both configured rule sets match the ADR inventory. The oracle must fail against the current single blocking lint invocation before the runner change.

### Task 1.2: Preserve severity meaning through check aggregation
Applying: ["separate-blocking-checks-from-advisories:errors-protect-validity", "separate-blocking-checks-from-advisories:judgement-warns", "separate-blocking-checks-from-advisories:optional-notes-inform", "separate-blocking-checks-from-advisories:fixed-ranks-preserved", "separate-blocking-checks-from-advisories:aggregate-remains-actionable"]
Paths: ["internal/project/check.go", "internal/project/currentstate.go", "internal/project/check_presentation.go", "internal/prosegate/presentation.go", "internal/prosegate/presentation_test.go", "cmd/awf/check_presentation.go", "cmd/awf/checkrepo.go", "cmd/awf/checkstaged.go", "cmd/awf/prosegate.go"]

Carry Warning and unranked Information separately through the existing operation reports without adding a shared rank or decomposing checker ownership. Route only `unused-var` and `unused-data` away from failing drift, and make repository failure depend on the remaining Error drift rather than raw drift length. Route prose findings to Warning while retaining nonzero operational failures. Keep direct and aggregate ordering, plan-note deduplication, universe separation, and serious-failure behavior intact. Render deterministic `errors`, `warnings`, then `information` categories and include all three in the summary while Information alone retains completed success.

### Task 1.3: Split blocking and advisory lint lanes
Applying: ["separate-blocking-checks-from-advisories:errors-protect-validity", "separate-blocking-checks-from-advisories:judgement-warns", "separate-blocking-checks-from-advisories:fixed-ranks-preserved"]
Paths: [".golangci.yml", ".golangci-advisory.yml", "x", "internal/project/gate_runner_test.go"]

Keep only the ADR's concrete defect rules in the blocking golangci configuration. Put its enumerated style, performance, formatting, preferred-idiom, possible-cohesion, and maintainability rule sets in the advisory configuration. Run both in deterministic gate order: a blocking finding or either tool's execution/configuration failure stops the gate, while advisory findings print an explicit Warning and allow later checks to run. Preserve timings, staged lane selection, dead-code and pin ordering, and the existing `set -e` safety semantics.

### Task 1.4: Apply authority and document the inventory
Kind: batch
Applying: ["separate-blocking-checks-from-advisories:errors-protect-validity", "separate-blocking-checks-from-advisories:judgement-warns", "separate-blocking-checks-from-advisories:optional-notes-inform", "separate-blocking-checks-from-advisories:fixed-ranks-preserved", "separate-blocking-checks-from-advisories:aggregate-remains-actionable"]
Paths: [".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/parts/tooling/audit-commands/current-state.md", ".awf/topics/parts/rendering/inplace-and-placeholders/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/docs/parts/testing/gate.md", ".awf/parts/working-with-awf/commands.md", "changelog/CHANGELOG.md", "docs/decisions/separate-blocking-checks-from-advisories.md", "docs/plans/2026-08-20-separate-blocking-checks-from-advisories.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "docs/topics/tooling/cli.md", "docs/topics/tooling/quality-gates.md", "docs/topics/tooling/audit-commands.md", "docs/topics/rendering/inplace-and-placeholders.md", "docs/topics/rendering/doc-outputs.md", "docs/topics/config/configuration.md", "docs/topics/adr-system/plan-artifacts.md", "docs/topics/rendering/sync-and-drift.md", "docs/testing.md", "docs/working-with-awf.md"]
Post-check: Apply the exact operation set below, run `./x render`, and require `./awf check`, `./awf check staged`, and `./x gate` to reach clean success with no lifecycle-authorized residual findings. Inspect the rendered Severity sections in `docs/working-with-awf.md` and `docs/testing.md` plus all eight changed topic pages, confirming that Error names a protected property, Warning and Information remain distinct, no old failing-style claim survives, and no generated path outside the declared set changed.

Apply every declared ADR operation in one Applied event with exactly this membership: add `tooling/cli:check-severity-by-protected-property`; add `tooling/quality-gates:gate-severity-by-protected-property`; update `tooling/cli:repo-check-capability-plan`; update `tooling/cli:terseness-advisory-nonfailing`; update `tooling/audit-commands:severity-single-spelling`; update `tooling/quality-gates:prose-gate-tracked-file-scan`; update `rendering/inplace-and-placeholders:unused-var-drift`; update `rendering/inplace-and-placeholders:unused-data-drift`; update `rendering/doc-outputs:glossary-terseness-advisory`; update `config/configuration:tag-coverage-note`; update `config/configuration:tag-frequency-note`; update `adr-system/plan-artifacts:plan-v2-assignment-advisories`; update `rendering/sync-and-drift:agent-guide-size-advisory`.

The current-state claims must name each Error's correctness, safety, authority, or reproducibility property; distinguish Warning from Information; retain the two shared ranks; and state the lint lane exit contract. Update the maintained command and testing sources with the exhaustive inventory and actionable labels, add the adopter-facing changelog entry, render all generated outputs and locks from their sources, and inspect the generated command and gate sections for contradictory old claims. Keep the audit remediation program's completion status for the orchestrator's serialized integration transaction.

### Phase close

Land one coherent behavior, test, claim, and documentation transaction after the ADR has entered Implementing and its complete operation set is Applied.

```commit
feat(awf): separate blocking checks from advisories
```

## Definition of done

- `dod: check-labels-and-exits` Direct and aggregate CLI output visibly separates Error, Warning, and unranked Information, and tested exit status is nonzero exactly when an Error exists.
- `dod: serious-blockers-preserved` Invalid inputs, serious drift, broken authority or references, unsafe lifecycle behavior, compatibility failures, and unavailable required verification remain blocking under focused regression tests.
- `dod: advisory-lint-visible` Every enabled lint rule has the ADR's protected-property classification; defect rules block, style and heuristic findings are visible Warning output with success, and lint execution failure blocks.
- `dod: authority-and-docs-current` All ADR operations are Applied with backed claims, generated documentation and locks are current, the changelog describes the adopter-visible change, and `./awf check`, `./awf check staged`, and `./x gate` pass.

## Notes

Apply the plan-flexibility rule above when recording deviations. Keep all implementation inside AF-013 and leave checker decomposition to RF-004.

- Implementation added `internal/presentation/shapes.go` and its existing report tests because the closed report-category boundary previously rejected `information`; changing only check presentation would have created a parallel invalid shape. It also edited `templates/docs/working-with-awf.md.tmpl` because the local commands part delegates to that source through `sectionDefault`. Both paths are required by existing presentation and generated-source authority and do not change the approved boundary.
- `CheckReport.Notes`, `TrackingNotes`, and `PlanNotes` remain as compatibility projections for existing internal callers while new operation reports carry Warning and Information separately. RF-004 may remove that residue when it decomposes checker ownership; AF-013 does not expand into that refactor.
- Review added `internal/configspec/spec.go`, `internal/project/configreference.go`, and generated `docs/config-reference.md` because those semantic owners still described unused vocabulary as failing drift. Correcting them completes AF-013's exhaustive inventory without changing configuration behavior.
- Terminal assurance added `.golangci-advisory.yml` to `.awf/config.yaml`'s `contextIgnore` inventory and kept `./x fmt` on the configuration that owns `goimports`. These settlements preserve path attribution and formatter behavior after the lint-lane split.
