---
format: current-state-v4
slug: define-core-and-full-as-governance-footprints
status: Accepted
date: 2026-08-20
---
# ADR-define-core-and-full-as-governance-footprints: Define Core and Full as governance footprints


## Context

ADR-0278 introduced the required `profile` configuration field and two closed selections. It intended Core to provide the complete operational workflow and Full to add ADR, plan, current-state, context, and audit capabilities. Adopter-facing prose nevertheless calls the selections workflow profiles, while workflow authority also says awf has one workflow and no profiles. That vocabulary makes the selections sound like different standards of rigor or autonomy even though their intended distinction is which governance artifacts and capabilities are available.

The selected value is load-bearing throughout configuration, rendering, command capability checks, hashes, and the existing upgrade path. Renaming it would impose migration and compatibility cost without changing the underlying two-selection model. Shared protected-contract and clean-integration doctrine already renders in both selections and supplies the quality bar that their descriptions must preserve.

## Decision

1. `decision: governance-footprints` Core and Full are two closed governance footprints of one workflow. They select available governance artifacts and capabilities, not different standards of correctness, autonomy, maintainability, or review quality. Core includes the complete operational workflow; Full adds ADR, plan, current-state, context, and audit capabilities.
2. `decision: retain-profile-key` The configuration key remains `profile`. Its compatibility value outweighs the benefit of renaming the mechanism merely to match adopter-facing terminology, so this decision adds no config migration.
3. `decision: shared-quality-doctrine` Both footprints carry the same protected-contract and clean-integration doctrine. Core can perform clean, maintainable implementation autonomously; Full's additional governance artifacts do not raise or lower that quality bar.

## State changes

- update `config/configuration:config-expresses-repo-facts-only`
- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/guide-and-doc-templates:maintainable-code-design-guide`
- update `rendering/workflow-skill-templates:protected-contract-over-route`
- update `rendering/workflow-skill-templates:independent-workflow-escalation`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:authority-guided-implementation-autonomy`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:clean-integration`
- update `rendering/workflow-skill-templates:concrete-maintainability-review`
- update `rendering/workflow-skill-templates:closed-workflow-profiles`
- update `rendering/pi-workflows:pi-structured-exploration-contract`
- update `tooling/cli:check-universe-groups`
- update `tooling/init-and-enablement:init-noninteractive-default`
- update `tooling/init-and-enablement:init-prompts-enabled-vars`

## Consequences

Readers can describe Core and Full by available artifacts without treating Core as a lesser workflow. Adopter prose, generated guidance, configuration descriptions, and command help use footprint terminology while code and literal configuration examples retain the `profile` identifier.

Parity checks must prove that both footprints preserve the common correctness and maintainability doctrine without requiring their generated output to be identical. Full-only governance remains additive, and the existing profile values, selection behavior, migration history, and capability boundaries remain compatible.

Keeping different names for the adopter-facing governance-footprint concept and the `profile` configuration mechanism creates a lasting consistency cost. Documentation, help, and tests must distinguish them deliberately, and readers may still confuse the two.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Rename `profile` to `footprint` | The migration and compatibility cost adds no behavioral value. |
| Describe Core as a lighter or basic workflow | It falsely implies a lower correctness, autonomy, maintainability, or review bar. |
| Make every Core and Full artifact identical | Legitimate Full-only governance capabilities require different artifact membership and additive clauses. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Accepted; content-sha256: 7f06c033714f43ec3ab419066eb525a28f6e3d2b94625e506fb32d4419fdeb78
