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

1. Migration spans two releases. The immediately preceding bridge release keeps the old authority runtime and adds only the preparation, inventory, normalization, and attestation machinery. Adopters on an older schema must upgrade to that bridge release first. The following current-state release accepts only an attested bridge lock and ships solely the topic-based context, invariant, ADR, audit, scaffolding, lifecycle, index, and current-guidance engines. It contains no legacy ADR-derived context fallback, supersession authority graph, tag-tier expansion, Implemented-ADR invariant checker, or bridge inventory code.

2. In the bridge release, `awf new topic` scaffolds topic metadata and an empty authored part without running sync, `awf upgrade --check` parses the prepared working tree and reports readiness without writes, `awf upgrade --attest-current-state` performs normalization and records readiness after validation, and `awf upgrade --recover` handles an interrupted attestation transaction. Every other new-schema project command refuses while the bridge lock remains. These migration-safe paths never render inferred claim prose, answer context through a second authority engine, or mark the project as operating under current-state authority.

3. The adopter manually authors every topic sidecar, topic claim, provenance link, and scope. Existing domain current-state prose, agent guidance, code, tests, and ADRs may be consulted, but no migration command extracts or promotes their prose. `awf new topic` creates an empty topic shell and metadata scaffold only. Empty topics do not satisfy coverage.

4. Bridge preflight builds a legacy invariant inventory from declarations on legacy Implemented or Superseded ADRs, then subtracts only machine-readable retirements effective under the legacy activation rules before any status normalization. Inactive or lapsed tokens remain historical but do not retire an obligation unless an effective successor re-carries them. Existing effective retirement tokens are normalized into an append-only `## Migration history` section on the carrier ADR with entries of the exact form ``- YYYY-MM-DD: retired invariant `ADR-NNNN#<slug>`; basis: encoded` ``; the date is the carrier's frontmatter date and the existing Decision item owns rationale. A deliberate migration-time retirement uses ``- YYYY-MM-DD: retired invariant `ADR-NNNN#<slug>`; basis: migration; rationale: <nonempty text>`` on the ADR recording that retirement decision, again using its frontmatter date. Unknown bases, duplicate declaration anchors, an encoded entry without a matching token, and a migration entry without rationale fail. This is a meaning-preserving bookkeeping retrofit, not an edit to historical rationale.

5. Every inventory slug not explicitly retired must appear as the local slug of exactly one current-state invariant claim, in whichever topic the adopter judges correct. The migrated claim cites the declaring legacy ADR as Origin and preserves backed versus unbacked classification unless a separately reviewed decision changes or retires the contract. Every proof marker under configured `currentState.sources` is rewritten to the resulting qualified ID; every advisory touches marker in that scan universe is rewritten to `touches-state:` with a nonempty note. No unqualified marker site may remain there. Frozen legacy invariant declarations and relation tokens may remain in historical ADR text and are ignored after cutover.

6. The bridge maps an enabled legacy `invariants` config mechanically: each source entry preserves `globs`, `marker`, and optional `close`; `testGlobs` is copied; `currentState.topicCoverage`, `topicFanout`, and `maxTopicsPerPath` receive `error`, `warn`, and 8; and the old key is removed in the attested result. `invariants.disabled: true` has no authority-disabling equivalent and is not converted: readiness refuses until the adopter removes it and authors an explicit `currentState` configuration, which may omit sources only when no markers or test-backed claims exist.

7. Preflight refuses unless every configured domain uses a canonical kebab key; every domain-owned eligible path has scoped topic coverage under `currentState.topicCoverage: error`; topic parsing, claim references, invariant backing, marker resolution, output planning, and render completeness pass over the proposed new result; and the proposed generated tree contains no stale legacy ACTIVE.md or domain ADR index output. `awf upgrade --check` runs these in-memory static predicates over the working tree. Attestation additionally requires a clean Git tree and records its HEAD plus a digest of every migration-relevant authored input. Migration guidance requires the adopter's configured test and full gate commands to pass before attestation; those project-owned commands are not misrepresented as in-memory awf validation.

8. Every legacy `Superseded` ADR is normalized to `Implemented`, because the new status means that the decision was incorporated historically rather than that its rule remains active. Every legacy Proposed or Accepted ADR must become Implemented or Abandoned before attestation. The awf repository's authority-model ADRs, including this record, become Implemented after their resulting topic claims exist. No legacy ADR is retrofitted with State changes.

9. The bridge attestation records `adrFormatV1From` as one greater than the highest existing ADR number and records every absent lower number in `legacyAdrGaps`. Together they close the migration-time identity set: a listed gap can never be backfilled as legacy, and every ADR at or above the cutoff requires new format. The attestation also records the prepared HEAD and relevant-tree digest. The current-state release refuses if either changed. New scaffolding then uses `format: current-state-v1` and the settled cutoff, frontmatter, and section contracts.

10. Attestation and final upgrade use a project-wide journal. They precompute and validate the complete output plan, snapshot every changed or deleted path with its mode and prior absence, and prepare replacements before destination changes. The lock is replaced last as the commit point. A pre-commit failure restores the journal; if rollback fails, the journal remains, every project command refuses, and `awf upgrade --recover` resumes restoration. A cleanup failure after lock commit leaves a successful upgrade plus a recoverable stale journal rather than rolling authority back. Success prunes ACTIVE.md and per-domain ADR indexes, creates INDEX.md and topic indexes, and leaves all ordinary commands operating solely on the new schema.

11. The bridge release validates the final old-HEAD/prepared-tree facts and seals them in the attestation. The current-state release verifies that attestation without shipping the migration-only cross-schema adapter; ordinary static and staged checks have no legacy branch.

