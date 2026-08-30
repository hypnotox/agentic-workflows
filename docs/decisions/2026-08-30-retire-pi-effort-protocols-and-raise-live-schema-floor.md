# 2026-08-30 retire-pi-effort-protocols-and-raise-live-schema-floor

## Context

All nine known external adopters have committed AWF 0.44.0 at schema 50 on their default branches. GitHub can prove those committed trees, but it cannot prove the absence of ignored local journals, stale worktrees, unpushed branches, or old Pi processes that still hold generated extension code in memory. The producer's old migration path remains defective for an untouched pre-schema-50 checkout, and no managed live repository now requires that path.

AWF's generic effort lifecycle remains useful, but its Pi-specific association bridge and effort-memory command family created a second protocol layer around ordinary workflow continuity. They introduced activity ownership, heartbeat, hidden session context, dynamic memory tools, structured metadata parsing, previews, diffs, and machine replies without making those mechanisms authoritative. The workflow already requires one user-managed memory writer and treats repository authority as stronger than an effort checkpoint.

This decision advances the live-source boundary established by `0297-bound-compatibility-support-to-managed-reality` and `0303-separate-live-upgrade-support-from-historical-audit-decoding`. It supersedes the current force of the Pi association and memory protocols introduced by `0218-associate-pi-sessions-with-efforts-and-live-checkout-context`, its later simplifications, `0239-add-associated-pi-effort-memory-tools`, and the Pi Cockpit integration adopted by `0319-adopt-the-pi-cockpit-effort-integration-contract`. Those records remain historical.

## Decision

AWF 0.45 supports live repositories only at schema 50. `LiveSchemaFloor` and `CurrentSchema` are both 50, and schema 50 continues to require AWF 0.44.0 or newer. A live repository below schema 50 refuses without mutation and must first use AWF 0.44; a repository above schema 50 continues to refuse as ahead. The generic migration planner, same-schema synchronization, journal recovery, lock-last publication, and historical audit decoding remain. AWF does not add schema 51 merely to remove readers.

Retire Pi effort association and the complete `awf effort activity` command family. Remove activity ownership, heartbeat, hidden effort context, dynamic memory-tool activation, Cockpit effort publication, the generated Pi effort extension, and the Pi-only `using-effort` bridge. The host-neutral `effort-workflow` skill remains available to Pi and Claude and continues to own continuity, checkpointing, integration, worktree removal, and finish order.

Retire the complete `awf effort memory read`, `edit`, and `update` command family, including owner-scoped JSON, preview, diff, metadata mutation, and presentation protocols. `memory.md` remains a required safely owned Markdown resident with a workflow-defined human scaffold and one user-managed writer. `state.json` alone owns effort identity. The workflow resolves the memory path through `awf effort show` and uses ordinary file tools; AWF does not parse, timestamp, patch, preview, or expose the memory through a special CLI.

Preserve existing memory bytes, including old four-key frontmatter, without migration. Continue accepting `activity.json` as an optional resident leaf, but treat it only as ignored opaque bytes: production code does not create, read, parse, validate, mutate, or delete it independently. Finishing an effort archives the complete accepted resident, including any `activity.json`, byte-for-byte. Missing or unsafe required memory, invalid state, unsafe ownership, and foreign resident leaves continue to refuse lifecycle mutation.

## Consequences

The public effort grammar becomes `new`, `list`, `show`, `finish`, `worktree`, and `integrate`. Repository synchronization prunes the retired bridge outputs, while ordinary Pi profile negotiation, model routing, subagents, handoff, and native-skill discovery remain. A Pi process that loaded the old extension must also be restarted because repository synchronization cannot unload code already held in memory.

Below-schema-50 live trees lose an in-place route in current AWF and receive a clear recovery direction instead. Generic transaction safety and historical decoding remain rather than being mistaken for obsolete migration code. Existing memories and activity files remain lifecycle-compatible without retaining their retired runtime protocols.

Generated outputs, current-state claims, changelog, version authority, and the lockfile move only during the parent integration transaction. Capability deletion and test-selection or native-assurance work remain separate implementation concerns so their canonical write sets and behavioral assertions can be reviewed independently.
