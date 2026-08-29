---
format: current-state-v4
slug: semantic-artifact-authoring-commands
status: Implementing
date: 2026-08-29
---
# ADR-semantic-artifact-authoring-commands: Semantic Artifact Authoring Commands


## Context

Managed Markdown artifacts have semantic identities in awf, but authoring an override currently
requires the caller to derive its `.awf` convention-part path and write the file directly. Structured
sidecar changes additionally require callers to reconstruct YAML mappings or lists without a
command-level type boundary. These mechanics are especially costly and error-prone for agents,
which already know the artifact kind, catalog name, part, or sidecar field they intend to change.

The existing model provides the necessary semantic and safety foundations. The catalog and kind
descriptors own artifact and section identity. Configuration owns sidecar and part paths, strict YAML
loading, and YAML-node round trips. The confined filesystem boundary provides expected-identity
replacement without requiring a user-visible concurrency token. Project loading and publication
already validate complete repository trees, serialize mutations with leases, preserve stable partial
effects, and commit generated state by replacing the lock last. Local documents differ only in that
their `body` input lives inside an awf-managed output boundary rather than a convention-part file.

A command that mutates only an authored source and leaves generated outputs stale would not provide
the intended low-friction transaction. Conversely, claiming crash atomicity across source and output
publication would exceed the repository's existing mutation guarantees. The authoring surface must
therefore combine semantic targeting and candidate validation with ordinary publication while
reporting any committed source or publication effects honestly.

## Decision

1. `decision: explicit-semantic-targets` Add parallel non-interactive authoring families with the
   grammar `awf edit <kind> <name> <part>`, `awf reset <kind> <name> <part>`,
   `awf edit sidecar <kind> <name> <field>`, and
   `awf reset sidecar <kind> <name> <field>`. The closed artifact kinds are `doc`, `skill`,
   `agent`, and `domain`; configured local documents use `doc`. Names, parts, and fields resolve
   through catalog and project semantics rather than rendered-output or `.awf` paths.
2. `decision: direct-part-input` Part editing creates or replaces the selected body from exactly one
   `--content <text>` value or `--stdin` input. Part reset removes the authored override and restores
   its inherited default. The common path invokes no editor and requires no temporary input file.
3. `decision: leaf-sidecar-mutation` A sidecar field is a leaf-only dotted semantic path, including
   data keys, data-default controls, section drop controls, and the domain paths leaf. Editing accepts
   exactly one of `--value <text>`, `--json-value <json>`, `--add <text>`, `--add-json <json>`,
   `--remove <text>`, or `--remove-json <json>`. Scalar flags carry strings and JSON flags carry
   structured values. Adds and removes are idempotent under scalar or structural equality, preserve
   retained list order, and require the complete structured value for JSON removal. Reset removes
   the leaf, prunes empty parent mappings, and removes a sidecar that has no authored fields left.
   Sidecar mutation uses the configuration owner's YAML-node round trip, preserving unrelated
   comments and ordering rather than re-marshaling the complete typed sidecar.
4. `decision: capability-aware-resolution` Authorability is a capability of the selected kind,
   artifact, part, and field combination rather than a document-wide generated or non-generated
   classification. Unsupported combinations and intermediate mapping replacement are rejected.
   Structured input and the complete candidate project are validated before source publication.
   Candidate validation and ordinary rendering preserve existing template semantics, including
   `missingkey=zero` behavior and the prohibition on emitted no-value tokens for empty strings.
5. `decision: local-document-body` A configured local document exposes only the synthetic part
   `body`. Edit replaces only that in-place body; reset restores the local-document template default,
   currently empty, while preserving the declaration and awf-owned shell. In-place boundary parsing
   has one semantic owner shared with ordinary publication behavior.
6. `decision: validated-publication-transaction` Each authoring operation acquires the complete
   applicable lease, observes exact source identities internally, validates one overlaid candidate
   universe through both configuration and project-tree readers without mutation, publishes the
   authored source through the confined filesystem boundary, reloads committed authority, and
   invokes ordinary render and synchronization. A failure before source publication changes no
   bytes. A later failure reports authoring-owned source and setup effects separately from publisher
   effects, with actionable recovery steps; the operation claims no crash atomicity.
7. `decision: friction-boundary` The commands expose no expected SHA or other concurrency token,
   no path-based identity, no arbitrary intermediate-mapping replacement, and no batch-mutation
   language. Sidecar mutation remains under `edit sidecar` rather than introducing a top-level `set`
   family.
8. `decision: current-state-assurance` The three added State changes are test-backed invariants, and
   the updated local-document-body claim remains a test-backed invariant. Implementation supplies
   the corresponding `invariant:` test evidence with each applied claim operation.

## State changes

- add `tooling/cli:semantic-artifact-authoring`
- add `config/configuration:sidecar-authoring-roundtrip`
- update `rendering/inplace-and-placeholders:local-doc-body-inline`
- add `rendering/sync-and-drift:authoring-sync-transaction`

## Consequences

Agents can author parts and structured sidecar leaves from stable semantic identities without
learning source layouts, constructing temporary files, or rebuilding unrelated YAML. Idempotent list
operations become safely retryable, while explicit kind and leaf identities prevent ambiguous or
broad mutations. Successful commands leave authored inputs, rendered outputs, and the lock coherent.

The implementation must add a candidate-tree overlay that is consistent across configuration and
project readers, and it must reload committed authority before ordinary synchronization rather than
publish a stale prepared plan. Local-document body editing requires the existing in-place boundary
policy to become reusable without creating a second marker parser. Sidecar YAML mutation must retain
one serialization owner and preserve unrelated comments and ordering.

A source publication followed by a render or synchronization failure can leave a valid authored
source with stale generated outputs. This is an accepted consequence of refusing false crash
atomicity. Typed partial outcomes must distinguish those committed effects and direct the caller to
recovery. Internal expected-identity checks still detect races even though users supply no token.

The command surface intentionally does not provide an editor, batch language, generic YAML mapping
replacement, or filesystem-path escape hatch. More complex changes continue to use ordinary source
authoring and awf validation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require `.awf` paths or temporary input files | Preserves the layout knowledge and file-construction friction the commands are intended to remove. |
| Use unqualified artifact names | Names can collide across kinds and do not expose the capability boundary as clearly as explicit kinds. |
| Add `awf set sidecar` | Creates a broad top-level family for one specialized authoring target instead of keeping edit and reset parallel. |
| Require an expected SHA from the caller | Adds read-before-write ceremony, especially for fresh content, while the filesystem boundary can enforce observed identity internally. |
| Mutate the source without rendering | Leaves generated outputs and the lock stale after the nominally successful low-friction operation. |
| Add batch mutation | Expands grammar, validation, and partial-outcome complexity beyond focused, retryable single-part and single-leaf operations. |
| Re-marshal complete typed sidecars | Loses unrelated YAML comments and representation details and bypasses the established YAML-node ownership model. |
| Promise rollback or crash atomicity across the transaction | Conflicts with existing honest partial-effect semantics and cannot cover process or filesystem failure reliably. |

## Status history

- 2026-08-29: Proposed
- 2026-08-29: Implementing; content-sha256: 3556b3006f1c1afe354aea299e19101a6e99b3bf8ef195fa9567f36ccdca64c3
- 2026-08-29: Applied; operations: update `rendering/inplace-and-placeholders:local-doc-body-inline`, add `rendering/sync-and-drift:authoring-sync-transaction`
- 2026-08-29: Applied; operations: add `tooling/cli:semantic-artifact-authoring`, add `config/configuration:sidecar-authoring-roundtrip`
