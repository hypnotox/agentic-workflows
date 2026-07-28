---
format: current-state-v2
status: Implemented
date: 2026-07-28
---
# ADR-0173: Request-Sensitive Context Authority Tiers

## Context

ADR-0165 made `awf context` request-oriented, compacted directory censuses, introduced explicit
facets, and safely spilled oversized results. Its implemented default nevertheless remains an
authority projection whose size is dominated by path-level claim rosters rather than useful
orientation. The representative bare query `./awf context internal/project cmd/awf` renders
495,396 bytes. `internal/project` alone renders 381,239 bytes, `cmd/awf` alone renders 114,241
bytes, and even the exact file `cmd/awf/context.go` renders 17,324 bytes despite having no direct
claim marker.

The dominant `cmd/awf` costs are about 52 KiB of repeated `Invariants:` rosters and 51 KiB of
`Proofs:` rosters, followed by about 12 KiB of invariant summaries in authority output. The
semantic model currently assigns every applicable test-backed invariant to each path's
`ProofIDs`; it does not mean that the selected path contains a proof marker. Directory grouping
also includes direct-claim IDs and complete invariant and proof rosters that the bare path
projection does not explain as relationships. Groups can therefore appear identical while being
split by invisible fields, and a directory's broad applicable authority dominates a result before
the caller asks for authority detail.

Request origin is already meaningful but is lost during relationship projection. Directory and
exact-file relationships are unioned before topic authority is projected. A mixed request can
therefore promote directory-only claims into what appears to be exact-file detail. Conversely,
removing all directory relationships during compaction would make an explicit detail facet unable
to recover them. The model needs separate request-level directory aggregation and file-specific
relationships.

Directory requests and file requests express different needs. A directory asks for an area map:
census, compact groups, classification, provenance, domains, topics, and enough pending state to
warn of in-flight change. An exact file, a staged file, or a range-selected file asks how that file
relates directly to current authority. Neither request asks for every applicable invariant by
default. Applicable non-direct invariants remain important, but they belong behind an explicit
facet rather than in every bare answer.

ADR-0165 is Implemented and explicitly requires the current invariant and proof rosters and
default invariant summaries. It remains frozen history. This successor changes the active
projection contract while preserving ADR-0165's request census, classification, snapshot, summary,
spill, and bounded pending contracts.

## Decision

1. `awf context` uses request-sensitive default authority tiers. A directory request defaults to
   tier 0 orientation. An exact-file request and every file selected by `--staged` or `--range`
   defaults to tier 1 file relationships. These tiers control visible relationship and authority
   fields; they do not change path classification, topic applicability, snapshot semantics, or
   output delivery.

2. Tier 0 contains the directory census, compact groups, primary classification, compact artifact
   provenance, owning domains, applicable topic IDs, one-line topic summaries, authority counts,
   warnings, and ADR-0165's bounded pending-operation summary. Authority counts report, per
   applicable topic, the number of active invariant claims and active non-invariant rule claims;
   pending operation counts remain separate. Hidden directory relationships do not contribute to
   these counts or to tier-0 group equivalence. Tier 0 does not render direct claim-ID rosters,
   invariant or proof rosters, or claim summaries.

3. Tier 1 adds only relationships actually declared on the selected file. File relationships are
   represented by marker kind and render only non-empty `State`, `Touches`, and `Proofs` claim-ID
   sets. `Proofs` means proof markers located on that file, never every test-backed invariant in
   an applicable topic. A file with no direct markers receives no relationship roster merely
   because topics apply to it.

4. Directory relationships remain in a separate request-level aggregate instead of being unioned
   into file relationships or discarded during tier-0 compaction. This aggregate records the
   direct state, touches, and proof relationships found among included descendants and supports
   explicit authority expansion. In a mixed directory and file invocation, directory-only
   relationships never enter the default tier-1 detail of an exact or Git-selected file.

5. Add `relationships` to the closed `--show` facet set. `--show relationships` promotes directory
   requests to tier 1 by expanding their request-level aggregated direct relationships in the
   authority projection. Claim bodies remain globally deduplicated, but every directly related
   claim carries sorted source attribution by request index and marker kind; source attribution is
   additive and is never removed by claim deduplication. Each request block retains its own marker
   sets, so a mixed invocation shows exactly which directory or file established each relationship.
   The facet does not add applicable non-direct invariants and does not fragment directory groups by
   descendant marker combinations.

6. Add `invariants` to the closed `--show` facet set. `--show invariants` renders one-line summaries
   for applicable non-direct invariant claims. Direct invariant claims already visible through a
   tier-1 relationship retain their closest category and are not repeated.

7. `--show all-rules` continues to render applicable non-direct rule summaries. It does not imply
   `relationships` or `invariants`. The closest-category order becomes directly related claims,
   applicable non-direct invariants, additional topic rules, referenced context, and pending
   changes, with empty categories omitted.

8. `--full` is exactly the deterministic union of `relationships`, `invariants`, `all-rules`,
   `evidence`, `selectors`, `references`, `pending`, and `artifacts`. It remains an explicit detail
   convenience, never a managed workflow default, and preserves the request-sensitive grouping
   model rather than restoring descendant-level path output.

9. `evidence` and `references` enrich claims already visible through the request's default tier or
   an authority-expansion facet. Neither facet makes a hidden claim visible merely to attach
   evidence or a reference edge. Reference expansion remains one level and closest-category
   deduplication still applies.

10. Default pending output remains the bounded operation summary for every applicable topic in
    both tiers. `--show pending` continues to expand applicable operations independently of claim
    relationship and authority facets.

