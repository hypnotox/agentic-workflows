---
status: Implemented
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [target-seam, multi-target]
related: [4, 9, 10, 15, 37]
domains: [config, tooling, rendering]
---
# ADR-0016: Tool-Agnostic Target Seam, `.awf/` Config Relocation, and the Claude Adapter

## Context

awf renders its standard into a project as files at fixed, Claude-Code-specific locations, and
keeps its **own** config tree inside that same runtime's directory. Two hardcodings entangle the
tool with one runtime:

- **Output paths for tool-specific artifacts are literals.** Skill output is
  `fmt.Sprintf(".claude/skills/%s-%s/SKILL.md", p.Cfg.Prefix, name)` (`internal/project/project.go:326,353`)
  and agent output is `fmt.Sprintf(".claude/agents/%s.md", name)` (`project.go:328,370`). There is
  no notion of a render *target*; `.claude` is baked into the render loop.
- **awf's own config lives under `.claude/awf/`.** `config.Load` reads
  `<awfDir>/config.yaml` with `awfDir = .claude/awf` (`project.go:43`, `config/config.go:66-67`);
  the lock is `.claude/awf/awf.lock` (`project.go:639`, `migrate.go:44,55`); `runInit` scaffolds
  `.claude/awf/config.yaml` (`cmd/awf/main.go:65`). The provenance banner text literally says
  "change `.claude/awf/`" (`project.go:280`) and `awf:edit` pointers cite
  `.claude/awf/<kind>/parts/...` (`project.go:201,203`). So awf *squats* inside a runtime's
  directory while also generating that runtime's output there.

Meanwhile two conventions have made tool-agnosticism cheap:

- **AGENTS.md is the cross-tool instruction standard** (root-level, read by ~30 tools). awf already
  renders it once to repo-root `"AGENTS.md"` (`project.go:413-414`) via the always-on agents-doc
  singleton; it is already target-neutral.
- **SKILL.md is an open *format* standard** (frontmatter `name`/`description` + markdown, validated
  since ADR-0006), but there is **no single canonical cross-tool directory** for skills. Each
  runtime discovers them from its own path (Claude: `.claude/skills/`). Portability of skills is
  therefore a *placement* problem, not a content problem.

The goal: position awf as a generic, multi-tool-agnostic renderer while shipping exactly one
adapter (Claude Code) today. Concretely: (a) move awf's config out of the runtime directory to
`.awf/`; (b) introduce a target/adapter seam so tool-specific output paths come from a named target
rather than literals; (c) have the Claude adapter additionally emit a whole-file root `CLAUDE.md`
whose body is the standard `@AGENTS.md` import, so Claude Code reliably ingests AGENTS.md (its
auto-fallback for AGENTS.md is not contractual); and (d) give `awf init` a collision guard so
growing awf's write surface (now including a root `CLAUDE.md`) cannot silently clobber a
pre-existing file.

Grounding discoveries that shape the design (verified against source):

- **Artifacts split cleanly into two layers by who owns the path.** *Neutral* (no runtime owns
  them, paths fixed or `DocsDir`-derived): `AGENTS.md` (`project.go:414`), docs
  (`docOutPath`, `project.go:169,395`), domain docs (`project.go:532`), ADR `ACTIVE.md`
  (`project.go:512`), and git hooks `.githooks/<h>` (`project.go:379`). *Tool-specific* (Claude
  literals): skills (`project.go:326`) and agents (`project.go:328`). Hooks are git-native and
  already neutral.
- **The migration machinery is in place and extensible.** `internal/migrate` holds an ordered
  registry (`{To:1,"tree-layout"}`, `{To:2,"drop-replacewith"}` (`migrate.go:23-26`)) with
  `Current()` = highest `To`. `Generation(root)` returns `0` for the legacy monolith
  (`.claude/awf.yaml` present, `.claude/awf/config.yaml` absent), else the `SchemaVersion` from
  `.claude/awf/awf.lock`, else `Current()` when no lock exists (`migrate.go:36-49`).
  `gateStateFor` classifies `ok|gate|autobump` and `gate()` (`main.go:92-98`) blocks `runSync`/
  `runCheck` with "run awf upgrade" when a covered migration path is pending. `legacy.go` is the
  frozen, sole reader of `.claude/awf.yaml` (ADR-0010 `legacy-read-isolation`).
  `manifest.Lock.SchemaVersion` already exists (`manifest.go:19-23`).
