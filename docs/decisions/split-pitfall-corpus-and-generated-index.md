---
format: current-state-v4
slug: split-pitfall-corpus-and-generated-index
status: Proposed
date: 2026-08-10
---
# ADR-split-pitfall-corpus-and-generated-index: Split Pitfall Corpus and Generated Index


## Context

Pitfalls are the durable home for active hazards that still require human judgment. Their current
representation no longer serves either authors or readers well. All entries are one ordered YAML
list in `.awf/docs/pitfalls.yaml`, and all bodies render into one flat `docs/pitfalls.md`. On
2026-08-10, `rg -c '^    - title:' .awf/docs/pitfalls.yaml` and `rg -c '^## ' docs/pitfalls.md`
each reported 47 entries; `wc` reported 672 lines and 5,713 words in the generated document. A
history sample with `git show <commit>:docs/pitfalls.md | rg -c '^## '` found 105 entries on
2026-08-02 before active-only pruning reduced the corpus. Pruning helps authority but does not make
47 full entries easy to browse, edit, review, or merge.

The structure already carries useful metadata, but the published document underuses it. Domains are
printed inline, governed tags remain invisible, and there is no index. The flat presentation also
makes a current usage constraint resemble a backlog item: the already-implemented global-topic path
ownership contract was mistaken for missing capability while selecting work from the document.
Pitfalls are no longer surfaced by `awf context`, so the only discovery surfaces are a whole-file
read and textual search.

ADR-0099 deliberately chose the single sidecar-derived document over independently authored files.
At the time, per-entry files, an index, a parser package, and `awf new pitfall` were rejected as too
much machinery for short entries and a skimmable document. The current scale and failure are the
counterfactual that decision named. Its context-surfacing benefit has since retired, while its
monolithic authoring and publication costs remain. Append-only history requires a successor rather
than rewriting that terminal record.

The replacement must fit awf's generated-source architecture. Pitfall sources need closed-tree
claiming and strict parsing. The index and every leaf need deterministic output-plan nodes, lock and
drift participation, source guidance, foreign-file backup, and deletion pruning. Existing domain,
tag, and ADR-link validation must continue. Adopters need a forward migration that preserves all
entry fields and any independent `prepend` or `append` section configuration without creating two
active registries. The project renders every catalog document unconditionally, so the replacement
must not reintroduce retired artifact selection.

A cohesive semantic boundary is now earned. Identity, parsing, corpus validation, serialization,
and scaffolding allocation are shared policy used by rendering, migration, and the CLI. Keeping
those rules in project orchestration or duplicating them in migration would create representation
leakage. Project orchestration still owns repository-specific domain, tag, and ADR resolution and
the output lifecycle.

## Decision

1. `decision: per-entry-authored-corpus` Replace `data.pitfalls` with independently authored
   Markdown files. Each file has one path-derived stable slug and strict YAML frontmatter containing
   required `title` and optional `domains`, `tags`, and `related`, followed by a nonblank Markdown
   body. The title is trimmed, nonempty, single-line display text; CR and LF are rejected, and title
   edits never rename identity. Duplicate-title comparison joins Go `strings.Fields` with one ASCII
   space and compares the results with `strings.EqualFold`, so cosmetically equivalent active titles
   fail under one exact Unicode whitespace and simple-folding rule. Only direct regular `.md`
   children with lowercase kebab slugs are valid; `index` is reserved. Unknown or duplicate keys,
   malformed or duplicate list members, nested paths, and non-regular inputs fail.

2. `decision: indexed-leaf-publication` Keep `docs/pitfalls.md` as the compact generated catalog
   entry and publish one generated leaf per source beneath `docs/pitfalls/` at the same slug. The
   index states that hazards are not backlog items, renders one title-sorted table containing links,
   domains, tags, and related ADRs, and adds compact title-sorted domain sections plus an unassigned
   section. A zero-entry corpus renders an explicit empty state. Each leaf renders one title,
   visible metadata, and the authored body verbatim, without published frontmatter. Titles are
   plain text rather than authored Markdown: rendering backslash-escapes CommonMark ASCII
   punctuation and literal backslashes in headings and link labels, with table delimiters escaped
   for cells, so punctuation cannot change generated structure while displayed title text is
   preserved.

