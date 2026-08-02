---
format: current-state-v3
slug: retain-only-claude-code-and-pi-runtime-targets
status: Implemented
date: 2026-08-02
---
# ADR-0214: Retain Only Claude Code and Pi Runtime Targets

## Context

awf currently has six built-in runtime targets: Claude Code, Codex, Copilot,
Cursor, Gemini, and Pi. The project owner uses only Claude Code and Pi. Keeping
the other four adapters therefore imposes implementation, dependency, template,
test-matrix, generated-output, documentation, and example-adopter costs without
serving a current project need.

The target abstraction itself remains useful. Claude Code and Pi already require
different bridges, capabilities, paths, rendered wording, and target-owned
outputs, and a future harness may warrant another descriptor. The simplification
must remove unused adapters without collapsing per-target customization or
rewriting descriptor-driven rendering into two hard-coded paths.

Codex is the only target that emits TOML agents, so its removal makes the TOML
profile schema, encoder, validation branch, and external TOML dependency dead.
Pi still uses the plain encoder for its five TypeScript outputs, so the shared
encoding representation cannot be collapsed indiscriminately. Similarly,
Claude Code currently owns the only remaining bridge, but the generic bridge
render loop must not identify every possible bridge as Claude-owned.

The repository root already enables only `claude` and `pi`. The Sundial adopter
enables all six and owns generated output trees for every adapter, including
Codex skills under `.agents`, Copilot artifacts under `.github`, and its target
lock entries. Rendering and pruning must reconcile those tracked outputs as part
of this change.

Historical ADRs, completed plans, research, and changelog entries record the
state that existed when they were written and remain unchanged. OpenAI Codex
provider and model identifiers used by Pi routing are also unrelated to the
Codex runtime target and remain supported.

## Decision

1. The built-in runtime target set is exactly `claude` and `pi`. Remove the
   `codex`, `copilot`, `cursor`, and `gemini` descriptors and all live code,
   templates, dependencies, tests, fixtures, generated outputs, active
   documentation, repository integration metadata, and obsolete roadmap
   material owned only by those adapters. Audit target-specific `.gitignore`
   comments and exceptions while preserving entries that still serve shared
   repository infrastructure.

2. Removal is immediate and complete. Do not add migrations, compatibility
   aliases, deprecation warnings, or silent configuration rewriting. A project
   that still names a removed target fails through the existing unknown-target
   validation and reports only the targets that remain available.

3. Retain the generic target registry and descriptor-driven render and output
   planning seams. Each target continues to own its skill and agent paths,
   suffixes and encodings, bridge declaration, capabilities, rendered
   customization, and additional outputs. Do not replace these seams with
   Claude- and Pi-specific render paths.

4. Remove the Codex-only TOML agent profile, encoder, validator branch, and
   dependency while preserving the structured agent rendering boundary and the
   plain encoding used by Pi target outputs. Broader separation or renaming of
   agent dialects and arbitrary target-output encodings is outside this
   decision.

5. Replace the generic bridge render path's Claude-specific internal identity
   with a neutral bridge identity. Input observation and tests use that neutral
   identity so a future target can declare a bridge without inheriting an
   accidental Claude sidecar or template association.

6. Preserve generic multi-target, ordering, customization, output ownership,
   pruning, and template missing-value coverage using Claude Code and Pi.
   Surviving target and bridge templates keep missing-key-zero behavior and
   emit no `<no value>` token when variables are empty. Where removed adapter
   tests carry invariant proof markers for retained or revised claims, move
   those proofs to tests that exercise the surviving descriptors or an
   explicit synthetic descriptor rather than weakening the claim.

7. Regenerate the root project and Sundial adopter from their authoring sources.
   Sundial enables only Claude Code and Pi after this change, and every managed
   output and lock entry belonging to a removed adapter is pruned. Existing
   ownership rules continue to protect unmanaged lookalike files.

8. Preserve immutable historical records and Pi's OpenAI Codex provider and
   model identifiers. A repository-wide lexical deletion of removed target
   names is not part of this decision.

## State changes

- add `rendering/catalog-and-targets:built-in-runtime-targets`
- update `rendering/catalog-and-targets:structured-agent-encoding`
- update `rendering/catalog-and-targets:target-dialect-render`
- remove `rendering/project-output-plan:cursor-no-bridge`
- add `rendering/project-output-plan:bridge-render-identity`
- update `rendering/project-output-plan:multi-target-render`

## Consequences

- awf ships and maintains only the two runtime targets the project owner uses,
  reducing production branches, embedded templates, dependencies, test fan-out,
  generated trees, and active documentation.
- Existing adopted projects that configure a removed target must edit their
  configuration before the new binary can open them. This is intentional; awf
  is pre-1.0 and no compatibility layer is retained.
- The abstraction cost that supports real Claude Code and Pi differences
  remains, as does the extension point for a future target. Synthetic target
  descriptors preserve structural customization coverage, but they cannot
  validate future harness contracts as strongly as supported production
  adapters did.
- Removing TOML support simplifies agent encoding but does not justify erasing
  target-level encoding customization or Pi's plain TypeScript outputs.
- The neutral bridge identity corrects an existing representation leak while
  bridge code is already being changed, without redesigning the output planner.
- Current-state claim mutations and their backing proof markers must land in
  the same checked implementation transactions. Historical decision material
  continues to mention runtimes that were supported previously.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Retain all six supported adapters | Continues dependency, testing, documentation, and generated-output costs for four runtimes the project owner does not use. |
| Remove the four targets only from the registry | Leaves unreachable implementations, templates, dependencies, tests, and documentation, preserving the unwanted maintenance cost. |
| Hard-code separate Claude Code and Pi render paths | Discards customization already required by both surviving targets and makes a future adapter a render-loop rewrite. |
| Purge the adapters while redesigning the whole target and encoding model | Mixes a bounded removal with speculative restructuring; the current generic seams can carry the change. |
| Migrate or temporarily accept removed target names | Retains compatibility machinery for projects the owner does not use and weakens the intended simplification. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Implementing; content-sha256: 80168b260b1c709601e7ee14c98870b027bb9dddcfbea4b6c48d6f69ed05f412
- 2026-08-02: Applied; operations: add `rendering/catalog-and-targets:built-in-runtime-targets`, update `rendering/catalog-and-targets:structured-agent-encoding`, update `rendering/catalog-and-targets:target-dialect-render`, remove `rendering/project-output-plan:cursor-no-bridge`
- 2026-08-02: Applied; operations: add `rendering/project-output-plan:bridge-render-identity`
- 2026-08-02: Applied; operations: update `rendering/project-output-plan:multi-target-render`
- 2026-08-02: Implemented; content-sha256: 80168b260b1c709601e7ee14c98870b027bb9dddcfbea4b6c48d6f69ed05f412
