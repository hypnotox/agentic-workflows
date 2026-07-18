---
status: Implemented
date: 2026-07-15
tags: [agents-guide, doc-standard]
related: []
domains: [rendering]
---
# ADR-0112: Core-only agent-guide Invariants section

## Context

The agent guide's `## Invariants` section is loaded into context every session, so it pays its word cost on every task. It is rendered from hand-authored data in `.awf/agents-doc.yaml` under `data.invariants` (plus three rules the template hardcodes), and the team has treated it as a mirror of the ADR ledger: roughly one bullet per Implemented ADR that declares an invariant. The list grew monotonically to **84 rendered bullets** (81 data entries + 3 hardcoded), because every new ADR felt like it earned a line and nothing pushed back.

Two forces make that ledger-mirroring both costly and redundant:

- **It is redundant with on-demand retrieval.** `awf context <paths>` now reports the governing invariants and related ADRs (tiered by relevance) for exactly the files a task touches, and the generated ACTIVE.md is the full standing index. An agent no longer needs the whole invariant set preloaded to find the rule for the file it is editing; the path-specific rules are reachable the moment they are relevant.
- **The authoring standard already asks for terseness but gives no decidable test.** `docs/agents-md-standard.md` says the guide is held to an "extra-terse bar" and to "push anything not every-session-critical into a doc," but supplies no criterion for *what* stays and no ceiling, so the guidance has not held, for this project or for adopters authoring their own guide.

The example adopter `examples/sundial` already models the terse target, shipping only three invariants. awf itself does not, and the standard does not tell an author how to decide.

Grounding confirmed nothing couples the guide's invariant count to the ADR set: no completeness or parity check exists against `docs/decisions/`. The only mechanical consumer of the list is the `kind: scopes` typed entry, which `TestGuideScopesDerived` (ADR-0055) asserts renders exactly one Conventional-Commits bullet, and the `kind: gated-commands` entry. Both are core rules and are retained.

## Decision

1. **The agent-guide `## Invariants` section holds only cross-cutting rules.** A rule belongs iff it is **not scoped to a single subsystem's files**: it constrains a whole cross-cutting surface (every commit's process, all code, all rendered output, or the toolchain's own preconditions) rather than one feature's territory. In awf's own guide those surfaces are: process rules (append-only ADRs, docs-travel-with-the-change), the pre-commit gate (green gate, 100% coverage, dead-code), commit hygiene (Conventional Commits and scopes), the drift oracle, the flagship rendering guarantee (publication-safe templates), the binary-version precondition on every gated command, and the meta-rule for how invariants are themselves declared and backed. Every path- or subsystem-specific invariant (one scoped to one feature's files, such as a glob dialect, a bootstrap contract, or a runner internal) is **contextual**: it stays authoritative in its owning ADR and is reached on demand, via `awf context` and the ACTIVE.md index where a project configures domains, or the ADR index otherwise.

2. **Record the criterion in the documentation standard.** `docs/agents-md-standard.md` (rendered from `templates/docs/agents-md-standard.md.tmpl`) states the retention test above in its Invariants guidance, so both awf's authors and adopters decide inclusion by a written rule rather than by feel. The standard is the single home for the criterion; it references `awf context` and ACTIVE.md generically, so it degrades to coherent guidance for a minimal adopter that configures neither.

3. **Trim awf's own list to the core set.** `.awf/agents-doc.yaml` `data.invariants` is reduced to the entries the criterion keeps, alongside the three the template hardcodes (append-only ADRs, docs-travel-with-the-change, green-gate-before-every-commit). The retained data entries are: publication-safe templates, `awf check` is the drift oracle, the `kind: scopes` Conventional-Commits entry, backed invariants (the meta-rule), the 100% coverage gate, the dead-code gate, and the `kind: gated-commands` binary-version-gate entry. Every other entry is removed; each removed invariant remains fully documented in its cited ADR, so the trim is de-duplication, not deletion. The trim commit runs `awf sync` + `awf check` (re-rendering `AGENTS.md` and `docs/agents-md-standard.md` from the edited config and template, and regenerating `docs/decisions/ACTIVE.md` when the same commit flips this ADR to `Implemented`), so reality and its rendered docs move together.

4. **No new mechanical check.** The criterion is enforced by convention through the standard and by workflow review, not by a gate or an `awf check` note. Guarding list size mechanically is explicitly out of scope.

## Invariants

These are deliberately marker-free textual contracts; this is a guidance decision that adds no machine-checkable slug and no gate:

- The agent-guide `## Invariants` section lists only cross-cutting rules not scoped to a single subsystem's files; a path- or subsystem-specific invariant lives in its owning ADR, not the guide.
- Every invariant trimmed from the guide remains fully documented in its cited ADR; removing a guide bullet loses no authoritative content.
- The `kind: scopes` and `kind: gated-commands` typed entries are retained, so `TestGuideScopesDerived` and the binary-version-gate rendering continue to hold.
- The retention criterion recorded in the documentation standard references `awf context` and ACTIVE.md generically and degrades to coherent guidance when a project configures no domains.

## Consequences

- **The guide gets materially cheaper every session.** The Invariants section drops from 84 bullets to ~10, cutting the standing per-task word cost while keeping the rules that actually gate every commit.
- **Contextual invariants move to just-in-time discovery.** An agent working on, say, the glob dialect or the bootstrap porcelain finds those rules through `awf context` and the ADR rather than in the guide. This is the intended trade: relevance over completeness. The risk (an agent missing a rule it would have seen in the full list) is mitigated because `awf context` surfaces path-governing invariants precisely when the files are touched, which the flat list never did.
- **Adopters inherit a decidable rule.** The standard now answers "does this belong in the guide?" with a test, so an adopter's guide should not re-accrete the ledger the way awf's did. The example adopter already demonstrates the target.
- **No enforcement backstop.** Because the criterion is guidance-only, a future author can still over-add; catching that is a review responsibility, not a mechanical one. This was chosen deliberately to avoid a rigid gate that would block a genuine spike of core rules.
- **One-time trim churn.** The trimming commit removes ~74 lines from `.awf/agents-doc.yaml` and re-renders `AGENTS.md`; reviewers should read it as de-duplication against the ADR corpus.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Advisory `awf check` note above a size threshold | A durable nudge, but adds a mechanism for a problem the standard plus review can hold; the user scoped this to guidance-only. |
| Hard gate on list size or redundancy | Strongest guarantee but rigid: a real spike of core rules would be blocked, and it couples the guide to a count the ADR set does not otherwise track. |
| Generate the core list mechanically from ADRs flagged "core" | Removes the manual vector but introduces a new ADR-flag schema and a generator for a list that is small and stable once trimmed; not worth the machinery. |
| Leave the criterion only in git history (no ADR) | The 84→10 shrink and the "don't re-add contextual invariants" rule are exactly the kind of load-bearing decision a future author needs to find; recording it prevents silent re-bloat. |
