---
format: current-state-v4
slug: strongest-practical-durable-oracle
status: Implemented
date: 2026-08-20
---
# ADR-0291: Strongest practical durable oracle


## Context

The bugfix and TDD skills currently require an automated regression test to be written and
observed failing for the right reason before every fix. Debugging, plan authoring, and review
surfaces repeat the same universal requirement. This is the strongest normal verification path,
but it blocks otherwise valid fixes when safe, deterministic, or economical automated reproduction
is impractical, including environment-specific integration failures, nondeterministic races, and
destructive migrations.

The protected-contract doctrine makes required verification strength non-negotiable and reserves
verification-oracle corrections for a separate decision. The correction therefore needs a legal
alternative route without turning impractical reproduction into permission to weaken expected
behaviour, assertions, retained evidence, or the gate. The audit remediation authority already
settles the outcome and excludes test-framework, coverage-policy, gate, and package refactors.

## Decision

1. `decision: strongest-practical-durable-oracle` Every behaviour-changing fix uses the strongest
   practical durable oracle. An automated regression test observed failing for the right reason and
   then passing is the normal and preferred path. When that path is impractical, the fix states a
   concrete reason, preserves or improves verification strength, and retains the strongest safe,
   reproducible alternative evidence.

2. `decision: oracle-preference-order` Select the strongest applicable evidence without mechanically
   attempting every earlier option: an observed red-then-green automated regression test; a
   deterministic integration or reproduction harness; a contract or invariant test that directly
   exercises the failure; scripted reproducible manual verification with recorded inputs and
   expected result; or an explicit explanation of why durable automation is unavailable together
   with the strongest safe evidence that can be retained. Stress or invariant evidence and safe
   fixture or dry-run evidence are valid applications of this order.

3. `decision: oracle-never-weakened` Impractical automated reproduction never permits weakened
   expected behaviour or verification strength. A fix corrects the root cause rather than adjusting
   an oracle to accept the symptom, and ordinary deterministic defects still require the preferred
   observed red-then-green regression test.

4. `decision: oracle-rule-single-home` The complete rule has one authored home. Applicable testing,
   implementation, planning, and review guidance projects that rule rather than maintaining
   independently changeable variants. The invariant uses `Backing: test` with a matching proof
   annotation under `internal/`; every affected template renders coherently with empty variables and
   emits no `<no value>` token.

## State changes

- add `rendering/workflow-skill-templates:strongest-practical-durable-oracle`

## Consequences

A valid fix is no longer blocked solely because an automated red-first reproduction is impractical.
Environment-specific failures can retain a scripted reproduction, races can retain stress or
invariant evidence, and destructive migration defects can retain safe fixture or dry-run evidence.
The concrete-reason requirement makes the exception visible and reviewable instead of allowing a
silent verification downgrade.

Automated red-then-green regression testing remains the default and remains mandatory for ordinary
deterministic defects. The rule changes only the legal evidence route when that default is
impractical; it does not relax gates, coverage, expected behaviour, or root-cause correction.

One canonical home reduces semantic drift, but each applicable workflow surface must project it and
keep only stage-specific procedure locally. Adopter-authored section overrides can retain older
wording; the mandatory canonical projection governs it without attempting an unsafe migration of
arbitrary adopter prose.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep automated red-first universal | Blocks valid fixes when faithful automation is unsafe, nondeterministic, environment-bound, or disproportionately costly. |
| Permit an unstructured exception | Allows silent verification reduction and makes review unable to distinguish necessity from convenience. |
| Require mechanical attempts in the full evidence order | Turns practical guidance into choreography and can require unsafe or wasteful reproduction before an evidently stronger applicable alternative. |
| Restate the adjusted rule in each workflow surface | Recreates independently changeable policy and permits the exception boundary to drift. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Implementing; content-sha256: a91ff332ddfc332ffba550feeaeada374a0a9294789ae4cbc825d6bd14da5c53
- 2026-08-20: Applied; operations: add `rendering/workflow-skill-templates:strongest-practical-durable-oracle`
- 2026-08-20: Implemented; content-sha256: a91ff332ddfc332ffba550feeaeada374a0a9294789ae4cbc825d6bd14da5c53
