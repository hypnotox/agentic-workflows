---
format: current-state-v2
status: Implemented
date: 2026-07-23
---
# ADR-0151: Split overloaded invariant claims into focused claims

## Context

The 2026-07-22 hands-on analysis of the context/topic system confirmed the system works
well but flagged claim density as a usability smell: the concise `awf context` projection
surfaces claim prose directly, and some single claims bind three to five independently
enforceable obligations in one sentence. The per-topic budget advisory caps claim count,
but nothing pressures per-claim scope, so an agent reading concise context must parse a
paragraph-length sentence to find the one obligation that applies to its change.

A survey of all 380 current claims (length-ranked, then judged by obligation count rather
than length alone) found two kinds of long claims. Most are single obligations stated
exhaustively: a migration recipe enumerating its steps, an audit rule listing its silence
conditions. Those are coherent and stay whole. Three claims, however, genuinely bundle
independently enforceable contracts:

- `tooling/cli:version-compat-gate` binds the ordinary two-axis binary-version gate
  together with the full validation litany of the private `dashboard-read` dispatch
  bypass.
- `tooling/cli:metrics-and-doctor-command-contract` binds the `awf metrics` command
  contract, the `awf doctor` command contract, and the read-helper shape closure that
  serves the pinned dashboard dispatch.
- `tooling/context-and-topic:context-full-authority-packet` binds the concise
  projection's topic-entry contract and the `--full` projection's contract.

Claim mutations are mechanically ADR-gated: the transition validator pairs every claim
add, update, or remove with an operation of a governing ADR, so this hygiene work
requires an ADR even though it changes no behavior.

Scope constraint: several other length-ranked offenders live in the Pi workflow and
dashboard-runtime topics on which ADR-0149 still holds remaining operations, and two
concurrent in-flight efforts hold their own proposed ADRs numbered 0151 on their
branches (the contiguity check forbids number gaps, so every branch scaffolds the next
contiguous number and later-merging efforts renumber at merge). Splitting claims under active mutation by another effort would force claim
conflicts, so those offenders are excluded here and can be revisited once ADR-0149 is
fully applied.

A sibling effort from the same analysis lands the ADR-free output changes (suppressing
glob-literal domain attribution for user-typed not-found queries, annotating collapsed
unowned directories with counts, and hoisting the repeated domain-selector block in the
text rendering); none of those mutate claim meaning, which is why this record covers
only the splits.

## Decision

1. A claim is overloaded when its prose binds multiple independently enforceable
   obligations, each verifiable by a distinct test. Splits are meaning-preserving: the
   successor claims' prose together carries every obligation of the predecessor, adding
   none and dropping none. Each successor claim's prose is self-contained: it names its
   own subjects, commands, and shared selectors directly rather than referring to an
   obligation that now lives in a sibling claim.
2. `tooling/cli:version-compat-gate` is narrowed to the ordinary gate contract: every
   ordinary gated command routes through the gate, refusal when the binary is behind the
   project on either the config schema generation or the lock `awfVersion` axis,
   permission at-or-ahead on both, and the statement that the only bypass is the private
   closed `dashboard-read` dispatch.
3. The `dashboard-read` bypass validation litany (exact read-only argv, absolute
   canonical project root, adjacent pinned executable, metadata and policy digests,
   repository identity, pinned commit, snapshot schema and protocol, confined metrics
   root, no live tracked config) moves to the new claim `tooling/cli:dashboard-read-dispatch`,
   which also absorbs the shape closure formerly held by
   `tooling/cli:metrics-and-doctor-command-contract`: the `awf metrics` and `awf doctor`
   read helpers accept an explicit
   root and validated telemetry policy, and the dispatch can invoke only the protocol,
   JSON metrics, JSON export, and JSON doctor shapes against its immutable policy
   snapshot while ordinary invocations retain live project and version gates. No
   mutation or maintenance shape can reach dispatch.
