---
format: current-state-v4
slug: bound-compatibility-support-to-managed-reality
status: Accepted
date: 2026-08-20
---
# ADR-bound-compatibility-support-to-managed-reality: Bound Compatibility Support to Managed Reality


## Context

awf is a private-use tool whose owner controls every managed adopter. Retaining migration,
parser, bridge, and resident compatibility for hypothetical adopters therefore imposes cost without
protecting a real user. Removing compatibility by release number alone would be equally unsound:
the live trees, durable authored corpora, active local residents, and reachable Git history have
different oldest required formats.

The owner confirmed that the complete managed adopter corpus is aeonseed, agentic-workflows, fleet,
go-php, jugend-im-zentrum, nouris, pi-science, pi-tools, remote_pi, and sudoku-solver. A read-only
inventory at this decision boundary found:

| Surface | Managed evidence |
|---|---|
| Installed releases and bootstrap pins | Nine adopters are at 0.39.1 and agentic-workflows is at 0.39.2 without a bootstrap pin. |
| Live config sources | Every current lock is schema 46. |
| Reachable `.awf` history | Managed lock history spans schemas 3 through 46. Pre-`.awf` history exists only as an empty audit universe. |
| ADR corpus | Current managed trees contain markerless ADRs and V1, V2, V3, and V4 ADRs. |
| Plan corpus | Current managed trees contain markerless, plan-v1, and plan-v2 plans. No plan-v2 reference uses a frozen pre-V4 ordinal selector. |
| Active effort residents | Active efforts use resident schema 2 and canonical YAML memory. No active effort uses legacy four-line memory. |
| Bridge and cutover state | No managed tree has a bridge attestation, migration approval, upgrade journal, partial marker, or other cutover residue. |
| Current compatibility inputs | Three current locks omit `initializedWithVersion`; four current configs retain inert punctuation exemption codepoints. |

The component-level inventory gives every compatibility surface a disposition. "Remove after gate"
is authorization for the later RF-008B or RF-014B lane, not removal by this decision:

| Compatibility component | Evidence | Disposition |
|---|---|---|
| Strict current config, schema-46 lock, and unknown-field decoding | Every live source is schema 46. | Keep. |
| Migration registry, ordered execution, atomic lock publication, and schema-ahead checks | Future supported schema transitions still require a safe migration boundary. | Keep the framework. |
| Legacy single-file and `.claude/awf/` layout readers and relocation | No live tree uses either layout; only agentic-workflows history contains them. | Remove after gate from live upgrade paths. |
| Schema 1 through 45 mutation steps, including retired config, catalog, telemetry, pitfall, singleton, and ADR-metadata conversions | No live managed source is below schema 46. | Remove after gate. |
| Historical config forward-porting for schemas 3 through 45 | Actual reachable managed audit history uses these schemas. | Keep read-only. |
| Pre-31 lock routing-field decoding | Actual reachable managed audit history includes pre-31 locks. | Keep read-only. |
| Current lock parsing without `initializedWithVersion` | Three current managed locks omit it. | Defer removal until authorized normalization removes those inputs. |
| Inert punctuation exemption codepoints | Four current managed configs contain them. | Defer removal until authorized config cleanup removes those inputs. |
| Markerless and V1 through V3 ADR parsers | Current managed ADR files use every format. | Keep. |
| V4 ADR parser, intrinsic marker routing, pending identity, numbering, and retained slug links | This is the current authored and integration format. | Keep. |
| Markerless and plan-v1 parsers, numeric links, and retained-slug links | Current managed plan files use these formats and links. | Keep. |
| Plan-v2 parser and stable Decision selectors | This is the current authored plan format. | Keep. |
| Frozen pre-V4 ordinal Decision selector | No managed plan-v2 reference uses it. | Remove after a final gate recheck. |
| Effort resident schema 2, canonical YAML memory, activity protocol 2, worktree topology, and archive move | These are current active lifecycle formats. | Keep. |
| Legacy four-line effort memory reader and structured-update conversion | No active managed effort uses it. | Remove after a final gate recheck. |
| Schema-1 effort, worktree, standalone-memory, and partial-evidence retirement classifiers | Every managed tree crossed the transition and no residue remains. | Remove after gate. |
| Bridge attestation v1 and current-state cutover approval, adjudication, and marker-cleanup inputs | No managed tree retains bridge or cutover state. | Remove after gate. |
| Journaled upgrade commit point, rollback, and recovery | This is current mutation safety, not old-source compatibility. | Keep. |
| Bootstrap pinning, checksum verification, upgrade override, and binary/schema checks | Managed installations still need deterministic supported binaries. | Keep. |
| Cached historical binaries | Caches are not managed pins or installations. | Ignore as support evidence. |

