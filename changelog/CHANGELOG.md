# Changelog

All notable changes to `awf` are documented here, newest first. Entries are grouped per release
into up to four categories — Breaking changes, Features, Bug fixes, Others — chosen by actual
adopter-facing effect (does it change rendered template output, CLI behavior, or config/lock
schema), not by mirroring a commit's Conventional Commits type. Run `awf changelog --help` to
query a single version or a range.

## [0.6.0] - 2026-07-03
### Breaking changes
- The three standard docs (`workflow`, `doc-standard`, `agents-md-standard`) are now mandatory
  always-on singletons instead of toggleable catalog docs; config schema migrates to
  generation 6 (ADR-0043). Run `awf upgrade` after updating.
- The rendered bootstrap moves off the repo root into the config tree at `.awf/bootstrap.sh`
  (ADR-0047); update any hook or CI reference to the old `awf-bootstrap.sh` path.
- The `commitScope` var is removed: commit scopes now live only in `audit.allowedScopes`, set
  at init via the comma-separated `commitScopes` answer, quoted by the reviewing skills from
  the same storage `awf commit-gate` enforces, and folded into the drift signal (ADR-0051).
  A leftover var entry is inert; set `audit.allowedScopes` and re-sync to keep the prose.
### Features
- Render three inert git-hook payload scripts (`pre-commit`/`commit-msg`/`pre-push`) under
  `.awf/hooks/` via a `hooks` singleton — enabled by default at init, toggled with
  `awf add/remove hooks`; awf still never touches git config (ADR-0048).
- Add `awf new adr`, scaffolding the next sequential ADR from the rendered template (ADR-0042).
- Add `awf changelog` with `--version`/`--since`/`--range` filters over an embedded changelog
  (ADR-0041).
- `awf add domain` scaffolds the domain's `current-state.md` convention part alongside the
  config edit.
- The rendered workflow doc gains gate-composition and CI-backstop sections.
- Every var/data interpolation degrades to coherent generic prose when unset — an empty
  `awf init` renders publication-safe output — and `awf check`/`awf init` print advisory
  notes for referenced-but-unset vars (ADR-0045).
- `awf check` fails on a rendered reference to a catalog skill outside the enabled set, and
  templates can read the enabled-skill set to conditionalize prose (ADR-0046).
- Reviewing skills and their reviewer agents are pair-validated: `awf add skill` enables the
  missing agent, `awf remove agent` refuses while an enabled skill requires it, and gated
  commands fail on an unpaired config (ADR-0050).
- `awf init` refuses collisions before asking a single prompt, prints unset-var notes and a
  next-steps block after rendering, and falls silent when stdin hits EOF instead of
  streaming the remaining prompts.
- Bare `awf list` shows all seven kinds (including targets, bootstrap, and hooks), and
  `awf help <command>` prints that command's help text.
- The rendered `AGENTS.md` Commands section shows a self-describing placeholder when no
  commands are configured and de-duplicates identical command values.
### Bug fixes
- Single-source the binary version on `project.Version` so the version gate, lock stamp, and
  bootstrap pin cannot disagree; the bootstrap prefers a matching local binary, prints only
  the binary path on stdout, and falls back to `shasum` where `sha256sum` is missing
  (ADR-0049).
- Anchor the rendered skill-reference scanner on a token boundary, so prose like
  `example-bootstrap.sh` no longer trips the dead-skill-reference check.
- Restore the ADR title heading dropped when a project overrides the ADR template's
  sections, and route the generated ACTIVE.md through the canonical generated-by banner.
### Others
- Sweep chain-prose seams, tool-specific vocabulary, and repo residue from the rendered
  templates; hook-command descriptor options no longer suggest unpinned `awf` invocations;
  the `domains` frontmatter guidance now scopes itself to projects with configured domains.

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
