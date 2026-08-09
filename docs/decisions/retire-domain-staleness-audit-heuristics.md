---
format: current-state-v4
slug: retire-domain-staleness-audit-heuristics
status: Proposed
date: 2026-08-09
---
# ADR-retire-domain-staleness-audit-heuristics: Retire Domain Staleness Audit Heuristics


## Context

ADR-0019 introduced branch-level warnings that inferred stale domain narratives from ADR
`domains` frontmatter. ADR-0077 added a second warning that inferred the same condition from
changed files matching domain sidecar paths. Those rules predate canonical current-state topics.

Governed ADRs now declare exact topic-claim operations, and audit replays each operation against its
claim mutation. Governed ADRs derive their affected domains from qualified operation IDs rather than
carrying `domains` frontmatter. Topic claims are current authority; domain narratives are overview
and navigation surfaces. The old ADR-driven rules therefore consume a legacy signal, while the
code-driven rule requires a non-authoritative document co-change without proving semantic
freshness. Redirecting the heuristic to topic files would encourage unrelated prose edits and would
duplicate deterministic coverage and render checks without proving that implementation and prose
agree.

Domain sidecar selectors remain structural input to topic coverage and context ownership. Their
anchored-glob validation must survive removal of the audit input assembly that currently performs
that validation.

## Decision

1. `decision: retire-domain-staleness-heuristics` Remove the domain-document staleness,
   domain-code staleness, and undocumented-domain audit rules. Audit will not infer semantic
   documentation freshness from ADR domain tags, changed implementation paths, or co-changed
   domain narratives.
2. `decision: preserve-exact-authority-checks` ADR operation replay remains the exact check for
   declared claim mutation and atomicity. Topic coverage, ownership, selector, navigation, and
   rendered-output checks remain responsible for structural current-state integrity; none is
   rebranded as semantic staleness detection.
3. `decision: validate-domain-selectors-structurally` Domain sidecar path selectors remain subject
   to the shared anchored-glob validation at a structural project-loading boundary independent of
   audit-rule input assembly.

## State changes

- update `tooling/audit-and-snapshots:audit-advisories-always-run`
- remove `tooling/audit-and-snapshots:audit-domain-code-staleness`
- remove `tooling/audit-and-snapshots:audit-domain-doc-staleness`
- remove `tooling/audit-and-snapshots:audit-undocumented-domain`
- add `config/validation:domain-path-globs-valid`

## Consequences

Audit findings no longer ask for domain prose churn merely because a branch changed code or
completed an ADR. Claim transitions retain a stronger, identity-specific check, and malformed or
uncovered authority retains its structural checks. ADR-less implementation churn no longer receives
a generic documentation reminder; semantic review and the project rule that documentation travels
with change remain responsible for deciding whether explanatory prose must change.

Moving domain-selector validation prevents retirement of the warnings from silently making malformed
ownership patterns match nothing. The audit implementation and its tests become smaller, while
historical ADRs remain available as decision history without keeping their obsolete warning model
active.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Retarget staleness warnings to topic files | A co-change remains only a heuristic, duplicates structural checks, and can reward meaningless edits. |
| Retire only the ADR-frontmatter rules | The code-driven rule would still target a domain narrative that is no longer current authority. |
| Keep all rules as harmless advisories | Persistent false-positive pressure obscures the exact operation and coverage checks that now own the enforceable contracts. |

## Status history

- 2026-08-09: Proposed
