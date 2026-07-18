---
status: Implemented
date: 2026-07-10
tags: [upgrade-flow, bootstrap-porcelain]
related: [39, 40, 49, 76, 79, 82, 131]
domains: [rendering, config]
---
# ADR-0085: Self-contained adopter upgrade flow

## Context

Upgrading awf in an adopter project has a chicken-and-egg: the rendered `.awf/bootstrap.sh`
(ADR-0040) pins exactly the version that rendered it, so every upgrade begins with manually
locating, downloading, checksum-verifying, and extracting the *new* binary before `awf upgrade`
can run. The first real adopter upgrade (v0.5.0→0.12.0, 2026-07-09) named this the top
friction point: the migration chain itself was smooth, but the documented flow is not
self-contained: "something like `AWF_VERSION=0.13.0 bash .awf/bootstrap.sh` honoring an
override, or `awf upgrade --to`, would make the documented flow self-contained."

Forces and observations shaping the design:

- The bootstrap already fetches `checksums.txt` from the target release and verifies the
  download against it. Trust is integrity-only by deliberate posture (ADR-0079: publisher
  compromise is a documented, accepted risk), so letting a caller choose *which* release to
  fetch weakens no security property; verification is identical for every version.
- The download/verify/cache logic (~40 lines of portable shell: OS/arch detection, checksum
  tool fallback, XDG cache layout) lives in the bootstrap template. A second script that
  duplicates it is a second copy to keep bug-free.
- `awf upgrade` early-returns "already current" *without syncing* when no migration
  applies (`cmd/awf/upgrade.go`). Same-schema version bumps (the common case for minor
  releases) therefore never re-render, so the bootstrap pin (and any template improvements)
  would not stick even after a successful binary swap. Any single-command upgrade flow breaks
  on this today.
- Rendered files are written truncate-in-place (`os.WriteFile` in `internal/project`;
  ADR-0076 reserves atomic rename for the lock and migration rewrites). A shell script that
  is re-rendered *while bash is executing it* can execute garbage, because bash reads script
  files incrementally. Any script that triggers a sync of itself must be structured so the
  interpreter never reads it again after the trigger.
- ADR-0082 Decision 2 pins the repo-identity exemption list at exactly two entries
  (bootstrap template, agents-doc template) and names a successor ADR as the only vehicle
  for extending it. A new script that downloads from awf's releases necessarily carries the
  repo slug.
- The hooks singleton (ADR-0048) is precedent for one config toggle rendering multiple files;
  the bootstrap render block sits beside it in `internal/project/render.go`, outside the doc
  collection (ADR-0060/0061 boundary for config-tree render units).

## Decision

1. **Bootstrap honors a pre-set `AWF_VERSION`.** The rendered bootstrap's version assignment
   becomes `AWF_VERSION="${AWF_VERSION:-<project.Version>}"`: an environment override selects
   which release to fetch and verify; absent the override, behavior is byte-identical to
   today. The pin remains the rendering binary's `project.Version` (ADR-0040's single source
   of truth is unchanged); ADR-0040's `inv: bootstrap-pin` (whose text demands the literal
   assignment `AWF_VERSION="<project.Version>"`) is retired by this ADR and replaced by
   `inv: bootstrap-env-override` (the ADR-0049 retire-and-replace precedent), with the
   backing golden-render test updated in the same commit. The override deliberately widens
   determinism: every bootstrap consumer (hook payloads, `./x`-style runners, CI) inherits
   the caller's environment, so an exported `AWF_VERSION` redirects them all, accepted
   because exporting that name plausibly *means* "use this awf version", and the lock-vs-
   binary gate (ADR-0039) refuses the dangerous binary-behind direction. The name stays
   `AWF_VERSION` (not a collision-proof variant) to match the script's own variable and the
   intuitive contract.

