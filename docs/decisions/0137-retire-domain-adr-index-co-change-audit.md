---
format: current-state-v1
status: Implemented
date: 2026-07-20
---
# ADR-0137: Retire domain ADR index co-change audit

## Context

The current-state cutover removes ADR indexes from domain documents. Domain documents now navigate
current-state topics, while `docs/decisions/INDEX.md` is the only generated decision index.

The migrated topic corpus nevertheless retained the test-backed
`tooling/audit-and-snapshots:audit-adr-domain-cochange` claim from ADR-0033. Its audit rule requires
each ADR status change to co-change every domain document named by the ADR. Because ADR status no
longer contributes to domain documents, that rule requires unrelated or empty domain-document
changes and contradicts the new output model.

The retained status-index audit also evaluates historical legacy ADR transitions. Retargeting it
unconditionally from `ACTIVE.md` to `INDEX.md` would misreport valid pre-cutover commits in explicit
audit ranges because those commits predate `INDEX.md`.

## Decision

1. Remove the `adr-domain-cochange` branch and finding from the status-cochange audit rule, the
   `DomainsIndexDir` audit input, its `Project.Audit` assignment, and the dedicated test and proof
   marker. Preserve the shared status-cochange machinery.
2. Apply `adr-status-cochange` only to current-state-v1 ADR additions and status transitions, and
   require those commits to co-change `docs/decisions/INDEX.md`.
3. Skip legacy-format ADRs in this rule. Historical transitions remain readable but are outside this
   current-state audit contract.
4. Remove the obsolete domain-cochange invariant claim and revise the status-cochange claim to state
   the v1 boundary. Do not reuse or reinterpret the removed claim id.

## State changes

- remove `tooling/audit-and-snapshots:audit-adr-domain-cochange`
- update `tooling/audit-and-snapshots:audit-adr-status-cochange`

## Consequences

Current-state ADR status changes continue to travel with the generated decision index. They no
longer require changes to domain documents whose content is derived only from current-state topics.

Explicit audit ranges no longer check status-index co-change for legacy-format ADR transitions. This
avoids judging historical commits against an output that did not yet exist. The staged transition
check and current-state claim handshake remain the stronger correctness checks for new work.

The cutover must delete the obsolete claim and proof marker together and revise the surviving claim.
This ADR records a forward correction after seal consumption instead of rewriting the reviewed
bridge preparation history.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the domain co-change rule | It enforces a dependency the new renderer deliberately removed. |
| Reinterpret the removed claim as INDEX co-change | That would silently change a sealed invariant's meaning instead of recording its retirement. |
| Retire status co-change auditing too | Per-commit range audit still detects a missing generated INDEX update independently of staged transition validation. |
| Require legacy commits to co-change ACTIVE.md | It would retain legacy output knowledge in the permanent current-state audit engine. |
| Rebuild the preparation commit and both seals | The consumed seals are sound; v1 State changes are the explicit forward mechanism for this correction. |

## Status history

- 2026-07-20: Proposed
- 2026-07-20: Implemented; content-sha256: 1cf0d4e461b5d9943c826eb3bf1a1e1bf20c419b3cf40b31e922a1a161339925; state-sequence: 1
