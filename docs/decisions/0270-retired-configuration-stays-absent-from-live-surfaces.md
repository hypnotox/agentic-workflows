---
format: current-state-v4
slug: retired-configuration-stays-absent-from-live-surfaces
status: Proposed
date: 2026-08-12
---
# ADR-0270: Retired configuration stays absent from live surfaces


## Context

The configuration-selection retirement removed project-local artifact selection and the sidecar
`local` field from the live schema. The shipped default working guide nevertheless continued to
advertise `local: true` docs and project-local skills, agents, and docs. This checkout's convention
part masked that stale default, so ordinary render and drift checks did not expose it.

The same audit found that historical projection removes recent retired keys before strict current
schema decoding but does not remove the older top-level `invariants` and `audit.baseBranch` keys.
Live guidance and historical projection therefore disagree with the completed retirement boundary
in different ways.

## Decision

1. `decision: live-retirement` Live templates, current-state guidance, and generated documentation never present retired configuration or the capabilities it selected as supported; truthful historical and vocabulary references remain.
2. `decision: historical-retirement` Every retired configuration key is removed when historical configuration is projected into the current schema, before strict decoding.

## State changes

- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- add `rendering/templates:retired-config-guidance-absent`
- update `config/migrations-and-locks:retired-keys-forward-ported`
- remove `config/migrations-and-locks:toggle-keys-forward-ported`

## Consequences

Adopters no longer receive documentation for unsupported project-local artifact selection. A
focused test-backed invariant protects live templates and current-state guidance because ordinary
self-host rendering can mask a stale default with a convention part.

Historical audit and staged checks can consume configurations containing any retired key without
mistaking historical syntax for current configuration. The retirement inventory must remain complete
when later keys are removed. Historical ADRs, migrations, fixtures, and vocabulary may continue to
name retired keys when the reference is truthful.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Correct only the stale guide template | It would leave the historical projection defect and adjacent live guidance discovered by the same audit unresolved. |
| Reject all retired-key text repository-wide | Historical decisions, migration authority, and fixtures must preserve retired syntax truthfully. |

## Status history

- 2026-08-12: Proposed