3. `decision: index-only-convention-framing` Preserve the existing `prepend` and `append`
   convention sections as index-only framing. Migration removes the retired data key and deletes
   `.awf/docs/pitfalls.yaml` only when no independent section configuration remains. A surviving
   sections-only sidecar is valid configuration, never a second entry registry, and leaf pages do
   not inherit its framing.

4. `decision: path-derived-scaffolding` Add `awf new pitfall "<Title>"` as the supported creation
   path. One shared allocator trims the title, lowercases it, replaces each maximal run outside
   ASCII `a-z` and `0-9` with one hyphen, trims edge hyphens, and refuses an empty result or the
   reserved result `index`. It inspects the unsuffixed slug and then integer suffixes from `-2`
   upward, choosing the first absent candidate; migration reserves each allocation before choosing
   the next. It refuses a duplicate active title, creates exclusively,
   and reports the authored path. Deleting a source retires its entry through ordinary pruning.
   Pitfalls gain no lifecycle states, registry, or per-entry enablement.

5. `decision: cohesive-pitfall-model` Give one internal pitfall package ownership of identity,
   strict frontmatter parsing, corpus ordering and validation, deterministic source serialization,
   and scaffold slug allocation. Project orchestration consumes that model and retains domain, tag,
   and ADR resolution, convention assembly, output declarations, rendering, hashing, drift, lock,
   backup, and pruning. Migration consumes the same parser and serializer rather than reimplementing
   the source format. Introduce the package with its real consumers; no preparatory package refactor
   or generalized document-family framework is required.

6. `decision: complete-generated-family` Treat the compact index and every pitfall leaf as one
   complete generated family in working-tree and staged projections. The config tree claims exactly
   the source directory shape. Every output has explicit Markdown policy, declarers, dependencies,
   source guidance, and lock membership, and ordinary render/check detects edits, missing outputs,
   stale outputs, and removals. A leaf recipe consumes its complete source. The index recipe projects
   only corpus metadata and framing inputs, so a body-only edit does not change an output whose bytes
   cannot change, while dependency reporting still names the source corpus.

7. `decision: safe-forward-migration` Add a new schema migration after the current generation and
   leave the historical generation-9 migration frozen. Convert old entries in authored order,
   allocating the same deterministic collision suffixes as scaffolding and preserving the legacy
   parser's canonical trimmed title plus every domain, tag, related ADR, and body. Raw surrounding
   title whitespace is not active legacy data because the existing parser already trims it.
   Preflight the whole conversion. Refuse relative Markdown links
   with entry-specific recovery guidance rather than rewriting prose, refuse conflicting destination
   files, accept byte-identical leaves from a partial prior attempt, create all missing leaves before
   removing old authority, and retain unrelated section configuration. Retrying after interruption
   is idempotent; no pitfall-specific journal is introduced.

8. `decision: preserve-validation-and-guidance` Move existing pitfall domain, tag, ADR-link, tag
   health, and frequency checks from sidecar entries to the parsed corpus. Generated index guidance
   names the source glob and each leaf names its exact source. Authoring and retrospective guidance
   directs users to `awf new pitfall` or the authored corpus, never to generated output. Template
   and section assembly retains `missingkey=zero`; empty variables, absent overrides, and a zero-entry
   corpus render coherent token-free Markdown with no `<no value>` output.

9. `decision: backed-pitfall-contracts` Back the new corpus-validation, output-completeness,
   semantic-ownership, scaffold, and migration claims with tests and matching invariant markers.
   Corpus tests refute each source and identity violation, including CR/LF titles. Project tests prove index and leaf output
   planning, hashing, render, drift, lock, backup, and pruning plus the single package ownership
   boundary. CLI tests prove creation dispatch and exclusive collision allocation. Migration tests
   prove field preservation, full preflight, relative-link and conflict refusal, byte-identical
   retry, create-before-retire ordering, and section preservation.

