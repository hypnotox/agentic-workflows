# 2026-09-03 name the broad evidence capability context

## Context

The `repository-context` skill combined orientation, bounded exploration, and premise challenge. Its name and role language framed all three lanes as repository work even when a question may require evidence from another project, dependency source or documentation, an authoritative external source, or an available search tool. That framing could route agents away from relevant evidence or make them silently treat the current checkout as the complete search boundary.

The three lanes remain useful and independent. The problem is their implied evidence boundary, not their responsibilities or their report-only constraints.

## Decision

Rename the skill to `context`. Define orientation as gathering applicable evidence from the current repository and, when the question requires it, other projects, dependencies, authoritative external sources, and available search tools. Require bounded exploration and premise challenge to state their evidence boundary and preserve uncertainty without assuming that boundary is one repository.

Broaden the explorer and premise-checker role contracts in the same way. Advance the live schema and migrate authored `repository-context` sidecars and declared section parts to `context`, refusing conflicting old and new sources rather than overwriting either.

## Consequences

The capability name now describes its purpose without prescribing where evidence lives. Runtime adapters may use whatever read-only sources and tools are available while retaining explicit boundaries, citations, uncertainty, and no mutation authority.

Generated skill paths change from `<prefix>-repository-context` to `<prefix>-context`. Supported upgrades preserve recognized adopter customizations and remove the old generated artifact during synchronization. A repository containing conflicting authored sources under both names must reconcile them before upgrade can continue.