- **Config path is already a parameter; the lock path is on-demand.** `config.Load(awfDir)` takes
  the awf dir (`config.go:66`) and stores it as `Config.root` for sidecar/part resolution
  (`config.go:46`; `PartPath` joins `c.root`, `config.go:110-115`). So relocation is a matter of
  changing the *callers'* awfDir plus the standalone lock/banner/pointer literals; `PartPath`
  follows `root` automatically.
- **`init` has no collision guard.** `runInit` (`main.go:64-86`) only checks whether
  `config.yaml` exists to decide whether to scaffold; it never checks the other paths it writes
  via the subsequent `runSync`. cmd/awf parses args positionally (`main.go:21-56`), with no
  `flag`-package usage and no existing `--force`.
- **A new always-emitted file fits the existing model.** AGENTS.md is a `RenderedFile` lock entry
  with template/config/output hashes; a `CLAUDE.md` emitted by the Claude adapter slots in the same
  way. The ADR-0015 banner is injected by `injectBanner` (`project.go:280-299`): a markdown file
  with no frontmatter gets the `<!-- ... -->` banner as line 1, body following, so `CLAUDE.md`
  would render as the banner comment then `@AGENTS.md`.

## Decision

1. **Two-layer artifact taxonomy.** Rendered artifacts are classified as **neutral** (emitted once
   at fixed/`DocsDir`-derived paths, owned by no runtime: `AGENTS.md`, docs, domain docs, ADR
   infra, `.githooks/`) or **adapter** (placed at a runtime-specific path: skills, agents, and the
   new `CLAUDE.md`). Neutral paths are unchanged by this ADR. Only adapter paths flow through the
   target seam (item 2).

2. **A target seam; `claude` is the sole built-in.** Introduce an internal `Target` value carrying
   the placement rules for adapter artifacts (for `claude`: skill →
   `.claude/skills/<prefix>-<name>/SKILL.md`, agent → `.claude/agents/<name>.md`, plus the
   `CLAUDE.md` bridge (item 4)). The render loop obtains adapter output paths from the active target
   instead of the literals at `project.go:326,328`. There is **no user-facing `targets:` config
   key**: `claude` is the implicit single target. The seam is structural (paths are data behind a
   named target, `claude` is one entry), not a transform/plugin interface; adding a second runtime
   later means adding a `Target` and a placement map, not reworking the render loop. Targets that
   reshape *content* (non-standard frontmatter, transformed bodies) are explicitly out of scope;
   the chosen target profile is "any AGENTS.md + standard-SKILL.md tool," for which placement
   suffices.

3. **Relocate awf's config tree `.claude/awf/` → `.awf/`.** Configuration loads from
   `.awf/config.yaml`; the lock is `.awf/awf.lock`. Every surface naming the old root moves:
   the `awfDir` passed at `project.go:43` and `main.go:65`; the `awf add` config path
   (`list_add.go:68`); `lockPath()` (`project.go:639`); the orphan scanner's scan base **and** its
   reported drift paths (`project.go:661,674,691,708`), a normal `check`-path reader/reporter of the
   config tree; the migrate lock path (`migrate.go:44,55`); the provenance **banner text**
   (`project.go:280`, "change `.awf/`"); and the `awf:edit` pointer relative paths
   (`project.go:201,203`, `.awf/<kind>/parts/...`). Adapter-output literals (`.claude/skills/...`,
   `.claude/agents/...` at `project.go:326,328,353,370`) and the frozen legacy/intermediate migration
   paths (`migrate.go:37`, `treelayout.go`, `dropreplacewith.go`, `legacy.go`) deliberately do **not**
   move. `Config.root` becomes `.awf`, so `PartPath`/`Sidecar` follow
   automatically. After this ADR, `.claude/` holds **only** the Claude adapter's rendered output
   (`.claude/skills/`, `.claude/agents/`); awf no longer writes its own config there. The
   `internal/migrate` legacy reader stays frozen on `.claude/awf.yaml` (ADR-0010); the relocation
   migration (item 6) reads the `.claude/awf/` *tree*, not that file, so `legacy-read-isolation`
   holds.

