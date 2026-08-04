---
format: current-state-v4
slug: proportional-simplicity-boundaries
status: Proposed
date: 2026-08-04
---
# ADR-proportional-simplicity-boundaries: Proportional Simplicity Boundaries


## Context

awf already tells agents to make abstraction earn its cost, reject speculative flexibility, and
bound enabling refactors. That guidance is sound but not sufficient as an operating boundary. The
workflow also rewards exhaustive plans, broad verification, and review findings across many lenses.
In combination, those incentives can make an agent add abstractions, checks, tests, cleanup, or
process simply because they could improve a hypothetical future rather than because the requested
change needs them.

The imbalance appears at several stages. Brainstorming scales its design sections to complexity but
does not explicitly settle how much structure and verification are sufficient. Plan authoring says
"When in doubt, write the plan" and then requires implementation-ready detail. Execution preserves
settled structural choices but does not consistently define which deviations must return to the
user. Review reliably finds missing obligations but does not equally constrain speculative demands.
A late review can reject overengineering, but it cannot recover the time spent building it.

The workflow therefore needs a proportional default before implementation. The user must retain
control over material scope and design choices, while agents must retain autonomy over equivalent
mechanical details. The rule must apply to direct, planned, delegated, and bug-fix execution without
turning simplicity into another schema, checklist, gate, or mandatory planning ceremony. Its
rationale belongs in the maintainable-code guide, with concise obligations at the stages where an
agent can act on it.

## Decision

1. `decision: simplest-sufficient-default` Make the simplest sufficient solution the default.
   Added abstraction, indirection, validation, test machinery, tooling, cleanup, or process must be
   justified by requested behavior, a reproduced defect, an existing documented contract, or a
   clearly applicable project invariant. Generic robustness, hypothetical future use, and the mere
   possibility of doing more are insufficient grounds.

2. `decision: pre-implementation-simplicity-contract` Before implementation, require brainstorming
   to settle with the user a proportionate simplicity contract covering scope and exclusions,
   structural approach and dependencies, patterns or abstractions, and checks and testing strategy.
   Scale the detail to the change: a straightforward change may settle the contract in a few
   sentences rather than a fixed checklist.

3. `decision: material-deviation-approval` Treat the approved simplicity contract as the
   implementation boundary. A newly discovered need that changes behavior, scope, structure,
   dependencies, patterns, checks, or testing strategy returns to the user before further mutation.
   The escalation identifies the changed fact, why the approved approach no longer fits, the
   affected approved categories, and the simplest viable options. Equivalent mechanical choices
   that preserve the approved design remain autonomous.

4. `decision: proportionate-planning` Use an implementation plan only when sequencing,
   coordination, or resumability materially helps. A plan records and operationalizes approved
   choices; it does not create speculative structure, checks, or work merely to make the plan more
   exhaustive.

5. `decision: stage-local-enforcement` Reinforce the canonical rule with short, stage-specific
   obligations in planning, test-driven work, every supported implementation path, scoped
   implementer contracts, and plan and code review. Review flags unjustified or unapproved
   machinery and scope growth, but does not demand additions merely because more abstraction,
   cleanup, testing, or validation is imaginable.

6. `decision: judgment-not-mechanism` Keep proportionality a user-approved judgment boundary, not
   a new machine-enforced policy model. Preserve the maintainable-code guide as the canonical
   rationale and project a concise global instruction through the agent guide. Introduce no new
   schema, manifest, approval artifact, automated check, shared enforcement engine, test framework,
   command, linter, scanner, or workflow stage. Add no template data or conditional surface for this
   rule; verify its rendered semantics through the existing rendering and invariant-backing tests.

## State changes

- update `rendering/guide-and-doc-templates:maintainable-code-design-guide`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:maintainable-code-stage-coverage`
- update `rendering/workflow-skill-templates:implementer-role-contract`
- update `rendering/workflow-skill-templates:maintainable-code-review-lenses`

## Consequences

Agents receive an explicit stopping rule: correctness, readability, and required verification are
not invitations to add every conceivable safeguard. Users see material structure and verification
choices before implementation rather than discovering them in a diff. Plans become a tool for real
execution complexity rather than a default response to uncertainty, and reviewers balance finding
missing obligations with rejecting speculative additions.

The simplicity contract adds one more design concern to brainstorming and can cause implementation
to pause when source facts invalidate an approved choice. Keeping the contract proportional and
leaving equivalent mechanical details autonomous limits interruption without reopening every
settled detail.

The valid grounds deliberately include existing contracts and applicable invariants, so simplicity
cannot excuse skipping project obligations. Conversely, a project obligation does not justify
unrelated hardening. The rule remains judgment-based: rendered tests can prove that each workflow
surface carries the obligation, but cannot mechanically decide whether a particular solution is
simple enough. User approval and independent review remain the semantic controls.

The guidance adds concise prose across several existing templates and reviewer lenses. Keeping the
full rationale in one guide reduces drift. No runtime feature, dependency, configuration surface, or
new enforcement mechanism is introduced.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Global guidance only | A distant principle is too easily outweighed by local planning, execution, and review incentives. |
| Mandatory plan with a simplicity assessment | Requiring a plan for every non-minimal change would add the ceremony this decision is intended to prevent. |
| Review-only enforcement | It detects overengineering after implementation effort has already been spent. |
| Automated simplicity check | Simplicity is contextual; a mechanical proxy would reward form, create false confidence, and require new policy machinery. |

## Status history

- 2026-08-04: Proposed
