# Define substantial changes

Use this workflow when brainstorming or defining a substantive change, or when an intent, specification, plan, or ADR becomes useful. A substantive change affects behavior, design, contracts, or guidance; line count does not decide. Keep documents proportionate: reuse an adequate established outcome rather than creating a document for every task. AWF creates each requested file independently; it does not interpret content, infer relationships, enforce stages, or decide readiness.

Use the repository's documented AWF runner; examples below use `./awf`. For continuity and implementation worktrees, use `docs effort`. During implementation, use `docs completion` for verification, commit cadence, and final completion or integration.

## Challenge substantive directions

Once brainstorming forms or materially changes a direction for a substantive change, and before that direction becomes the basis for planning or implementation, obtain or reuse a directly applicable independent challenge performed from fresh context. Check whether the direction addresses the actual problem and whether its consequential assumptions hold against available evidence, established constraints, and applicable project guidance.

When permitted and available, delegate the challenge to a suitable agent with a self-contained brief covering its purpose, scope, relevant evidence, constraints, and guidance. Request findings rather than edits. Evaluate the findings, distinguishing established problems, grounded risks, and optional improvements, and address material findings before relying on the direction. Keep the challenge bounded; it should not restart brainstorming or expand the change without evidence.

An existing fresh-context challenge can satisfy this checkpoint when it is directly applicable. If delegation is unavailable or not permitted, perform the check directly and disclose that it was not independent. This checkpoint does not require an effort or change document and is not a separate approval stage.

## Establish the outcome and route

Keep a change's intent and optional specification together in `docs/changes/<slug>/`. From the implementation checkout, create only what the work needs:

```sh
./awf new intent <slug>  # docs/changes/<slug>/intent.md
./awf new spec <slug>    # docs/changes/<slug>/spec.md
```

**Intent** preserves the problem, desired outcome, scope, non-goals, constraints, and success criteria. Establish these before selecting the implementation route. Distinguish requirements from proposed mechanisms, and agreements from open questions.

**Specification** is optional. Use one when important behavior or design remains unclear after intent is established. Make the intended result precise through necessary interactions, boundaries, examples, and acceptance conditions. Refine rather than repeat the intent; reference applicable ADRs instead of copying rationale. Omit incidental implementation details and speculative cases.

Use the agreed change as the basis for each plan or ADR it needs:

```sh
./awf new plan <plan-slug>     # docs/plans/<plan-slug>.md
./awf new adr <decision-slug>  # docs/decisions/<decision-slug>.md
```

**Plan** owns one implementation route: coherent changes, real dependencies, integration points, and proportionate verification. Reference the agreed outcome, criteria, and decisions; do not hide unresolved requirements or design choices in steps. A plan may state its basis briefly when separate definition documents add no value. Revise the route as evidence changes without silently changing the agreed result.

**ADR** preserves a consequential choice whose rationale should outlive the change. It is not a specification summary. Use one record per coherent decision area, organized under `docs/decisions/`; split choices with independent reasons to change. Routine implementation details need no ADR.

Relationships are authored references, not automated links. Use descriptive slugs, distinct when a change needs several plans or decisions, and reference the originating intent or specification from derived documents. A related effort may use the change's slug, but no document requires effort memory or another document. Creation refuses replacement. Adapt or omit starter sections and remove unused prompts.

Review a specification against intent, each plan and proposed ADR against the agreed change and active decisions, and implementation against those agreements—not merely completed plan steps. These comparisons do not require separate review documents or approval stages.

Give each requirement and decision one authoritative home. Preserve agreed outcomes and criteria while work proceeds; record material deviations and agreed changes explicitly rather than rewriting success to match implementation. Commit documents while they guide implementation or review so Git retains that basis.

## Maintain ADR authority

ADRs begin with hand-maintained `status: pending` frontmatter:

- `pending`: the decision is proposed; agreement has not been established.
- `accepted`: the direction is agreed but not yet fully in effect.
- `active`: the decision governs the repository and its applicable implementation has been verified.

Status records established agreement and implementation state, not an additional approval gate. Do not invent acceptance. A decision may become accepted and active in the same change. For existing records, use their documented agreement and implementation evidence. AWF does not parse, validate, activate, or retire ADRs.

Keep pending, accepted, and active records together in `docs/decisions/`. Discover relevant ADRs through topics and ordinary repository inspection; there is no roster, query command, or clause-level lifecycle. Topics describe current practical implications and link to active ADRs for enduring choices and rationale.

Before implementation changes an active decision, identify affected ADRs and intended replacements—even if the conflict is discovered after work began. A successor should identify what it intends to replace and why. Pending and accepted records do not displace active authority: an accepted replacement guides implementation while the predecessor's still-governing commitments remain in effect.

For substantive supersession, prefer replacing the containing record, even when only part changes. Restate the resulting decisions in one or more independently understandable ADRs so readers need not reconstruct current authority from a chain. Carry forward still-binding commitments with their original meaning and necessary rationale, distinguishing inherited commitments from new choices. Reuse a suitable authoritative record where that keeps ownership clear.

Retire a predecessor only when its remaining commitments have active homes and the accepted changes are in effect with applicable verification. Reuse valid evidence for unchanged implementation. If some replacements are not active yet, preserve the predecessor's still-governing content. After retirement, every still-binding decision must have one authoritative active home.

Complete activation, retirement, and reference updates in the same coherent change. Update surviving references, including topic links and a successor's predecessor reference, to the relevant replacements or, when discussing history, an ordinary committed version such as `git show <commit>:docs/decisions/<slug>.md`. Once nothing in a predecessor still governs, it can be deleted with history retained in Git.

Ordinary corrections need no new ADR. Revision or deletion must not discard binding content without an authoritative surviving home. Retire withdrawn pending or accepted records and decisions that cease to apply; no replacement is needed when no commitments remain binding.

## Retain documents while useful

Tracked plans belong in `docs/plans/`. Existing effort-local plans remain untouched. If a still-useful local plan should become tracked, deliberately relocate it, reconcile any destination, and repair references while retaining one authoritative copy. No bulk migration is required.

Keep change definitions and plans while they serve implementation, verification, review, handoff, or a maintained reference. At completion, review their continued use under `./awf docs completion`. Preserve still-needed requirements and knowledge before removing a document; an ADR does not replace a behavior specification.
