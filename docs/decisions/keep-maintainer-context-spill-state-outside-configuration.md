---
format: current-state-v4
slug: keep-maintainer-context-spill-state-outside-configuration
status: Proposed
date: 2026-08-11
---
# ADR-keep-maintainer-context-spill-state-outside-configuration: Keep Maintainer Context Spill State Outside Configuration


## Context

ADR-0165 established a repository-maintainer observability side effect for this repository's
hand-written `./x context` wrapper. When `awf context` spills an oversized result to its secure
external delivery file, the wrapper records a path-free event in an ignored local log. `./x check`
is then meant to emit a non-failing advisory while that log remains nonempty.

The implemented location is below `.awf/`. That conflicts with the public closed configuration-tree
contract established by ADR-0086: every entry below `.awf/` must be part of the claimed-path model or
an owned resident root. The wrapper creates its log successfully, but the next `./x check` reports the
containing local directory as orphaned drift before the wrapper can run the intended advisory. The
existing wrapper test replaced `./awf check` with a successful stub and therefore proved the advisory
in isolation without proving composition with real drift detection.

This state is owned only by the awf repository's maintainer runner. It is not adopter configuration,
a rendered artifact, or lifecycle-managed resident data. Adding its directory to the resident-root
table would project a private concern through public rendering, discovery, preservation, and
uninstall behavior while exempting arbitrary descendants. Narrowly claiming its exact path would
still reserve repository-private operational storage in every adopter's public configuration model.
The repository already has an ignored checkout-local cache boundary for maintainer tooling.

## Decision

1. `decision: separate-maintainer-state` Maintainer-owned context-spill observability lives in a dedicated ignored checkout-local cache outside `.awf`, keeping repository-private operational state separate from public adopter configuration and resident-state authority.
2. `decision: preserve-advisory-security` Relocation preserves the approved path-free, owner-only, no-follow, serialized, warning-only observability and operator-removal contract.

## State changes

- update `tooling/context-and-topic:context-spill-observability`

## Consequences

A spill observation no longer makes the next check fail closed-tree drift, so the non-failing
advisory becomes reachable as intended. The public claimed-path and resident-root models remain
closed and gain no repository-specific exception. Each linked checkout continues to own its own
observability state.

The maintainer log changes location, so its helper, diagnostics, tests, and reader-facing guidance
must move together. Operators with a log at the old ignored location may remove it manually; the
new helper does not migrate or interpret that obsolete repository-local file. A dedicated private
cache descendant retains the current security boundary even though the shared cache parent may also
contain state from other maintainer tools.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Claim only the existing log path in the public config-tree model | It reserves a repository-private runner path for every adopter and gives operational state configuration-tree meaning. |
| Add the existing local directory as a resident root | It exempts arbitrary descendants and fans a private runner concern through public rendering, anchoring, preservation, and uninstall behavior. |
| Reorder `./x check` so the advisory runs before drift | The advisory would print, but the same invocation would still fail on the state that generated it. |

## Status history

- 2026-08-11: Proposed
