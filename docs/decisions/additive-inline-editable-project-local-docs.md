---
format: current-state-v4
slug: additive-inline-editable-project-local-docs
status: Implementing
date: 2026-08-12
---
# ADR-additive-inline-editable-project-local-docs: Additive Inline-Editable Project-Local Docs


## Context

ADR-0251 made every catalog document unconditional and retired project-local artifacts. That was the
right simplification for a house standard whose served repositories do not vary awf's behavior: the
old local-doc channel was coupled to artifact selection, effective-catalog synthesis, sidecars, base
templates, and enable and disable commands. It protected general adopter autonomy after the project
owner had withdrawn that product premise.

The same record established a different admission test for configuration: a key is justified when
its steady-state value expresses a fact that genuinely differs between served repositories. The
set of repository-specific operational and domain documents passes that test. One repository may
need deployment and rollback runbooks, another a protocol contract, a threat model, or a subsystem
debugging guide. Making every such document a catalog entry would publish irrelevant empty
artifacts to every repository. Leaving them entirely outside awf loses the document map, managed
structure, reference checks, and drift coverage that keep guidance discoverable and mechanically
sound.

The retired ADR-0091 channel solved that content need, but its representation solved the former
selection model too: a custom name entered a project-scoped catalog clone, rendered from a base
template, and obtained its body from a convention part. Restoring that design would also restore
machinery ADR-0251 intentionally removed. Local documents instead need to be additive to the
unconditional standard, not selectable members of it.

The in-place editing primitive already provides the fitting ownership boundary. awf can own a
shared document shell, path, provenance, and heading while reading one adopter-owned body back from
the rendered output. The output is then the natural and sole authoring location. That last property
creates a safety obligation absent from ordinary generated files: removing the declaration or
uninstalling awf must not silently delete the only copy of authored prose.

## Decision

1. `decision: additive-local-doc-declarations` Configuration admits one `localDocs` repository-fact
   list. Each entry declares exactly a path-like name beneath the fixed documentation root, a title,
   and a one-line description. Names are unique, every deterministic local-document projection is
   sorted by name, and YAML list order has no behavioral effect. No entry selects, disables,
   shadows, or changes any standard catalog artifact. Every catalog document continues to render
   unconditionally. Local documents remain a separate
   config-derived output family rather than joining the catalog or its layout map.

2. `decision: local-doc-name-boundary` A local document name is composed of lowercase kebab-case
   path segments and does not carry a Markdown suffix. The authored and generated families rooted
   at `decisions`, `plans`, `domains`, `topics`, and `pitfalls` are reserved, and an exact standard
   output collision is rejected. Unrelated namespaces such as operations, runbooks, and research
   remain available. Title and description are explicit, nonblank, single-line metadata.

3. `decision: inline-freeform-local-doc` Every declared local document renders through one shared
   awf-owned Markdown shell. awf owns its provenance, title heading, framing, output identity, and
   metadata projection; the adopter owns one unrestricted body edited directly in the rendered
   file through `awf:edit-in-place`. There are no local-doc sidecars, convention parts, custom
   templates, frontmatter, declared subsections, tags, ordering controls, or per-document check
   toggles.

4. `decision: local-doc-managed-coverage` Every local document participates in the ordinary managed
   output plan, lock, working-tree drift checks, Markdown link checks, and skill-reference checks,
   and appears in the rendered agent guide's document map with its title, path, and description.
   Each output's hash folds in its normalized entry metadata, while the agent guide's hash folds in
   the sorted complete local-document projection. Changing metadata regenerates only the awf-owned
   projections that consume it while preserving the inline body. This record does not claim semantic
   freshness analysis for free-form prose, and it does not widen the deliberately narrower
   staged-drift contract.

5. `decision: scaffold-local-doc-command` `awf new doc <name> <description> [--title <title>]`
   validates the declaration and destination, refuses an existing output, adds the config entry,
   renders the project, and reports the created document. Without `--title`, it derives the title
   from the final name segment by replacing hyphens with spaces and capitalizing each word; the
   explicit flag is the answer for acronyms and specialized spelling. The command does not create
   a sidecar or convention part.

6. `decision: preserve-local-doc-on-removal` Before render prunes a previously managed local
   document whose declaration disappeared, it copies the complete document to a sibling
   `.awf-bak`, retrying numbered suffixes under the existing collision protocol, and reports the
   backup. An absent document needs no backup; an unsafe, unreadable, broken, or escaping path
   refuses. The lock advances only after complete preservation and successful removal; a failure in
   either step retains the old lock. The same preservation rule applies to local documents removed
   by `awf uninstall`; other generated outputs retain their existing uninstall behavior.

7. `decision: local-doc-guidance-travels` Adopter guidance and the applicable workflow skills teach
   the declaration, scaffold command, inline ownership boundary, checking behavior, reserved
   namespaces, and removal and uninstall recovery behavior. A feature for durable documentation is
   incomplete if the standard's routing and authoring guidance cannot lead adopters to it.

