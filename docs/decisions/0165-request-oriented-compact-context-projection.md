---
format: current-state-v2
status: Proposed
date: 2026-07-27
---
# ADR-0165: Request-Oriented Compact Context Projection

## Context

`awf context` is intended to give an agent concise implementation orientation, but its current
path census and topic-grouped authority projection still scale with directory size rather than
with distinct context. An explicit query over three implementation directories emitted 141,450
bytes for 201 effective files: 96 under `internal/project`, 52 under `internal/telemetry`, 49
under `cmd/awf`, and four elsewhere. Broad topic selectors repeated nearly identical domain,
topic, claim, and path attribution across those files. The result was too large to use as an
orientation packet and required more tooling merely to reason about the tooling output.

ADR-0144 introduced per-path attribution and concise/full projections. ADR-0147 corrected
per-path authority repetition by grouping authority by topic. ADR-0155 then made concise and
full context projections part of managed workflow policy. Those decisions remain historical
records, but measurements against their implemented result show that topic grouping alone does
not solve directory expansion, uncapped claim rosters, evidence-site censuses, or the use of
`--full` as a routine reviewer packet. Their current-state effects must be corrected forward.

Git selections and explicit file arguments have different intent from directory arguments. A
Git range already names exact changed files, and a user who needs exact detail can pass exact
files. A directory argument instead asks for orientation over an implementation area. Expanding
that request into one visible entry per descendant discards the request boundary and makes mixed
areas especially noisy. The command needs a request-oriented semantic model, not renderer-only
line folding or a second drilldown identifier protocol.

The current human and JSON projections also impose two public contracts despite no known JSON
consumer in this repository. Concise JSON is not a compact escape from the text problem, and
preserving a complete hidden path census there would defeat compaction. A future structured
format should be designed around a demonstrated consumer rather than maintained speculatively.

Output size remains data-dependent even after semantic compaction. Optional evidence, reference,
pending, and artifact detail can still exceed a terminal or agent-context budget. The command
therefore needs a hard delivery boundary that preserves the complete rendering without silently
truncating it. That spill is the sole exception needed to the command's no-write wording: it is
an output-delivery file outside the repository, not project state. This repository also needs a
local, ignored advisory record when its source-running wrapper observes such a spill, so repeated
oversize queries can be corrected or promoted rather than forgotten.

## Decision

1. `awf context` becomes a request-oriented impact report. The semantic result preserves
   first-seen positional request order and renders each top-level human entry as `[n] <original
   path argument>`. An exact-file request reports that file directly. `--staged` and `--range`
   selections identify their selection origin and remain sorted exact-file entries. Overlapping
   positional arguments retain separate request blocks; authority shared by them is deduplicated
   globally.

2. A directory request reports included and excluded descendant counts. Excluded counts are
   broken down by primary path classification, without excluded path names. A directory with no
   included descendants remains an explicit request entry. Existing snapshot isolation,
   eligibility rules, classification precedence, symlink handling, nested-adopter boundaries,
   and deterministic ordering remain unchanged.

3. Included directory descendants are grouped when their classification, artifact provenance,
   domains, topics, direct claims, invariants and proofs, and warnings are identical. Every group
   reports its member count. A group of at most three files lists every member; a larger group
   lists no members, examples, or directory tree. Exact-file and Git-selected entries are never
   compacted this way. No JSON or other projection retains a hidden complete directory census.

4. Path groups carry compact classification and provenance, owning domains, applicable topics,
   directly marked rule IDs, and applicable invariant and proof IDs. Topic authority is
   partitioned into directly related claims, applicable non-direct invariants, additional topic
   rules, referenced context, and pending changes. A claim appears in only its closest applicable
   category, so an optional facet cannot make tangential authority look directly selected.

5. Default authority is an impact map, not embedded full authority. It shows each applicable
   topic's one-line summary, every applicable invariant's one-line summary, and one-line
   summaries only for rules directly marked by selected files. It does not print an uncapped
   claim-ID roster or every topic-wide rule. Implemented ADR prose and unrelated history remain
   excluded from current authority.

