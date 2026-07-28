---
format: current-state-v2
status: Implementing
date: 2026-07-28
---
# ADR-0168: Make maintainable code design a workflow obligation

## Context

awf strongly governs correctness, test coverage, documentation currency, planning detail, and
independent review. It does not give an agent one authoritative standard for shaping code that
remains understandable and changeable after those checks pass. A locally correct change can still
bolt new behavior onto an unsuitable representation, duplicate policy, reverse a dependency, leak
a storage or transport shape across a boundary, or preserve an avoidable coupling because the
workflow asks how to implement the requested behavior but not whether the surrounding design can
carry it cleanly.

Generic advice to "use clean code" does not solve the problem. SOLID, DRY, YAGNI, and named design
patterns are useful vocabulary, but mechanical compliance can create indirection, premature
abstraction, and pattern-driven code that is harder to maintain than a direct model. The needed
posture is contextual: model the domain semantics explicitly, keep policy and representation
boundaries visible, direct dependencies deliberately, prefer cohesive ownership, and make the
smallest design investment that prevents foreseeable duplication or workarounds.

Refactoring creates a second tension. Refusing all preparatory work entrenches weak seams and makes
features accumulate around them. Treating every observed smell as current scope makes delivery
unbounded. The workflow needs an explicit assessment that includes bounded enabling refactors while
surfacing larger choices to the user rather than silently expanding or silently accepting debt.

This guidance must be language-agnostic and useful to adopters with unset project data. It must apply
where designs are formed, converted into plans, implemented through every supported path, delegated
to scoped subagents, and reviewed. Reviewer agents must remain report-only.

## Decision

1. Add `docs/maintainable-code-design.md` to the standard catalog as a mandatory document-map
   singleton. Render it for every adopted project regardless of the optional docs selection, and
   list it in the generated agent guide with its catalog title and description. Give the document
   ordered convention-part extension points under
   `.awf/parts/maintainable-code-design/<section>.md`, using the same marker, layout, render,
   drift, and coherent-missing-data contracts as other mandatory documentation.

2. Make the guide a decision framework rather than a pattern mandate. Its standard sections cover:
   decision posture; SOLID, DRY, and YAGNI as contextual heuristics; semantic modeling; boundaries
   and dependency direction; an open, illustrative pattern toolbox; preparatory-refactor
   assessment; and recurring failure modes. The guide favors cohesive models, explicit ownership,
   low coupling, representation isolation, and testable seams, while requiring agents to justify
   indirection and abstraction against the actual change. Pattern examples are non-exhaustive and
   never become a checklist or a language-specific prescription.

3. Require every non-trivial design or implementation path to assess preparatory refactoring before
   adding behavior. Include bounded work in the change when it prevents duplication, inappropriate
   coupling, representation leakage, or a workaround that the requested behavior would otherwise
   introduce. When the enabling work is materially larger than the approved scope, present the user
   with an explicit choice to perform it first, include it in the current effort, defer it in a
   durable project-owned record, or decline it with the resulting trade-off stated. Do not expand
   scope, hide debt, or manufacture a refactor solely to satisfy a heuristic.

4. Integrate concise, stage-appropriate obligations into brainstorming, proposing ADRs, coupling
   audits, writing plans, test-driven development, executing plans, executing direct changes,
   subagent-driven development, and bug fixing. Brainstorming identifies the relevant model,
   boundaries, representations, dependency direction, and refactor decision. ADR authoring and
   coupling audits preserve applicable structural choices and expose constraints or enabling work;
   test-driven development selects seams and tests that support rather than distort the model.
   Planning turns those choices into executable tasks and validation. Every implementation path
   preserves them, reassesses when source facts invalidate them, and rejects shortcuts that merely
   bolt correctness onto the wrong abstraction. The concise workflow text points to the mandatory
   guide rather than duplicating it.

5. Require implementation orchestrators to pass the scoped implementer the relevant semantic
   boundaries, representations, dependency direction, refactor decision, prohibited shortcuts, and
   validation expectations. The brief includes only facts relevant to that task; this contract does
   not turn a scoped subagent into a second planner or authorize it to broaden the task.

