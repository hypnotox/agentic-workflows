---
type: Change Intent
title: Streamline AWF workflows and guidance
description: Shorter, clearly triggered workflow skills that rely on agentic-skills for method, add human steering points, and tidy the adoption surface.
status: stable
---

# Intent: Streamline AWF workflows and guidance

## Problem

An audit against the owner's doctrine and current published agent guidance (2026-11-15) found that AWF's five workflows are sound in detail but hard to follow as a whole:

- Each skill loads 900–1,400 words at once, including CLI reference detail, repeated delegation-authorization paragraphs, and "AWF does not…" disclaimers that do not change behavior.
- Skill descriptions say what a workflow covers, not when to use it, so adopters hand-copy trigger conditions into `AGENTS.md`.
- Premise challenge, result review, and retrospective are defined both here and in `agentic-skills`. The challenge checkpoint is triggered by "the end of brainstorming", which AWF does not define.
- No workflow says when to bring a direction back to the user. Agreed outcomes are referenced without a point at which they are agreed.
- One threshold, "requires nontrivial judgment", gates about eight review checkpoints with only negative examples. A retrospective is required for every task.
- The adoption material mixes first-time adoption with migration history, and the README leads with mechanism.

No observation of agents following these texts exists, so their effect on behavior is inferred.

## Desired outcome

Workflows that are simple but useful: easy for an agent to follow, steering it toward the intended behavior with the right base behaviors applied, with less text loaded and clear triggers, and with human steering where it matters.

Success means:

- An agent can tell from a skill's description when to load it, and from its body what is required, what is a default, and what is optional.
- Each rule and method has one home. AWF workflows state when a checkpoint occurs in the repository lifecycle and which documents result; `agentic-skills` owns how to brainstorm, review, challenge, and run a retrospective.
- An adopter's `AGENTS.md` needs one stable sentence to route to AWF, not a copied list of trigger conditions.
- A one-off observation trial shows agents following the rewritten skills at least as well as the current ones on representative tasks.

## Agreed scope

Agreed with the owner on 2026-11-15:

1. **`agentic-skills` is required.** State it in the README and hard-wire references to its methods, using bare method names because skill prefixes differ per harness. Reword the "usable without external skills" constraint in `AGENTS.md` and the README; the CLI itself stays usable offline.
2. **Human steering points.** Confirm the direction before planning. Surface material deviation from the agreed outcome. Confirm before pushing, opening a pull request, tagging, or releasing, and before merging into the default branch unless the request already authorized it. Routine work needs no plan approval.
3. **Rewrite the five skills.** Descriptions state what and when. Bodies keep a small core of required steps and defaults. CLI reference detail moves to command help and error messages. Shared rules are stated once.
4. **Worktrees.** An effort decides whether it needs a worktree and records the choice.
5. **Commit cadence** stays an AWF default that repository and user instructions override. It does not cover pushing or merging.
6. **`AGENTS.md` stays author-owned.** Skill descriptions carry routing. Rendered templates with in-place editing are not reintroduced now; revisit if the trial shows descriptions do not route reliably.
7. **Drop the `.pi/skills` output.** Pi documents discovery from `.agents/skills`; omp 18.11.1 was observed reading `.agents/skills` and `.claude/skills` and listing an identical skill once. Pi's own project-skill discovery was not observed: a temporary-repository check listed no project skills from either directory. Say so in the changelog.
8. **Observation trial.** Run once, with a small harness kept for explicit future runs outside `./x gate` and the test suite. Results are directional.
9. **Accepted audit findings** that fit this change: warn at adoption that an existing `docs/` tree becomes the knowledge bundle; separate migration material from the adoption guide and retire the root migration file; give the literal `init` command; lead the README with purpose and remove its stale launcher condition; make CLI errors name a next step consistently; fix this repository's dead topic selector and procedure-in-topic deviations.

10. **Review triggers.** Replace "requires nontrivial judgment" with positive cues: several viable designs, a public contract or compatibility change, a hard-to-reverse step, or guidance that changes agent behavior.
11. **Retrospective.** Required for work that used an effort or met a meaningful issue; routine effort-free changes need none.
12. **`description` gains a consumer.** `resolve` prints each referenced topic's description, and an unedited starter placeholder description fails `check`.

## Non-goals

- Changing the product shape: the pinned binary, `resolve` versus harness-native path rules, effort memory, or hooks and configuration.
- Generating indexes or other knowledge-bundle files. Indexes remain optional.
- A standing evaluation suite.
- Release cadence and effort-archive pruning.

## Route

Two plans: the skills rewrite with the observation trial first, then the adoption surface, CLI changes, and this repository's own fixes.