6. A claim may declare optional single-line `Summary:` metadata of at most 160 Unicode code
   points. Context prefers that summary. Without it, context whitespace-folds the first claim
   paragraph and truncates at a word boundary to at most 160 code points, using ASCII `...` when
   truncated. Summary derivation and ordering are deterministic in every snapshot mode.

7. Optional detail is selected by repeatable `--show <facet>`. The closed initial facet set is
   `all-rules`, `evidence`, `selectors`, `references`, `pending`, and `artifacts`. `--full`
   remains shorthand for the union of every facet, but it changes only detail and never changes
   request grouping, restores a path census, or repeats authority per path. `--show` and `--full`
   are invalid with `--uncovered`.

8. The `evidence` facet uses bounded disclosure per claim and marker kind. It lists all state,
   touches, or proof sites when there are at most three of that kind and otherwise prints only
   the count. The `selectors` facet exposes owning-domain and topic selectors with their
   both-must-match rule. `awf topic --coverage` remains the explicit unbounded census drilldown.

9. The `references` facet expands direct incoming and outgoing claim-reference edges by one
   level only. Referenced claims outside applicable topics receive one-line summaries under
   referenced context. A claim already present in a closer category is not repeated, while the
   origin claim retains the direction and target ID of its edge.

10. Default pending output is one summary line containing the applicable operation count and at
    most three ADR IDs, followed by `+N ADRs` when needed. `--show pending` expands the applicable
    remaining operations without changing their relevance category. An explicit ADR path keeps
    its lifecycle summary; pending detail expands its declarations and operation progress, while
    evidence detail adds only linked current or historical evidence. ADR prose remains pending
    intent or decision history, never current authority.

11. Artifact attribution remains derived from loaded config, layout, catalog, output plan,
    manifest, topic layout, and ADR corpus rather than path-lookalike heuristics. The default
    group projection shows compact provenance; `--show artifacts` expands deterministic roles,
    source and output edges, catalog identity, and navigation. Artifact detail participates in
    grouping but never forces disclosure of a large group's member paths.

12. Human text becomes the sole output contract. Remove `--json` from normal context and from
    `--uncovered`; combinations that previously selected JSON fail as unknown or invalid flags.
    No compatibility bridge or alternate hidden serialization is retained.

13. Both normal and uncovered output render completely into a buffer. At or below 8,192 final
    bytes, the command writes that exact rendering to stdout. Above 8,192 bytes, it securely
    creates a mode-0600 temporary text file outside the repository, writes and closes the exact
    complete rendering, and prints only a compact notice containing rendered byte count, format,
    and path. The successful caller owns deletion. Create, write, or close failure, or failure to
    deliver the stdout notice, triggers best-effort deletion and returns the original error.

14. Context assembly remains snapshot-consistent and does not mutate config, lock, outputs,
    caches, or other repository state. A successful oversize query may create only the temporary
    delivery file described above. Working and staged authorities never mix. Static fallback
    remains successful, human-only, subject to the same output cap, and states that live
    classification and authority require an adopted tree.

15. `--uncovered` retains its existing coverage universe and topmost fully-unowned directory
    collapse. Its human report uses the same final-byte cap and spill delivery path as ordinary
    context. Removal of JSON retires output-parity authority rather than replacing it with a
    second projection contract.

16. This repository's hand-written `./x context` verb delegates to `./awf context`. When it
    recognizes the stable spill notice, it prints the notice normally and appends a timestamp,
    rendered byte count, and shell-escaped invocation, but not the ephemeral spill path, to the
    ignored permission-restricted `.awf/local/context-spills.log`. `./x check` emits a
    non-failing advisory while that log is nonempty, directing the operator to resolve or
    promote the issue and remove the log. There is no clearing command and no gate failure.

17. Managed skills never prescribe `--full`. Brainstorming and implementation callers use bare
    context. Plan and implementation review request `all-rules`, `evidence`, and `pending`.
    Plan/ADR resync requests `all-rules` and `pending`. ADR lifecycle requests pending detail
    where lifecycle work needs it. Dispatchers continue to pass resolved arguments, and agents
    that need a packet run the command themselves. Authored templates, rendered target copies,
    command guidance, and the projection-pinning spine test move together.

