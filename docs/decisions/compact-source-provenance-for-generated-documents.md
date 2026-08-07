---
format: current-state-v4
slug: compact-source-provenance-for-generated-documents
status: Implementing
date: 2026-08-07
---
# ADR-compact-source-provenance-for-generated-documents: Compact Source Provenance for Generated Documents


## Context

ADR-0015 introduced the generated-by banner and section-scoped `awf:edit` pointers so an agent
reading rendered output could identify generated ownership and the convention part for a section.
That model is effective for ordinary template sections, but it leaves generated documentation with
non-section inputs opaque. A rendered current-state topic, for example, is produced from one metadata
YAML file and one authored Markdown part yet carries only the generic banner. Topic indexes, domain
topic navigation, sidecar-derived tables, regenerated indexes and references, and target bridges
have similar gaps.

The source model is already exact inside the project output plan, but it is not a good public editing
instruction by itself. Some outputs depend on many topic pairs, config-reference content is composed
from configspec, catalog, live configuration, and enabled-output projections, and the lock retains
drift hashes rather than source paths. Automatically dumping machine inputs would therefore be
verbose, incomplete for some producers, and misleading about what a reader should edit.

A broad documentation audit also found that source guidance is split across the README, rendered
working guide, config reference, glossary, and individual outputs. The shipped working-guide template
knows the topic pair, while this repository's replacement section omits it; the glossary describes a
topic as authored only under the parts tree even though metadata is equally authoritative. The
solution must travel with opaque generated content while retaining one canonical operational
explanation and keeping marker token cost small for agents that load these files as context.

ADR-0015's `no-section-marker-leak` invariant says the banner and `awf:edit` pointers are the only
awf markers permitted in rendered output. Because ADR-0015 is Implemented, this record changes that
contract forward rather than amending history.

## Decision

1. `decision: source-marker-contract` Rendered documentation may carry one informational HTML
   comment of the form `<!-- awf:source <source> [<source>...] -->` immediately after its
   generated-by banner. Each source is a project-relative path, a comprehensible glob when a family
   of files contributes, or a compact `derived:<authority>` identifier when no project path fully
   owns the content. The marker is neither a section boundary nor an edit or in-place read-back
   instruction.

2. `decision: reader-facing-source-policy` Source-marker payloads are a reader-facing provenance
   policy selected by the producer that understands the document family. They identify concise,
   actionable content authorities rather than claiming to enumerate every render dependency.
   Machine dependency authority remains in the output plan, while the lock remains the drift
   authority. A shared formatter owns syntax and placement, but generic consumed inputs do not
   automatically become public marker payloads.

3. `decision: opaque-output-scope` `awf:source` is emitted only when existing `awf:edit` pointers do
   not adequately explain a generated documentation source: individual topic pages, topic indexes,
   generated topic navigation in domain pages, sidecar-derived glossary and pitfalls content, the
   ADR index, the regenerated config reference, and target bridges. Exact paths are used for small
   fixed source sets and globs for clear multi-file families. Other section-overridable standard
   docs, `AGENTS.md`, ADR and plan support templates, and generated local docs do not duplicate
   adequate `awf:edit` guidance. Glossary and pitfalls are exceptions only for their computed
   content; their ordinary section pointers remain unchanged. Authored ADRs, authored plans, and
   `local: true` documents remain banner-free.

4. `decision: instruction-authority` The rendered working-with-awf guide is the canonical
   operational map from generated document families to their editable or derived sources. Individual
   markers provide the immediate concise pointer, the README retains an onboarding summary, the
   glossary defines the provenance vocabulary, and the config reference explains configuration and
   data ownership rather than duplicating template mechanics.

5. `decision: publication-safe-source-markers` Source markers are assembled from known nonempty
   reader-facing authorities rather than optional template values. Affected templates retain the
   engine's `missingkey=zero` behavior and render coherently when project variables or optional data
   are empty, without `<no value>` or another unresolved-value token in marker or document content.

## State changes

- update `rendering/render-engine:no-section-marker-leak`
- add `rendering/render-engine:source-marker-informational`
- add `rendering/doc-outputs:opaque-doc-source-guidance`

## Consequences

Agents can move from an opaque generated document to the right source without consulting the lock or
reverse-engineering the renderer. One compact comment and glob-capable payload keep the recurring
context cost lower than one verbose marker per input. Existing `awf:edit` semantics remain stable,
and the explicit producer boundary prevents dependency inventories from becoming accidental public
instructions.

The marker grammar and family selection become published behavior that needs deterministic tests and
current documentation. Mixed and regenerated outputs require explicit source policy rather than a
single automatic derivation. A source marker intentionally need not be a complete build-dependency
manifest, so the canonical working guide must explain that distinction. The distinct marker accepts
ongoing grammar and per-family policy maintenance in exchange for keeping banner ownership and source
provenance independently legible. Adding it deliberately regenerates affected adopter outputs and
changes their lock hashes on upgrade and re-render.

This record narrows ADR-0015's marker exclusivity: `awf:section` and `awf:end` still never survive
assembly, while the allowed rendered marker set gains informational `awf:source` alongside the
banner and the `awf:edit` family. It does not change convention-part precedence, in-place editing, or
the rule that rendered files are not hand-edited.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Documentation-only guidance | A reader starting from an opaque rendered file would still need to know where the separate guide lives. |
| Extend the generated-by banner with source payloads | Conflates the universal ownership warning with optional family-specific provenance, lengthens the first-line contract, and makes source grammar changes part of banner compatibility. |
| Overload `awf:edit` for non-section provenance | `awf:edit` is a structural section and read-back protocol; topic pairs, aggregates, and regenerated state are not editable sections. |
| Emit one typed marker per source | Repeats syntax, consumes more agent-context tokens, and is noisier than one compact path-and-glob list. |
| Add `awf:source` to every generated document | Duplicates already-actionable `awf:edit` pointers without improving source discovery. |
| Derive payloads automatically from consumed inputs | Machine dependencies are verbose and do not encode reader intent; several producers assemble important authority outside ordinary consumed inputs. |
| Keep provenance only in the output plan or lock | Neither travels with the rendered content, and the lock does not retain source paths. |

## Status history

- 2026-08-07: Proposed
- 2026-08-07: Implementing; content-sha256: 27b3dd4fdd7e9b15a63f760c137e7e63384f30dcfeea5175801f281218e49e51
- 2026-08-07: Applied; operations: update `rendering/render-engine:no-section-marker-leak`, add `rendering/render-engine:source-marker-informational`
- 2026-08-07: Applied; operations: add `rendering/doc-outputs:opaque-doc-source-guidance`
- 2026-08-07: Reapplied; operations: add `rendering/doc-outputs:opaque-doc-source-guidance`
