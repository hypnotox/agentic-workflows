---
format: current-state-v4
slug: move-pi-session-handoff-authority-to-pi-runtime
status: Implementing
date: 2026-08-20
---
# ADR-move-pi-session-handoff-authority-to-pi-runtime: Move Pi Session Handoff Authority to Pi Runtime

## Context

Awf's generic working-memory and checkpoint guidance currently carries Pi-specific session
replacement protocol alongside capability-neutral continuity rules. It names Pi's handoff tool,
its exact effort kickoff, transient session-context evidence, replacement logging, and failure
semantics. The same protocol is also an active claim under Pi workflows. This makes a target
adapter look like universal workflow authority and leaves the operative contract distributed
across generic documentation, shared checkpoint partials, and Pi-specific output.

The Pi runtime topic already owns the target's runtime boundaries and its integration with the
independently installed `pi-tools` handoff implementation. Pi workflows instead owns awf profile
policy, native-skill selection, and target projections. The handoff claim remained there from an
earlier grouping even after the implementation and compatibility boundary moved to `pi-tools`.
Moving that claim to Pi runtime follows the current semantic owner without introducing another
reference surface. Generic workflow authority still needs to define mandatory checkpoint
persistence, safe resumability, repository precedence, single-writer memory, reorientation,
truthful boundary logging, and the choice between continuing locally and using a target-native
successor. Pi output must retain every executable detail so the ownership move does not weaken or
silently change session replacement.

The broader classification of daily, advanced-lifecycle, recovery, and configuration material is
reserved for AF-011. This decision corrects protocol ownership only.

## Decision

1. `decision: target-owned-pi-session-handoff` Give Pi's executable session-replacement protocol one target-specific canonical owner. Preserve Pi's transient evidence source, tool invocation, exact effort kickoff, association mechanic, and executable projection of the generic continuation obligations rather than teaching those target mechanics as universal workflow rules.
2. `decision: capability-neutral-continuity-authority` Keep generic continuity authority capability-neutral. It owns checkpoint persistence, safe resumability and eligibility, repository and memory authority, reorientation, truthful fresh-boundary logging, cancellation and failure disposition, retained-context and successor-work judgment, and continuation through a target-native successor, while target output projects the executable protocol for capabilities it actually supplies.

## State changes

- remove `rendering/pi-workflows:pi-session-handoff-workflow`
- add `rendering/pi-runtime:pi-session-handoff-workflow`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-workflows:using-effort-skill`

## Consequences

Generic workflow documents and shared checkpoint semantics no longer present Pi-only tool calls,
transient runtime facts, or association mechanics as universal obligations. They retain the
capability-neutral continuation, reorientation, and truthful logging rules. Pi users retain one
complete, executable projection in target-owned guidance, and active current-state authority names
that owner directly.

The target projection must stay synchronized with the canonical Pi runtime claim, and tests must
prove both lossless Pi behavior and capability-neutral output for targets without session handoff.
The Pi runtime topic's source selectors must cover the templates that implement its protocol.

This does not reorganize daily-use, recovery, migration, or configuration documentation. AF-011
remains responsible for that broader information architecture after this ownership correction
lands.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the complete protocol in generic workflow authority behind target conditionals | Conditional rendering avoids leakage in some outputs but still assigns target-specific runtime semantics to the generic owner. |
| Retain Pi workflows as the canonical owner | That topic owns awf profile policy, native skills, and their projections, while Pi runtime already owns the `pi-tools` handoff implementation and compatibility boundary; retaining the historical grouping would preserve the semantic mismatch. |
| Add a new standalone Pi handoff reference | The existing Pi runtime topic already owns the relevant target boundary, so another reference would duplicate authority and navigation. |
| Split operative details between Pi workflows and the effort-association skill | Distributed ownership would preserve the ambiguity and make lossless evolution harder to verify. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Implementing; content-sha256: 2203d2c20263c63fcf75901e18af43ca8b41e053c10e0cb9af5badfceb5d9f06
- 2026-08-20: Applied; operations: remove `rendering/pi-workflows:pi-session-handoff-workflow`, add `rendering/pi-runtime:pi-session-handoff-workflow`, update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, update `rendering/pi-workflows:using-effort-skill`
