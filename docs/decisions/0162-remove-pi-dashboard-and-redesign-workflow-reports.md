---
format: current-state-v2
status: Implemented
date: 2026-07-27
---
# ADR-0162: Remove the Pi dashboard and redesign workflow reports

## Context

Pi's workflow telemetry dashboard duplicates an interface that does not justify its runtime,
maintenance, and context cost. It depends on canonical metrics and doctor refreshes, a private
`dashboard-read` CLI dispatch, an immutable launcher, and a pinned development-runtime cache.
Those reads are needed only to populate the overlay and dashboard-only maintenance controls.
Removing the overlay leaves the Pi query tools without a transport; retaining them would require
inventing a second local aggregation path.

The useful parts are smaller: durable lifecycle writes, association and workflow routing, and the
muted below-editor status and cost bar. Reports still need to be useful, but unscoped default
reports can flood an agent's context and can accidentally expose repository-wide state. The CLI
has canonical JSON and export formats that scripts need, but it lacks a concise, effort-specific
human report and a bounded way to discover resident efforts.

Two observed failures confirm that the dashboard's dependency chain is the wrong boundary. Its
resolution was degraded when `awf` was absent from PATH. The runner fallback could resolve through
`./x dashboard-awf-path`, but an already-recorded startup failure remained visible instead of
clearing automatically. Separately, the awf bar's independent active-branch usage accounting did
not have guaranteed parity with Pi's built-in usage and context display.

## Decision

1. Remove the Pi dashboard TUI and all of its machinery. `/awf-dashboard`, its overlay,
   canonical-refresh loop, dashboard-only maintenance controls, the private `dashboard-read`
   dispatch, `cmd/awf-dashboard-launcher`, `internal/dashboardruntime`, the pinned dashboard
   runtime/cache and ref, and Pi `awf_metrics` and `awf_doctor` query tools are removed. Remove
   `./x dashboard-awf-path` and `./x dashboard-awf-advance`, including their runner help,
   release procedure, tests, and every fallback or advertisement that invokes them. There is no
   compatibility alias, local historical aggregation, background refresh, or replacement TUI.

2. Retain a slim Pi telemetry extension for lifecycle mutations, effort association and adoption,
   detours, workflow-router integration, shutdown draining, and the muted below-editor bar. The
   bar keeps the existing phase-oriented badge, updates only after a successful explicit
   lifecycle or association action, and never uses canonical metrics or doctor reads. It has no
   degraded-message suffix or report-resolution state; a failed explicit action reports through
   that action without changing the badge.

3. Define bar usage from Pi's public active-branch usage and public context APIs only. Count each
   unique active-branch assistant entry once, including Pi-public restored and nested-subagent
   usage where the API exposes it; obtain current context from the public context API without
   recharging history. Retain input, output, cache-read, cache-write, and cost fields when their
   public usage and pricing inputs are available, but drop the subscription label. The extension
   must not import private footer state or claim display parity that Pi's public API does not
   guarantee. Tests pin the public-source accounting and state the remaining difference from Pi's
   built-in display explicitly rather than treating it as an implicit parity promise.

4. Make the human report commands effort-scoped: `awf metrics --effort <id>` and
   `awf metrics doctor --effort <id>`. Remove top-level `awf doctor`. Both commands require one
   existing resident effort ID and continue to accept the existing session, phase, since, and
   until selectors; all supplied selectors combine with logical AND inside that effort only. A
   missing, unknown, incompatible, or selector-filtered-out ID is a bounded error and never falls
   back to repository-wide output. Default output is concise, deterministic, and human-readable:
   metrics reports the effort identity, lifecycle state, route, open phase, terminal outcome, or
   `discovery` when neither exists, current-path and all-work usage/counters for the selected
   projection, and bounded diagnostics summary; doctor reports that effort's severity/rule and
   integrity summaries plus bounded actionable findings. The selected effort's existing canonical projection remains available
   with `--json`, and supported metrics export formats remain unchanged.

5. Add `awf metrics list [--limit N] [--cursor TOKEN] [--json]` as the only unscoped telemetry
   discovery command. It lists every resident on-disk effort newest-first by immutable creation
   time, breaking equal timestamps by effort ID in ascending byte order. Its default limit is 10
   and its maximum limit is 100. A compatible row has ID, creation
   time, last-applied-event time, lifecycle state, route, and open phase, terminal outcome, or
   `discovery` when neither exists. Last-applied-event time is the greatest timestamp among the
   effort's valid applied ledger events, and is never filesystem mtime. An effort suppressed by
   an unsupported required record remains discoverable and cursor-eligible, but emits the fixed
   bounded `incompatible` state with its ID and immutable creation time only; it has no
   lifecycle, route, phase, outcome, or event-time fields, and selecting it for either scoped
   report is a bounded incompatibility error. Text output has one concise row per effort and a
   next cursor when another page exists. JSON serializes the same page and cursor without
   changing the canonical selected-effort projection.

6. A list cursor is an opaque, deterministic base64url encoding of a versioned canonical payload
   containing only the ordering tuple of the last returned row: format version, creation time,
   and effort ID. It is not report data, but it identifies a required resident pagination
   location, including an incompatible row, and is validated strictly against that effort's
   immutable creation tuple. An empty, malformed, unsupported-version, tuple-mismatched, or
   no-longer-resident cursor is a bounded unknown-location error. Pagination is explicit: it
   never guesses, resumes past a deleted boundary, duplicates, or skips a row because an effort
   changed after a prior page. A caller must restart listing after retention or purge deletes its
   cursor location.

