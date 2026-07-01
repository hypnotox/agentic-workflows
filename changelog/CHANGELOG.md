# Changelog

All notable changes to `awf` are documented here, newest first. Entries are grouped per release
into up to four categories — Breaking changes, Features, Bug fixes, Others — chosen by actual
adopter-facing effect (does it change rendered template output, CLI behavior, or config/lock
schema), not by mirroring a commit's Conventional Commits type. Run `awf changelog --help` to
query a single version or a range.

## [0.5.1] - 2026-07-01
### Bug fixes
- Fix `awf audit`/`awf check` failing to open a repository with `extensions.worktreeConfig` set (a
  flag `git worktree add` can leave behind) due to an upstream go-git bug; also make the internal
  `.git` path join portable across OSes.

## [0.5.0] - 2026-06-30
### Breaking changes
- Add a self-pinning `awf-bootstrap.sh` installer singleton (toggle with `awf add/remove
  bootstrap`), pinned to the exact rendering binary's version and checksum-verified before
  install; config schema migrates to generation 5 (ADR-0040). Run `awf upgrade` after updating.
### Features
- Add a binary-version compatibility gate: `sync`/`check`/`invariants`/`audit`/`list` now refuse
  to run when the awf binary is behind the project's schema generation or recorded release
  version (ADR-0039).

## [0.4.0] - 2026-06-29
### Breaking changes
- Multi-target rendering goes live: adapter artifacts (skills, agents) now render once per
  enabled adapter via a `targets` config array (default `[claude]`), replacing the implicit
  Claude-only output path (ADR-0037, ADR-0038). Run `awf upgrade` after updating.
### Features
- Add a Cursor adapter (`.cursor/skills`, `.cursor/agents`, no `CLAUDE.md`-style bridge — Cursor
  reads `AGENTS.md` natively); manage adapters via `awf add/remove/list target <name>` (ADR-0037).
- Skill and agent prose is now tool-agnostic — neutral vocabulary instead of Claude Code-specific
  terms — so it reads correctly under any adapter (ADR-0038).

## [0.3.1] - 2026-06-29
### Others
- Sharpen the rendered workflow doc's guidance to explicitly name `awf check` as the pre-commit
  drift guard your own gate must run, rather than vaguely "your check and gate commands".

## [0.3.0] - 2026-06-29
### Features
- Convention-part bodies now render as raw input — never template-interpolated — closing a class
  of accidental-`{{`-breakage bugs (ADR-0034).
- `awf sync` now backs up any foreign file it would otherwise overwrite to a free
  `<path>.awf-bak[.N]` sibling, so adopting awf into an existing repo no longer risks silently
  clobbering unrelated files (ADR-0035).
- Add `awf commit-gate`, the deterministic, blocking counterpart to `awf audit`'s advisory
  Conventional-Commits rule — wire it into your own `commit-msg` hook (ADR-0036).
- Add `--help`/`-h` support to every subcommand.

## [0.2.0] - 2026-06-29
### Breaking changes
- Remove the `hook` artifact kind entirely; config schema migrates to generation 4, and awf no
  longer installs or manages git hooks (ADR-0032). Run `awf upgrade` after updating.
### Features
- Add an invariant-retirement mechanism: a successor ADR can now formally retire a predecessor's
  invariant tags via `retires_invariants` (ADR-0031).
- Add an opt-in local-hooks section to the rendered workflow doc, describing how to wire your own
  hooks now that awf doesn't (ADR-0032).
- `awf audit` gains a rule flagging an ADR status change whose per-domain index wasn't
  regenerated in the same commit (ADR-0033).

## [0.1.0] - 2026-06-28
_Initial public release._
### Features
- `awf init`/`sync`/`check` render a `.awf/`-configured tree of skills, review agents, docs, and
  the `AGENTS.md` agent guide from embedded templates into a project, with drift detection
  against a schema-versioned lock.
- `awf add`/`remove`/`list` manage which skills, agents, docs, and domains are enabled.
- `awf audit` reports advisory workflow-conformance findings (Conventional Commits, ADR/index
  co-change, and more) over a branch's git history.
- `awf upgrade` migrates a project's `.awf/` config tree across schema versions.
- Ships as prebuilt cross-platform binaries (linux/darwin/windows × amd64/arm64), with
  `go install` as a source fallback.
