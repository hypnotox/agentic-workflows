---
format: current-state-v4
slug: repository-local-template-source-symbols
status: Implementing
date: 2026-08-11
---
# ADR-repository-local-template-source-symbols: Repository-Local Template Source Symbols


## Context

Generated Markdown currently carries two kinds of source guidance. `awf:edit` identifies the effective adopter-owned or default source for an overridable section, while `awf:source` identifies compact reader-facing authorities for opaque generated documents. Neither answers an awf maintainer's different question: which checked-in awf template or included partial structurally produced a region of a self-hosted generated file.

Document-level template identity is insufficient because include expansion currently flattens partials into their parents before section assembly. Once flattened, output cannot distinguish a root-template region from an included region. Section overrides further separate structural origin from effective content origin: an adopter part may replace the default body while the declaring template remains the source of the section's position and identity.

awf's generated outputs and drift checks are deterministic. Development annotations therefore cannot depend on an environment flag or an incidental post-render pass. Normal adopter bytes must also remain unchanged. The configuration model permits repository facts, not preferences about awf behavior, so activation must describe the repository-local source tree that makes the pointers meaningful rather than expose a generic debug boolean.

In-place sections create a further boundary. Their bodies become adopter-owned readback after the first render, so renderer-owned include transitions cannot remain stable inside those bodies without attempting to align arbitrary edits with the original template. Their structural section provenance can remain stable outside the editable interior.

## Decision

1. `decision: repository-template-root` The config may declare an optional normalized repository-relative `render.templateSourceRoot`. Its presence states that the repository contains the awf implementation template tree at that location and enables template source symbols; its absence disables them. Config loading and reference generation expose the optional fact, while render and check reject traversal or a configured mapping that cannot resolve an emitted template source. The additive schema generation recognizes the field without rewriting configs that omit it, and fresh adopter config omits the fact.

2. `decision: distinct-template-source-marker` An enabled generated Markdown region carries an HTML `awf:template-source` comment whose value is the repository-relative implementation source path. A structural section appends its stable section ID as a fragment. This marker is maintainer-facing template provenance and remains semantically distinct from effective-source `awf:edit` pointers and reader-facing `awf:source` guidance. It carries neither source line numbers nor Go producer symbols.

3. `decision: renderer-owned-regional-provenance` The render model preserves root-template and included-partial provenance through expansion, section parsing, overlay assembly, and template execution. The renderer emits a symbol when provenance enters an included partial and restores the parent source when it returns, suppressing redundant adjacent symbols. Provenance remains renderer-owned because a flattened string or post-render heuristic cannot recover exact regional ownership after section resolution.

4. `decision: section-and-editing-semantics` Every surviving section emits its section-qualified structural template symbol immediately before its edit-family pointer, including when a convention part replaces the default. A dropped section emits neither region nor symbol. Re-injecting a section default restores its template and include transitions. An in-place section emits its structural section symbol but no include transitions inside its adopter-editable body; readback treats neighboring template-source symbols as awf-owned framing rather than preserved body content.

5. `decision: markdown-scope-and-placement` Symbols apply to outputs selected by awf's declared Markdown representation, including guides, docs, skills, agents, topics, domains, and Markdown bridges. Native-format scripts and hooks, template-less procedural regions, and YAML frontmatter are not instrumented. The root-template symbol follows the generated banner and any reader-facing `awf:source` marker so existing frontmatter and public provenance placement remain valid.

6. `decision: deterministic-output-participation` Enabling a template source root changes ordinary planned render bytes and the affected Markdown artifacts' per-artifact manifest config hashes, while authored template hashes remain based on authored include-expanded source rather than generated instrumentation. Render and check consume the same normalized config fact. Provenance instrumentation does not alter missing-key handling or permit `<no value>` residue, and absent configuration preserves existing adopter output byte for byte.

## State changes

- add `config/configuration:template-source-root`
- update `rendering/render-engine:no-section-marker-leak`
- add `rendering/render-engine:template-source-symbol`

## Consequences

The two added claims are behavioral invariants backed by automated tests. Their proofs cover config activation and validation, marker identity and ordering, root/include transitions, overrides, drops, section-default re-injection, in-place readback, declared-Markdown scope, hash participation, publication-safe empty data, and absent-config byte compatibility. The updated marker-exclusivity invariant retains automated backing.

Maintainers can navigate directly from checked-in generated Markdown to the root template, declaring section, or included partial responsible for a region. Structural provenance remains visible even when self-hosted convention parts replace default bodies, without overloading adopter guidance.

The renderer must carry source identity instead of treating include-expanded text as its only representation. In-place readback and Markdown producers outside the ordinary target path must participate explicitly. These costs are accepted because regional attribution cannot be reconstructed reliably after flattening.

Moving the configured source tree or an awf template changes checked-in self-hosted output and is detected by normal drift checks. Ordinary adopters receive no additional markers or config entries. The design does not create a general debug-mode preference surface, annotate executable payloads, or promise attribution to procedural Go producers.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Boolean `render.sourceSymbols` preference | It conflicts with the config-facts-only rule and does not establish that emitted paths resolve in the repository. |
| Unconditional template markers | It would publish maintainer-oriented implementation details into every adopter's generated documents. |
| On-demand annotated rendering or a sidecar source map | It would not provide navigation from the ordinary checked-in self-hosted documents. |
| One document-level template pointer | It cannot identify the structural source of individual include and overridden-section regions. |
| Annotate the flattened expanded string or post-process rendered text | Either approach loses reliable provenance across includes, overrides, frontmatter, and in-place readback. |
| Source line numbers or Go producer symbols | They create churn or widen attribution beyond the current template-navigation need. |

## Status history

- 2026-08-11: Proposed
- 2026-08-11: Accepted; content-sha256: ddd49353b01dda515701896774831ac6aebbee7436109a30a980e8664febe288
- 2026-08-11: Implementing; content-sha256: ddd49353b01dda515701896774831ac6aebbee7436109a30a980e8664febe288
- 2026-08-11: Applied; operations: add `config/configuration:template-source-root`, update `rendering/render-engine:no-section-marker-leak`, add `rendering/render-engine:template-source-symbol`
