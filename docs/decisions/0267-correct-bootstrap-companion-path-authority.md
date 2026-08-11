---
format: current-state-v4
slug: correct-bootstrap-companion-path-authority
status: Implemented
date: 2026-08-11
---
# ADR-0267: Correct Bootstrap Companion Path Authority


## Context

The active `rendering/companion-scripts:bootstrap-checksum` claim identifies the bootstrap as the
rendered `awf-bootstrap.sh`. The active `rendering/singletons-and-payloads:bootstrap-config-tree-path`
claim and the output plan instead establish `.awf/bootstrap.sh` as the only rendered bootstrap path
and retire the repository-root `awf-bootstrap.sh` location. The checksum behavior remains correct;
only its authoritative path name is stale.

## Decision

1. `decision: bootstrap-claim-uses-live-path` The companion-script checksum authority identifies the
   bootstrap by its live rendered path, `.awf/bootstrap.sh`.

## State changes

- update `rendering/companion-scripts:bootstrap-checksum`

## Consequences

The companion-script and singleton current-state topics agree on bootstrap identity. The change does
not alter rendering, checksum behavior, compatibility, or output layout. It requires only a
forward current-state claim correction and its generated documentation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave the stale path in place | Active authority would continue to contradict the rendered output plan. |
| Remove the checksum claim | The checksum behavior remains current and test-backed; only its path name is stale. |

## Status history

- 2026-08-11: Proposed
- 2026-08-11: Implemented; content-sha256: 2e8976db1b1180a4479c91bb7deb47b28852a1da70a6deb2f65448996155452e
