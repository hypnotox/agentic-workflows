---
format: current-state-v4
slug: introduce-core-and-full-workflow-profiles
status: Implementing
date: 2026-08-14
---
# ADR-0278: Introduce Core and Full workflow profiles


## Context

awf currently publishes one opinionated workflow to every adopter. Its operational discipline - brainstorming, testing, implementation, review, repository orientation, generated-tree maintenance, and durable effort continuity - is useful in repositories that cannot adopt awf's ADR, plan, and current-state governance. The single workflow prevents those repositories from adopting the useful operational core without also accepting governance artifacts and lifecycle rules they do not own.

The current implementation assumes one complete catalog at several boundaries. The project output plan renders every catalog artifact, generated ADR and current-state producers sit outside ordinary catalog selection, checks always load ADR and current-state corpora, and shared workflow prose couples otherwise operational skills to ADRs, plans, current-state claims, and `awf context`. The root guide and code reviewer have the same coupling. A skill-map filter would therefore produce an incoherent workflow rather than a smaller one.

ADR-0251 deliberately removed per-artifact selection and established full-catalog rendering. ADR-0255 explicitly retained a single workflow without profiles. Those choices made the standard coherent at the time, but they also make its governance inseparable from its operational tooling. This decision succeeds those constraints with two coarse, closed repository profiles rather than restoring arbitrary artifact selection.

## Decision

1. `decision: two-closed-profiles` awf provides exactly two repository workflow profiles, Core and Full. Core is a complete operational workflow, and Full contains Core plus awf's ADR, plan, current-state, context, and governance workflow. Profiles are coarse closed footprints, not arbitrary artifact toggles.
2. `decision: core-operational-workflow` Core retains brainstorming and explicit boundary approval, repository orientation and grounding, exploration, debugging, test-driven development and bug fixing, direct or delegated implementation, independent implementation review, generated-tree and documentation maintenance, maintainable-code guidance, and retrospective. It also retains efforts, memory, managed worktrees, handoff, integration, and cleanup. Core excludes ADR and plan authoring, lifecycle, review, and execution; current-state topics and domains; `awf context`; and governance-specific checks and audit behavior.
3. `decision: profile-dependency-direction` Every Core artifact is semantically complete using only Core artifacts and ordinary adopter documentation, source, tests, and repository history. Core has no reference or dependency on a Full-only skill, agent, document, generated producer, command capability, check, template fragment, or authority concept. Full may depend on and extend Core.
4. `decision: profile-default-and-migration` New projects select Core by default and may explicitly select Full during initialization. The selected profile is a visible repository configuration fact that may later be changed. Existing repositories migrate explicitly and idempotently to Full so an upgrade preserves their current workflow and generated outputs.
5. `decision: closed-profile-transition` A repository changing to Core must remove Full-only awf configuration sources, and synchronization prunes Full-only awf-generated workflow outputs. Full-only sources are not retained as dormant configuration. Authored historical ADR and plan documents are preserved and become ordinary adopter-owned documents that Core does not parse, validate, index, lifecycle-check, or otherwise govern.
6. `decision: full-reactivation` Returning a repository to Full restores Full-generated workflow outputs and governance. Any retained historical ADR or plan documents must again satisfy Full's active contracts or be changed or removed by the adopter.
7. `decision: profile-aware-capabilities` One awf binary serves both profiles. Full-only commands include workflow-conformance `awf audit` and ADR, plan, current-state, topic, and context operations. Invoking one under Core reports that the capability is unavailable for the selected profile instead of failing through a missing generated file or accidentally activating Full machinery. Repository, generated-output, commit, prose, memory, code-quality, and effort checks that do not require Full authority remain Core capabilities.
8. `decision: single-profile-projection` The complete catalog owns profile membership and dependency closure. Project opening derives one immutable selected-profile view that is threaded through output planning, rendering, generated producers, layout, lock and config-hash inputs, pruning, validation, checks, and command capability decisions. Consumers do not independently reconstruct profile membership or filter only one artifact family.

## State changes

