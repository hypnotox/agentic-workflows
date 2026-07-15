---
status: Implemented
date: 2026-06-25
supersedes: []
superseded_by: ""
tags: [schema-migration, upgrade-flow]
related: [9]
domains: [config]
---
# ADR-0010: Schema-Versioned Lock and the `awf upgrade` Migration Mechanism

## Context

ADR-0009 changes the on-disk config layout from a single `.claude/awf.yaml` to a
tree under `.claude/awf/` (skeleton `config.yaml`, per-target sidecars,
convention parts, relocated `.claude/awf/awf.lock`). That ADR is implementable on
its own by **hand-porting** this repo's config, but it leaves every existing
adopter (and any future breaking config-format change) without an ergonomic,
safe migration path. A downstream project that upgrades its `awf` binary past the
layout change would silently fail to load (`config.Load` reads the new path; the
old file is invisible) with no guidance. This ADR adds the migration mechanism:
a schema-versioned lock, a gate that refuses to operate against a stale layout,
and an `awf upgrade` command that applies registered migrations.

Grounding discoveries that shape the design (verified against source unless noted):

- **The lock is a small JSON document with a tool-version field.** `manifest.Lock`
  is `{ AWFVersion string; Files map[string]Entry }` (`internal/manifest/manifest.go:19-22`),
  round-tripped by `Load`/`Save` via `encoding/json`. Adding an integer field is
  backward-compatible: an older lock without it unmarshals to the zero value `0`.
- **`AWFVersion` is the tool version, written on every sync.** `Sync` builds
  `&manifest.Lock{AWFVersion: Version, ...}` (`internal/project/project.go:363`) where
  `const Version = "0.1.0"` (`project.go:23`). It is a release string, not a
  config-format version; coupling format migrations to it would force a tool
  release for every format change and vice-versa.
- **`Sync` writes the lock; `Check` reads it.** `Sync` (`project.go:345-394`)
  regenerates and saves the lock at `p.lockPath()`; `Check` (`project.go:408-475`)
  loads it and diffs hashes. Both run through `project.Open(root)` →
  `config.Load(<new path>)`, which (post-ADR-0009) reads `.claude/awf/config.yaml`
  and therefore **cannot open a legacy single-file project at all**: the failure
  happens before any version logic could run. A migration gate must act *before*
  the normal open/render path, keyed on layout/lock state rather than on a
  successfully-loaded config.
- **Subcommands dispatch through a flat switch** in `cmd/awf/main.go` (`init`,
  `sync`, `check`, `invariants`, `list`, `add`, `setup`; `default` errors on
  unknown). Adding `upgrade` is a new `case` plus a `runUpgrade` handler, mirroring
  the others. `runInit` (`main.go:46`) scaffolds and syncs; `runSync` opens the
  project and calls `Sync`.
- **The pre-commit hook runs the gate.** Per ADR-0008/AGENTS.md the hook runs
  `awf check` (`./x check`), so any hard failure surfaced by `check` already blocks
  commits: the natural home for a "run `awf upgrade`" gate.
- **ADR-0009 `inv: config-root`** asserts "no code path reads or writes
  `.claude/awf.yaml` or `.claude/awf.lock`." A migration that transforms the legacy
  file necessarily reads it; this ADR must carve a narrow, explicit exemption rather
  than contradict that invariant silently.

**User constraints driving the design (verbatim intent):** "I would make an 'awf
upgrade' command and a gate that fires until it was done, with tracking of the
current version in the lock file so it knows when the awf version was upgraded. If no
changes are required, it can automatically upgrade the lock file version with the next
sync." Plus the design-pass selections: a **dedicated schema version** (separate from
the tool version), an **ordered migration registry** (not a one-off), and **both
`check` and `sync` hard-fail** when behind while a no-op gap auto-bumps on sync.

## Decision

