---
date: 2026-07-27
adrs: [162]
status: Proposed
---
# Plan: Remove the Pi dashboard and redesign workflow reports

## Goal

Implement [ADR-0162](../decisions/0162-remove-pi-dashboard-and-redesign-workflow-reports.md): replace the Pi dashboard with a slim local telemetry extension and bounded, effort-scoped CLI reports. Non-goals are rewriting resident ledgers, adding a local Pi report projection, retaining a compatibility dashboard command or top-level `awf doctor`, or claiming private-footer parity.

## Architecture summary

First add and test the new scoped report/list model as a compatibility preparation. Then atomically replace the two dashboard outputs with `awf-telemetry`, remove the old report grammar, and apply the matching ADR operations. Finally remove the dashboard runtime and private dispatch, update all remaining active documentation, and apply the final ADR operations. The generated telemetry extension keeps durable writes, association, router, handoff integration, shutdown draining, and the muted local bar but has no process transport or canonical refresh. Each behavior batch changes its matching current-state claims, authored documentation, and regenerated root and Sundial outputs in the same checked transaction.

## File structure

- **Created:** `docs/plans/2026-07-27-remove-pi-dashboard-and-redesign-workflow-reports.md`; new focused telemetry list/report tests where a separate file makes the contract clearer.
- **Modified:** `cmd/awf/{main.go,metrics.go,doctor.go,metrics_test.go}`; `internal/{clispec/clispec.go,telemetry/{types.go,reader.go,aggregate.go,render.go}}` and focused tests; `internal/project/{target.go,render.go,target_test.go,pi_workflow_render_test.go,output_plan_test.go,example_wiring_test.go}`; `templates/pi/awf-telemetry/{index.ts.tmpl,protocol.ts.tmpl}`; Pi extension harness tests; `x`; relevant `.awf/` current-state, domain, authored-doc, and metadata sources; `changelog/CHANGELOG.md`; root and Sundial generated artifacts and locks.
- **Deleted:** `cmd/awf/dashboardread*.go`; `cmd/awf-dashboard-launcher/`; `internal/dashboardruntime/`; `templates/pi/awf-dashboard/`; generated `.pi/extensions/awf-dashboard/` trees at root and Sundial; the dashboard-runtime topic metadata/current-state files; retired dashboard-specific tests and active documentation.

## Phase 1: Add selected-effort report and discovery primitives

- [ ] **Task 1.1: Specify failing Go tests for scoped reports and discovery.** Extend `cmd/awf/metrics_test.go`, `internal/telemetry/{aggregate_test.go,diagnostics_test.go,reader_test.go}`, and `internal/clispec` tests to cover these final shapes:

  ```text
  awf metrics --effort <id> [--session S] [--phase P] [--since T] [--until T] [--json]
  awf metrics doctor --effort <id> [--session S] [--phase P] [--since T] [--until T] [--json]
  awf metrics list [--limit N] [--cursor TOKEN] [--json]
  ```

  Require exactly one resident selected effort for each report; unknown, incompatible, or selector-empty selection fails without aggregating another effort. Require AND-combined selectors, selected-effort canonical JSON compatibility, and unchanged `metrics export` formats. Add list fixtures for creation-time descending order with byte-ascending ID ties, default limit 10 and maximum limit 100 (reject every value below 1 or above 100), opaque versioned base64url cursor round-trip, malformed/unsupported/tuple-mismatched/deleted cursor errors, no duplicate/skip after a non-ordering change, and next-cursor omission on the final page. A compatible row has ID, creation time, greatest valid applied-event timestamp, state, route, and phase/outcome/discovery. An unsupported-required-record effort is cursor-eligible but renders only ID, creation time, and `incompatible`; it cannot be selected. Run `go test ./internal/telemetry ./internal/clispec ./cmd/awf`; the new assertions initially fail.

- [ ] **Task 1.2: Implement the shared selected-effort/list model.** In `internal/telemetry/types.go`, add internal/public page and row types only as required to make text and JSON consume one deterministic result. In `reader.go`, enumerate resident metadata and records without filesystem mtime; derive the compatible row's last-applied time from valid applied events and retain an incompatible row without exposing its projection. In `aggregate.go`, retain canonical suppression of incompatible efforts, but provide a selected-effort adapter that returns the bounded selection error rather than a repository-wide result. In `render.go`, replace the broad human renderers with concise selected-effort metrics and doctor renderers and a one-row-per-effort list renderer. Keep normalized event export and the canonical selected projection unchanged. Add unit tests for discovery/unselected, open, terminal, and incompatible states. Run `gofmt -w internal/telemetry && go test ./internal/telemetry` successfully.

