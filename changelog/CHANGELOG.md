# Changelog

All notable changes to `awf` are documented here, newest first. Entries are grouped per release
into up to four categories — Breaking changes, Features, Bug fixes, Others — chosen by actual
adopter-facing effect (does it change rendered template output, CLI behavior, or config/lock
schema), not by mirroring a commit's Conventional Commits type. Run `awf changelog --help` to
query a single version or a range.

## [Unreleased]
### Features
- Granular, domain-aligned commit scopes: `audit.allowedScopes` expands from `[adr, awf, plans]`
  to eight domain-named scopes, and each entry may carry a `{name, meaning}` mapping so the scope
  taxonomy renders from config (ADR-0055, ADR-0056).
- Convention parts can splice awf-derived values via the `awf:`-namespaced placeholder syntax — a
  dynamic, non-empty-only registry (scope list/table/sentence, prefix, gate commands),
  hard-error guards, and a backslash escape for documenting the syntax (ADR-0057, ADR-0058).
- New mandatory `working-with-awf` usage doc rendered into every project — a post-adoption guide
  to the CLI, overrides, the placeholder registry, and the sync/check loop (ADR-0059).
### Others
- The agent guide's commit-scope prose and the `docs/workflow.md` taxonomy table now derive from
  `audit.allowedScopes`; editing scopes reflags them (ADR-0055, ADR-0057). The guide's
  `awf-setup` section now points at the new usage doc rather than carrying the whole reference.

## [0.7.0] - 2026-07-04
### Breaking changes
- The brainstorming skill's terminal-handoff section is renamed from `terminal-handoff` to
  `terminal-step` for uniform chain-handoff naming (ADR-0054). Its rendered prose is unchanged,
  but any override at `.awf/skills/parts/brainstorming/terminal-handoff.md` must be renamed to
  `terminal-step.md` to keep applying.
### Features
- Add a `Red flags` rationalization-guard section (a "Rationalization | Reality" table) to the
  `tdd`, `debugging`, `executing-plans`, and `subagent-driven-development` skills, each
  overridable via `.awf/skills/parts/<skill>/red-flags.md`.
### Others
- Add a deterministic golden-task eval suite (`internal/evals`) that renders every catalog skill
  and agent through a full `Project.Sync` and asserts cross-artifact workflow-chain seams —
  forward handoffs name their successor on an invocation-verb line, and the chain graph is
  connected and reachable from `brainstorming` (ADR-0053, ADR-0054). Test-only; no change to
  rendered output or CLI behavior.
- Enforce `skill-section-parity`: every catalog skill/agent template's `awf:section` markers must
  match its declared sections, so a section rename can no longer half-land with a blank override
  path (ADR-0054).

## [0.6.2] - 2026-07-03
### Others
- Add a general `awf:include` template-partials directive — awf-owned embedded partials under
  `templates/partials/`, spliced before section parsing, with the drift hash computed over the
  expanded source so a partial edit still flags dependent artifacts stale — and use it to
  deduplicate the review-discipline spine shared by the three reviewer agents. An awf-internal
  change: rendered template output is byte-for-byte unchanged (ADR-0052).

## [0.6.1] - 2026-07-03
### Bug fixes
- Converting a managed skill or agent to `local: true` no longer deletes its file on the next
  sync. The prune step now preserves every declared local artifact's output path, so a
  managed→local conversion keeps the hand-authored file instead of breaking later syncs with
  "local skill file absent".

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
