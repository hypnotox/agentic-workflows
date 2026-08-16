---
format: current-state-v4
slug: keep-repository-local-instructions-out-of-shipped-surfaces
status: Proposed
date: 2026-08-16
---
# ADR-0282: Keep Repository-Local Instructions Out of Shipped Surfaces


## Context

The standard workflow templates instructed every Full-profile adopter to run `./x audit-local`, but that command and its `cmd/repoaudit` implementation exist only in the awf repository. The same templates must support self-hosted repository guidance without publishing commands that an ordinary adopter cannot run.

## Decision

1. `decision: repository-local-instructions-stay-local` Shipped templates and rendered adopter surfaces name only workflow commands the standard provides; repository-local instructions remain in self-hosted convention overrides.

## State changes

- update `rendering/workflow-skill-templates:phase-transaction-ownership`

## Consequences

Adopter workflow instructions remain executable without copying awf's private command runner or auxiliary tools. The awf repository can retain stronger local assurance through its convention parts. Publication checks must distinguish shipped sources from deliberate self-hosted overrides.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Ship `audit-local` as a standard command | Its changelog and coverage-ignore checks are specific to this repository rather than the language-agnostic standard. |
| Remove the repository-local audit everywhere | That would discard useful self-hosted assurance instead of confining it to its owner. |

## Status history

- 2026-08-16: Proposed