10. `decision: bounded-navigation-scope` Do not restore `awf context` pitfall surfacing, add a tag
   index, split indexes by domain, model remediation status, or reintroduce catalog-document toggles.
   The metadata table, domain link sections, ordinary search, and independent leaves are the bounded
   navigation improvement justified by the observed corpus.

## State changes

- remove `rendering/doc-outputs:pitfall-data-validated`
- add `rendering/doc-outputs:pitfall-corpus-validated`
- add `rendering/doc-outputs:pitfall-output-complete`
- add `code-design/single-home:pitfall-model-single-home`
- update `rendering/doc-outputs:opaque-doc-source-guidance`
- update `tooling/cli:cli-creation-and-inventory`
- add `tooling/cli:pitfall-scaffold`
- add `config/migrations-and-locks:pitfall-corpus-migration`

## Consequences

Pitfall discovery becomes a bounded index read instead of a 5,700-word scan. Readers can browse all
active hazards, narrow by domain, see tags, and open only the relevant bodies. Authors edit one
ordinary Markdown file, get isolated diffs and merge conflicts, and can add an entry through a
validated scaffold. Stable slugs keep inbound links intact when titles improve. Deletion naturally
removes both authority and publication through the existing generated-output lifecycle.

The change replaces a specialized sidecar transform with a real producer family. That is more code
and more generated files than ADR-0099 accepted, and it touches schema migration, closed-tree
validation, output planning, locking, pruning, CLI dispatch, documentation, and current-state
claims. The cost is now justified by the corpus size and by one structure solving both authoring and
reading failures. A dedicated semantic package is an ownership boundary, not a framework for other
document types.

Migration is intentionally strict. An adopter with relative links or conflicting pre-existing files
must correct them and retry; awf does not attempt lossy Markdown surgery. A partially completed
multi-file migration can leave byte-identical leaves before the old authority is removed, but the
retry contract completes that state deterministically. Section overrides survive even though the
entry registry moves.

The main index still has 47 rows at today's scale, but rows are compact metadata rather than full
prose. Tags are visible rather than separately expanded, avoiding a second large taxonomy. Domain
sections duplicate links but never bodies. Pitfalls remain current hazards rather than an issue
tracker, and the roadmap remains the durable backlog surface.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one source and add only an index | Improves published scanning but leaves the authoring, review, merge, and retirement problems intact. |
| Split generated leaves while retaining the YAML list | Creates a misleading representation boundary: independently published artifacts still contend in one 38 KiB source and migration does not improve authoring. |
| Keep full bodies in the main document and add a table of contents | Navigation improves slightly, but every read still loads the whole corpus and metadata remains underused. |
| Group leaf files or the main index by one domain | Many pitfalls cross domains, so physical grouping creates arbitrary ownership or duplicate bodies. Compact domain links provide the useful projection without a second identity. |
| Add a tag index beside domain sections | The governed tag vocabulary is substantially larger and more cross-cutting; visible table tags plus search are sufficient until observed use proves another projection necessary. |
| Retain `data.pitfalls` as a compatibility registry | Two writable authorities can disagree. Only unrelated section configuration may remain in the sidecar. |
| Rewrite relative links during migration | Correct Markdown rewriting requires handling inline, reference, image, escaped, and fenced forms. Refusal with exact recovery is safer than a partial rewriter. |
| Keep parsing and scaffold rules in `internal/project` | Migration and CLI would either depend on project orchestration or duplicate source-format policy. The independently consumed corpus model earns a focused package. |
| Reuse the topic producer as a generic indexed-content framework | Similar output cardinality does not make topics and pitfalls one policy. Generalization adds unused flexibility and couples unrelated schemas. |

## Status history

- 2026-08-10: Proposed