12. The implementation deletes legacy authority consumers from ordinary audit, ADR validation and index generation, scaffolding, lifecycle tooling, context, and invariant reporting, not only from context assembly. A source-level denylist and production import-boundary test pin removal of the supersession graph, tag-tier expansion, Superseded-state derivation, and Implemented-ADR invariant authority; the dead-code gate prevents stranded production remnants.

13. Migration documentation presents a human-owned checklist and exact diagnostics rather than an automatic conversion promise. awf and `examples/sundial/` complete the same cutover, including config, lock, authored topics, markers, and rendered outputs, before release. The same-cutover changes cover the `.awf/` sources for AGENTS.md, architecture, config, upgrade, workflow, and release guidance; `docs/releasing.md`; generated command documentation; and the breaking changelog entry. Both topic and ADR scaffolds use the publication-safe rendering path and are tested with empty or unset values. Rollback after a successful upgrade is Git restoration plus reinstalling the bridge release; awf provides no mixed-mode downgrade.

## Invariants

- `unbacked-invariant: upgrade-requires-complete-current-state`: The bridge cannot attest and the new schema lock cannot be written until every awf-owned readiness predicate succeeds; configured tests and the full project gate remain mandatory workflow prerequisites for committing the attestation rather than facts stored in the lock. **Verify:** fail each internal readiness predicate independently in bridge fixtures, invalidate the clean-HEAD attestation, and confirm the current-state release refuses every mechanically unready case; inspect the rendered migration workflow and pre-commit payload for the configured test and gate steps.
- `unbacked-invariant: migration-never-authors-claims`: Migration commands scaffold empty structure and report obligations but never generate normative claim prose or provenance. **Verify:** run topic scaffolding and upgrade preflight on a legacy fixture and confirm no claim heading, body, Origin, or revision metadata is invented.
- `unbacked-invariant: every-live-legacy-invariant-adjudicated`: Each nonretired legacy invariant slug maps to exactly one current-state invariant with preserved backing classification, and every retired slug has a machine-readable migration-history entry. **Verify:** exercise missing, duplicate, classification-changed, and retired-without-rationale inventories and confirm preflight reports the exact slug.
- `unbacked-invariant: no-unqualified-markers-after-upgrade`: A migrated project's configured current-state source paths contain no active unqualified proof or touches marker, while frozen declarations may remain in historical ADR text outside that authority scan. **Verify:** place each legacy marker form in every configured source language and in one historical ADR; confirm readiness requires source-site rewrites but ignores the historical declaration.
- `unbacked-invariant: upgrade-failure-is-recoverable`: A failed upgrade either restores the pre-transaction bytes and modes or preserves a journal from which recovery completes before any project command may run. **Verify:** inject failures during preparation, rename, prune, lock replacement, rollback, and cleanup; compare recovered tree digests and assert command refusal while a journal remains.
- `unbacked-invariant: upgraded-runtime-has-one-authority-engine`: After the new lock lands, normal context and invariant reporting cannot consume legacy ADR tags, supersession edges, or invariant declarations. **Verify:** retain contradictory legacy metadata in a migrated fixture and confirm only topic claims affect output and enforcement.
- `unbacked-invariant: legacy-format-set-is-closed`: The lock's cutoff plus recorded lower-number gaps exactly close the attested legacy identity set, and every later ADR uses new format. **Verify:** attempt missing-format ADRs at an existing lower identity, a recorded lower gap, the cutoff, and above it; confirm only the attested existing identity is accepted.

## Consequences

Migration demands real project work. Adopters must decide their current claims, split domains into focused topics, and review every invariant. That cost is the point: awf refuses to manufacture semantic confidence from historical prose.

The new release cannot be adopted incrementally in a long-lived mixed project. Teams perform the migration on a branch, keep using the previous release until it is ready, and merge the complete schema transition. This simplifies the shipped runtime and prevents contradictory authority during an extended rollout.

A narrow amount of legacy parsing exists only in the bridge release so it can prove that old obligations were not lost and seal the prepared tree. It is bounded to inventory and format facts and cannot answer a second context or invariant query. The current-state release verifies the sealed attestation and drops that migration code.

Normalizing Superseded to Implemented preserves historical truth under the new semantics and deletes a status whose only purpose was active-currentness inference. Existing supersession prose and tokens may remain frozen historical text, but no checker maintains their graph after cutover.

The migration-history retrofit records invariant retirement without backporting current rules into old ADR rationale. The one-time inventory reads it; afterward current-state claims alone own enforcement. Historical entries remain useful when explaining why an old invariant was not migrated.

The bridge path must tolerate prepared new files beside an old lock without letting ordinary gated commands treat that tree as adopted new state. The readiness attestation, journaled writes, and following release boundary add explicit surfaces and require strong no-mutation, recovery, and invalidation tests.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Automatically convert domain prose and ADR decisions into claims | Semantic extraction would recreate the false-confidence problem the new model addresses. |
| Keep legacy and topic authority modes selectable | It would preserve obsolete code and let projects remain indefinitely ambiguous. |
| Cut over one domain or topic at a time | Partial cutover requires both authority engines and cross-mode precedence. |
| Drop legacy invariants without reconciliation | Mechanically backed obligations could disappear silently during an apparently successful upgrade. |
| Rewrite every legacy ADR into the new format | Historical records lack truthful State changes transactions and should not be fabricated retroactively. |
| Require manual migration with no readiness tooling | Adopters need deterministic evidence that no enumerable obligation or coverage gap was missed. |
