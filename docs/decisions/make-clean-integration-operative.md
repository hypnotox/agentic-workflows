---
format: current-state-v4
slug: make-clean-integration-operative
status: Proposed
date: 2026-08-19
---
# ADR-make-clean-integration-operative: Make Clean Integration Operative


## Context

The maintainable-code-design guide already makes cohesive ownership, deliberate dependency direction,
boundary translation, bounded preparatory refactoring, and rejection of speculative abstraction
active design principles. Implementation and review surfaces project those principles unevenly.
Some ask only for a general maintainability assessment, some carry independent stage-specific lists,
and delegated agents use separate review lenses. An implementation can therefore comply
superficially while adding behavior to the nearest file, duplicating policy, leaking an adapter
representation, preserving an obsolete route, or omitting the bounded refactor needed for clean
ownership.

ADR-0286 distinguishes a change's protected contract from its revisable execution route, and
ADR-0287 permits implementation owners to correct that route. Route freedom does not define what a
clean correction is. The operative rule must make necessary integration work visible without
turning the design guide into a mandatory long-form checklist or authorizing attractive unrelated
cleanup. It must also stop short of AF-005's separate decision about which review findings block.

The concern has two levels of ownership. The maintainable-code-design guide owns adopter-neutral
doctrine. One shared operative rule can translate that doctrine into proportional implementation
and review action while stage-specific surfaces retain only their local protocol. This mirrors the
existing single-home pattern for protected-contract and plan-flexibility guidance and avoids a
second maintainability doctrine.

## Decision

1. `decision: canonical-guide-operative-rule` Keep the maintainable-code-design guide as the
   canonical doctrine and establish one shared clean-integration rule as the operative semantic home
   for implementation and review. The rule applies proportionally, so a simple change may satisfy it
   in a few sentences rather than mandatory checklist output.

2. `decision: clean-integration-questions` Every primary implementation and review path determines
   the current and target owner, the narrowest clean integration point, any bounded enabling
   refactor, the obsolete or parallel path to remove or migrate when practical, the tests, docs,
   generated outputs, migrations, and compatibility surfaces that move with the change, and any
   residual debt with its reason.

3. `decision: bounded-refactor-inside-scope` A bounded refactor necessary to prevent duplicated
   policy, inappropriate coupling, representation leakage, or a workaround is inside the selected
   change. It requires a separate material decision rather than silent scope growth when it creates
   a durable choice, increases risk, changes external behavior, or expands the requested outcome.

4. `decision: retirement-without-speculation` Remove or migrate an obsolete or parallel path in the
   same change when practical; otherwise make the residual debt explicit. Preserve YAGNI, reject
   unrelated cleanup and speculative flexibility, and do not distort production design for test
   convenience when an existing real seam suffices.

5. `decision: operative-consumer-and-proof` Project the shared rule through primary brainstorming,
   planning, direct and delegated implementation, bugfix, TDD, plan review, and implementation
   review surfaces, including their delegated agents. Prove one authored home, proportional
   behavior, coherent empty-data rendering, and equivalent applicable behavior across governance
   footprints and supported runtimes. Review applies explicit one-home, obsolete-path,
   dependency-direction, representation-boundary, and residual-debt lenses without deciding
   AF-005's finding-severity policy.

## State changes

- add `rendering/workflow-skill-templates:clean-integration`
- update `rendering/workflow-skill-templates:maintainable-code-stage-coverage`
- update `rendering/workflow-skill-templates:maintainable-code-subagent-contract`
- update `rendering/workflow-skill-templates:implementer-role-contract`
- update `rendering/workflow-skill-templates:maintainable-code-review-lenses`
- update `rendering/guide-and-doc-templates:maintainable-code-design-guide`

## Consequences

An implementation owner must integrate behavior into its semantic home rather than merely make the
nearest mechanism pass. Necessary bounded refactoring, moving verification surfaces, and practical
retirement become part of completing the requested change, while a separate durable choice, risk
increase, external behavior change, or outcome expansion still returns to the material-decision
boundary.

The shared operative rule adds one reusable workflow concept but retires independently phrased
maintainability obligations. The guide remains the doctrine owner, stage-specific consumers retain
their own execution and review protocol, and the rule cannot become another rigor mode or workflow
layer.

Proportionality relies on judgment. Contract and behavioral scenarios mitigate the risk that agents
either emit ceremonial checklists or skip owner, retirement, and residual-debt questions. Review can
identify concrete integration weaknesses, but whether a finding blocks remains outside this decision
until AF-005.

Required enabling work can enlarge the immediate implementation route. The boundary is bounded by
the failure it prevents and by the protected-contract stop; unrelated cleanup and speculative
abstraction remain prohibited.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Expand the guide and leave consumers with general references | A general reference preserves the current superficial-compliance gap and supplies no common operative rule. |
| Copy the integration questions into every skill and agent | Parallel doctrine would drift and violate the single-home rule. |
| Make every maintainability improvement part of the selected change | This would authorize unbounded cleanup and conflict with YAGNI and the protected-contract boundary. |
| Define blocking severity with the review lenses | AF-005 owns the dependent decision about concrete review risk and blocking policy. |

## Status history

- 2026-08-19: Proposed