1. **The lock gains an integer `schemaVersion`, distinct from `AWFVersion`.**
   `manifest.Lock` adds `SchemaVersion int` (`json:"schemaVersion"`). It records the
   config-layout/format generation the lock was written at; `AWFVersion` stays the
   tool release string and the two move independently. An absent/zero `schemaVersion`
   (an older or legacy lock) means "pre-tree" generation `0`. The single source of
   truth for "current" is the migration registry (item 2): the current schema version
   is the highest `To` among registered migrations, so registering a migration is what
   bumps it: no separate hand-maintained constant to drift. `Sync` stamps the current
   schema version into every lock it writes.

2. **An ordered migration registry under a new `internal/migrate` package.** A
   migration is `{ To int; Name string; Apply(root string) error }`; the registry is
   an ascending-`To` ordered slice. `awf upgrade` computes the project's current
   generation (the lock's `schemaVersion`, or `0` when the legacy layout is detected;
   item 3) and runs every migration with `To > current` in ascending order, then
   writes the lock at the highest applied `To`. Each `Apply` is idempotent: re-running
   `awf upgrade` on an already-current project applies nothing and exits zero. The
   first registered migration is `{ To: 1, Name: "tree-layout" }`, performing the
   ADR-0009 transformation (`.claude/awf.yaml` → `.claude/awf/config.yaml` + per-kind
   sidecars + extracted parts, including agents-doc prose → `.claude/awf/parts/agents-doc/`,
   and relocating the lock to `.claude/awf/awf.lock`).

