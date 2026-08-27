---
format: current-state-v4
slug: require-complete-and-unambiguous-mutable-authority
status: Implementing
date: 2026-08-26
---
# ADR-0310: Require Complete and Unambiguous Mutable Authority


## Context

Project configuration, sidecars, the live lock, and the upgrade journal are mutable authority: awf
uses their decoded meaning to select or authorize later behavior. Their existing claims establish
strict root-key ownership, corrupt-lock refusal, and recoverable upgrades, but leave narrower parser
integrity gaps.

A valid first YAML or JSON value could conceal trailing content. Live lock decoding could accept
unknown or duplicate fields and an empty permanent inventory. Recovery could trust a syntactically
valid journal without proving that its final operation replaced the lock named by its recorded
digest. In each case, awf could act on only part of the bytes or on authority whose internal identity
did not match the mutation being recovered. Refusing at the consuming boundary is safer than
allowing downstream operations to infer whether partially decoded authority was intentional.

## Decision

1. `decision: mutable-authority-fails-closed` Before acting on mutable project authority, awf must
   establish that the complete input is unambiguous and internally consistent. Configuration and
   sidecars contain exactly one complete known-field YAML document. A present live lock is a single
   closed JSON object with unique fields and a nonempty, well-formed permanent file inventory.
   Upgrade recovery accepts only a complete journal whose ordered terminal lock replacement and
   recorded digest agree. Any failure refuses before the affected operation mutates project state;
   lock absence remains the distinct first-adoption case.

## State changes

- update `config/configuration:root-sidecar-keys-rejected`
- update `config/migrations-and-locks:corrupt-lock-refuses`
- update `tooling/upgrade-runtime:upgrade-failure-is-recoverable`

## Consequences

- Mutable authority cannot gain an ignored suffix or depend on permissive decoder behavior.
- Canonical prior inputs remain compatible. Previously tolerated multi-document configuration,
  ambiguous live locks, and unbound journals require operator correction rather than automatic
  migration.
- Working-tree and snapshot configuration loaders share the strict contract, so render and Publisher
  refuse ambiguous config or sidecars before mutation. The ordinary manifest live-lock parser owns
  permanent-inventory validity for every current-authority consumer, rather than leaving it to
  Publisher alone. Upgrade recovery validates its journal before recovery mutation.
- Recovery remains journal based and lock-last. It gains an integrity precondition rather than a new
  recovery protocol.
- Historical config and lock decoding used by audit or migration classification remains separate
  from the strict live-authority contract.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Retain permissive decoding and document canonical serialization only | Canonical writers do not protect readers from ambiguous or adversarial authority bytes. |
| Validate locks and journals only immediately before individual mutations | Scattered checks permit inconsistent consumers and make refusal depend on the later operation selected. |
| Replace upgrade journaling with a new transaction protocol | The existing journal and lock-last model remains suitable once its accepted input is bound to the recovered mutation. |

## Status history

- 2026-08-26: Proposed
- 2026-08-26: Accepted; content-sha256: 67fc6a030b11cb116d23bb7b186b0d50dd4c1408539ab1346fb6b370b28a48b7
- 2026-08-26: Implementing; content-sha256: 67fc6a030b11cb116d23bb7b186b0d50dd4c1408539ab1346fb6b370b28a48b7
- 2026-08-26: Applied; operations: update `config/configuration:root-sidecar-keys-rejected`, update `config/migrations-and-locks:corrupt-lock-refuses`, update `tooling/upgrade-runtime:upgrade-failure-is-recoverable`