2. **A rendered `.awf/upgrade.sh` is the single-command upgrade porcelain.** It ships as the
   second file of the existing `bootstrap` singleton: same `bootstrap.enabled` toggle, no
   new config surface, no schema bump (unlike ADR-0040 there is no new key; the lock-version
   gate already blocks stale binaries against a newer lock). The script:
   - resolves the repo root from its own location (`cd "$(dirname "$0")/.."`) so it works
     from any CWD, unlike hook payloads which inherit git's CWD guarantee;
   - resolves the target version: `$1` when given; otherwise the newest release, by
     following the GitHub `releases/latest` redirect with plain `curl` and extracting the
     tag from the effective URL, failing loudly when the resolved URL carries no `/tag/`
     segment (e.g. a repo with no releases). Both forms are normalized to the bare version
     (`v` prefix stripped): the redirect tag is `vX.Y.Z` and the bootstrap composes
     `v${AWF_VERSION}` for the URL but the no-`v` form for the asset name, so an unstripped
     value fetches a nonexistent asset;
   - fetches and verifies via the bootstrap: `binary="$(AWF_VERSION="$target" bash
     .awf/bootstrap.sh)"`: zero duplicated download logic; a bootstrap failure aborts the
     script (plain assignment propagates the substitution's exit status under `set -e`);
   - hands off with `exec "$binary" upgrade` as its **final statement**. The `exec` is
     load-bearing, not stylistic: the upgrade re-renders `.awf/upgrade.sh` itself
     truncate-in-place, and replacing the shell process before the rewrite is what makes
     self-modification unreachable.
   Diagnostics go to stderr, matching the house shell style; the one-line-stdout contract
   (ADR-0049 `inv: bootstrap-stdout-path-only`) remains a property of `bootstrap.sh` alone
   and is explicitly *not* extended to the porcelain, whose stdout after `exec` belongs to
   `awf upgrade`.

3. **Bootstrap stays deterministic; all nondeterminism lives in the porcelain.** The
   bootstrap never resolves "latest": CI and hooks resolve exactly the pin (or an explicit
   override) reproducibly. `upgrade.sh` is the only rendered artifact that asks GitHub what
   is newest, and only a deliberate invocation runs it.

4. **`awf upgrade` always ends in a sync.** The no-migrations early return is removed:
   `runUpgrade` reports "config already at schema N" when nothing migrates but proceeds to
   `runSync` regardless, so a same-schema binary bump re-renders every managed file and
   re-pins the bootstrap in the same run. This is the piece that makes
   `exec "$binary" upgrade` sufficient, and it is independently a bugfix: today a
   template-only release leaves adopters rendered-stale until the next unrelated sync.

5. **ADR-0082's identity-exemption list gains a third entry** (`refines: ADR-0082#2`). Per ADR-0082 Decision 2's own
   extension rule, this ADR amends that item: the exemption list is pinned at exactly three
   entries (bootstrap template, agents-doc template, and the upgrade-script template),
   each still failing when stale. `inv: residue-exemptions-pinned` (`supersedes-invariant: ADR-0082#residue-exemptions-pinned`) is reworded accordingly
   (ADR-0082 stays Implemented; partial-item amendment via `related:`).

6. **Adjacent guards learn the new unit.** The markdown dead-link scan's non-markdown
   exclusion switches from the exact bootstrap template id to a `bootstrap/` prefix match so
   the shell script is never scanned as markdown; CLI surfaces that describe the bootstrap
   singleton (`awf list`, add/remove wording) name both files.

7. **Docs travel with the change.** `docs/working-with-awf.md` gains an "upgrading awf"
   section (template edit plus the catalog `Sections` entry, which are parity-checked
   together): the single-command flow, the explicit-version form, the env override on its
   own, and the residual manual work a renderer cannot do (adopter-owned call sites, hook
   wiring, prose parts): the adopter upgrade runbook owed since the upgrade rehearsal. The
   rendered `AGENTS.md` invariant entries for ADR-0040/0049 *and* the ADR-0082 exemption
   bullet (two entries → three) are reworded via their `.awf/agents-doc.yaml` data entries
   in the same change, and the commit that flips this ADR's status adds this ADR's
   invariants to the guide's list and regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

8. **Bash, linux/darwin, downgrades out of scope.** Same platform posture as ADR-0040. A
   downgrade attempt via an explicit older `$1` is not special-cased: the older binary's
   ADR-0039 version gate refuses it with its standard message: the schema-ahead arm when
   the config's schema generation is newer, the lock-behind arm for a same-schema
   downgrade.

9. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,
   ADR-0120).** This ADR retires `supersedes-invariant: ADR-0040#bootstrap-pin`.

## Invariants

- `invariant: bootstrap-env-override`: the rendered bootstrap's version assignment is the
  default-expansion form `AWF_VERSION="${AWF_VERSION:-<project.Version>}"`: a pre-set
  environment value wins, and absent one the script resolves exactly the rendering binary's
  `project.Version`. (Golden-render assertion; replaces the retired ADR-0040
  `inv: bootstrap-pin` (see `retires_invariants`) keeping its single-source-of-truth
  property under the new assignment form.)