18. Tests cover request order and overlap, grouping signatures and the three-file disclosure
    boundary, empty and excluded directories, exact-file and Git selection behavior, relevance
    deduplication, every independent facet and full-as-union, classification and snapshot
    preservation, artifact and ADR facets, JSON removal and invalid combinations, exact
    8,192/8,193-byte delivery, secure spill cleanup failures, uncovered spill reuse, static
    fallback, generated-output parity, managed caller policy, and a mixed directory of more than
    20 files. Documentation, glossary, roadmap, testing guidance, help, templates, and rendered
    copies travel with implementation.

## State changes

- update `tooling/context-and-topic:context-adr-operation-projection`
- update `tooling/context-and-topic:context-applicability-navigation`
- update `tooling/context-and-topic:context-default-excludes-history`
- update `tooling/context-and-topic:context-concise-projection`
- update `tooling/context-and-topic:context-full-authority-packet`
- update `tooling/context-and-topic:context-known-artifact-navigation`
- remove `tooling/context-and-topic:context-output-parity`
- update `tooling/context-and-topic:context-path-attribution`
- update `tooling/context-and-topic:context-read-only`
- update `tooling/context-and-topic:context-static-fallback`
- remove `tooling/context-and-topic:uncovered-output-parity`
- add `tooling/context-and-topic:context-summary-projection`
- add `tooling/context-and-topic:context-terminal-output-cap`
- add `tooling/context-and-topic:context-spill-observability`
- update `rendering/workflow-skill-templates:implementer-context-grounding`

## Consequences

Directory queries scale with distinct implementation context rather than descendant count, while
exact files and Git changes preserve the attribution needed for implementation and review.
Agents receive a small default impact map and can buy only the detail needed for the current
lens. Bounded evidence and member disclosure prevent optional facets from recreating the census
through another field.

The semantic model and tests become more involved. Group equivalence must include every field
whose difference matters, relationship categories require stable precedence, and summary
fallback must be Unicode-aware and deterministic. This complexity is accepted in the model so
all renderers and callers share one truthful projection rather than applying output-only folding.

Removing JSON is an intentional pre-1.0 compatibility break. Repository search found no internal
consumer, but external consumers cannot be proven absent. They will fail visibly instead of
silently receiving a structurally incomplete census. A future structured format remains possible
when a real consumer can define its contract.

The hard cap prevents oversized terminal and prompt injection while preserving the exact output
for deliberate inspection. It also creates temporary-file lifecycle and sensitive-data risks,
mitigated by owner-only creation, repository-external placement, explicit ownership, cleanup on
failed delivery, and omission of the ephemeral path from the local advisory log. A caller that
successfully receives a spill notice must remove the file when finished.

`awf context` is no longer literally write-free in oversize cases, so the read-only claim becomes
precise about repository state rather than pretending output delivery is not a write. The
repository wrapper intentionally has a separate ignored local side effect; direct `./awf
context` remains free of repository mutation.

Managed review packets become narrower and more intentional, but a reviewer needing every facet
must request it independently rather than relying on a prescribed `--full`. `--full` remains an
interactive convenience and an equivalence oracle for tests, not a workflow default.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Compact only in the text renderer | Leaves a path-census semantic model, makes facets and future consumers reconstruct grouping, and risks hidden JSON divergence. |
| Keep JSON with a complete path census | Defeats compaction, preserves an unused second contract, and makes structured output much larger than the human orientation surface. |
| Add a `--paths` escape hatch | Exact-file arguments already express the need for exact detail; another mode would weaken the predictable directory contract. |
| Emit examples from large groups | Examples can be mistaken for the complete member set and add unstable path noise without preserving exact attribution. |
| Add drilldown query IDs | Introduces ephemeral identifiers and stateful follow-up semantics when existing exact paths and `awf topic` already provide drilldown. |
| Truncate at the cap | Silently loses authority and makes it impossible to distinguish a complete answer from a clipped one. |
| Fail when output exceeds the cap | Protects the terminal but discards a valid complete result and gives the caller no inspection path. |
| Continue prescribing `--full` to reviewers | Recreates lore-dump behavior and ignores that review lenses need different, bounded detail facets. |
| Amend ADR-0144, ADR-0147, or ADR-0155 | Implemented ADRs are frozen historical records; their current-state effects must be corrected by explicit operations in a later decision. |

## Status history

- 2026-07-27: Proposed