7. Redesign generated Pi outputs, runner and release surfaces, tests, and documentation together.
   The Pi target no longer renders dashboard protocol or UI files; its reduced telemetry output
   has no dashboard-runtime dependency. Remove dashboard-specific runner verbs, cache and release
   checks, private dispatch tests, and real-Pi dashboard smoke. Replace them with coverage for
   lifecycle-local badge updates, no canonical reads, public-usage source accounting, required
   effort selectors, selected-effort JSON compatibility, report/list formatting and pagination,
   and the absence of retired Pi commands, tools, generated outputs, launcher, runtime cache,
   private dispatch, and runner machinery. The new
   `rendering/adapter-outputs:pi-workflow-telemetry-runtime` claim has `Backing: test` with a
   proof marker on the generated-output/runtime test that establishes that contract.

8. Each implementation batch changes the matching current-state claims in the same checked
   transaction. Every status transition runs `./x render` and stages the regenerated
   `docs/decisions/INDEX.md`; implementation also removes all rendered documentation and source
   references to the retired dashboard surfaces rather than leaving them as active guidance.

## State changes

- update `tooling/workflow-telemetry:event-protocol-and-ledger`
- update `tooling/workflow-telemetry:privacy-integrity-and-retention`
- update `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- remove `tooling/dashboard-runtime:pinned-development-runtime-cache`
- update `tooling/cli:version-compat-gate`
- update `tooling/cli:metrics-command-contract`
- add `tooling/cli:metrics-legacy-doctor-bridge`
- remove `tooling/cli:doctor-command-contract`
- remove `tooling/cli:dashboard-read-dispatch`
- remove `rendering/companion-scripts:dashboard-development-runtime-commands`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- remove `rendering/pi-workflows:pi-workflow-dashboard-public-contract`
- add `rendering/pi-workflows:pi-workflow-telemetry-public-contract`
- remove `rendering/adapter-outputs:pi-workflow-dashboard-runtime`
- add `rendering/adapter-outputs:pi-workflow-telemetry-runtime`
- remove `rendering/pi-runtime:pi-pinned-development-runtime`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`

## Consequences

Pi telemetry becomes smaller and no longer has a report-resolution lifecycle distinct from its
workflow lifecycle. A missing `awf` executable cannot leave a stale dashboard-resolution error,
because no Pi report transport invokes it. The bar remains useful while its values are honest
about the public API semantics available to it.

Operators and agents use effort-scoped CLI reports rather than an overlay. Discovery requires an
explicit list command and pagination, so routine report calls remain bounded and an unknown ID
cannot become an accidental repository-wide query. Scripts retain the selected-effort canonical
JSON and export interfaces.

The change removes a development cache, launcher, private CLI grammar, generated UI, and their
release/test burden, but it requires coordinated removal across Go, templates, rendered outputs,
documentation, and current-state authority. Existing resident ledgers are not rewritten.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the dashboard and repair resolution retry | It retains a large private runtime and overlay solely to serve reports that the CLI can express directly. |
| Keep Pi metrics and doctor tools with local aggregation | A second projection would diverge from canonical telemetry and recreates context-flood risks. |
| Permit bare `awf metrics` or repository-wide doctor | Default unscoped output can be large and makes an agent's intended effort ambiguous. |
| Add a full-output switch to Pi tools | It moves canonical detail back into model context rather than providing a bounded operator interface. |
| Promise exact visual parity with Pi's footer | Public APIs do not guarantee every built-in display semantic; an unsupported promise would conceal observed differences. |

## Status history

- 2026-07-27: Proposed
- 2026-07-27: Implementing; content-sha256: b92150c006e08e8191cc753101824b35a2ed50f8e65c6facda645c9a90c8f6c8
- 2026-07-27: Applied; state-sequence: 57; operations: update `tooling/cli:metrics-command-contract`, add `tooling/cli:metrics-legacy-doctor-bridge`
- 2026-07-27: Applied; state-sequence: 58; operations: update `tooling/workflow-telemetry:canonical-projections-and-diagnostics`, update `rendering/pi-workflows:pi-session-handoff-public-contract`, remove `rendering/pi-workflows:pi-workflow-dashboard-public-contract`, add `rendering/pi-workflows:pi-workflow-telemetry-public-contract`, remove `rendering/adapter-outputs:pi-workflow-dashboard-runtime`, add `rendering/adapter-outputs:pi-workflow-telemetry-runtime`, update `rendering/pi-runtime:pi-extension-target-render`, update `rendering/pi-runtime:pi-minimum-runtime`, update `rendering/pi-runtime:pi-real-runtime-smoke`
- 2026-07-27: Applied; state-sequence: 59; operations: update `tooling/workflow-telemetry:event-protocol-and-ledger`, update `tooling/workflow-telemetry:privacy-integrity-and-retention`, remove `tooling/dashboard-runtime:pinned-development-runtime-cache`, update `tooling/cli:version-compat-gate`, remove `tooling/cli:doctor-command-contract`, remove `tooling/cli:dashboard-read-dispatch`, remove `rendering/companion-scripts:dashboard-development-runtime-commands`, remove `rendering/pi-runtime:pi-pinned-development-runtime`
- 2026-07-27: Implemented; content-sha256: b92150c006e08e8191cc753101824b35a2ed50f8e65c6facda645c9a90c8f6c8
