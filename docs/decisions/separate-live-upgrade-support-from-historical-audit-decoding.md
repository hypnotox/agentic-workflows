---
format: current-state-v4
slug: separate-live-upgrade-support-from-historical-audit-decoding
status: Proposed
date: 2026-08-25
---
# ADR-separate-live-upgrade-support-from-historical-audit-decoding: Separate Live Upgrade Support from Historical Audit Decoding


## Context

ADR-0297 bounds compatibility to owner-managed reality. Every managed installation now uses release
0.39.2 and every live source uses schema 46, so the managed upgrade precondition is satisfied. The
remaining no-dependency gate is candidate-specific: represented authored formats, active resident
formats, and reachable history still require some old readers even though no live source may upgrade
from an old schema.

The implementation currently blurs that distinction. Live upgrade classification and execution use
the same migration generation model that supplies read-only config forward decoding to audit and
staged checks. The live registry still exposes legacy layouts and filesystem mutations from schemas
1 through 45. The permanent manifest value also carries bridge authority fields used by a cutover
path that no managed tree retains. This coupling can either preserve an accidental live route below
the supported floor or break reachable historical audit when obsolete mutation code is removed.

Bridge lock compatibility crosses several live consumers. The manifest model parses and validates
bridge authority. Command guarding, initialization, and publication consult that authority before
config loading or rendering. Upgrade dispatch verifies and finalizes the bridge, while migration
layout and generation classification route old projects toward upgrade. Release checking carries a
permanently true bridge sentinel. Audit also parses historical locks through the live manifest
model. The final census therefore permits live manifest, command, initialization, publication,
upgrade, migration, and release-check bridge branches to disappear, but any bridge shape present in
reachable managed history must move behind audit's historical decoder rather than remain live.

Historical decoding and live mutation have different owners and constraints. Live operations need
strict current authority, a future migration seam from the supported floor, and journaled atomic
mutation. Audit needs read-only interpretation of the actual managed history horizon, including old
config and lock shapes, without making those shapes valid live inputs. Staged checks operate on live
project state rather than historical audit state. A genuinely empty pre-adoption HEAD is the only
staged case with no live schema to require.

The managed corpus contains no plan-v2 ordinal Decision reference, active legacy four-line effort
memory, schema-1 effort resident, or bridge and cutover residue. Their removal still requires one
final candidate-specific recheck, including reachable history where a removed live representation
may remain historical evidence. Current authored formats, schema-2 effort residents, generic upgrade
recovery, missing initialization provenance, and inert punctuation inputs remain required or
explicitly deferred.

## Decision

1. `decision: separate-live-and-historical-compatibility` Live project operations and historical audit use separate compatibility boundaries. The current layout at schema 46 through the binary's current schema is classified as supported live source. Only upgrade may execute a required supported migration; every other live operation gates before authority dispatch while the source is below the binary's current schema. An older schema or retired layout is rejected before authority dispatch or mutation with the supported floor and recovery direction, and no live operation calls the historical decoder. Staged checks apply the live floor to both compared project states, except that a genuinely empty pre-adoption HEAD remains an empty universe.
2. `decision: retain-supported-floor-migration-seam` Live upgrade retains one ordered migration seam beginning at the supported schema-46 floor, schema-ahead and binary-version checks, atomic lock publication, and journaled rollback and recovery. Mutation steps, layout readers, and reset paths below the live floor do not remain registered as hypothetical upgrade support.
3. `decision: explicit-managed-history-decoder` Audit owns read-only config and lock decoding for the actual managed history horizon. Its lower bound remains schema 3, and an explicit tracked upper bound begins at schema 46 and advances only after managed-corpus evidence proves that a newer binary-supported schema entered reachable history. A genuinely pre-`.awf` revision remains an empty audit universe. Where historical `.awf` authority is expected, audit refuses a missing, malformed, future, below-horizon, or otherwise unsupported shape with the horizon and recovery direction.
4. `decision: remove-unrepresented-compatibility` After the final candidate-specific managed-corpus recheck, remove plan-v2 ordinal Decision selectors, legacy four-line effort memory support, schema-1 effort and worktree retirement paths, and bridge-only attestation, approval, adjudication, marker-cleanup, and release-sentinel support. Preserve represented ADR and plan formats, canonical schema-2 effort residents, generic journal safety, and historical forms required inside the audit horizon.

## State changes