- [ ] **Task 1.3: Add CLI grammar and report wiring as a compatibility preparation.** In `internal/clispec/clispec.go` add closed `metrics doctor` and `metrics list` children, require `--effort` for the new report child, and keep export/maintenance children closed to their existing flags. In `cmd/awf/metrics.go`, parse the shared selectors once, route `metrics doctor` through `runDoctorWith`, and route list through the new page reader. Refactor `cmd/awf/doctor.go` into the reusable implementation. Leave the top-level doctor and private dashboard-read route unchanged only until Phase 3, because the generated dashboard still invokes them; tests must prove new public forms do not fall back to them. Update the report/list sections in `templates/docs/working-with-awf.md.tmpl`, `.awf/parts/working-with-awf/commands.md`, and matching Sundial authored sources to describe the added commands without falsely claiming old forms were removed. Update `tooling/cli:metrics-command-contract` in `.awf/topics/parts/tooling/cli/current-state.md` to add the selected-effort report/list grammar while retaining the still-supported top-level doctor bridge. Do not invoke lifecycle/state tools or append ADR-0162 status/Applied events while the known state-system fault remains; record the deferred ADR lifecycle reconciliation after that separate system is repaired. Run `./x render && ./x check`, `go test ./cmd/awf ./internal/clispec ./internal/telemetry`, and `./x gate`. Stage the CLI claim, root and Sundial generated output, and report documentation; run `./x check --staged`. Commit:

  ```commit
  feat(tooling): add scoped workflow reports and effort discovery
  ```

## Phase 2: Replace dashboard rendering with local Pi telemetry

- [ ] **Task 2.1: Write replacement extension tests before moving the template.** Rename `tools/pi-extension-test/tests/dashboard.test.ts` to `telemetry.test.ts`; update `tools/pi-extension-test/tests/{index,workflow,protocol,runtime,handoff}.test.ts` to import `.pi/extensions/awf-telemetry/{index,protocol}.ts` as applicable. Preserve tests for ledger confinement/durability, provisional settlement, lifecycle/router/adoption/detour behavior, handoff association, passive observations, shutdown drain, and public active-branch accounting. Add exact bar assertions: update only after successful explicit lifecycle/association actions; badge is phase/done/abandoned only; no canonical process call occurs; input/output/cache-read/cache-write/cost use public active-branch data when available; context uses public context data without recharging history; subscription and degraded suffixes are absent. Delete/replace tests for overlay, `/awf-dashboard`, canonical refresh generations, launcher fallback, query tools, compact query output, and dashboard-only maintenance. `./x pi-test` must fail until the replacement exists.

- [ ] **Task 2.2: Replace the two dashboard outputs with telemetry outputs.** Rename `templates/pi/awf-dashboard/` to `templates/pi/awf-telemetry/` and update `internal/project/target.go` declarations, `render.go`, output-plan tests, and target tests so Pi still renders exactly five extension files: subagents, handoff, telemetry index, and descriptor-derived telemetry protocol. Preserve the descriptor input attribution and ordinary lock/cleanup behavior. In the telemetry index retain only the writer, local lifecycle projection, association/provisional manager, lifecycle/adoption/detour/router tools, passive event handling, local widget, and shutdown drain. Remove `DashboardState`, canonical executable resolution, refresh state, overlay, dashboard command, metrics/doctor tools, repair/waiver/retention/purge controls, and every `exec` dependency used solely for reads. Ensure the handoff template continues to obtain the validated active association through the replacement extension.

