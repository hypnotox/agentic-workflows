---
format: current-state-v4
slug: correct-adr-0219-reference-and-rendering-handoff-prose
status: Implemented
date: 2026-08-02
---
# ADR-correct-adr-0219-reference-and-rendering-handoff-prose: Correct ADR-0219 reference and rendering handoff prose

## Context

Renewed integration review found two retained documentation errors after ADR-0219 restored the
kickoff-only Pi handoff contract. Its Alternatives Considered table names unrelated ADR-0217 where
it intended the effort decision now numbered ADR-0218. Because ADR-0219 is terminal, its body is
frozen and the mistaken historical reference must be corrected forward. The rendering domain's
current-state overview also retains an older optional-memory handoff paragraph immediately before
the truthful kickoff-only paragraph.

## Decision

1. `decision: correct-0219-reference-forward` Record that ADR-0219's alternative labeled "Rewrite
   ADR-0217" refers to ADR-0218, the terminal effort-associated Pi sessions decision. Preserve
   ADR-0219 byte-for-byte rather than rewriting frozen history.
2. `decision: remove-stale-rendering-handoff-prose` Remove the rendering-domain paragraph that says
   `handoff_session` accepts an effort memory path. Retain the adjacent kickoff-only paragraph as
   the current domain overview.

## State changes

None.

## Consequences

The durable record resolves the mistaken cross-reference without mutating terminal history, and
the rendering domain no longer contradicts the kickoff-only public contract. No runtime behavior
or current-state claim changes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Rewrite ADR-0219 in place | Terminal ADR bodies are frozen; stable history is corrected forward. |
| Leave the rendering paragraph as historical context | The domain overview describes current state and the paragraph contradicts its successor. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Implemented; content-sha256: fe4eab6d46406fe2d24636869bc38a6a877f4145f1222e096bbbe78eb7ab4106
