---
status: Proposed
date: 2026-07-17
supersedes: []
superseded_by: ""
tags: [multi-target, target-seam, lock-manifest, render-completeness]
related: [20, 37, 86, 100, 122, 123]
domains: [rendering]
---
# ADR-0124: Deterministic Output Plans and Target Capabilities

## Context

The target seam has grown from Claude/Cursor path placement into six runtime descriptors
with Markdown and TOML agent encodings, optional instruction bridges, Pi-only TypeScript
extension files, and Pi-specific workflow capability. `RenderAll` still discovers the
intended output set through several separate loops. `SyncReport`, `Check`, `PlannedOutputs`,
and config-reference generation then reconstruct or extend parts of that set.

This leaves output semantics distributed. The dead-link and dead-skill-reference checks infer
Markdown eligibility from template IDs plus a `.toml` path exception, even though Pi adds
TypeScript outputs. The target descriptor assumes every runtime has the same skill and agent
shapes, while its `SubagentTools` boolean is a one-off template-facing capability. A future
runtime with a different artifact mix risks adding another render branch and another drift
exception.

ADR-0037 established target descriptors and target-derived rendering. ADR-0122 added
format-neutral agent encoding and the expanded target registry. ADR-0123 added target-owned
extension outputs. Their intended shared ownership of rendering, hashes, and drift is sound,
but it now needs one explicit internal representation of every managed file.

The model stays entirely compile-time and internal. Adopters continue to select the existing
named targets in `config.yaml`; they do not configure output graphs.

## Decision

1. Replace the distributed target-output construction with a deterministic internal output
plan. The plan is a flat, path-keyed collection of structured nodes, with explicit dependency
edges only where one generated output needs another's rendered metadata. It is not an
adopter-configurable graph and introduces no configuration schema.

2. Compile neutral producers, enabled target descriptors, and enabled singleton producers into
this plan. Every managed output participates, including ordinary rendered artifacts, bridges,
target-owned extension files, `ACTIVE.md`, domain docs, the configuration reference, and
in-place/regeneration-checked files. The plan also carries non-writing local-artifact
reservation nodes: their paths remain protected from prune and their frontmatter policy remains
validated, but Sync never overwrites them. The configuration-reference node depends on the
regular and domain nodes it describes, excludes itself from that input set, and therefore
introduces no cycle.

3. Make each output-plan node the complete declaration of how one path is handled: producer
identity, path, template or generator recipe, section and sidecar inputs, target capability
projection, output encoder, provenance comment style, explicit validation/check policy, and
all drift/hash inputs. Render, sync/manifest writing, pruning, check, planned-output reporting,
and collision detection consume this one plan or its rendered-node results. Content policies,
not template IDs or filename suffixes, select frontmatter validation, dead-link scanning,
dead-skill-reference scanning, and regeneration checking.

4. Replace target-specific scalar behavior with strict compile-time target descriptors composed
from typed placements, artifact contributions, encodings, and a closed awf-owned capability
set. The current Pi subagent-tools behavior becomes a named capability with an explicit
projection into template data. A planner-precondition validator rejects unsupported combinations
before planning: every emitted artifact declares a known encoder and policy, bridge path and
template are either both present or both absent, target-output paths and template IDs are
non-empty and path-safe, and each output's provenance style is valid for its declared content
policy. Capability projection preserves the existing `missingkey=zero` publication-safety
contract; empty-variable fixtures must render coherent output without an unresolved-value
token.

5. Permit several target declarations to request the same path only when their normalized
render recipes are equivalent. Recipe equivalence includes every output-affecting behavior but
excludes target identity. The planner coalesces compatible declarations into one node and one
write, retaining sorted declarer identities for diagnostics and drift hashing. Any differing
recipe is a pre-render collision error; there is never last-writer-wins behavior.

6. Drift hashes include the normalized recipe and the sorted declaring target descriptors.
Changing target membership or a descriptor remains observable even when the output bytes would
otherwise stay identical. Reject duplicate target names in `config.targets` during strict
configuration validation rather than relying on planning or path collision failures.

## Invariants

- `invariant: output-plan-complete`: the planner emits writing nodes for catalog artifacts,
  bridges, target outputs, neutral/singleton outputs, generated indexes, domain docs, and the
  configuration reference, plus non-writing reservations for local artifacts; Sync, Check,
  PlannedOutputs, manifest ownership, and prune protection consume that result.
- `invariant: output-policy-explicit`: fixtures for Markdown, TOML, TypeScript, generated, and
  local nodes prove that frontmatter validation, link and skill-reference scans, regeneration,
  and local validation are selected solely by declared node policy, not a template ID or path
  suffix.
- `invariant: shared-output-coalesced`: equivalent recipes for one path produce one planned
  write and manifest entry, while a differing template, context, encoder, provenance, or policy
  fails plan construction before any write.
- `invariant: target-capabilities-closed`: a target can carry only a declared capability value;
  invalid descriptor combinations fail precondition validation and templates receive only the
  defined capability projection.
- `invariant: duplicate-target-rejected`: repeated names in `config.targets` are rejected before
  target resolution and output planning.

## Consequences

- Adding a target or target-owned output becomes a descriptor and plan-expansion change with
  explicit policies, rather than a coordinated edit to rendering and every check.
- Sync, check, manifest, prune, and planned-output behavior share one desired-output authority,
  reducing drift blind spots and duplicated path logic.
- The refactor is broad: it must preserve all six current target surfaces, local artifacts,
  generated indexes, in-place ownership, and selective configuration hashes. Focused tests must
  cover deterministic plan order, duplicate target rejection, compatible coalescing and
  pre-write collision, policy routing independent of path/template spelling, local reservations,
  regeneration-checked nodes, target removal/pruning, hash selectivity, and sync/check repair
  before the ADR can be Implemented.
- Coalescing is intentionally strict. It supports truly shared outputs but rejects accidental
  overlap before any write; target identity cannot silently alter the bytes used to establish
  compatibility.
- A target remains a compile-time awf concept. This ADR does not add third-party target plugins
  or adopter-defined output declarations.
- Implementation updates `docs/architecture.md`, the rendering-domain current state, and
  target/rendering guidance that names the descriptor seam in the same commits as behavior.
  The final lifecycle transition runs `./x sync` and commits regenerated decision and domain
  indexes with the implementation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add content-kind flags only to `RenderedFile` | Fixes the immediate TypeScript scan issue but leaves output discovery and lifecycle ownership distributed. |
| Keep target scalar fields and add more conditional render loops | Repeats the coupling that made Pi-specific outputs and capabilities special cases. |
| Make the graph adopter-configurable | Expands configuration and validation surface without serving the internal refactoring goal. |
| Allow shared paths with last-writer-wins or byte comparison after rendering | Makes ownership order-dependent or performs work before detecting an invalid plan; normalized recipes fail earlier and explain the conflict. |
