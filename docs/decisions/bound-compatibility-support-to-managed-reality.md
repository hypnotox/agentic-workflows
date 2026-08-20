---
format: current-state-v4
slug: bound-compatibility-support-to-managed-reality
status: Proposed
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

| Surface | Managed evidence | Inventory-backed disposition |
|---|---|---|
| Installed releases and bootstrap pins | Nine adopters are at 0.39.1 and agentic-workflows is at 0.39.2. | Support current plus one previous release. The present floor is 0.39.1. |
| Live config sources | Every current lock is schema 46. | Keep schema 46 as the live source floor; retire older live migration paths only after the removal gate. |
| Reachable `.awf` history | Managed lock history spans schemas 3 through 46. Pre-`.awf` history exists only as an empty audit universe. | Keep read-only historical config and lock decoding for schemas 3 through 46, including pre-31 lock fields. |
| ADR corpus | Current managed trees contain markerless ADRs and V1, V2, V3, and V4 ADRs. | Keep every represented ADR parser and intrinsic format routing. |
| Plan corpus | Current managed trees contain markerless, plan-v1, and plan-v2 plans. No plan-v2 reference uses a frozen pre-V4 ordinal selector. | Keep represented plan parsers and links; the unrepresented ordinal selector is removable after a final recheck. |
| Active effort residents | Active efforts use resident schema 2 and canonical YAML memory. No active effort uses legacy four-line memory. | Keep current effort, memory, activity, worktree, and archive behavior; legacy memory and schema-1 retirement paths are removable after a final recheck. |
| Bridge and cutover state | No managed tree has a bridge attestation, migration approval, upgrade journal, partial marker, or other cutover residue. | Remove obsolete bridge-specific inputs after the gate; keep the general journaled rollback and recovery safety boundary. |
| Current compatibility inputs | Three current locks omit `initializedWithVersion`; four current configs retain inert punctuation exemption codepoints. | Keep these readers until separately authorized managed cleanup removes the actual inputs. |

All ten pinned binaries pass their own read-only check. Five 0.39.1 adopters are clean,
single-worktree upgrade candidates. Aeonseed, go-php, and jugend-im-zentrum require coordination with
active worktrees; pi-tools has uncommitted tracked changes. These constraints do not lower the floor,
but they prevent compatibility removal from assuming that every prompt upgrade is already complete.

## Decision

1. `decision: rolling-installed-release-floor` Support the current awf release and one previous release for managed installations. At adoption, current is 0.39.2 and the supported floor is 0.39.1. Older binaries are unsupported. Before the rolling floor advances, every managed adopter must be upgraded and verified at or above the new floor.
2. `decision: live-source-schema-floor` Support live source trees from schema 46. Once the managed removal gate is satisfied, a live source below schema 46 is unsupported and must fail clearly rather than traverse retained historical migrations. Keep the migration framework so future current schemas can provide one direct path from the oldest live schema still present in the managed corpus.
3. `decision: actual-managed-history-horizon` Preserve read-only audit decoding for actual reachable managed `.awf` history, presently schemas 3 through 46. Pre-`.awf` history remains an empty audit universe. As managed history gains current schemas, the upper horizon advances with it; the lower bound changes only through a later explicit decision. Audit must clearly refuse schemas below 3, unknown future schemas, malformed inputs, and shapes outside the declared horizon.
4. `decision: represented-authored-formats` Retain compatibility readers for every ADR, plan, lock, config, and active resident format represented in the managed corpus. A format absent from current managed inputs and outside the historical-audit horizon is removable after a final inventory recheck. Do not retain a parser solely for hypothetical external adoption.
5. `decision: managed-removal-gate` Block compatibility removal until a fresh inventory proves every managed adopter is at the applicable installed-release and live-schema floors, all required upgrades and separately authorized cleanups are committed, no active resident or cutover state needs the candidate component, and the component is not required by the historical-audit horizon. Record each component's keep or remove result against that evidence before deletion.
6. `decision: actionable-unsupported-refusal` Unsupported installations, live sources, and historical inputs must fail with the applicable supported floor or horizon and an actionable recovery direction. A cache containing an old binary is not itself a supported installation or a reason to retain compatibility.

## State changes

- add `config/migrations-and-locks:private-compatibility-floor`
- add `config/migrations-and-locks:managed-compatibility-removal-gate`
- add `tooling/audit-and-snapshots:managed-history-decode-horizon`

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

Old installations and live trees below the declared floor lose best-effort operation. Clear refusal
and recovery guidance replaces silent acceptance. Historical audit retains its actual managed range,
but inputs below schema 3 or outside the declared shape are explicitly unsupported.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Support only release 0.39.2 | Nine managed adopters still use 0.39.1, and a one-release recovery window is proportionate. |
| Keep live migrations from every historical schema | Every live managed source is already schema 46, so the old mutation paths protect no current tree. |
| Limit audit to schema 46 | Eight managed repositories have real pre-46 `.awf` history; discarding that range would break an existing audit capability. |
| Retain every existing parser indefinitely | Corpus absence, not hypothetical adoption, is the approved removal boundary. |

## Status history

- 2026-08-20: Proposed
