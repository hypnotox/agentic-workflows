---
format: current-state-v4
slug: advance-pi-runtime-floor-to-0-84-2
status: Proposed
date: 2026-08-16
---
# ADR-0283: Advance Pi Runtime Floor to 0.84.2


## Context

Awf's retained effort extension declares Pi 0.81.1 as its minimum runtime and verifies that boundary
against a checksummed 0.81.1 fork artifact. The generated profile adapter separately negotiates
protocol-v2 compatibility with independently installed pi-tools and does not use a pi-tools package
revision as an adopter runtime requirement.

Pi-tools v0.3.0 introduced the shared source-only extension recorder that awf intends to consume for
generic Pi-boundary tests. That recorder is developed and verified against Pi 0.84.2. Because its
package export points to TypeScript source, importing it under awf's 0.81.1 dependency graph checks
the recorder against the older Pi types and fails at the first newer `ExtensionContext` member.
Merely raising a rendered version string would leave the pinned proof graph and the declared runtime
contract inconsistent.

The maintained Pi fork already publishes `fork-v0.84.2.2` with embedded upstream package version
0.84.2. Pi 0.84.2 retains the active-tool, command-queue, and file-mutation-queue capabilities used
by awf. A clean temporary graph aligned on Pi 0.84.2 and pi-tools v0.3.0 type-checked and passed all
existing Pi tests before this decision was proposed.

## Decision

1. `decision: pi-0-84-2-floor` Set Pi 0.84.2 as the minimum supported runtime for awf's retained Pi effort integration. Older Pi runtimes are no longer supported by that output.
2. `decision: aligned-pi-verification` Verify the retained Pi integration against one coherent Pi 0.84.2 SDK family while keeping independently installed adopter pi-tools compatibility protocol-based rather than revision-based.
3. `decision: source-only-recorder-boundary` Permit the Pi test lane to pin and consume the source-only `pi-tools/testing` boundary for generic Pi API recordings without importing or behavior-testing the adopter pi-tools runtime.
4. `decision: six-profile-registration` Preserve atomic protocol-v2 registration of the current six governed awf profiles; the runtime-floor change does not alter profile policy or behavior.

## State changes

- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`
- update `rendering/pi-runtime:pi-tools-integration-boundary`

## Consequences

The Pi runtime declaration, pinned SDK proof, and reusable recorder compile against the same API
generation. Awf can remove generic hand-built Pi test seams in favor of their shared owner while
keeping filesystem, Git, routing, effort, and transport policy local.

Adopters using Pi 0.81.1 must upgrade before using the retained effort output. The dependency update
therefore requires renewed strict type checking, extension coverage, and real SDK smoke evidence for
profile negotiation and effort integration. The source-only recorder pin is test infrastructure; it
does not become a pi-tools runtime pin or broaden awf's assurance into external scheduling, child
execution, confinement, or presentation mechanics.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Backport the recorder to Pi 0.81.1 | It preserves an obsolete awf runtime floor and makes pi-tools carry compatibility work after the maintained fork has advanced. |
| Add a separate Pi 0.84.2 recorder lane | It leaves awf with two Pi SDK generations, duplicate dependency policy, and a runtime proof that does not match its shared test seam. |
| Bump only pi-tools without importing the recorder | The existing type-only profile API gains little, and calling that a recorder migration would overstate the result. |
| Copy or locally adapt the recorder | It forks a generic Pi testing concern whose shared owner is pi-tools. |

## Status history

- 2026-08-16: Proposed