4. **The Claude adapter emits a whole-file `CLAUDE.md` bridge.** The `claude` target writes a
   repo-root `CLAUDE.md` that awf fully owns, whose body is exactly the standard import directive
   `@AGENTS.md` and nothing else. This makes Claude Code ingest `AGENTS.md` verbatim regardless of
   any disputed auto-fallback. It is an adapter artifact (other future targets would not get a
   `CLAUDE.md`) and a tracked `RenderedFile` lock entry, drift-checked like every other output. Per
   ADR-0015 it carries the provenance banner; the rendered file is therefore the banner comment
   followed by `@AGENTS.md`. The `@AGENTS.md` import must remain functional with the leading HTML
   comment present (the comment is inert to Claude Code's import resolution), the one behaviour the
   implementing commit verifies.

5. **`awf init` gains a collision guard and a `--force` flag.** Before writing anything, `init`
   pre-flights every path it would create (the scaffolded `.awf/` config tree and all rendered
   outputs: skills, agents, `AGENTS.md`, `CLAUDE.md`, docs, domain docs, `.githooks/`). If any of
   those paths already exists, `init` aborts with the list of collisions and writes **nothing**.
   `awf init --force` bypasses the guard and overwrites. cmd/awf gains minimal flag handling for
   `--force` (positional parse extended; no `flag`-package migration required). This is a general
   safety net the tool lacked; it is motivated by (but not limited to) the new root `CLAUDE.md`.

6. **Relocation migration `{To:3}` and two-location generation detection.** Append
   `{To:3, "awf-dir-relocation", applyAwfRelocation}` to the registry; `Current()` becomes `3`.
   `applyAwfRelocation` moves `.claude/awf/{config.yaml, awf.lock, skills/, agents/, docs/,
   domains/, parts/, ...}` to `.awf/...` and re-stamps the lock to schema 3; it is idempotent (no-op if
   `.claude/awf/` is absent). `Generation(root)` learns three states, **keyed on directory presence, not on a readable
   lock**, checked in order: `.awf/config.yaml` present → read `.awf/awf.lock` `SchemaVersion` (no
   lock yet → `Current()`, per the existing fresh-init branch); else `.claude/awf/config.yaml` present
   → the pre-relocation tree, reported as a generation strictly below `Current()` (its
   `.claude/awf/awf.lock` schema ≤ 2 when present, and a sentinel < `Current()` when the lock is
   absent, so a pre-relocation tree always gates, never silently fails `project.Open` against a
   missing `.awf/config.yaml`); so `gate()` blocks with "run awf upgrade"; else legacy
   `.claude/awf.yaml` → `0`. `awf upgrade` runs
   migrations then syncs; `Sync` stamps `SchemaVersion: migrate.Current()`. `runSync`/`runCheck`
   keep calling `gate()` before `project.Open` (`main.go:92-98`).

7. **Supersedence scope.** This ADR overrides **ADR-0009 Decision item 1** and its
   `inv: config-root` (config now loads from `.awf/config.yaml`, lock at `.awf/awf.lock`), and
   narrows **ADR-0015 `inv: provenance-banner`** only insofar as the banner text now names `.awf/`
   rather than `.claude/awf/`. Both predecessors keep their `Implemented` status (this is
   partial-item supersedence recorded via `related`, not a full replacement) and their backing
   tests (`config-root`, `provenance-banner`, any `awf:edit`-pointer assertion, **and the orphan/drift
   tests asserting `.claude/awf/...` paths**: `drift_test.go`, `docs_sections_test.go`,
   `coverage_test.go`) update in the same commit that flips this ADR to `Implemented`. ADR-0010 `legacy-read-isolation` is unaffected
   (item 3). The CLAUDE.md bridge realises the AGENTS.md↔runtime link left open by ADR-0004.

Applying this to awf's own repo (running `awf upgrade` to relocate `.claude/awf/` → `.awf/` and
re-syncing so rendered output is byte-identical except for the relocated paths/banner) is **not** a
Decision item; it is adopter (dogfood) work, the final task of the implementation plan (as for
ADR-0009/0010).

## Invariants

Tagged slugs are backed by tests landing with implementation (enforced by `awf check` once this ADR
is `Implemented`); untagged bullets are textual contracts.

- `invariant: awf-config-root`: config loads from `.awf/config.yaml` and the lock is written to/read
  from `.awf/awf.lock`; no normal load/render/sync/check path reads or writes `.claude/awf/...`. The
  `internal/migrate` package under `awf upgrade` is the single exception, reading the legacy
  `.claude/awf.yaml` (ADR-0010) and the pre-relocation `.claude/awf/` tree only to port forward.
- `invariant: target-output-paths`: adapter-artifact output paths are produced by the active `Target`,
  not by literals in the render loop; for the `claude` target a skill renders to
  `.claude/skills/<prefix>-<name>/SKILL.md` and an agent to `.claude/agents/<name>.md`.
- `invariant: claude-md-bridge`: the `claude` target emits a repo-root `CLAUDE.md` whose body is exactly
  the `@AGENTS.md` import (plus the ADR-0015 banner), tracked as a rendered file.
- `invariant: init-collision-guard`: `awf init` writes nothing and reports the offending paths when any
  file it would create already exists; `awf init --force` overwrites.
