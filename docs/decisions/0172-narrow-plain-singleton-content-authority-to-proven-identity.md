---
format: current-state-v2
status: Implemented
date: 2026-07-28
---
# ADR-0172: Narrow plain singleton content authority to proven identity

## Context

ADR-0171 qualified local suppression and strengthened the plain-singleton proof. Its verify review
found one remaining overclaim: asserting catalog-template content requires an independent comparison
against fully rendered template output, while the backing test proves catalog `TemplateID`, nonempty
content, and the shared `renderKind` path. A hypothetical post-render content replacement could retain
those observed properties while violating the stronger wording.

The user chose to narrow authority to the behavior that is deterministically proven rather than add
a second full rendering implementation to the test suite. ADR-0171 is Implemented and frozen, so the
claim must move forward through this decision.

## Decision

1. Define plain-singleton content authority as a catalog `TemplateID` plus nonempty rendered content,
   not byte-level equivalence to an independently rendered catalog template.
2. Retain the fixed catalog-derived path, exactly-once output, local-sidecar suppression, derived
   membership, neutral declarer identity, and common `renderKind` call-path guarantees.
3. Keep the existing ADR-0171 backing tests unchanged because they directly prove the narrowed claim.

## State changes

- update `rendering/singletons-and-payloads:plain-singleton-via-renderkind`

## Consequences

The current-state claim no longer promises more content provenance than its deterministic proof can
refute. Template identity and the shared renderer remain explicit, while exact rendered bytes stay
governed by the broader rendering and drift contracts.

No runtime or adopter output changes. A future decision may add stronger independent content proof if
a concrete failure demonstrates that the narrower guarantee is insufficient.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Independently render and compare every singleton template in this test | It duplicates rendering setup and adds coupling without a demonstrated defect. |
| Keep the stronger claim with only TemplateID and nonempty-content assertions | A backing test must be capable of refuting every clause it claims to prove. |
| Remove content from the claim entirely | Nonempty output is already cheap, deterministic, and useful to guarantee. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Implemented; content-sha256: b083a89bccb12b4e4320bb092de4c071623feebb3fdc798fc3f6c2e79aacb42f