6. Add maintainability lenses to plan and implementation review. Plan review checks that structural
   choices and necessary enabling refactors are explicit, ordered, bounded, and verifiable. Code
   review checks cohesion, coupling, dependency direction, representation leakage, duplication,
   testability, needless indirection, and conformance to the settled design. ADR review applies the
   same structural lens only when the proposed decision changes a semantic model, representation,
   module or package boundary, dependency direction, or comparable structural contract. Reviewer
   agents continue to read and report only; their orchestrating review skills retain ownership of
   routing findings and applying approved corrections.

7. Keep all default guidance adopter-neutral and language-agnostic, with no project-specific package,
   command, file-layout, or implementation vocabulary leaking into generic output. Render the same
   semantic obligations across every supported target, while allowing target-native syntax and
   dispatch mechanisms. Existing section overrides remain intentional extensions rather than an
   alternative source of the standard's core obligation.

8. Back the new current-state claims with focused deterministic tests. Cover mandatory catalog and
   document-map membership, singleton output planning and layout, ordered section-marker parity,
   convention-part replacement, coherent generic rendering with unset data, absence of
   project-specific content leakage, every required workflow and review stage, scoped subagent
   handoff content, conditional ADR-review wording, reviewer report-only preservation, and generated
   multi-target semantic parity.

9. Every lifecycle status transition of this ADR runs `./x render` and commits the regenerated
    `docs/decisions/INDEX.md` and lock update in the same transaction.

## State changes

- add `rendering/guide-and-doc-templates:maintainable-code-design-guide`
- add `rendering/workflow-skill-templates:maintainable-code-stage-coverage`
- add `rendering/workflow-skill-templates:maintainable-code-subagent-contract`
- add `rendering/workflow-skill-templates:maintainable-code-review-lenses`

## Consequences

Adopters receive one stable, extensible maintainable-code standard and encounter its obligations at
the point where each kind of work can act on them. Design quality becomes an explicit workflow
concern rather than an optional reviewer preference. Scoped implementers receive enough structural
context to avoid locally convenient but globally harmful choices, and reviewers have concrete
questions instead of an unbounded "clean code" judgment.

The guidance adds prompt and review surface across several skills and agents. Keeping the complete
rationale in one mandatory document and stage prompts concise limits duplication and drift. Focused
coverage must pin semantic presence without freezing all prose or requiring identical runtime
syntax.

Preparatory refactoring can increase the size of a feature or fix, but only when the work is bounded
and directly prevents a foreseeable design failure. Larger enabling work becomes visible user choice.
This may defer delivery, create a durable follow-up, or record an accepted trade-off; it avoids both
silent scope growth and silent structural debt.

Heuristic application remains judgment-based. Deterministic tests can prove that the guide and stage
obligations render, not that every design choice is good. Independent review supplies the semantic
check, while the guide's anti-mechanical posture reduces the risk of cargo-cult abstractions.

The implementation is cross-cutting: catalog metadata, a new template and extension layout, project
output planning, workflow skills, reviewer agents, rendered adopter outputs, and current-state claims
must move together. It therefore requires a reviewed implementation plan after this ADR settles.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Publish a maintainable-code guide without workflow integration | A passive document is easy to ignore and does not affect the decisions, delegation, or review points where structural quality is won or lost. |
| Copy the full standard into every skill and reviewer | Repetition would inflate prompts, create drift, and make project extension ambiguous; concise stage obligations can refer to one mandatory source. |
| Mandate SOLID rules or a closed set of design patterns | Mechanical pattern compliance rewards indirection and vocabulary over fit, contradicting YAGNI and the contextual nature of maintainability. |
| Require all discovered refactoring before feature work | This makes scope unbounded and lets unrelated cleanup block delivery. |
| Forbid preparatory refactoring unless separately requested | This institutionalizes bolt-on changes, duplication, leaky boundaries, and avoidable workarounds. |
| Add a standalone maintainability skill or reviewer | Quality must shape the existing design-to-review chain; an optional extra step would fragment authority and miss direct and bug-fix paths. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Implementing; content-sha256: 5e6e3b2f3b3b066a5faec3ad1a7d81accd2599ce89546edb2d5f556a371eaa49
- 2026-07-28: Applied; state-sequence: 69; operations: add `rendering/guide-and-doc-templates:maintainable-code-design-guide`
