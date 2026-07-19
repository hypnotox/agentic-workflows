---
status: Proposed
date: 2026-07-19
tags: [invariant-retirement, schema-migration, uncovered-coverage, upgrade-flow]
related: [7, 8, 10, 14, 31, 39, 64, 85, 102, 105, 110, 114, 120, 133, 134, 135]
domains: [adr-system, invariants, rendering, tooling]
---
# ADR-0136: Project-Atomic Migration to Current-State Authority

## Context

ADR-0133 rejects a permanent compatibility authority mode. ADR-0134 introduces a breaking topic, claim, marker, and configuration schema. ADR-0135 makes State changes mandatory only above a lock-recorded ADR number. Adopters cannot receive the new semantics safely through a migration that silently generates authoritative prose or temporarily treats both ADRs and topics as current truth.

Invariant obligations are the mechanically enumerable part of current ADR authority. Every live legacy invariant declaration is either still owed or has an explicit retirement record. The migration can require an adopter to classify that closed set without pretending to infer the correct current wording, topic, or path scope. Proof and touches markers then provide a second closed set that must be retargeted.

Ordinary architectural rules are not equivalently enumerable. Domain current-state prose, code, tests, agent guidance, and historical ADRs are source material, but deciding the bounded current claims remains authored judgment. Topic coverage can expose paths with no focused current-state owner; it cannot prove semantic completeness.

The upgrade must therefore be strict, project-atomic, and honest. An adopter prepares the new inputs, runs a read-only readiness check, and either crosses the schema boundary completely or remains on the preceding release.

## Decision

1. The release containing current-state authority ships only the topic-based context, invariant, and current-guidance engines. It contains no legacy ADR-derived context fallback, supersession authority graph, tag-tier expansion, or Implemented-ADR invariant checker. A project whose migration is incomplete cannot update its lock to the new schema and continues using the preceding awf release.

2. The new binary exposes `awf upgrade --check` as a read-only migration readiness report. While the lock is on the immediately preceding schema, the binary also permits `awf new topic` and parsing of prepared `currentState` configuration and topic inputs as migration-safe authoring operations. These paths scaffold or validate structure only; they never render inferred claim prose, answer context through the legacy engine, or mark the upgrade complete. All other version-gated commands retain their normal compatibility refusal until migration succeeds.

3. The adopter manually authors every topic sidecar, topic claim, provenance link, and scope. Existing domain current-state prose, agent guidance, code, tests, and ADRs may be consulted, but no migration command extracts or promotes their prose. `awf new topic` creates an empty topic shell and metadata scaffold only. Empty topics do not satisfy coverage.

4. Upgrade preflight builds a legacy invariant inventory from declarations on legacy Implemented or Superseded ADRs, then subtracts every machine-readable retirement. Existing retirement tokens are normalized into an append-only `## Migration history` section on the carrier ADR with entries of the exact form ``- YYYY-MM-DD: retired invariant `<slug>` ``. An adopter deliberately retiring an otherwise live invariant adds the same entry plus ` - <nonempty rationale>` to the ADR recording that retirement decision. This is a meaning-preserving bookkeeping retrofit, not an edit to historical rationale.

5. Every inventory slug not explicitly retired must appear as the local slug of exactly one current-state invariant claim, in whichever topic the adopter judges correct. The migrated claim cites the declaring legacy ADR as Origin and preserves backed versus unbacked classification unless a separately reviewed decision changes or retires the contract. Every legacy proof marker is rewritten to the resulting qualified ID; every advisory touches marker is rewritten to `touches-state:` with a nonempty note. No unqualified invariant, proof, or touches marker may remain.

6. Preflight refuses unless all configured domains that own topics use canonical kebab keys; every domain-owned eligible path has scoped topic coverage under `currentState.topicCoverage: error`; topic parsing, claim references, invariant backing, marker resolution, output planning, render completeness, and the full gate pass; and the generated tree contains no stale legacy ACTIVE.md or domain ADR index output.

7. Every legacy `Superseded` ADR is normalized to `Implemented`, because the new status means that the decision was incorporated historically rather than that its rule remains active. Every legacy Proposed or Accepted ADR must become Implemented or Abandoned before cutover. The awf repository's authority-model ADRs, including this record, become Implemented in the final cutover transaction after their resulting topic claims exist. No legacy ADR is retrofitted with State changes.

8. On successful upgrade, the lock records `adrFormatV1From` as one greater than the highest existing ADR number and advances schema generation and `awfVersion` atomically. The recorded lower-number set is the only permitted legacy encoding and the only Origin set exempt from inverse State changes. New scaffolding immediately uses `format: current-state-v1` and the ADR-0135 template.

