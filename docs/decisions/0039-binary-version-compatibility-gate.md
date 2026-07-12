---
status: Implemented
date: 2026-06-30
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, release]
related: [30, 40]
domains: [tooling]
---
# ADR-0039: Binary-Version Compatibility Gate

## Context

awf's first external adopter (the sibling project `fleet`) pins the awf binary in two
independent places: a hand-rolled `AWF_VERSION="0.4.0"` in its `./x` dev wrapper, and the
`awfVersion` field of `.awf/awf.lock`, which awf itself stamps from `project.Version` on every
`sync`. Nothing keeps these two in step — `awf upgrade` bumps the lock's `schemaVersion` (and a
later `sync` rewrites `awfVersion`), but no mechanism notices when the *running* binary disagrees
with what the committed config was rendered by.

The existing schema gate is one-directional. `migrate.gateStateFor` returns `"ok"` whenever the
config generation is at or *above* the binary (`gen >= current`), and `cmd/awf/main.go`'s `gate()`
only errors on the `"gate"` state (config *behind* binary → "run awf upgrade"). The opposite
hazard — a binary *older* than the project, running against a config a newer awf already migrated
or rendered — is silently treated as `"ok"`. Such a binary renders against a schema or templates it
does not understand, and on `sync` it stamps the lock *backwards*, a stealth downgrade with no
diagnostic.

Two version axes exist and fail differently:

- **Schema generation** (`migrate.Generation` vs `migrate.Current`) — an integer. A binary whose
  `Current()` is below the config's generation cannot correctly interpret the config layout. This is
  a correctness failure.
