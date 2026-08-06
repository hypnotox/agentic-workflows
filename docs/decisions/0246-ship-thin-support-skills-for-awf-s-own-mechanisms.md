---
format: current-state-v4
slug: ship-thin-support-skills-for-awf-s-own-mechanisms
status: Proposed
date: 2026-08-07
---
# ADR-0246: Ship thin support skills for awf's own mechanisms

## Context

awf ships support skills for two of its three mechanisms. `adr-lifecycle` owns transitioning a
record between lifecycle states, and `effort-workflow` owns an effort from continuity evaluation
through finish. The third mechanism, the generated tree itself, has no skill: nothing fires when an
agent is asked to change `.awf/`, re-render, resolve drift, or upgrade awf. Doc authoring has no
skill either, though `docs/doc-standard.md` holds its rules and the guide's document map reaches
them.

ADR-0157 set aside "a new using-awf skill carrying command and usage detail" because it "overlaps
`docs/working-with-awf.md` nearly one-to-one, creating a second canonical home that must be kept in
sync". That reasoning binds the content a skill may carry, not whether a skill for the mechanism may
exist: `adr-lifecycle` is self-contained about the ADR mechanism and coexists with the same doc. A
skill that carries a transaction shape and points at the doc for detail is not the artifact 0157
declined.

Three repository facts shape the cost side, and one of them inverts an assumption worth stating
because it was load-bearing in the discussion that produced this record. First, the rendered guide no
longer lists skills at all: ADR-0241 replaced the catalog-derived trigger table, and
`rendering/guide-and-doc-templates:guide-entry-point-routing` now holds that the guide routes to
enabled native skills "whose exposed descriptions fit the work, without duplicating enabled standard
or local skill names, purposes, triggers, kinds, relationships, or a fallback catalog". A new skill
therefore costs zero guide bytes, and its exposed description is the whole selection mechanism.
Second, the authority-routing rule that sends durable choices to ADRs, active rules to current-state
topics, directives to plans, and transient context to effort memory already renders unconditionally
in every adopter's guide, so routing a fact to a surface is not the gap; what follows once the
surface is a doc is. Third, `orienting-single-home` establishes the shape a per-skill claim takes:
it names what the skill's rendered body owns, and it is backed by test.

The in-flight configuration-surface collapse constrains implementation timing rather than the
decision. Its accepted plan makes the whole catalog render unconditionally, then deletes the
selection machinery including the curated core selection, then sweeps residual enablement wording.
Implementing these two skills before that lands would author a core-selection flag, a selection-array
entry, and skill-roster expectations across test fixtures that the collapse then deletes, and would
add two prose bodies its wording sweep would have to cover. Implementing them after is a catalog
entry and a template per skill.

## Decision

1. `decision: mechanism-support-skills` awf ships a support skill for each of its own mechanisms. A
   mechanism support skill's rendered body carries only what an agent needs in order to decide its
   next action, and delegates every rule to the surface that owns it. This is the boundary that
   distinguishes it from the artifact ADR-0157 declined: a transaction shape with pointers is
   admissible, a command or configuration reference is not.
2. `decision: using-awf-skill` `using-awf` joins the catalog as a support skill owning the
   generated-tree transaction: that `.awf/` is the source and rendered outputs are never
   hand-edited, that the transaction runs from a source edit through render and check to staging the
   rendered outputs and the lock alongside the source and then the gate, that drift findings carry
   their own repair hints, and that upgrading runs the bootstrap script and then the residue sweep.
   It carries neither the configuration key inventory nor the command reference, which
   `docs/config-reference.md` and `docs/working-with-awf.md` own. Scoping the skill to the
   transaction rather than to the surface is what keeps it correct across a configuration-surface
   change.
3. `decision: writing-docs-skill` `writing-docs` joins the catalog as a support skill owning doc
   authoring: selecting the single doc that owns the fact, reading the documentation standard before
   writing, referencing rather than restating what another surface owns, editing the convention part
   rather than the rendered file, and letting the doc travel in the commit that makes it true. It
   carries neither the documentation standard's own rules nor the render transaction, delegating the
   first to `docs/doc-standard.md` and the second to `using-awf`.
4. `decision: description-only-selection` Both skills are selected by their exposed descriptions
   alone. Neither is named, listed, or triggered from the rendered guide, so
   `guide-entry-point-routing` remains satisfied unchanged.
5. `decision: overridable-sections` Both skills declare a sections list, so adopters shape their
   content through convention parts rather than by replacing a single opaque body.

## State changes

- add `rendering/workflow-skill-templates:using-awf-transaction-home`
- add `rendering/workflow-skill-templates:writing-docs-delegation`

## Consequences

- Two mechanisms that previously had no entry point acquire one, and the acquisition costs no guide
  bytes because the guide stopped listing skills at ADR-0241. The cost that remains is selection
  competition among the enabled skills, paid in the harness's own skill listing rather than in any
  awf-rendered surface.
- Each added claim is backed by test, and each of its clauses needs its own assertion: presence
  assertions for the transaction prose and the pointers, absence assertions for the delegated
  content. A single marker over a multi-clause claim would leave the delegation clauses proven by
  nothing, which is the failure `docs/pitfalls.md` records for proof markers that do not reach every
  clause. The absence assertions are what mechanically preserve ADR-0157's reasoning: they fail if a
  later edit grows either body into a second canonical home.
- Coordination debt: implementation waits for the configuration-surface collapse to integrate. Until
  then this record is the only carrier of the decision, which is the correct surface for something
  that has to survive many sessions; effort memory is not. Implementing early is not merely
  redundant work, it collides with that effort's render-set and selection-retirement test rewrites.
- `writing-docs` depends on `using-awf` for the render transaction, so the two land together rather
  than separately; a `writing-docs` that pointed at an absent skill would be a dead reference.
- Downstream work this record creates but does not commit to: a support skill owning current-state
  claim declaration and proof-marker placement was considered and deliberately excluded, because its
  boundary against `adr-lifecycle`'s State-changes handshake is unsettled. It needs that boundary
  drawn before it is worth proposing.
- Doc authoring gains a second surface an author must consult, the skill in addition to the
  standard. This is accepted on the ground that the skill answers a question the standard does not:
  which doc owns the fact, and what the commit must contain.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep both as documentation only, sharpening `doc-standard.md` and `working-with-awf.md` | Costs nothing per session, but nothing fires at the moment an agent starts writing prose or touching `.awf/`; the description-based selection ADR-0241 established is exactly the trigger mechanism these mechanisms lack |
| Self-contained skills restating the rules so no second read is needed | Creates a second canonical home with no drift check between skill body and doc, which is what ADR-0157 declined; the absence assertions in this record's claims exist to prevent that shape from returning |
| One combined skill covering both doc authoring and the generated-tree transaction | Doc authoring is a special case of editing the generated tree, so the overlap is real, but a combined skill fires on both triggers at once and its description could not discriminate; two descriptions with an explicit handoff edge select more precisely |
| Also ship a claim-declaration and proof-marker skill in this record | Highest friction surface of the three by the evidence in `docs/pitfalls.md`, but its boundary against `adr-lifecycle` is unsettled, and widening this record would delay the two whose boundaries are clear |
| Implement now, ahead of the configuration-surface collapse | Would author a core-selection flag, a selection-array entry, and roster fixtures that collapse deletes, and collide with its render-set and selection-retirement test rewrites |
| Name the skill for its trigger, such as `maintaining-config` | Narrower than the skill's actual scope, which spans render, drift, and upgrade rather than configuration alone; the exposed description carries the trigger, so the name does not have to |

## Status history

- 2026-08-07: Proposed
