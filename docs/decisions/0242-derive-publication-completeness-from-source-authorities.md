---
format: current-state-v4
slug: derive-publication-completeness-from-source-authorities
status: Implementing
date: 2026-08-06
---
# ADR-0242: Derive Publication Completeness from Source Authorities


## Context

The active pitfalls catalog retains three publication-completeness hazards with the same
failure shape: a human-maintained projection can omit a new member while the authority it
projects and the repository gate both remain valid.

The command set, help order, structured help, and gated-command projection already derive
from `clispec`, but the root README command table is hand-maintained. A new command can
therefore ship without a table row. The embedded template filesystem resolves every template
identity already declared by the catalog and output models, but a new source directory or
file absent from the embed allowlist belongs to no declared population and is invisible to
those checks. The config reference classifies every published field through a second explicit
map; its parity check catches an omitted classification, but a new project value deliberately
classified as static remains valid and renders `n/a`.

ADR-0235 chose exhaustive explicit live-state classification because it prevented omission.
That representation still duplicates the semantic boundary it is meant to protect and cannot
reject the wrong-but-valid static choice for a future project value. The stable distinction is
structural: a project config value has one observable current value, while a sidecar field or
an item-schema leaf has no single project-relative value. Correcting that decision forward is
necessary to make the config-reference pitfall obsolete rather than merely adding another
review reminder.

These three surfaces have different owners and representations. A generalized publication
registry would compete with them. The shared decision is instead that each completeness proof
derives its population from the source authority it checks and keeps formatting at the
presentation boundary.

## Decision

1. `decision: source-derived-publication-completeness` The root README command block, repository template embedding, and config-reference live state each derive and check their population at the owning boundary rather than maintaining a parallel manual membership decision. The README command block derives its ordered top-level command usages and summaries from `clispec`, while surrounding prose remains hand-owned. Repository template parity compares every source template file with the embedded filesystem, including files in new directories and files requiring `all:` semantics; template execution retains its existing missingkey=zero behavior and token-free empty-value rendering. Config-reference classification derives from configspec path structure: project config values require live resolvers, while sidecar fields and item-schema leaves are statically not applicable because they have no singular project value. Bidirectional checks fail with the missing or divergent member, and no generalized publication registry or semantic prose inference is introduced.

## State changes

- update `tooling/cli:cli-command-spec-single-source`
- add `rendering/templates:source-embed-parity`
- update `config/configspec-and-reference:live-state-projection-explicit`

## Consequences

Adding or changing a top-level command makes its README projection stale until the bounded
block matches `clispec`. Adding any template source file makes the repository test fail until
the file is embedded. Adding a project config value makes the config reference incomplete
until a live resolver exists; a future author cannot silence that failure by selecting a
static class that is structurally inapplicable.

The README loses hand-curated command-row descriptions in favor of the same concise summaries
used by the CLI authority; detailed and nested-command guidance remains available through
structured help and hand-authored prose outside the bounded block. Template parity is a
repository-source proof rather than an adopter runtime check. Config-reference output grows
live representations for project values that previously displayed `n/a`, and compact
collection summaries remain presentation policy owned by the config reference.

The three human reminders leave the active pitfalls catalog only after tests demonstrate the
original omissions fail. Semantic README accuracy, singleton render fan-out, general pitfalls
taxonomy, and arbitrary prose meaning remain outside this decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the three pitfalls as review reminders | The omissions are mechanically knowable and recurring manual checks add friction without improving judgment. |
| Preserve ADR-0235's explicit live/static map and add more examples | A future meaningful field can still be intentionally classified static and remain check-clean. |
| Add production machinery to generate the bounded README command block | A repository-only projection test can derive the same exact block and diagnostic without making repository-specific Markdown part of the shipped CLI. |
| Generate the entire README | Only the command population has a stable source authority; generating surrounding narrative would erase useful hand ownership. |
| Add a generalized publication or render registry | The command table, template source tree, embedded filesystem, and configspec already own their facts; another registry would duplicate them. |

## Status history

- 2026-08-06: Proposed
- 2026-08-06: Accepted; content-sha256: 1e20c11bb050ef4fa028f959ff8293b4e49fa17a36809d935a3436513466709d
- 2026-08-06: Implementing; content-sha256: 1e20c11bb050ef4fa028f959ff8293b4e49fa17a36809d935a3436513466709d
- 2026-08-06: Applied; operations: update `tooling/cli:cli-command-spec-single-source`
- 2026-08-06: Applied; operations: add `rendering/templates:source-embed-parity`
- 2026-08-06: Applied; operations: update `config/configspec-and-reference:live-state-projection-explicit`
- 2026-08-06: Reapplied; operations: update `tooling/cli:cli-command-spec-single-source`