- remove `config/migrations-and-locks:pitfall-corpus-migration`
- remove `config/migrations-and-locks:selection-keys-dropped`
- remove `config/migrations-and-locks:sidecar-local-field-dropped`
- update `config/migrations-and-locks:retired-keys-forward-ported`
- remove `config/migrations-and-locks:toggle-keys-dropped`
- remove `config/migrations-and-locks:grounding-skill-backfill`
- remove `config/migrations-and-locks:audit-migration-announces-removal`
- remove `config/migrations-and-locks:awf-relocation-migration`
- remove `config/migrations-and-locks:claim-budget-key-dropped`
- remove `config/migrations-and-locks:close-enabled-set-migration`
- remove `config/migrations-and-locks:hooks-config-dropped`
- remove `config/migrations-and-locks:list-replacement-fixed-snapshot`
- remove `config/migrations-and-locks:legacy-read-isolation`
- update `config/migrations-and-locks:lock-atomic-save`
- update `config/migrations-and-locks:migration-ordering`
- remove `config/migrations-and-locks:noop-autobump`
- remove `config/migrations-and-locks:orienting-skill-backfill`
- remove `config/migrations-and-locks:retired-plan-resync-selection-migration`
- remove `config/migrations-and-locks:archive-root-upgrade-boundary`
- update `config/migrations-and-locks:schema-min-version`
- update `config/migrations-and-locks:schema-version-lock`
- remove `config/migrations-and-locks:severity-keys-dropped`
- remove `config/migrations-and-locks:singleton-doc-migration-relocates-parts`
- remove `config/migrations-and-locks:structural-heading-part-migration`
- remove `config/migrations-and-locks:unified-effort-resident-migration`
- update `config/migrations-and-locks:upgrade-gate`
- remove `config/migrations-and-locks:upgrade-migrates-retirements`
- remove `config/migrations-and-locks:upgrade-migrates-supersession-keys`
- remove `config/migrations-and-locks:workflow-telemetry-config-migration`
- remove `config/migrations-and-locks:profile-full-migration`
- update `config/migrations-and-locks:live-source-compatibility-floor`
- update `config/configuration:awf-config-root`
- update `adr-system/adr-lifecycle:corpus-raw-access-enumerated`
- update `adr-system/plan-artifacts:plan-v2-decision-references`
- update `adr-system/plan-artifacts:managed-plan-format-support`
- update `rendering/pi-runtime:pi-session-handoff-workflow`
- update `rendering/singletons-and-payloads:resident-output-preservation`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- remove `code-design/dependency-composition:upgrade-attestation-filesystem-wiring`
- update `tooling/audit-and-snapshots:managed-history-decode-horizon`
- update `tooling/cli:group-child-project-guard-exemption`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `tooling/effort-management:managed-effort-format-support`
- remove `tooling/upgrade-runtime:current-state-cutover-is-atomic`
- remove `tooling/upgrade-runtime:every-live-legacy-invariant-adjudicated`
- remove `tooling/upgrade-runtime:bridge-attestation-cutoff-payload-discarded`
- remove `tooling/upgrade-runtime:migration-approval-artifact-is-ephemeral`
- remove `tooling/upgrade-runtime:migration-does-not-infer-authority`
- remove `tooling/upgrade-runtime:migration-never-authors-claims`
- remove `tooling/upgrade-runtime:no-unqualified-markers-after-upgrade`
- remove `tooling/upgrade-runtime:upgrade-requires-complete-current-state`
- update `tooling/upgrade-runtime:managed-cutover-format-support`

## Consequences

Unsupported live sources stop at one explicit boundary instead of traversing code retained for
history. A future supported schema advance still has an ordered and recoverable mutation route, but
adding it cannot silently widen the historical horizon. Staged validation no longer acts as a second
historical decoder.

Audit carries a distinct compatibility burden and must validate historical config and lock shapes
without importing live upgrade policy. The explicit upper-bound value needs evidence-backed upkeep
when a newer schema enters reachable managed history. Historical format activation data remains even
when the corresponding live mutation disappears.

Final managed-corpus proofs become deletion preconditions rather than reasons to keep broad
compatibility indefinitely. Removed behavior travels with obsolete tests, fixtures, templates, and
current documentation only after replacement refusal, historical-horizon, represented-format, and
recovery coverage exists. Factual ADR, plan, and changelog history remains unchanged. Missing lock
initialization provenance, inert punctuation inputs, `project.Open`, and broad historical-comment
cleanup remain outside this decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one registry and guard its callers | The old mutation graph would remain reachable and continue coupling live support to historical parsing. |
| Delete all migration and old lock decoding | Reachable managed history requires schemas 3 through 46 and pre-31 lock routing shapes. |
| Derive the audit upper bound from the current live schema | A schema would become accepted history before evidence shows that it entered reachable managed history. |
| Continue staged forward decoding below schema 46 | Staged checks are live-source validation and would preserve a non-audit compatibility route. |
| Retain every parser for hypothetical adopters | ADR-0297 makes represented managed inputs and reachable managed history the retention boundary. |

## Status history

- 2026-08-25: Proposed