- `invariant: awf-relocation-migration`: the migrate registry contains `{To:3}`; `Generation` reports a
  tree still at `.claude/awf/` as a pending upgrade (generation < `Current()`), and `awf upgrade`
  relocates it to `.awf/` and stamps the new schema version.
- Neutral artifacts (`AGENTS.md`, docs, domain docs, ADR infra, `.githooks/`) keep their existing
  paths; only adapter artifacts move through the target seam. (Textual.)
- With the dogfood ported, every rendered file is byte-identical to its pre-change output except
  for path/banner strings that name the relocated config root. (Textual.)

## Consequences

Easier:
- awf stops squatting in a runtime's directory: `.awf/` is its config home and `.claude/` becomes
  purely the Claude adapter's output. Adding a second runtime later is a `Target` plus a placement
  map, with no render-loop surgery.
- Claude Code reliably ingests `AGENTS.md` via the owned `CLAUDE.md` bridge, removing dependence on
  a disputed auto-fallback.
- `init` can no longer silently clobber a hand-written `CLAUDE.md` (or any pre-existing managed
  file); the failure is explicit and `--force` is the deliberate override.

Harder / accepted trade-offs:
- Wide but mechanical string churn: every surface naming `.claude/awf/` (config/lock paths, the
  banner const, `awf:edit` pointers, and **template/doc/skill prose** (the agents-doc "Working with
  awf" section, the proposing-adr skill, README, architecture/config domain docs)) must move to
  `.awf/`, then re-render from the ported config. Bounded and covered by the byte-identical port
  test.
- A third migration step plus two-location `Generation` detection adds branches to a gated path;
  mitigated by migrate tests covering legacy→tree→relocated and the gate states.
- The `claude` target is a thin new seam threading kind/name to a placement map; golden/spine tests
  that assert `.claude/skills/...` and `.claude/agents/...` paths now assert them *via* the target.

Ruled out:
- A user-facing `targets:` config key now (YAGNI: one adapter exists; the key lands with the
  second).
- Content-transforming adapters (non-standard frontmatter/body reshaping): the chosen target
  profile is placement-only.
- Symlinking a canonical rendered tree into adapter dirs: breaks on Windows, fights the
  file-hash drift model, and buys nothing since rendered output is never hand-edited.

Doc-currency obligations the implementing commit(s) must satisfy:
- `docs/architecture.md` documents the target seam, the neutral/adapter taxonomy, the `.awf/`
  config root, and the `CLAUDE.md` bridge; the `config` and `tooling` domain docs' current-state
  narratives are refreshed.
- ADR-0009's `config-root` and ADR-0015's `provenance-banner` (and any `awf:edit`-pointer, plus the
  orphan/drift path assertions in `drift_test.go`/`docs_sections_test.go`/`coverage_test.go`)
  backing tests update to the `.awf/` paths in the commit that flips this ADR to `Implemented`.
- When this ADR flips to Accepted/Implemented, the same commit regenerates
  `docs/decisions/ACTIVE.md` via `./x sync`. No `docs/decisions/README.md` index row is owed
  beyond this proposal's row (the README is a how-to guide; `ACTIVE.md` is the generated index,
  ADR-0005).

Downstream work unblocked: an implementation plan covering the target seam, the relocation +
migration `{To:3}`, the `CLAUDE.md` emitter, the `init` collision guard + `--force`, the
prose/banner sweep, and the dogfood `awf upgrade` port, with tests at each step.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Pluggable writer interface (per-kind transform+write methods) | Machinery for a need that does not exist yet; violates the "keep it simple, only Claude now" scope. The declarative target seam delivers structural agnosticism without it. |
| Relocate config only, defer the target seam | Under-delivers the stated intent: agnosticism would be aspirational, not structural. The user chose to fold the relocation into the multi-target work as one effort. |
| Keep config under `.claude/awf/` | Leaves awf squatting in a runtime directory and conflates awf's config with that runtime's output: the very coupling this ADR removes. |
| Symlink a canonical rendered tree into adapter dirs | OS-portability footgun (Windows), special-cases the file-hash drift model, and centralises edits that never happen (rendered output is generated, not hand-edited). Copy/placement strictly dominates. |
| CLAUDE.md as an awf-managed marker section inside a possibly-existing file | The user chose whole-file ownership ("awf always owns what it manages"); the standard bridge is a one-line `@AGENTS.md` import, so a full owned file is simpler than an overlay section. |
| Split the `init` collision guard into its own ADR | It is a general safety feature, but it is motivated by this ADR's growth of the write surface (root `CLAUDE.md`) and was raised as part of the same decision; bundling keeps the adapter model safe by construction. |