- `invariant: upgrade-exec-final`: the rendered `.awf/upgrade.sh` triggers the upgrade with
  `exec "$binary" upgrade` and that `exec` line is the script's final statement, so the
  shell process is replaced before `awf upgrade` re-renders the script truncate-in-place.
  (Golden-render assertion.)
- `invariant: upgrade-delegates-fetch`: the rendered `.awf/upgrade.sh` obtains the binary
  exclusively by invoking `.awf/bootstrap.sh` with `AWF_VERSION` set; it contains no
  `curl` of a release asset and no checksum invocation of its own; the latest-tag redirect
  probe is its only direct network call. (Golden-render assertion.)
- `invariant: upgrade-always-syncs`: `runUpgrade` reaches `runSync` on every successful run,
  including the zero-migrations case.
- `invariant: bootstrap-two-files`: with the `bootstrap` singleton enabled, the render set
  contains exactly two files under it: `.awf/bootstrap.sh` and `.awf/upgrade.sh`.
  (Golden-render assertion over the sync output paths; lock tracking, prune-on-disable,
  and uninstall removal follow structurally from the shared render/lock path.)

## Consequences

- **The documented upgrade flow becomes one command**, `bash .awf/upgrade.sh` (latest) or
  `bash .awf/upgrade.sh 0.13.0`, and it closes its own loop: the exec'd binary migrates,
  re-renders, and re-pins the bootstrap. The env override is independently useful (CI
  trialing a release before committing the pin).
- **One bridging upgrade still starts manually.** `.awf/upgrade.sh` exists in an adopter
  tree only after the first upgrade to a release that ships it, so that one upgrade (the first adopter's
  next, the motivating case) uses the env override by hand:
  `AWF_VERSION=X bash .awf/bootstrap.sh` then `<path> upgrade`. Every upgrade after that is
  the single command.
- **Determinism widening (Decision 1) is real and accepted.** An environment `AWF_VERSION`
  now redirects every bootstrap consumer, including hook payloads on every commit. The
  failure modes are bounded: verification still runs, and the ADR-0039 gates refuse a
  behind-binary. The trade was taken for the intuitive contract.
- **`awf upgrade` gets slower in the no-op case** (it now syncs), and its "already current"
  message becomes "already at schema N" followed by normal sync output. That cost buys
  template-only releases actually landing, previously a silent gap.
- **The latest-tag resolution is a live-network behavior no test covers**: the shell
  runtime sits outside the coverage gate exactly as ADR-0040 accepted for the bootstrap;
  the redirect-shape assumption (effective URL ends in `/tag/vX.Y.Z`) must be confirmed
  manually when implementing and is guarded at runtime by the loud no-`/tag/` failure.
  awf's own repo disables the bootstrap singleton (builds from source), so dogfooding
  happens in adopters, same as the bootstrap itself.
- **ADR-0082's exemption ceiling moves from two to three.** The rule that extension
  requires a successor ADR held in practice; this ADR is that successor.
- **A `.ps1`/Windows companion remains out of scope**, inherited from ADR-0040.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| `awf upgrade --to <version>` (Go self-update: binary downloads and re-execs its successor) | Real machinery (download/verify/exec logic in Go, platform handling, self-replacement edge cases) for the same outcome the two-layer shell design gets nearly for free. |
| Standalone `upgrade.sh` with its own download logic | Duplicates the bootstrap's ~40 lines of fetch/verify/cache shell; two copies to keep bug-free. Delegation via the env override keeps one implementation. |
| Env override only, no porcelain (document `AWF_VERSION=X bash .awf/bootstrap.sh` + manual `upgrade`) | Two commands and the adopter must look up the version number first: re-adds the manual step the feedback named; the user explicitly asked for a single command. |
| Teach the bootstrap itself to resolve "latest" | Breaks bootstrap determinism: CI and hooks must resolve the pin reproducibly. Nondeterminism belongs only in the deliberately-run porcelain. |
| Resolve latest via the GitHub releases API (JSON) | Needs JSON parsing with no guaranteed `jq` on adopter machines; the redirect probe needs only the `curl` already required, and fails loudly on shape drift. |
| Collision-proof override name (`AWF_BOOTSTRAP_VERSION`) | An exported `AWF_VERSION` plausibly means "use this awf version"; honoring it is the intuitive contract; the gates bound the risk. |
| Interactive confirm before upgrading to latest | Breaks unattended use unless flagged around; the resolved target is printed to stderr before anything happens, and `$1` pins explicitly. |
