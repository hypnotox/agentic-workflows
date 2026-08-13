---
format: current-state-v4
slug: select-gate-test-suites-independently
status: Proposed
date: 2026-08-13
---
# ADR-0276: Select Gate Test Suites Independently


## Context

ADR-0275 introduced staged-path test selection, but couples the Go suite to every
non-documentation transaction. A change confined to the generated Pi extension therefore runs the
profiled repository-wide Go suite even when no Go-tested surface changed. The two suites have
separate dependency sets and can be selected independently without weakening unconditional vet,
cross-build, lint, dead-code, and pin checks.

The dependency sets overlap. Pi templates are embedded and rendered by Go tests; generated Pi
agents and skills are read by Go golden and guidance tests; shared renderer, configuration, and
catalog code affects both lanes; and parts of the Pi harness are inspected by Go tests. Those
surfaces must select both suites rather than being classified by their Pi-facing name alone.

## Decision

1. `decision: independent-suite-selection` The gate selects the profiled Go suite with its coverage check and the Pi runtime suite independently from the same staged-index snapshot. Documentation-only changes select neither, Pi-only changes select only Pi, Go-only changes select only Go, and changes spanning both dependency sets select both.
2. `decision: dependency-based-overlap` A path selects every suite that directly tests or depends on it. Pi extensions and harness inputs without Go-test consumers may remain Pi-only; ordinary Go and Claude-only surfaces may remain Go-only; Pi templates, generated Pi agents and skills, shared rendering and configuration surfaces, the runner, and Pi harness files consumed by Go tests select both.
3. `decision: independent-selection-fails-closed` Empty, unreadable, malformed, or unrecognized staged snapshots select both suites. New or uncertain paths remain in this fail-closed class until repository evidence justifies a narrower category.
4. `decision: non-test-gates-remain-unconditional` Suite selection remains only a runtime optimization. Vet, released-platform builds, lint, dead-code analysis, and the workflow-pin check continue to run for every gate invocation.
5. `decision: independent-selection-observability` The gate prints an explicit notice for each skipped suite, and timed mode reports only stages that execute.

## State changes

- update `tooling/quality-gates:staged-test-selection`
- update `tooling/quality-gates:pi-extension-container-gate`

## Consequences

Pi-only extension and harness changes avoid the repository-wide Go suite, while Go-only changes
continue to avoid the Pi runtime lane. Shared and uncertain changes retain both suites. The path
taxonomy must follow actual test dependencies and can deliberately accept false positives where a
narrow category is not proven.

The classifier becomes more explicit because every recognized path category supplies two selection
bits rather than deriving Go selection from the absence of a documentation-only classification.
This makes additions to the repository fail closed instead of silently inheriting an unrelated
suite category.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep Go tests for every non-documentation change | Retains avoidable Go-suite runtime for changes confined to Pi-only dependencies. |
| Classify all Pi-named paths as Pi-only | Skips Go tests that directly validate embedded Pi templates, generated agents and skills, and parts of the harness. |
| Default unmatched files to one suite | Risks silently skipping the other suite when a new dependency surface is introduced. |

## Status history

- 2026-08-13: Proposed