11. Bare directory grouping uses only fields visible in tier 0. Invisible relationship sets and
    expanded artifact details never split a bare group. `--show artifacts` may refine groups when
    the detailed provenance it makes visible differs. Relationship and authority-only facets,
    including `relationships`, `invariants`, `all-rules`, `evidence`, and `references`, do not
    refine descendant path groups.

12. Projection and grouping are semantic and deterministic, not byte-driven. The implementation
    does not truncate a first-N claim roster, choose fields based on rendered byte size, or change
    result meaning to avoid a spill. Explicit detail can legitimately exceed the delivery cap and
    spill under ADR-0165's existing contract.

13. The repository fixtures establish regression budgets rather than a universal adopter limit.
    Bare `awf context internal/project cmd/awf` and bare `awf context cmd/awf/context.go` each render
    no more than 8,192 bytes. Arbitrarily many requests, adopter topics, explicit facets, and
    pending data remain data-dependent.

14. Managed workflow callers keep lens-specific invocations: orientation and implementation begin
    bare; plan and implementation review request `invariants`, `all-rules`, `evidence`, and
    `pending`; plan/ADR resync requests `invariants`, `all-rules`, and `pending`; ADR lifecycle
    requests pending detail where needed; and no managed caller prescribes `--full`. Exact, staged,
    and range-selected files receive tier-1 relationships without a new flag, while a managed
    directory query remains tier 0 unless its lens explicitly needs another facet. Spill
    consumption remains unchanged.

15. Help, `README.md`, the working guide and its template, the agent-guide command convention,
    glossary and architecture descriptions, testing guidance, current-state claims, rendered topic
    documents, managed caller guards, stale testing prose, and an Unreleased entry in
    `changelog/CHANGELOG.md` move with implementation. Documentation must no longer describe removed
    context JSON parity or recommend routine managed `--full` use.

16. Tests cover independent and composed new facets, full-as-union, evidence and references as
    non-expanding enrichments, marker-kind file relationships, actual proof-marker filtering,
    request-level directory aggregation and source attribution, mixed directory and exact-file
    isolation, defined tier-0 counts and grouping, artifact-sensitive grouping, bounded pending
    summaries, help errors, managed caller packets, representative byte budgets, snapshot
    preservation, and generated documentation parity. Every affected template continues to render
    with empty-string variables under `missingkey=zero` without emitting a no-value token.

17. State changes apply in declaration order through checked application batches. Updates preserve
    `Origin`, the existing `Revised-by` prefix, and backing mode while appending ADR-0173. Every
    Applied event lands with exactly its corresponding claim mutations and proof changes, and no
    Remaining operation lands early.

18. Every lifecycle status transition runs `./x render` and commits the regenerated
    `docs/decisions/INDEX.md` and lock update in the same transaction.

## State changes

- update `tooling/context-and-topic:context-default-excludes-history`
- update `tooling/context-and-topic:context-concise-projection`
- update `tooling/context-and-topic:context-full-authority-packet`
- update `tooling/context-and-topic:context-known-artifact-navigation`
- update `tooling/context-and-topic:context-path-attribution`
- update `rendering/workflow-skill-templates:implementer-context-grounding`

## Consequences

Bare directory context becomes an implementation map instead of a topic-wide authority dump, and
bare file context answers which claims the file itself marks rather than listing everything that
could apply. The representative repository queries fit within the normal delivery boundary, so
the spill mechanism becomes an exception for requested detail or genuinely large inputs instead
of the ordinary result of asking about a central package.

Separating directory aggregates from file relationships makes mixed requests truthful and keeps
explicit detail recoverable. The semantic model grows another boundary: request-level directory
relationships cannot be inferred from the file projection after grouping, so assembly, projection,
and tests must preserve them deliberately.

The two new facets make authority expansion explicit and composable. Callers can independently ask
for direct relationships, non-direct invariants, or non-direct rules. More facet combinations must
be tested, and `--full` changes as the union grows, but existing facet names retain their narrow
meaning.

Projection-sensitive grouping prevents invisible data from creating visibly duplicate groups.
Artifact detail is the sole facet allowed to refine groups because it exposes descendant-level
fields; relationship and authority facets operate on request-level or topic-level authority and
therefore do not trade compaction for detail.

No hard universal output guarantee is introduced. Repository byte budgets catch regressions in the
known central-package fixture, while semantic completeness and the existing secure spill contract
remain authoritative for larger adopter data and explicit detail.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep invariant summaries but remove only path rosters | The central directory remains dominated by non-direct authority, and a bare query still answers a question the directory caller did not ask. |
| Remove path rosters without preserving directory relationships | Compaction would irreversibly discard direct marker information, so an explicit relationship facet could not recover it. |
| Treat every applicable test-backed invariant as a file proof | Backing is a claim property; `Proofs` must describe proof markers located on the selected file. |
| Let evidence or references make claims visible | An enrichment facet would become an implicit authority-expansion switch and make composition surprising. |
| Refine groups for every selected facet | Authority-only detail would recreate descendant fragmentation even though it renders at request or topic level. |
| Truncate claim lists to meet 8,192 bytes | First-N authority is arbitrary, hides relevant claims, and makes semantics depend on repository ordering or rendering size. |
| Make 8,192 bytes a universal command guarantee | Request count, adopter authority, explicit detail, and pending operations are unbounded; ADR-0165 already provides safe complete delivery. |
| Amend ADR-0165 | Implemented ADRs are frozen history; this decision corrects its active claims through explicit successor operations. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Implemented; content-sha256: aa39c3bd2b29e99083a7dd35f3867211c02e4112e14cf4ec816aa16668af17ca; state-sequence: 77