4. `tooling/cli:metrics-and-doctor-command-contract` is retired and its remaining
   obligations partition into two focused claims: `tooling/cli:metrics-command-contract`
   (the gated, runner-forwarded `awf metrics` command queries canonical projections or
   exports validated normalized events through the shared effort, session, phase, and
   time selectors, with mutation and maintenance children closed to their own flags) and
   `tooling/cli:doctor-command-contract` (the gated, runner-forwarded `awf doctor`
   command consumes the same shared effort, session, phase, and time selectors and the
   same storage interpretation as `awf metrics`, remains read-only, and never changes
   exit status merely because findings exist).
5. `tooling/context-and-topic:context-full-authority-packet` is narrowed to the `--full`
   projection contract: `context --full` renders every current claim's full detail and
   pending operations once per applicable topic with no omission line, from the same
   non-recursive semantic model as the concise projection, and managed
   complete-authority callers request `--full` explicitly. The concise-projection
   obligations (one topic entry per applicable topic per invocation carrying the
   uncapped current claim-ID roster, the full detail of exactly the marker-selected
   direct-claim union, and an explicit detail-omission line with the topic drilldown
   whenever any rostered claim's detail is omitted) move to the new claim
   `tooling/context-and-topic:context-concise-projection`.
6. All four added claims are invariant claims with `Backing: test`, each proven by its
   own proof marker on a test asserting exactly its obligations. Markers follow their
   obligations: a marker site whose asserted obligation stays with a narrowed claim
   keeps its marker unchanged, while a site whose asserted obligation moved to a
   successor is repointed to that successor in the same transaction that applies the
   operation.
7. Each Applied batch carries exactly its matching claim mutations. The two updated
   claims preserve their `Origin` and exact prior `Revised-by` prefix and append
   ADR-0151 to `Revised-by`; the four added claims carry `Origin` ADR-0151.
8. Every status transition of this ADR regenerates `docs/decisions/INDEX.md` via
   `./x sync` and stages it within the same transaction, which passes
   `awf check --staged` and `./x gate` before committing.
9. Surveyed long claims that state one obligation exhaustively (the migration recipes,
   the audit staleness rules, the in-place readback mechanism) stay whole. No mechanical
   claim-length gate or advisory is introduced; whether length pressure should become a
   doctor heuristic remains a separate future decision.

## State changes

- update `tooling/cli:version-compat-gate`
- remove `tooling/cli:metrics-and-doctor-command-contract`
- add `tooling/cli:metrics-command-contract`
- add `tooling/cli:doctor-command-contract`
- add `tooling/cli:dashboard-read-dispatch`
- update `tooling/context-and-topic:context-full-authority-packet`
- add `tooling/context-and-topic:context-concise-projection`

## Consequences

- Concise context output surfaces shorter, single-obligation claims, and direct-claim
  markers can select the narrow contract a site actually touches instead of a bundle.
- The implementation must re-author prose for both narrowed claims and all four
  successors meaning-preservingly, repoint every existing proof and relevance marker
  that names the retired or narrowed ids, and give each successor its own backing test
  proof marker; the transition validator and ADR review are the guards against meaning
  drift during re-authoring.
- Claim count rises by three while total prose volume stays roughly constant; the
  per-topic budget advisory accounts for the higher count.
- The Pi-area offenders excluded for concurrency reasons remain overloaded until a
  follow-up after ADR-0149 is fully applied; this record neither blocks nor schedules
  that follow-up.
- A removed id is never reused: `tooling/cli:metrics-and-doctor-command-contract`
  retires permanently.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a doctor heuristic pressuring claim length | New mechanism with advisory noise; does not fix the existing offenders; length is a weak proxy for overload |
| Split every length-ranked claim repo-wide | Length is not the criterion; migration recipes and rule-boundary claims are single obligations stated exhaustively and lose coherence when fragmented |
| Include the Pi workflow and dashboard-runtime offenders now | ADR-0149 holds remaining operations on those topics and two concurrent efforts are mid-flight there; cross-effort claim churn is worse than deferred hygiene |
| Leave the mega-claims as they are | User-confirmed usability smell; the concise projection exists to be read, and bundled obligations defeat its purpose |

## Status history

- 2026-07-23: Proposed
- 2026-07-23: Implemented; content-sha256: 9c1bb3013ca4dce4cf88e1b5895009b70a522170615a50beee8457226ca6b07d; state-sequence: 28