- **Release version** (`awfVersion()` vs the lock's `awfVersion`) — a semver string. Two binaries at
  the same schema may still render different output across releases; an *older* binary against a
  newer-rendered project risks silent template drift and the backward-stamp described above.

Relevant verified facts (grounding): `awfVersion()` and `gate()` are both in `package main`
(`cmd/awf`), so the gate can read the running version directly. `manifest.Load(path)` returns a
`*Lock` whose `AWFVersion` field carries the stamped string. `migrate.Upgrade` restamps only
`SchemaVersion`; `sync` (`internal/project/project.go`) unconditionally rewrites `AWFVersion` from
`project.Version`. Version-string forms differ by build mode: GoReleaser stamps the no-`v` form
(`0.4.0`), `go install` reports the `v` form (`v0.4.0`), and the `project.Version` constant is the
no-`v` form. `golang.org/x/mod` is already in the module graph (indirect); `golang.org/x/mod/semver`
resolves without a new dependency but requires a leading `v` (it rejects `0.4.0`).

## Decision

1. **Add an `"ahead"` schema state.** `migrate.gateStateFor` returns a new `"ahead"` state when
   `gen > current` (today this collapses into `"ok"`). `gen == current` stays `"ok"`; the existing
   `"gate"`/`"autobump"` logic for `gen < current` is unchanged. `GateState` surfaces `"ahead"`.

2. **`gate()` hard-errors on `"ahead"`.** When `GateState(root) == "ahead"`, `gate()` returns an
   error naming the running binary version and the config's schema generation, instructing the user
   to update their pinned awf. This is symmetric with the existing config-behind error.

3. **Add a release-version sub-check.** After the schema check, `gate()` loads
   `.awf/awf.lock` and compares the lock's `AWFVersion` against `awfVersion()` using normalized
   semver ordering. Each operand is normalized idempotently — any existing leading `v` is stripped
   and exactly one `v` re-added (`"v" + strings.TrimPrefix(s, "v")`) — because `x/mod/semver`
   requires a single leading `v` and `awfVersion()` already returns the `v`-prefixed form for
   `go install` builds (a naive prefix would yield `vv0.4.0`, which fails normalization and would
   silently skip the check for exactly that build mode):
   - lock version **newer** than the running binary (binary behind) → hard error instructing the
     user to update their pinned awf.
   - running binary **at or ahead of** the lock → permitted; this is the legitimate pre-upgrade /
     pre-resync state (`sync` will restamp `awfVersion` to the running version).

4. **`check` surfaces ahead-skew as a non-failing notice.** When the running binary is *ahead* of
   the lock's `awfVersion`, `check` prints a one-line drift notice (binary X, project rendered by Y)
   without failing, so the difference is always visible while remaining the permitted pre-upgrade
   state.

5. **Skip the version sub-check when it cannot be computed.** An absent or unparseable lock, an
   empty `AWFVersion`, or a version string that fails semver normalization causes the version
   sub-check to be skipped (not error), mirroring `Generation`'s no-lock branch. The schema check
   still applies.

6. **Gate surface.** The gate guards every command that renders or reads the config for output:
   `sync`, `check`, `invariants`, `audit`, and `list` (and therefore `add`/`remove`, which call
   `sync`). `version` and `uninstall` carry no gate call — they do not interpret the config.
   `upgrade` and `init` carry no *direct* gate call but both chain into `runSync`, so they route
   through the gate transitively; neither can trip it in practice (`upgrade` restamps `SchemaVersion`
   to `Current()` before its chained sync, so the schema check reads `"ok"` and the version sub-check
   sees a binary at-or-ahead of the old lock — the legitimate pre-upgrade state; `init` runs before
   any lock exists, so `Generation` reports `Current()` and the version sub-check is skipped per
   item 5). The new `"ahead"` schema error and the lock-version error are therefore reachable only
   from `sync`/`check`/`invariants`/`audit`/`list`, never from `upgrade` or `init`.

## Invariants

- `inv: version-compat-gate` — every gated command (`sync`, `check`, `invariants`, `audit`, `list`)
  routes through `gate()`, which refuses to proceed when the running binary is behind the project on
  either axis: config schema generation greater than `migrate.Current()`, or lock `awfVersion`
  semver-greater than `awfVersion()`. A binary at or ahead of the project on both axes is permitted.
- The version sub-check never errors on a missing, empty, or unparseable version; it is skipped, and
  only the schema check applies. (Textual contract.)

## Consequences

- **Behavior change for `invariants`, `audit`, and `list`.** These commands gate today only
  via the schema path on `sync`/`check`; routing them through `gate()` means they begin to fail
  against a binary-behind project where they previously ran. This is intended — an advisory report
  produced by the wrong binary is itself untrustworthy — and is called out here as the cost.
- **Forward protection only.** The `"ahead"` gate lives in the binary that *has* it; a binary
  predating this ADR cannot benefit from it. The current schema 4→5 transition (introduced by
  [ADR-0040](0040-self-pinning-rendered-bootstrap.md)) therefore still relies on the
  pinned-binary-bump flow: an old binary reading a schema-5 config fails in the strict YAML decoder,
  not in this gate. The gate protects every transition *after* adopters run a binary that contains
  it.
- **No new direct dependency.** `golang.org/x/mod/semver` is promoted from indirect to direct; no
  `go get` of a new module is required.
- **Corrupt lock passes the schema-ahead gate.** `Generation` returns `Current()` for an unloadable
  lock, so a corrupt lock can never look `"ahead"`. This is accepted: a corrupt lock surfaces as a
  load/drift failure elsewhere, and conflating it with version-skew would worsen the diagnostic.
- **Unblocks** [ADR-0040](0040-self-pinning-rendered-bootstrap.md): once the pin has a single source
  of truth (the rendered bootstrap), this gate is the defense-in-depth that catches a bypassed or
  stale binary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Pure equality (lock version ≠ binary version → error) | Cannot distinguish behind from ahead; the two need opposite remediation, and blocking the ahead case deadlocks `upgrade → sync` (sync is what restamps the lock). |
| Version check only in `check`, not in the `sync` gate | A behind binary running `sync` would re-render and stamp the lock backward before `check` ever runs — the stealth downgrade this ADR exists to prevent. |
| Advisory warnings only (never block) | `check` is the drift oracle; a non-blocking warning lets a wrong-binary result pass the gate and land in a commit. |
| Hand-rolled `major.minor.patch` parser | `x/mod/semver` is already reachable and correctly orders prerelease/pseudo-versions; re-implementing it adds surface for no benefit. |
