---
status: Implemented
date: 2026-07-15
supersedes: []
superseded_by: ""
tags: [adr-lifecycle, cross-references, review-agents]
related: [73, 79, 93, 103, 112, 114, 115, 120]
domains: [adr-system, rendering]
---
# ADR-0116: Partial-amendment back-pointers belong in the procedure, not a check

## Context

Partial-item supersedence lets ADR X override specific Decision items or Invariants of ADR Y
without replacing Y wholesale: X carries `related: [Y]`, Y's status stays live, and X's prose
cites the overridden items. The convention has no owning ADR. It grew in the rendered
`awf-adr-lifecycle` skill, which is its sole authoritative surface; the ADR README documents the
`related:` field itself but never the convention. ADR-0031 already refers to it as "the existing
partial-item supersedence convention".

It has a known failure mode. When X amends Y and nothing points from Y back to X, Y's amended
Decision item reads as current guidance with no signal that it was amended, so a reader of Y acts
on overridden text. The remedy has been convention since the ADR-0079 review: add X to Y's
`related:` in the same commit, a metadata-only edit leaving the append-only body untouched.
ADR-0103 made `related:` load-bearing rather than decorative: `internal/adr` parses it, `awf check`
resolves each entry against the decisions directory (`invariant: adr-related-link-resolved`), and
`awf context` spends it as relevance currency.

**The convention does not hold.** A corpus sweep on 2026-07-15 found 16 genuine partial
amendments, of which **10 lack the back-pointer**. Compliance is 6 of 16. The missing edges are
8 to 7, 30 to 3, 32 to 23, 45 to 1, 49 to 30, 76 to 39, 76 to 16, 89 to 11, 105 to 8, and 106 to
104. The conformant ones are 79 to 65, 81 to 46, 81 to 50, 93 to 24, 101 to 2, and 107 to 73.

**Recording the rule as a pitfall has not moved it.** The pitfall note landed 2026-07-08 stating
the convention verbatim. ADR-0093 broke it on 07-11. An escalation edit landed the same day.
ADR-0105 and ADR-0106 then both broke it on 07-13. That is three failures after the note. The
pitfall also mis-stated the problem's size: it recorded two instances, but those were the two that
review happened to catch, and both were subsequently fixed (they sit in the conformant six above).
It never measured the corpus, where 10 more were sitting unrecorded. Counting both, roughly 12
partial amendments have missed the back-pointer over the corpus's life. A rule described only by
the failures someone noticed will always understate itself, which is why this ADR records a
measured number.

The gap this ADR acts on is that **the rule was never in the procedure that governs the act**.
`awf-adr-lifecycle`'s "Partial-item supersedence" section is what an agent loads while performing
a partial amendment. It names the successor's `related:`, the predecessor's unflipped status, and
the successor's prose citation. It then states that the override information "lives in the
successor's prose and `related:` linkage", teaching the one-directional model outright. The
back-pointer appears nowhere in it, nor in the `adr-reviewer`'s charge. Every failure above is
consistent with an agent correctly following the procedure it was given. The convention was
documented only where someone had to already know it mattered.

A mechanical detector is feasible, which the pitfall's own escalation note obscured. That note
proposed requiring a back-pointer wherever an ADR cites another ADR's Decision item; such a
trigger fires on well over a hundred sites against roughly 16 genuine cases, and is correctly
infeasible. A verb-anchored detector (`amends`, `revises`, `supersedes`, `overrides`, optionally
prefixed `partially`) is a different proposition: it fires on a few dozen sites at worst, and
narrows further by excluding Alternatives-table rows and cases already covered by `supersedes:`
frontmatter. Two independent sweeps produced materially different hit counts for both triggers, so
this ADR deliberately records no precision figure: the ordering is robust and is all the argument
needs, while the exact numbers are not reproducible without a stated pattern and corpus scope. A
follow-up that builds the detector must measure it properly, with the pattern written down. What
matters here is that the infeasibility of the cite-a-Decision-item trigger does not transfer to
the verb-anchored one, and this ADR does not claim it does. `cmd/repoaudit` (ADR-0073) is the
natural home for such a rule: its rules are range-scoped and advisory rather than gating, so one
would fire only on newly-added amendments and would need no corpus backfill.

## Decision

1. **State the back-pointer rule in the procedure that governs the act.** The
   `supersedence-partial` section of `templates/skills/adr-lifecycle/SKILL.md.tmpl` gains a
   predecessor bullet: the successor's number is added to the predecessor's `related:` in the same
   commit. The section's closing bullet, which currently teaches the one-directional model, is
   corrected to name the linkage on both ADRs. The section renders from the template default with
   no local override, so the rule reaches every adopter.