- update `config/configuration:config-expresses-repo-facts-only`
- update `config/configuration:no-artifact-selection-surface`
- update `config/configspec-and-reference:configspec-key-parity`
- add `config/migrations-and-locks:profile-full-migration`
- update `config/migrations-and-locks:schema-version-lock`
- update `rendering/catalog-and-targets:target-dialect-render`
- update `rendering/catalog-and-targets:unified-doc-model`
- add `rendering/catalog-and-targets:profile-dependency-closure`
- update `rendering/project-output-plan:multi-target-render`
- update `rendering/project-output-plan:output-plan-complete`
- remove `rendering/project-output-plan:full-catalog-render`
- add `rendering/project-output-plan:profile-projected-render`
- update `rendering/project-output-plan:scaffold-seeds-all-vars`
- update `rendering/singletons-and-payloads:adr-system-singletons-rendered`
- update `rendering/doc-outputs:layout-derivation`
- remove `rendering/doc-outputs:layout-docs-full-catalog`
- add `rendering/doc-outputs:layout-docs-profile-projection`
- update `rendering/sync-and-drift:check-active-md-stale`
- update `rendering/sync-and-drift:closed-config-tree`
- update `rendering/sync-and-drift:drift-source-set`
- update `rendering/sync-and-drift:managed-output-attribution`
- update `rendering/sync-and-drift:sync-always-writes-active-md`
- update `rendering/sync-and-drift:target-prune-ancestors`
- update `rendering/sync-and-drift:coverage-evaluation-unconditional`
- add `rendering/sync-and-drift:profile-config-hash`
- update `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`
- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/guide-and-doc-templates:maintainable-code-design-guide`
- update `rendering/workflow-skill-templates:independent-workflow-escalation`
- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:authority-guided-implementation-autonomy`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:effort-workflow`
- update `rendering/workflow-skill-templates:orienting-single-home`
- update `rendering/workflow-skill-templates:maintainable-code-stage-coverage`
- remove `rendering/workflow-skill-templates:single-workflow-no-depth-controls`
- add `rendering/workflow-skill-templates:closed-workflow-profiles`
- update `tooling/cli:cli-creation-and-inventory`
- update `tooling/cli:invariants-in-check`
- update `tooling/cli:check-universe-groups`
- update `tooling/cli:upgrade-always-syncs`
- add `tooling/audit-commands:audit-full-profile-only`
- add `tooling/context-and-topic:context-full-profile-only`
- update `tooling/init-and-enablement:init-noninteractive-default`
- update `tooling/init-and-enablement:init-prompts-enabled-vars`
- add `tooling/init-and-enablement:init-profile-default-core`

## Consequences

Projects can adopt awf's coding discipline and tooling without adopting its governance model. Full remains available for repositories that want the entire standard, while existing adopters do not silently lose behavior during upgrade.

The catalog, output plan, generated producers, command capabilities, checks, templates, configuration reference, and documentation must all honor one profile projection. Config parsing, validation, scaffolding, and serialization expose the selected profile; lock manifests, per-output hashes, pruning, and migration preserve a coherent selected footprint. This is more substantial than filtering skills, but it prevents partial profiles and dangling references. Core-neutral shared semantic homes will require Full-only additive guidance where today's shared prose assumes ADR or plan authority.

Changing profiles is deliberately visible. Moving to Core requires removing Full-only awf sources and prunes generated workflow artifacts, but it never deletes authored project history. Moving back to Full can surface validation work if those now-unmanaged historical documents changed while Core was active. Supported upgrades make existing repositories explicitly Full before current validation and synchronization, preserving their prior outputs; fresh Core repositories carry an explicit profile readable by later supported versions.

The profile field is an artifact-selection surface at a deliberately coarse level. Per-artifact selection remains unavailable, avoiding the closure and support burden that ADR-0251 removed.

Every new profile claim is backed by focused tests. The proof set covers catalog dependency closure; profile-projected output plans, layouts, generated producers, hashes, and pruning; profile-specific workflow rendering and links; explicit capability refusal; Core-default initialization; and existing-repository Full migration. Templates and shared fragments render coherently for both profiles with empty project data and emit neither `<no value>` nor unresolved placeholder tokens.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Separate `awf-core` project or binary | It would duplicate the renderer, tooling, migrations, and operational workflow and allow the two products to drift. |
| Restore individual skill, agent, and document selection | Arbitrary combinations cannot guarantee a coherent workflow or dependency closure. |
| Keep one Full workflow and make governance advisory | Core adopters would still receive unwanted artifacts, dead links, and instructions tied to authority they do not use. |
| Preserve Full-only configuration dormantly under Core | A dormant layer makes the selected profile ambiguous and permits stale Full configuration to survive unnoticed. |
| Delete historical ADRs and plans when selecting Core | Those files are adopter-owned project history rather than disposable generated workflow machinery. |

## Status history

- 2026-08-14: Proposed
- 2026-08-14: Accepted; content-sha256: 688fcda2464327e4b0757961da56c9c50a12c20f25e26f6ccd859ed006118837
- 2026-08-14: Implementing; content-sha256: 688fcda2464327e4b0757961da56c9c50a12c20f25e26f6ccd859ed006118837
- 2026-08-14: Applied; operations: update `config/configuration:config-expresses-repo-facts-only`, update `config/configuration:no-artifact-selection-surface`, update `config/configspec-and-reference:configspec-key-parity`, add `config/migrations-and-locks:profile-full-migration`, update `config/migrations-and-locks:schema-version-lock`, update `rendering/catalog-and-targets:target-dialect-render`, update `rendering/catalog-and-targets:unified-doc-model`, add `rendering/catalog-and-targets:profile-dependency-closure`, update `rendering/project-output-plan:multi-target-render`, update `rendering/project-output-plan:output-plan-complete`, remove `rendering/project-output-plan:full-catalog-render`, add `rendering/project-output-plan:profile-projected-render`, update `rendering/project-output-plan:scaffold-seeds-all-vars`, update `rendering/singletons-and-payloads:adr-system-singletons-rendered`, update `rendering/doc-outputs:layout-derivation`, remove `rendering/doc-outputs:layout-docs-full-catalog`, add `rendering/doc-outputs:layout-docs-profile-projection`, update `rendering/sync-and-drift:check-active-md-stale`, update `rendering/sync-and-drift:closed-config-tree`, update `rendering/sync-and-drift:drift-source-set`, update `rendering/sync-and-drift:managed-output-attribution`, update `rendering/sync-and-drift:sync-always-writes-active-md`, update `rendering/sync-and-drift:target-prune-ancestors`, update `rendering/sync-and-drift:coverage-evaluation-unconditional`, add `rendering/sync-and-drift:profile-config-hash`, update `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`, update `rendering/guide-and-doc-templates:guide-entry-point-routing`, update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/guide-and-doc-templates:maintainable-code-design-guide`, update `rendering/workflow-skill-templates:independent-workflow-escalation`, update `rendering/workflow-skill-templates:implementer-context-grounding`, update `rendering/workflow-skill-templates:mandatory-approval-boundaries`, update `rendering/workflow-skill-templates:authority-guided-implementation-autonomy`, update `rendering/workflow-skill-templates:authority-guided-review-remediation`, update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`, update `rendering/workflow-skill-templates:effort-workflow`, update `rendering/workflow-skill-templates:orienting-single-home`, update `rendering/workflow-skill-templates:maintainable-code-stage-coverage`, remove `rendering/workflow-skill-templates:single-workflow-no-depth-controls`, add `rendering/workflow-skill-templates:closed-workflow-profiles`, update `tooling/cli:cli-creation-and-inventory`, update `tooling/cli:invariants-in-check`, update `tooling/cli:check-universe-groups`, update `tooling/cli:upgrade-always-syncs`, add `tooling/audit-commands:audit-full-profile-only`, add `tooling/context-and-topic:context-full-profile-only`, update `tooling/init-and-enablement:init-noninteractive-default`, update `tooling/init-and-enablement:init-prompts-enabled-vars`, add `tooling/init-and-enablement:init-profile-default-core`
