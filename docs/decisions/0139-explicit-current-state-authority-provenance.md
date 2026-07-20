---
format: current-state-v1
status: Proposed
date: 2026-07-21
---
# ADR-0139: Explicit Current-State Authority Provenance


## Context

The current-state cutover established an attestation-derived ADR format cutoff for migrated
projects, but fresh adoption reused the ordinary sync path. A new lock therefore omitted
`adrFormatV1From`, whose zero value means that every ADR is legacy, while the scaffolded ADR
template required `format: current-state-v1`. The first ADR created in a fresh project could
not pass `awf check`.

Treating every missing cutoff as fresh would be unsafe. A missing cutoff can also identify a
pre-cutover or unattested adopter, and schema migration removes the retired invariant config
without synthesizing topic authority. Ordinary sync likewise cannot distinguish a genuinely
new adoption from deliberate no-lock re-adoption. The lock needs explicit, immutable
provenance recording when the workflow was first adopted.

The same audit found another inference error at the transaction boundary. Static checking
interpreted an absent claim as proof that an Abandoned remove had been applied, even when a
separate later Implemented transaction legitimately removed it. Final absence cannot encode
which historical operation caused a removal; the HEAD-to-index comparison can.

Fresh repositories also have no resolvable `HEAD` before their first commit. That state is a
valid empty baseline for initialization checks, not a malformed repository.

## Decision

1. The lock carries an optional `initializedWithVersion` field containing the exact awf
   semantic version that completed the repository's first successful `awf init`. The field is
   immutable repository provenance: sync, upgrade, and `awf init --force` preserve it unchanged,
   and no command infers or backfills it for an adopter whose lock predates the field.

2. A first adoption is an `awf init` invocation that finds neither an existing awf config nor
   an existing awf lock. It writes `initializedWithVersion` from the running binary and seals
   permanent current-state ADR identity authority in the same init-specific operation. Existing
   `NNNN-*.md` decisions are validated as a closed legacy identity set: `adrFormatV1From` is one
   greater than the highest existing number, or `1` when none exists, and `legacyAdrGaps`
   explicitly lists every absent positive number below the cutoff. Every ADR at or above the
   cutoff uses current-state-v1. Ordinary sync never creates or repairs these authority fields.

3. Brownfield first adoption with an existing valid ADR corpus is supported by item 2 so a
   project can retain its prior decision history. An existing awf config or lock means the
   repository is not a first adoption; `awf init --force` may replace scaffold and managed output
   surfaces there, but it preserves any existing initialization version, permanent ADR cutoff,
   and legacy gap set. An absent initialization version remains absent, and missing permanent
   authority is refused rather than invented.

4. Lock validation and upgrade routing use these mutually exclusive states:
   - **Bridge:** `bridgeAttestation` is present, `adrFormatV1From` is zero, and
     `initializedWithVersion` is absent. Only the sealed final-cutover upgrade may mutate it.
   - **Permanent:** `bridgeAttestation` is absent, `adrFormatV1From` is positive, and the explicit
     sorted, unique `legacyAdrGaps` contains only positive numbers below the cutoff. This is the
     ordinary operating state. `initializedWithVersion` may be absent for a migrated older
     adopter or present for a first adoption made after provenance tracking began.
   - **Pre-tracking:** attestation and permanent cutoff are absent and initialization provenance
     is absent. An existing awf config or lock in this state is legacy or unattested; the current
     binary refuses mutation and directs the adopter to the bridge/recovery path.
   - **Invalid:** every other combination, including attestation mixed with permanent or init
     authority, initialization provenance without a permanent cutoff, gaps without a cutoff, or
     malformed cutoff/gaps. Every mutating or authority-consuming command refuses it with
     restoration or recovery guidance.

5. A present initialization version is valid semantic-version syntax, is not later than the
   lock's current `awfVersion`, and cannot change across a HEAD-to-index transaction. Missing
   provenance remains the supported encoding for adopters initialized before this field existed.
   A missing lock beside an existing awf config is Pre-tracking rather than permission to create
   new authority.

6. The lock model, canonical parser and serializer, init command, project initialization and sync
   paths, binary/version gate, staged lock-authority comparison, upgrade router, snapshot loaders,
   and recovery tests consume this state model. Older locks remain readable because the field is
   optional. A new lock retains binary downgrade protection through `awfVersion`, so an older
   binary cannot successfully sync and erase the unknown field. No config-schema migration
   backfills initial-version provenance.

7. In a Git repository whose `HEAD` is specifically unborn, working-state assembly uses an
   empty committed baseline and continues to include eligible working files. Other reference,
   object, or repository failures remain errors.

8. Static current-state checking does not use final claim absence to attribute an Abandoned
   remove. Claim removals are attributed by the snapshot-pair transaction check, which already
   requires every mutation to match exactly one ADR becoming Implemented. Static checking keeps
   the provenance-based rejection of Abandoned adds and updates.

9. In the Implemented transaction, all four declared claims are authored with `Origin: ADR-0139`,
   `Backing: test`, and matching proof markers on focused integration or snapshot tests. Those
   tests cover initialization provenance, brownfield cutoff/gaps, empty fresh cutoff, unborn
   HEAD, refusal before mutation, immutable lock authority, and removal attribution.

10. The rendered lifecycle and implementation workflows explicitly run `awf check --staged`
    after staging and before the project gate. The same commit updates their authored template
    and agent-guide sources, regenerates every enabled runtime artifact and `AGENTS.md` with
    `./x sync`, and preserves missingkey-zero publication safety when command variables are empty.
    Hook execution remains a second enforcement layer rather than the only transaction check.

11. Every ADR-0139 status transition runs `./x sync` and commits the regenerated
    `docs/decisions/INDEX.md` in the same transaction. Its Implemented transition also applies all
    four State changes and their proof markers atomically.

## State changes

- add `tooling/upgrade-runtime:initial-adoption-version-immutable`
- add `adr-system/adr-lifecycle:fresh-adoption-v1-cutoff`
- add `invariants/current-state-authority:abandoned-remove-pair-attributed`
- add `tooling/cli:init-unborn-head-supported`

## Consequences

Fresh adopters can create a first v1 ADR immediately, and future awf versions can tell when a
repository first adopted the workflow without treating the most recent sync version as origin
provenance. Older migrated adopters remain valid without fabricated history. Unsafe legacy and
unattested states fail before mutation instead of being guessed from an absent field.

The lock gains another immutable authority field and init needs a distinct seeding path rather
than being only config scaffolding followed by ordinary sync. Forced initialization becomes less
absolute: it can rebuild managed surfaces but cannot reset repository identity. A project created
by the defective cutoff-zero implementation cannot be silently repaired because its origin is
ambiguous; it receives explicit recovery guidance.

Static checking intentionally reports less about an Abandoned remove from a final tree alone.
Atomic staged checking and range audit provide the stronger temporal evidence, so agents must run
the staged check explicitly even where Git hook activation is absent.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Infer freshness from a missing cutoff or empty ADR directory | The same shape can belong to a legacy, unattested, or deliberately re-adopted project. |
| Store the initial version in authored config | Repository origin is runtime lock provenance, not adopter-authored workflow policy, and config edits would make it mutable. |
| Put adoption provenance in a separate file | A second authority file would complicate atomic writes, staged comparison, recovery, and project-state gating without adding information. |
| Keep rejecting every absent claim for an Abandoned remove | Final absence cannot distinguish that operation from a separately checked later Implemented removal. |

## Status history

- 2026-07-21: Proposed