The version-matched binary tested for each adopter passes its read-only check. Five 0.39.1 adopters
are clean, single-worktree upgrade candidates. Aeonseed, go-php, and jugend-im-zentrum require
coordination with active worktrees; pi-tools has uncommitted tracked changes. These constraints do
not lower the floor, but they prevent compatibility removal from assuming that every prompt upgrade
is already complete.

## Decision

1. `decision: rolling-installed-release-floor` Support the current awf release and one previous release for managed installations. At adoption, current is 0.39.2 and the supported floor is 0.39.1. Older binaries are unsupported. The floor does not advance until every managed adopter pin is upgraded to remain at or above it.
2. `decision: live-source-schema-floor` Support live source trees from schema 46. Once the managed removal gate is satisfied, a live source below schema 46 is unsupported and must fail clearly with the supported floor and recovery direction rather than traverse retained historical migrations.
3. `decision: actual-managed-history-horizon` Preserve read-only audit decoding for actual reachable managed `.awf` history. Its lower bound is schema 3; its upper bound is the highest schema supported by the current binary that has actually entered reachable managed history, presently schema 46. Pre-`.awf` history remains an empty audit universe. Audit must clearly refuse schemas below 3, unknown future schemas, malformed inputs, and shapes outside the declared horizon with the supported horizon and recovery direction.
4. `decision: represented-authored-formats` Retain compatibility readers for every ADR, plan, lock, config, and active resident format represented in the managed corpus. A format absent from current managed inputs and outside the historical-audit horizon is removable after a final inventory recheck. Do not retain a parser solely for hypothetical external adoption.
5. `decision: managed-removal-gate` Block compatibility removal while any managed adopter pin or live source is below the applicable floor, or while any current managed input, active resident, cutover state, or reachable managed history inside the audit horizon requires the candidate component. Floor advancement does not itself authorize deletion.
6. `decision: unsupported-boundaries` Document installed releases below the rolling floor as unsupported. After the removal gate is satisfied, make below-floor live sources refuse as stated above. Make historical inputs outside the audit horizon refuse as stated above.

## State changes

- add `config/migrations-and-locks:live-source-compatibility-floor`
- add `adr-system/adr-lifecycle:managed-adr-format-support`
- add `adr-system/plan-artifacts:managed-plan-format-support`
- add `tooling/audit-and-snapshots:managed-history-decode-horizon`
- add `tooling/effort-management:managed-effort-format-support`
- add `tooling/upgrade-runtime:installed-release-compatibility-floor`
- add `tooling/upgrade-runtime:managed-compatibility-removal-gate`
- add `tooling/upgrade-runtime:managed-cutover-format-support`

## Consequences

The live compatibility burden becomes proportional to repositories the owner actually controls.
RF-008B may remove the old live migration stack, legacy resident reset paths, bridge-only inputs,
and other unrepresented compatibility only after its managed gate proves each deletion safe.
RF-014B remains conditional on that work.

Old ADR and plan parsers remain because old authored files are present in current managed trees.
Historical config and lock forward decoding also remains because it protects real reachable audit
history. This is intentional retained compatibility, not residue justified by hypothetical users.

A rolling two-release installation window requires prompt managed upgrades before each floor advance.
Some current inputs, including missing lock initialization provenance and inert punctuation exemption
codes, continue to block their associated cleanup until separate authorized transactions normalize
them.

Old installations below the declared release floor are unsupported. After the removal gate is
satisfied, live trees below schema 46 refuse clearly instead of receiving best-effort migration.
Historical audit retains its actual managed range, but clearly refuses inputs below schema 3,
unknown future schemas, malformed inputs, and shapes outside the declared horizon.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Support only release 0.39.2 | Nine managed adopters still use 0.39.1, and a one-release recovery window is proportionate. |
| Advance the rolling floor before every managed pin is upgraded | A managed adopter could fall below declared support. |
| Keep live migrations from every historical schema | Every live managed source is already schema 46, so the old mutation paths protect no current tree. |
| Limit audit to schema 46 | Eight managed repositories have real pre-46 `.awf` history; discarding that range would break an existing audit capability. |
| Freeze the audit upper bound permanently at schema 46 | A binary-supported schema that enters reachable managed history should join the horizon without another owner decision. |
| Retain every existing parser indefinitely | Corpus absence, not hypothetical adoption, is the approved removal boundary. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Accepted; content-sha256: a160f70549e92d51922e18bae0e584a8b0f3c0da006e2ebbc28dc39f0b0dc603
