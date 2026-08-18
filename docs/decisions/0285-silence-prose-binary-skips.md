---
format: current-state-v4
slug: silence-prose-binary-skips
status: Proposed
date: 2026-08-18
---
# ADR-0285: Silence Prose Binary Skips


## Context

The prose gate skips non-UTF-8 staged files because they cannot be safely scanned for banned typographic punctuation. It currently emits one warning per skipped path. Adopter repositories that track many binary assets receive noisy check output even when their prose is clean.

## Decision

1. `decision: silence-binary-skips` The prose gate silently skips files that are not valid UTF-8. It continues to report banned punctuation in text files with the existing failure and exit-status behavior, and retains the existing exemption behavior.

## State changes

- update `tooling/quality-gates:prose-gate-tracked-file-scan`

## Consequences

Clean prose checks no longer produce warnings for tracked binary assets. The gate retains its conservative handling of non-text content but no longer gives a per-file account of skipped paths.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Continue reporting every skipped path | Creates noisy output proportional to binary assets rather than prose problems. |
| Add a configuration switch | Adds adoption configuration for behavior that should be the default. |

## Status history

- 2026-08-18: Proposed
