# 2026-08-30 simplify-workflow-governance

## Context

Awf had accumulated machine-managed ADR and plan formats, mandatory workflow choreography, Core and Full profiles, artifact-specific review routes, and exact coverage, mutation, and performance qualification. Those mechanisms duplicated authority, coupled implementation to process representation, and made local and CI assurance repeat the same work. The useful boundaries are narrower: durable repository truth, current-state ownership and evidence, generated-source drift protection, effort continuity, and destructive, integration, and release safety.

## Decision

Awf enforces durable repository truth and destructive-operation safety while treating the route to completion as advisory and risk-based.

Decision history is plain append-only Markdown. New records use accepted date-slug files with Context, Decision, and Consequences; existing numbered records remain historical bytes. Current-state topics independently own implemented rules and invariants. Operational plans are unparsed `scratch/plan.md` files inside their effort and archive only with that effort.

Awf renders one standard footprint with a lean skill catalog and four fresh-context roles: explorer, premise-checker, implementer, and reviewer. One implementation capability chooses inline, sequential, or bounded same-worktree parallel execution; parallel work requires dependency-independent canonical disjoint write sets, while the parent retains shared files, integration, staging, commits, review judgment, and final verification. One optional report-only review covers the combined relevant context when risk or uncertainty warrants it.

The effort subsystem retains fixed identity, one managed worktree, one memory writer, safe integration and removal, recovery, finish, and archive semantics. Local verification emphasizes focused affected feedback. One aggregate `CI / gate` result is the definitive repository verdict; exact line-coverage, blocking mutation, tracked performance, exhaustive local hooks, and duplicated CI qualification are retired.

## Consequences

This is an intentional breaking simplification. Awf no longer parses, scaffolds, numbers, indexes, reviews, or coordinates decision records; it no longer parses or permanently publishes operational plans; and it exposes no governance profile or compatibility layer for retired workflow formats. Supported upgrades migrate recognized historical configuration safely and refuse meaningful retired overrides rather than silently discarding them.

Agents and maintainers exercise more judgment about planning, delegation, and review, but do so against fewer and clearer authorities. Current-state checks, proof markers, rendering and drift checks, effort topology safety, signed-history policy, and release integrity remain enforceable. Historical numbered decisions preserve prior rationale without determining what is currently true.