- [ ] **Task 2.3: Apply the Pi-output operations, documentation, and commit.** Update `rendering/pi-workflows:pi-session-handoff-public-contract` so handoff names the validated telemetry association; replace `rendering/pi-workflows:pi-workflow-dashboard-public-contract` with `pi-workflow-telemetry-public-contract`, and replace `rendering/adapter-outputs:pi-workflow-dashboard-runtime` with `pi-workflow-telemetry-runtime` in their `.awf/topics/parts/**/current-state.md` files. The new adapter-output invariant has `Backing: test`; define it with the replacement telemetry-runtime contract, preserve its Origin/Revision history, and put this exact proof marker on the behavioral generated-output/runtime test in `internal/project/target_test.go`: `// invariant: rendering/adapter-outputs:pi-workflow-telemetry-runtime`. Update the corresponding `rendering/pi-runtime` output-count/minimum-runtime/smoke claims and authored Pi architecture/testing guidance in this same transaction: `.awf/parts/{agents-doc/identity.md,workflow/chain.md,working-with-awf/commands.md}`, `.awf/docs/parts/architecture/{overview,components,data-flow,dependencies}.md`, `.awf/docs/parts/testing/layout.md`, `.awf/docs/parts/pitfalls/prepend.md`, and their exact Sundial counterparts. When the separate state-system repair is complete, reconcile ADR-0162 lifecycle state with an Applied event for exactly these operations: `update tooling/workflow-telemetry:canonical-projections-and-diagnostics`; `update rendering/pi-workflows:pi-session-handoff-public-contract`; `remove rendering/pi-workflows:pi-workflow-dashboard-public-contract`; `add rendering/pi-workflows:pi-workflow-telemetry-public-contract`; `remove rendering/adapter-outputs:pi-workflow-dashboard-runtime`; `add rendering/adapter-outputs:pi-workflow-telemetry-runtime`; `update rendering/pi-runtime:pi-extension-target-render`; `update rendering/pi-runtime:pi-minimum-runtime`; and `update rendering/pi-runtime:pi-real-runtime-smoke`. Use the state-sequence value reported at reconciliation time, retain the Phase 3 operations as Remaining, and regenerate `docs/decisions/INDEX.md`. Until then, do not invoke the state tools; this does not block the code/documentation commit. Run `./x render && ./x check`, then `./x pi-test` and `./x gate`. Stage the authored sources, root and Sundial generated telemetry files, removed dashboard files, and regenerated locks/docs; run `./x check --staged`. Commit:

  ```commit
  feat(rendering): replace the Pi dashboard with local telemetry
  ```

## Phase 3: Retire dashboard transports and enforce final CLI grammar

- [ ] **Task 3.1: Remove the now-unreachable dashboard read/runtime surface.** Delete `cmd/awf/dashboardread.go` and its tests, delete `cmd/awf-dashboard-launcher/`, and delete all of `internal/dashboardruntime/`, including `runnercmd` and platform-specific helpers/tests. Remove the pre-gate dispatch branch from `cmd/awf/main.go`; remove the `dashboard-awf-path` and `dashboard-awf-advance` cases and help text from `x`. Delete the dashboard runtime topic from `.awf/domains/tooling.yaml`, `.awf/topics/metadata/tooling/dashboard-runtime.yaml`, and its authored current-state file. Mechanically verify no live source remains with:

  ```sh
  rg -n 'dashboard-read|dashboard-awf-(path|advance)|cmd/awf-dashboard-launcher|internal/dashboardruntime' \
    cmd internal templates tools x --glob '!templates/pi/awf-telemetry/**'
  ```

  The command must return no matches after intentional template/path renames are included.

- [ ] **Task 3.2: Remove transitional CLI compatibility and finalize reports.** Delete the top-level `doctor` command grammar and `runDoctor` entrypoint; retain only `awf metrics doctor --effort <id>`. Remove the temporary top-level-doctor/dashboard-read test cases and add exact help/error tests proving bare `awf metrics`, bare `awf doctor`, and reports without effort are rejected. Preserve `metrics export`, lifecycle, retention, and purge behavior exactly unless their grammar is dashboard-only. Run `go test ./cmd/awf ./internal/clispec ./internal/telemetry` and `./x gate` successfully.

