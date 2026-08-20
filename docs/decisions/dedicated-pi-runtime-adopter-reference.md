---
format: current-state-v4
slug: dedicated-pi-runtime-adopter-reference
status: Proposed
date: 2026-08-20
---
# ADR-dedicated-pi-runtime-adopter-reference: Dedicated Pi runtime adopter reference

## Context

AF-011 separates daily adopter guidance from advanced lifecycle, recovery, configuration, and
runtime protocols. `working-with-awf.md` currently carries detailed Pi requirements, model routing,
preference-file, wizard, and subagent protocol facts. The repository's Pi current-state topics own
implementation authority, but those topics are project-authored inputs rather than portable standard
adopter documentation. The generated Pi skills each own one operation and cannot serve as a complete
runtime reference. Removing the detail from the daily guide would therefore leave shipped adopters
without one reachable documentation owner.

The approved AF-011 boundary requires every moved fact to retain one most-specific owner, permits a
new document only when no existing owner fits, and requires one-hop document-map navigation. Generic
workflow, configuration, development, and debugging documents do not cleanly own target-specific Pi
runtime protocol.

## Decision

1. `decision: dedicated-pi-adopter-reference` Ship one standard Pi runtime reference as the single adopter-facing owner for awf's Pi runtime requirements, ownership boundary, model routing and preferences, subagent profiles, effort integration, and session-replacement navigation. Keep daily and generic workflow documents as links rather than parallel protocol homes, and make the reference directly reachable from the generated document map.

## State changes

- add `rendering/doc-outputs:pi-runtime-reference-output`

## Consequences

Pi protocol remains complete and reachable after the daily guide is shortened. Runtime-specific
changes update one adopter reference alongside their implementation authority. Under the current
standard-document selection model, the reference also renders for projects that do not use Pi; its
scope statement makes that applicability explicit. A future selection mechanism may remove that
irrelevant entry without changing this decision, provided Pi adopters retain the complete mapped
reference.

The reference is another standard output whose template, generated file, lock membership, drift,
link checks, and document-map entry must remain covered together.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep Pi detail in `working-with-awf.md` | It preserves the daily-versus-advanced mixture AF-011 removes. |
| Use workflow, development, or configuration reference | Each has a different generic or field-semantics owner and would misplace target protocol. |
| Rely on self-hosted current-state topics or individual Pi skills | They do not ship one complete portable adopter reference. |

## Status history

- 2026-08-20: Proposed
