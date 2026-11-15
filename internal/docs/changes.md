# Define changes and implementation routes

Use this workflow when a change involves a material choice about its outcome, scope, compatibility, or direction, a consequential code-structure choice, or would benefit from an intent, specification, or plan, whether or not a document is written. Routine changes whose outcome and route are already clear skip it and go straight to implementation under `awf docs completion`.

Commands use `./awf`; substitute the repository's AWF runner. Other workflows are cited as `awf docs <page>`, the same text as the skill `awf-<page>`. Methods named `agentic-…` come from the required agentic-skills package: this workflow says where in a change they apply and which documents result, and the method says how, including when independent review is warranted.

## Review cues

The review cues are the situations listed in `agentic-reviewing` under "When independent review is warranted"; read that list before deciding. `agentic-reviewing` owns it, together with choosing the reviewer, briefing, reuse, and fallback. Give the reviewer the target and its basis, not your conclusion.

## Settle the direction

1. Resolve material choices with `agentic-brainstorming` and structural ones with `agentic-code-design`.
2. When the review cues apply, have the direction's premises challenged before planning relies on it, using the premise check in `agentic-brainstorming`. Target the current `docs/changes/<slug>/intent.md` if there is one, otherwise the problem, outcome, and constraints from the conversation. Writing a claim into an intent does not establish it. Settle consequential factual uncertainty with a proportionate trial before costly dependent work.
3. Confirm the direction with the user before planning or dependent implementation: the recommended direction, its main trade-offs, and what remains open. The outcome is agreed once the user confirms it; a request that states every material choice is already agreed and needs no further stop. A request settles a material choice only where it states that choice or explicitly leaves it to you; when it is silent on one, such as a new output format or which inputs a feature covers, bring it to the user with your recommendation before building on it, instead of choosing and reporting afterwards. Record the agreement in the intent or effort memory when the work has one; otherwise the conversation is enough.

The plan then needs no separate approval unless the user asks to see it. Keep working until the next steering point in `awf docs completion`, a genuine need for user input, or a blocker.

## Write the documents the change needs

Many changes need none. Create only what helps, from the implementation checkout:

```sh
./awf new intent <slug>     # docs/changes/<slug>/intent.md
./awf new spec <slug>       # docs/changes/<slug>/spec.md
./awf new plan <plan-slug>  # docs/plans/<plan-slug>.md
```

- **Intent** records the problem, desired outcome, scope, non-goals, constraints, and success criteria, keeping requirements apart from proposed mechanisms and agreements apart from open questions.
- **Specification**, optional, makes important behavior or design precise when the intent leaves it unclear: interactions, boundaries, examples, and acceptance conditions, without repeating the intent or adding incidental detail.
- **Plan** owns one implementation route, built with `agentic-planning`. Reference the agreed outcome and decisions, and do not hide unresolved choices in steps. Revise the route as evidence changes without silently changing the agreed result.
- **ADR**: for rationale that outlives the change, use `awf docs adr`.

AWF does not link these documents, so maintain relationships yourself: use descriptive slugs and reference the originating intent or specification. Follow `awf docs knowledge` for metadata and replace starter prompts.

Give each requirement and decision one authoritative home. Record material deviations and agreed changes explicitly instead of rewriting success to match the implementation. Commit documents while they guide implementation or review.

## Review before relying on a document or design

When the review cues apply, obtain independent review before dependent work relies on:

- an intent, for a supported problem and coherent outcome; an applicable premise check covers this;
- a specification, against the agreed intent;
- a plan, for scope, dependencies, integration, and verification, as `agentic-planning` describes;
- a code structure, before a specification, plan, or implementation depends on it, as `agentic-code-design` describes; an inline outline is enough.

The same applies to substantial definitions and plans kept in the conversation. One review can cover related documents. Reconcile material findings before dependent work. Review informs the user's consequential choices; it does not settle them or add an approval stage.

## Keep documents while they are useful

Tracked plans belong in `docs/plans/`. Leave existing effort-local plans alone unless one should become tracked; then move it deliberately and repair references. `awf docs completion` decides at completion whether each change definition and plan stays.