- [ ] **Task 3.3: Apply final removals, documentation, and ADR status in one commit.** Update `tooling/workflow-telemetry:{event-protocol-and-ledger,privacy-integrity-and-retention}`; update `tooling/cli:version-compat-gate`; remove `tooling/dashboard-runtime:pinned-development-runtime-cache`, `tooling/cli:doctor-command-contract`, `tooling/cli:dashboard-read-dispatch`, `rendering/companion-scripts:dashboard-development-runtime-commands`, and `rendering/pi-runtime:pi-pinned-development-runtime`. Preserve Origin/Revised-by history and add ADR-0162 provenance. Update proof markers with focused Go/Pi tests.

  In this same transaction update the remaining active documentation at `.awf/docs/parts/development/{command-runner,dependencies,setup}.md`, `.awf/docs/parts/{releasing/content,testing/layout}.md`, `README.md`, `internal/configspec/{spec.go,spec_test.go}`, rendering/tooling domain prose, and exact Sundial counterparts. Add an Unreleased breaking-change entry to `changelog/CHANGELOG.md` stating that the Pi dashboard, Pi query tools, top-level `awf doctor`, and dashboard runtime commands are removed, and that scoped reports use `awf metrics --effort`, `awf metrics doctor --effort`, and `awf metrics list`. Regenerate root and Sundial output and locks.

  After the separate state-system repair, append the final Applied event and Implemented status in the same ADR-0162 reconciliation transaction for exactly these remaining operations: `update tooling/workflow-telemetry:event-protocol-and-ledger`; `update tooling/workflow-telemetry:privacy-integrity-and-retention`; `remove tooling/dashboard-runtime:pinned-development-runtime-cache`; `update tooling/cli:version-compat-gate`; `remove tooling/cli:doctor-command-contract`; `remove tooling/cli:dashboard-read-dispatch`; `remove rendering/companion-scripts:dashboard-development-runtime-commands`; and `remove rendering/pi-runtime:pi-pinned-development-runtime`. Use checker-reported sequence values and regenerate `docs/decisions/INDEX.md`. Do not block this implementation commit on that deferred reconciliation and do not invoke the broken state tools.

  Run `./x render && ./x check`, `./x pi-test`, and `./x gate`. Then run this exact active-surface audit, which must print no matches:

  ```sh
  rg -n -i '/awf-dashboard|awf_metrics|awf_doctor|dashboard-read|dashboard-awf-(path|advance)|internal/dashboardruntime|cmd/awf-dashboard-launcher|awf doctor' \
    README.md AGENTS.md docs .awf templates .pi examples/sundial examples/sundial/.pi \
    --hidden --glob '!docs/decisions/**' --glob '!docs/plans/**' \
    --glob '!examples/sundial/docs/decisions/**' --glob '!examples/sundial/docs/plans/**' \
    --glob '!**/awf.lock'
  ```

  Stage the complete source, generated, and changelog transaction; run `./x check --staged` and `./x gate`, then commit. Reconcile and stage ADR-0162 plus `docs/decisions/INDEX.md` only after the separate state-system repair:

  ```commit
  feat(tooling): retire dashboard report transports
  ```

## Phase 4: Review and freeze

- [ ] **Task 4.1: Review implementation against ADR-0162 and this plan.** From a clean committed tree, set `<implementation-base>` to the parent of the Phase 1 implementation commit and `<implementation-head>` to `HEAD`; record both resolved SHA values. Run `awf audit <implementation-base>..<implementation-head>` and `./x audit-local <implementation-base>..<implementation-head>`, resolving every Error before review. Run `awf context --full $(git diff --name-only <implementation-base>..<implementation-head>)`. Invoke `subagent_review` kind `code` with the concrete SHA range, ADR-0162, this plan path and goal, the full-context output, and the requirement to test dashboard retirement, scoped-report behavior, current-state operations, generated root/Sundial output, and documentation fan-out. Resolve mechanical and reasoned findings in new checked commits; stop for user-decision findings; each fix updates its matching documentation and passes `./x check --staged` and `./x gate`. Recompute `<implementation-head>`, repeat audit/context, and run one final `subagent_review` kind `code` over the updated concrete range; the terminal result is zero findings and a clean `git status --short`.

- [ ] **Task 4.2: Freeze implementation records.** Set this plan to `Implemented`, append concise review/verification outcomes to Notes, run `./x render && ./x check`, stage this plan and regenerated files only if changed, run `./x check --staged` and `./x gate`, and commit:

  ```commit
  docs(plans): implement dashboard removal report plan
  ```

## Verification

- `./x check`, `./x check --staged`, `./x pi-test`, and `./x gate` pass for every committed phase.
- `awf metrics --effort <id>`, `awf metrics doctor --effort <id>`, and list pagination have deterministic text and JSON tests; selected-effort JSON/export compatibility remains covered.
- Pi tests prove lifecycle-local badge updates and public usage accounting without canonical reads, dashboard command/tool registration, or an overlay.
- Root and Sundial render/check tests prove stale dashboard outputs are pruned and the telemetry descriptor remains attributed.
- Active sources contain no dashboard transport/runtime/query guidance; historical records remain untouched.

## Notes

The Phase 1 compatibility preparation is deliberately short-lived: it keeps the existing generated dashboard operational while the new CLI contract is introduced. It is deleted in Phase 3 and is never documented as the final interface. No lifecycle/state operation is attempted during current planning or implementation while the known fault remains. ADR-0162 status and Applied-event reconciliation is deferred until the separate state-system repair is complete; it does not block this work.