9. The upgrade transaction writes mechanical configuration renames, legacy status and migration-history normalization, the new lock, and generated outputs only after the complete proposed result passes readiness validation in memory. Failure leaves the config tree, ADRs, lock, and rendered outputs byte-for-byte unchanged. Success prunes ACTIVE.md and per-domain ADR indexes, creates INDEX.md and topic indexes, and leaves `awf sync`, `awf check`, `awf context`, `awf topic`, and the invariant report operating solely on the new schema.

10. `awf check --staged` validates the final old-HEAD/new-index transaction through ADR-0135's migration-only adapter. It reads legacy identity, status, invariant declaration and retirement, and marker facts solely for readiness comparison. It does not expose a legacy authority result. After the new lock lands, ordinary static and staged checks have no legacy authority branch.

11. Migration documentation presents a human-owned checklist and exact diagnostics rather than an automatic conversion promise. The committed example adopter and awf itself complete the same cutover before release. Rollback after a successful upgrade is Git restoration plus reinstalling the preceding awf release; awf provides no mixed-mode downgrade.

## Invariants

- `unbacked-invariant: upgrade-requires-complete-current-state`: The new schema lock cannot be written until topic coverage, invariant reconciliation, marker retargeting, in-flight ADR resolution, rendering, and gates all succeed. **Verify:** fail each readiness predicate independently in upgrade fixtures and confirm the lock and tree remain byte-identical.
- `unbacked-invariant: migration-never-authors-claims`: Migration commands scaffold empty structure and report obligations but never generate normative claim prose or provenance. **Verify:** run topic scaffolding and upgrade preflight on a legacy fixture and confirm no claim heading, body, Origin, or revision metadata is invented.
- `unbacked-invariant: every-live-legacy-invariant-adjudicated`: Each nonretired legacy invariant slug maps to exactly one current-state invariant with preserved backing classification, and every retired slug has a machine-readable migration-history entry. **Verify:** exercise missing, duplicate, classification-changed, and retired-without-rationale inventories and confirm preflight reports the exact slug.
- `unbacked-invariant: no-unqualified-markers-after-upgrade`: A migrated project contains no active unqualified invariant, proof, or touches marker. **Verify:** place each legacy marker form in every configured source language and confirm readiness refuses until it is removed or rewritten to a qualified claim ID.
- `unbacked-invariant: upgrade-failure-is-byte-preserving`: Any failed readiness or write step leaves authored inputs, historical ADRs, the lock, and rendered outputs unchanged. **Verify:** inject failure at each validation and atomic-write boundary and compare a recursive pre/post tree digest.
- `unbacked-invariant: upgraded-runtime-has-one-authority-engine`: After the new lock lands, normal context and invariant reporting cannot consume legacy ADR tags, supersession edges, or invariant declarations. **Verify:** retain contradictory legacy metadata in a migrated fixture and confirm only topic claims affect output and enforcement.
- `unbacked-invariant: legacy-format-set-is-closed`: The lock's `adrFormatV1From` boundary is one greater than the migration-time maximum, and every later ADR uses the new format. **Verify:** attempt missing-format ADRs below, at, and above the recorded boundary and confirm only the closed lower set is accepted.

## Consequences

Migration demands real project work. Adopters must decide their current claims, split domains into focused topics, and review every invariant. That cost is the point: awf refuses to manufacture semantic confidence from historical prose.

The new release cannot be adopted incrementally in a long-lived mixed project. Teams perform the migration on a branch, keep using the previous release until it is ready, and merge the complete schema transition. This simplifies the shipped runtime and prevents contradictory authority during an extended rollout.

A narrow amount of legacy parsing remains in upgrade preflight so the new binary can prove that old obligations were not lost. It is bounded to inventory and format facts and cannot answer normal context or invariant queries. This is migration support, not a second operating mode.

Normalizing Superseded to Implemented preserves historical truth under the new semantics and deletes a status whose only purpose was active-currentness inference. Existing supersession prose and tokens may remain frozen historical text, but no checker maintains their graph after cutover.

The migration-history retrofit records invariant retirement without backporting current rules into old ADR rationale. The one-time inventory reads it; afterward current-state claims alone own enforcement. Historical entries remain useful when explaining why an old invariant was not migrated.

The upgrade path must tolerate prepared new files beside an old lock without letting ordinary gated commands treat that tree as adopted new state. This adds a small, explicit preflight surface and requires strong no-mutation tests.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Automatically convert domain prose and ADR decisions into claims | Semantic extraction would recreate the false-confidence problem the new model addresses. |
| Keep legacy and topic authority modes selectable | It would preserve obsolete code and let projects remain indefinitely ambiguous. |
| Cut over one domain or topic at a time | Partial cutover requires both authority engines and cross-mode precedence. |
| Drop legacy invariants without reconciliation | Mechanically backed obligations could disappear silently during an apparently successful upgrade. |
| Rewrite every legacy ADR into the new format | Historical records lack truthful State changes transactions and should not be fabricated retroactively. |
| Require manual migration with no readiness tooling | Adopters need deterministic evidence that no enumerable obligation or coverage gap was missed. |