## State changes

- update `config/configuration:config-expresses-repo-facts-only`
- update `config/configuration:no-artifact-selection-surface`
- add `config/configuration:local-doc-declarations`
- add `rendering/doc-outputs:local-doc-output-complete`
- add `rendering/inplace-and-placeholders:local-doc-body-inline`
- update `rendering/project-output-plan:output-plan-complete`
- update `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`
- add `rendering/sync-and-drift:local-doc-prune-preserved`
- update `rendering/sync-and-drift:uninstall-removes-lock-entries`
- update `tooling/cli:cli-creation-and-inventory`

## Consequences

Repository-specific documentation becomes a first-class managed output without reopening catalog
selection. Standard docs stay universal; local docs are an additive statement of repository
content. A repository can keep runbooks and specialized guidance beside its other docs while awf
keeps their framing current, exposes them to agents, and checks their references. Every added claim
in this record is mechanically enforced and lands as a test-backed invariant; the revised
`config-expresses-repo-facts-only` claim remains the governing rule it already is.

The rendered file is intentionally both output and authoring source for its body. This removes the
old mirrored convention-part location and makes free-form editing natural, but it also means the
file cannot be treated as disposable generated output. Removal and uninstall therefore do more
work and leave recovery files that the user must inspect and eventually remove.

The central list duplicates a document's metadata outside its body. That duplication is bounded to
facts awf needs before rendering the body: output identity, heading, and document-map description.
The body stays entirely free-form. Sorting the projection by name makes list reordering inert, while
metadata changes affect the local document and the agent-guide projection that consumes them. Each
entry also grows the agent guide and the context an agent receives; a large declaration set can
trigger the existing guide-size advisory and creates real context-budget pressure.

The shared shell inherits the render engine's existing missing-key behavior and remains
publication-safe under empty render values: it emits neither `<no value>` nor unresolved template or
placeholder tokens. Validation still rejects empty title and description before ordinary rendering,
so this is defense in depth rather than an alternate accepted metadata state.

A schema generation and config-reference update are required even though existing repositories
need no new bytes: absence means that the repository declares no local documents. Strict parsing,
render projection, manifest and lock attribution, and config-reference reflection all learn the new
shape. Upgrade advances the no-byte schema generation and writes the current lock while leaving an
existing config byte-identical. Historical selection keys remain retired and forward-ported; this is
a new additive shape, not a restoration of the old `docs` array or `local` sidecar field.

The ordinary in-place guarantees apply to local documents, including regeneration of awf-owned
framing and preservation of body bytes. Working-tree check provides the full output and reference
coverage. The narrower staged drift surface continues to compare eligible existing locked outputs
and does not gain new-output, removal, backup, or link-check semantics through this record.

A future awf release can add a standard document whose output collides with an existing local name.
That repository then cannot render the ambiguous pair until the local declaration is renamed or an
upgrade migration supplies an unambiguous repair. This is the accepted cost of reserving standard
output identity while permitting additive names outside today's catalog.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Restore ADR-0091's catalog synthesis, sidecars, parts, and `docs` selection | It couples a valid repository-content need back to the selection machinery ADR-0251 intentionally removed. |
| Add every repository-specific document to the standard catalog | It makes unrelated repositories render irrelevant artifacts and mistakes repository content for universal workflow guidance. |
| Register completely adopter-owned files without rendering them | It can add discovery and reference checks but cannot enforce an awf-owned shell, provenance, heading, or structural drift boundary. |
| Keep the body in a convention part | It duplicates the document into a less natural authoring path when the existing in-place primitive can safely preserve the rendered body. |
| Allow arbitrary local-document metadata and sections | It recreates a configurable document framework; name, title, description, and one free-form body satisfy the demonstrated need. |
| Delete local documents like ordinary generated outputs | The rendered file is the only source of adopter-owned prose, so ordinary pruning and uninstall would cause data loss. |

## Status history

- 2026-08-12: Proposed
- 2026-08-12: Implementing; content-sha256: c20a3f87e2a20820fa2dcf9152a50c87c1a1800b0d253c98cc027e2d211c6264
- 2026-08-12: Applied; operations: update `config/configuration:config-expresses-repo-facts-only`, update `config/configuration:no-artifact-selection-surface`, add `config/configuration:local-doc-declarations`, add `rendering/inplace-and-placeholders:local-doc-body-inline`, update `rendering/project-output-plan:output-plan-complete`, add `rendering/sync-and-drift:local-doc-prune-preserved`, update `rendering/sync-and-drift:uninstall-removes-lock-entries`
- 2026-08-12: Applied; operations: add `rendering/doc-outputs:local-doc-output-complete`, update `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`
- 2026-08-12: Reapplied; operations: update `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`