3. **The migration reads the legacy config through a frozen legacy reader, the sole
   exemption to ADR-0009 `inv: config-root`.** The `tree-layout` migration parses
   `.claude/awf.yaml` with a snapshot of the pre-ADR-0009 config shape carried inside
   `internal/migrate` (not via `config.Load`, which now reads the new path). Legacy
   layout is detected by `.claude/awf.yaml` existing while `.claude/awf/config.yaml`
   does not. This legacy read is performed **only** by the migrate registry, **only**
   under `awf upgrade`, never by any render/sync/check load path. ADR-0009's
   `inv: config-root` was originally worded absolutely ("no code path reads or writes
   `.claude/awf.yaml`"); rather than leave its words at odds with its enforced meaning,
   ADR-0009 was reopened (`Accepted → Proposed`) and its Decision 1 and `inv: config-root`
   reworded to bake in this exemption ("no *normal load/render/sync/check* path reads the
   legacy path, with `internal/migrate` under `awf upgrade` as the single named exception"),
   then re-accepted. ADR-0009 and this ADR cross-reference via `related`, and ADR-0009's
   `inv: config-root` and this ADR's `inv: legacy-read-isolation` co-own the shared backing
   test that permits exactly the migrate package.

4. **The gate lives in `runSync`/`runCheck` (cmd/awf), ahead of `project.Open`, and
   hard-fails when the schema is behind.** The check **cannot** live in `project.Sync`/
   `project.Check`: those are methods on an already-opened `*Project`, and `project.Open`
   (`project.go:38-39`) calls `config.Load` on the post-ADR-0009 path and so cannot open
   a legacy single-file project: by the time `Sync`/`Check` run, a stale-layout project
   has already failed to open. The gate therefore runs in the `cmd/awf` handlers
   (`runSync`, `runCheck`) **before** `project.Open`, keyed on filesystem/lock state. Each
   handler determines the project's **effective generation**: `0` if the legacy layout is
   detected (`.claude/awf.yaml` exists and `.claude/awf/config.yaml` does not), else the
   loaded lock's `schemaVersion`. **Gate predicate:** if the effective generation is below
   current **and at least one** registered migration has a `To` in the open interval
   `(generation, current]`, the command exits non-zero with a message naming the gap and
   instructing `run awf upgrade`. Because the pre-commit hook runs `awf check`, a stale
   layout blocks commits until upgraded. A project already at the current schema (a fresh
   `awf init`, or a just-upgraded project) never gates.

5. **A schema gap covered by *no* registered migration auto-bumps on sync.** If the
   effective generation is below current but **no** registered migration has a `To` in the
   open interval `(generation, current]` (i.e. the whole gap is reserved version numbers
   with no file transformation), `sync` does not gate: it writes the lock at the current
   schema version (Decision 1 already has `Sync` stamp the current version into every lock
   it writes, so the bump is implicit once the gate passes). Auto-bump presupposes the new
   layout already loads: a legacy-layout project (generation `0`) always has the
   `tree-layout` migration (`To: 1`) in its gap, so it gates and never auto-bumps; only a
   project whose tree already loads but whose lock records a reserved-but-empty older
   generation auto-bumps. The gate (item 4) and auto-bump are therefore mutually exclusive
   on the same project at the same time: a single predicate ("does any migration cover the
   open interval?") routes to exactly one of gate-or-bump. This keeps format-version bumps
   that need no on-disk change frictionless, per the user's "automatically upgrade the lock
   file version with the next sync." When a gap mixes reserved and transforming versions
   (e.g. migrations for `To:1` and `To:3` exist but not `To:2`, generation `0`), the
   predicate is satisfied by the `To:1`/`To:3` migrations, so the project **gates**;
   `awf upgrade` then runs every covering migration in ascending order and writes the lock
   at `3`, carrying the project past the reserved `To:2` in one pass (`inv: migration-ordering`).

6. **A new `upgrade` subcommand.** `cmd/awf` gains `case "upgrade": runUpgrade(cwd)`.
   `runUpgrade` runs the registry from the detected generation, prints each applied
   migration's name, and is a zero-exit no-op when already current. It does **not** go
   through `project.Open` (which cannot open a legacy project); it operates directly on
   the filesystem via the migrate package, then performs a normal `sync` to write the
   lock and verify the rendered output.

Performing this repo's own ADR-0009 port **by running `awf upgrade`** (rather than the
hand-port ADR-0009's plan allowed for) is the cleanest validation of the mechanism and
is the recommended sequencing for the implementation plan, but it is adopter/dogfood
work, not a Decision item. This ADR earns its own record because it is load-bearing
(new lock field and format-version concept, a new package and command, a new gate in
the sync/check contract, and a refinement of an Accepted ADR's invariant) and a plan
because it is multi-commit.

## Invariants

Checkable contracts that must hold while this decision stands. Tagged slugs are backed
by tests landing with implementation (enforced by `awf check` once this ADR is
`Implemented`; ADR-0008); untagged bullets are textual contracts.

- `invariant: schema-version-lock`: `manifest.Lock` carries an integer `schemaVersion`; a
  lock written by `Sync` has `schemaVersion` equal to the current schema version (the
  highest registered migration `To`), and `AWFVersion` remains an independent tool
  release string.
- `invariant: upgrade-gate`: `awf sync` and `awf check` (in their `cmd/awf` handlers, before
  `project.Open`) exit non-zero with a "run `awf upgrade`" message when the project's
  effective generation (legacy layout → `0`, else `lock.schemaVersion`) is below current
  **and at least one** registered migration has a `To` in the open interval
  `(generation, current]`; a project at the current schema does not gate.
- `invariant: migration-ordering`: `awf upgrade` applies exactly the registered migrations
  with `To` greater than the detected generation, in ascending `To` order, and is
  idempotent: re-running at the current schema applies nothing and exits zero.
- `invariant: legacy-read-isolation`: The legacy `.claude/awf.yaml` is read only by the
  `internal/migrate` registry under `awf upgrade`; no `config.Load`/render/`Sync`/`Check`
  path reads it (the named exemption to ADR-0009 `config-root`).
- `invariant: noop-autobump`: When the effective generation is below current but **no**
  registered migration has a `To` in the open interval `(generation, current]`, `awf sync`
  writes the lock at the current schema version without gating and without error. (This
  case only arises for a project whose tree layout already loads; a legacy-layout project
  always has the `tree-layout` migration in its gap and gates instead.)
- An older lock lacking `schemaVersion` unmarshals to generation `0` and is treated as
  legacy, not as a parse error.

## Consequences

Easier:
- Adopters cross a breaking config-format change with one command and a clear gate,
  instead of a silent load failure; their customizations are migrated, not discarded.
- Every future breaking config change has a home: register a migration (its `To`
  becomes the new current schema) and the gate, ordering, and auto-bump all follow.
- The pre-commit hook already running `awf check` means the gate needs no new wiring to
  block commits against a stale layout.
- Running `awf upgrade` on awf's own repo both ports the dogfood and serves as the
  end-to-end integration test of ADR-0009 + ADR-0010 together.

Harder / accepted trade-offs:
- `internal/migrate` carries a **frozen** snapshot of the pre-ADR-0009 config shape so
  the `tree-layout` migration can keep parsing legacy files. That snapshot is a
  standing maintenance cost (it must not drift toward the live config structs) until a
  future ADR drops generation-0 support.
- The `runSync`/`runCheck` handlers in `cmd/awf` gain a pre-flight generation check
  ahead of `project.Open` (it cannot live in `project.Sync`/`Check`, which run on an
  already-opened project; Decision 4); the gate must distinguish "behind with a covering
  migration" (gate) from "behind but no covering migration" (auto-bump on sync) so it does
  not falsely block. The auto-bump branch's lock write is performed by `project.Sync`'s
  normal stamping (Decision 1), so its rendered output still passes the publication-safe
  `<no value>` check (`renderTemplate`, ADR-0001); the migration's terminal `sync`
  (Decision 6) likewise re-renders, so a botched `tree-layout` transformation surfaces as a
  render/frontmatter failure rather than a silent bad lock.
- The ADR-0009 `inv: config-root` backing test (a `.go` test, per `invariants.sources`)
  must encode the migrate-package exemption, so the two invariants (`config-root`,
  `legacy-read-isolation`) are written to agree. This edits the *test*, not ADR-0009's
  file or invariant slug, but it does change the enforced meaning of an Accepted ADR's
  absolute invariant, which is the escalation flagged in Decision 3.
- Schema version is a second versioning axis beside `AWFVersion`; contributors must
  learn that a breaking *format* change bumps the registry (a new migration), not the
  tool semver.

Doc-currency obligations the implementing commit(s) must satisfy:
- `docs/architecture.md` gains `internal/migrate`, the `awf upgrade` command, and the
  `schemaVersion` lock field.
- `AGENTS.md` Commands gains `awf upgrade`, and the drift-oracle text notes the
  version gate; this text lives in the relocated `agents-doc` data/parts (ADR-0009) and
  re-renders from there.
- When this ADR flips to Accepted/Implemented, the same commit regenerates `ACTIVE.md`
  via `./x sync`. No `docs/decisions/README.md` index row is owed (ADR-0005).

Downstream work unblocked: an implementation plan covering the `schemaVersion` lock
field, the registry + `tree-layout` migration + frozen legacy reader, the sync/check
gate + no-op auto-bump, and the `upgrade` subcommand, sequenced so the ADR-0009 tree
layout lands, then `awf upgrade` ports this repo (validating both ADRs).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Reuse `AWFVersion` (tool semver) as the migration trigger | Couples every config-format decision to a tool release and vice-versa, and needs semver-range comparison; a dedicated integer `schemaVersion` that bumps only on breaking format changes is a cleaner, orthogonal axis (user-selected). |
| One-off `awf upgrade` hardcoding the 0009 move | A registry costs little now and every future breaking format change needs ordered, idempotent, version-keyed migration anyway; building it once avoids a second rework (user-selected). |
| `awf sync` auto-migrates in place (no explicit command) | Mutating the whole config tree as a silent side effect of sync is surprising and hard to review/revert; an explicit `awf upgrade` keeps the destructive step deliberate while the gate still forces it (user-selected). |
| Gate only in `check`, not `sync` | `sync` would render against a stale layout and write a wrong-generation lock; gating both keeps either entry point from operating on an unmigrated project. |
| No version tracking: adopters re-run `awf init` | Re-init overwrites their customizations and offers no safety net or detection; the gate + migration preserve config and fail loudly. |
| Read legacy config via `config.Load` during migration | Post-ADR-0009 `config.Load` reads the new path and cannot see the legacy file; a frozen legacy reader in `internal/migrate` is required and keeps the exemption to `inv: config-root` narrow and named. |