2. **Reconcile the skill's append-only statements, which currently forbid the rule.** Seven
   surfaces state that a live ADR is status-only, and they live in two files. Three are the
   lifecycle table's `Accepted`, `Implemented`, and `Superseded` rows ("Status field only; the
   body is frozen"), which are **catalog data** (`adrStates` in `internal/catalog/standard.go`)
   rendered into the table, not template prose. Four are template prose in
   `templates/skills/adr-lifecycle/SKILL.md.tmpl`: the `supersedence-full` bullet ("the only
   allowed edit on a non-Proposed ADR"), `procedure-status-edit` step 1 ("the only allowed
   in-place edit on a live ADR"), the `amendment-while-proposed` closing sentence, and the
   `## Notes` append-only rule (the last two both reading "only the `status` field is editable in
   place"). Unlike Decision 4's list, `adrStates` is not overridden in `.awf/`, so it needs no
   second site. Left alone these contradict Decision 1 outright, and an agent has no way to obey
   both. Each is reworded to permit the `status` field
   **and cross-reference metadata** (`superseded_by:`, `related:`), on the principle that
   append-only protects rationale, not bookkeeping: the body stays frozen, and appending a number
   to `related:` rewrites no rationale. Sibling ADR-0115 draws the same line for orthography
   ("append-only protects rationale, not orthography"). No ADR Decision item owns the status-only
   rule, which grew in the skill alongside the convention itself, so this broadens a convention
   rather than amending a predecessor and owes no back-pointer under Decision 3.

3. **Scope the obligation precisely.** The back-pointer is owed when **this** ADR overrides a
   **live** (`Accepted` or `Implemented`) predecessor's **Decision item or Invariant**. Three
   cases are deliberately outside it: a citation that does not override (ADR-0079 cites ADR-0065
   Decision 4 informationally while amending Decision 3, and owes a back-pointer only for the
   latter); an amendment of a `Proposed` ADR, which can be edited in place instead; and an edit
   that changes wording without changing meaning, such as ADR-0115's retroactive title orthography.

4. **Charge the `adr-reviewer` with the rule, on both surfaces.** The shipped `adr-reviewer`
   doc-currency lens gains an item requiring that an ADR overriding a live predecessor's Decision
   item leaves the predecessor's `related:` naming it. The lens is the backstop for the case the
   procedure cannot reach, an author who did not realise they were amending. It is added in **two**
   places in the same commit: the catalog default (`docCurrencyItems` in
   `internal/catalog/standard.go`), so every adopter inherits it, **and**
   `.awf/agents/adr-reviewer.yaml`, because that file already overrides `docCurrencyItems`
   wholesale and would otherwise silently drop the new item for awf's own reviewer.
   `.awf/agents/code-reviewer.yaml` records the same hazard against its own `focusItems` override
   in a comment ("the first two entries are the catalog defaults, kept deliberately"), though that
   override restates the defaults it wants to keep while `adr-reviewer.yaml`'s `docCurrencyItems`
   restates none; the shared lesson is that a wholesale `data:` list override must be maintained
   deliberately at both sites, not that the two files are shaped alike. The
   ADR-0114 Decision 3 precedent is cited for *where the rule belongs* (the catalog, because the
   property is general to the standard), not for the mechanism: 0114 extended a template section,
   which awf does not override, whereas this is a `data:` list, which awf does.

5. **No mechanical check, for now.** This ADR adds no gate, no `awf check` note, and no
   `repoaudit` rule. The rationale is narrow and stated honestly: the procedure has never been
   tried, so the next measurement is the first informative one. This is a deferral on evidence,
   not a judgement that mechanism is impossible; the verb-anchored advisory rule sketched in
   Context remains available and is the expected next step if the rule still does not hold.

6. **The corpus is knowingly left non-conformant.** The 10 missing back-pointers are not
   backfilled here. They are pre-existing residue that neither the procedure (which governs new
   acts) nor the reviewer (which sees the ADR under review) will reach. Recording this explicitly
   is the point: a future observer must not mistake residue for recurrence.

7. **Correct the pitfall entry.** `.awf/docs/pitfalls.yaml`'s partial-amendment entry is rewritten
   to carry the measured 6-of-16 count, to distinguish the cite-a-Decision-item detector it
   proposed from the verb-anchored one that is actually available, and to restate its promotion
   condition as recurrence **despite** the procedure and the reviewer lens.

8. **The implementing commit regenerates the index.** Decisions 1, 2, 4, and 7 land in one commit
   together with this ADR's flip to `Implemented`; that commit runs `./x sync` and stages the
   regenerated `docs/decisions/ACTIVE.md`, the rendered skill and agent outputs across every
   enabled target, the `examples/sundial` outputs, and both `.awf/awf.lock` files. No plan exists
   for this effort, so this ADR is the only artifact that can carry the obligation.

## Invariants

This decision installs a convention and a review responsibility. It declares no backed or unbacked
invariant slug, following ADR-0112 and ADR-0114: the property it establishes is a division of
labour between the procedure and the reviewer, not a mechanically checkable state. In particular,
"every partial amendment carries a back-pointer" is **not** declared as an invariant, because it
is currently false in 10 places and Decision 6 declines to fix them; declaring it would assert a
contract the corpus contradicts.

These are therefore marker-free textual contracts:

- The `awf-adr-lifecycle` partial-supersedence procedure states the predecessor back-pointer
  alongside the successor's `related:`, and no rendered surface of that skill teaches the
  one-directional model or forbids the cross-reference metadata edit the rule requires.
- The obligation is scoped to a live predecessor's Decision item or Invariant overridden by this
  ADR; a non-overriding citation, an amendment of a `Proposed` ADR, and a meaning-preserving edit
  owe nothing.
- The `adr-reviewer` doc-currency lens carries the back-pointer check in both the catalog default
  and awf's own `.awf/agents/adr-reviewer.yaml` override, so every adopter's reviewer inherits it
  and awf's own does not silently drop it.

## Consequences

- **The rule finally reaches the point of use.** An agent performing a partial amendment now reads
  the obligation in the same section that tells it to set the successor's `related:`, rather than
  needing to have read a pitfall entry first.
- **The append-only rule is stated more precisely than before.** Decision 2 narrows "only the
  status field" to "status and cross-reference metadata". This is a real broadening of what may be
  edited on a live ADR, accepted because the alternative is a procedure that contradicts itself.
  The body remains frozen, which is the part the rule exists to protect.
- **The next failure is informative.** Because the procedure has never stated the rule, the three
  prior prose failures measured nothing about whether a stated procedure holds. The next
  measurement does, and Decision 5 names the concrete fallback so the follow-up is not a fresh
  investigation.
- **The corpus stays at 6 of 16 conformant.** Anyone re-measuring will find the same 10 gaps and
  must read Decision 6 before concluding the rule is failing. This is the accepted cost of not
  bundling a 10-file metadata migration into a convention change.
- **Three surfaces of prose can still drift from each other.** The skill, the catalog default, and
  awf's own reviewer override state the same rule, so a future refinement must sweep all three.
  This is the standing cost of a convention held by prose rather than by a single mechanical
  definition, and the reviewer-override duplication is the standing cost of the wholesale-list
  override semantics.
- **An adopter inherits the rule automatically** through the rendered skill and reviewer, with no
  configuration.
- **Deferring the check is a real bet.** Three prose failures in five days is evidence that prose
  is weak here; the counter-evidence is that none of those three had the rule in the procedure. If
  the bet loses, the cost is another silently-stale predecessor, caught at review rather than at
  commit.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Advisory verb-anchored `repoaudit` rule now, alongside the prose | The strongest rejected option, and not refuted: range-scoped, needs no backfill, materially better precision than the cite-a-Decision-item trigger, and consistent with ADR-0114 (which declined a gate, not all mechanism). Set aside because the procedure has never been tried, so building the check now would permanently confound what the prose alone can achieve. Explicitly the expected next step. |
| A gate or `awf check` note on back-pointers | Would red the gate on the 10 pre-existing violations, forcing a backfill migration into a convention change, and would harden a heuristic signal into a blocking one where a false positive costs a broken commit rather than a warning. |
| Declared `amends: [N]` frontmatter plus a derived back-edge | Converts the problem from detection to declaration, and is the most durable option long-term: the back-edge is rendered, so drift becomes impossible. Deferred as premature ahead of a schema addition; note it must render a list, since ADR-0104 is amended by both ADR-0105 and ADR-0106. |
| Require `related:` to be symmetric in general | Over 500 of the corpus's ~540 `related:` edges are asymmetric; the field is a forward reading list. Symmetry would demand a corpus-wide migration to fix ~16 edges, destroying the field's meaning. |
| Backfill the 10 violations in this commit | Metadata-only and mechanical, but it is a distinct concern from installing the convention, and bundling it would hide the convention change inside a migration diff. Left for a follow-up that can be reviewed on its own terms. |
| State the reviewer rule in the agent's template section instead of a `data:` list | Would sidestep the wholesale-override problem (a template section is not overridden here), and is what ADR-0114 did. Not chosen because the doc-currency checklist is data-shaped by design and the project override is legitimate; Decision 4 pays the two-site cost explicitly rather than reshaping the artifact to dodge it. |
| Leave the rule in the pitfalls doc only | The status quo, measured: three failures in the five days after the note landed. |
