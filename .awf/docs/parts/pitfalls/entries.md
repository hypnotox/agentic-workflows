## `awf audit` and `extensions.worktreeConfig`

`git.PlainOpen` (go-git) refuses to open a repo whose `.git/config` has `extensions.worktreeConfig = true` — a flag `git worktree add` can leave behind even after the worktree is removed — regardless of `core.repositoryformatversion`. Cause: go-git's extension-support check lowercases the extension name before comparing it against its allow-list, whose key is mixed-case, so the lookup never matches. `internal/audit/git.go`'s `openRepo` works around it by opening through a `storage.Storer` wrapper that hides the `[extensions]` config section from go-git before the check runs; neither `Collect` nor `ruleUncommittedChanges` reads repo extensions, so hiding the section is safe. Any future `internal/audit` code opening a repo must go through `openRepo`, not `git.PlainOpen` directly.

## Stdout is API in command-substitution scripts

`.awf/bootstrap.sh` is consumed as `"$(bash .awf/bootstrap.sh)" <args>`, so its stdout is the
binary path and nothing else — a checksum tool's `<asset>: OK` line on stdout execs as part of
the command and fails only on cache-miss runs, which presents as flaky CI. Every diagnostic in
rendered shell must carry `>&2`; `TestBootstrapStdoutPathOnly` pins this (`inv:
bootstrap-stdout-path-only`, ADR-0049).

## Adding a catalog skill: hand-enumerated test touch points

A new `SkillSpec` in `internal/catalog/standard.go` is covered automatically by the
catalog-derived eval fixture (`inv: evals-full-catalog-coverage`, ADR-0053), but several sibling
tests still hand-enumerate their skill set and fail only at `./x gate` time, not at authoring:

- `TestUnsetFallbackRenders` (`internal/project/spine_test.go`) — the publication-safety regression
  lock that renders each skill under empty data and bans leaks; a skill with conditional
  (`{{ if … }}`) branches must be added here or its degradation goes unlocked.
- The per-skill `Test<Skill>Template` golden in the same file — one per skill by convention.
- Chain-enabling fixtures that hardcode a skills list (e.g. `TestScopesEditReflagsReferencingArtifacts`
  in `internal/project/drift_test.go`): once a Core chain skill is referenced unconditionally by a
  sibling — as `reviewing-impl` names `retrospective` and `executing-plans` — every such fixture
  must enable it too, or trip the no-dead-skill-references gate (ADR-0046).

The durable fix is to derive these skill sets from `catalog.Standard` the way ADR-0053 did for the
eval fixture, closing the silent-rot gap; until then, adding a skill means updating these by hand.

## Hard-coded counts in domain narratives drift

The domain current-state narratives (`.awf/domains/parts/*/current-state.md`) are free prose, so a
spelled-out cardinal ("the nine chain-progression skills", "only the ten workflow-chain skills")
is invisible to every deterministic check and goes stale the moment the catalog grows — a 2026-07-07
currency sweep found three such stale counts from ADR-0067/0068 alone, in paragraphs the
feature commits never touched. When a narrative must quantify the catalog, prefer count-free
phrasing ("the `core`-flagged skills", "the chain-progression skills") or expect to sweep every
count in the doc — not just the paragraph you are editing — whenever a chain artifact lands.

## README.md is outside the drift oracle

`README.md` is hand-owned — not rendered, so `awf check` never flags it. Its command table and
feature claims drift silently when the CLI grows (`awf new` and `awf commit-gate` shipped without
README rows). Adding or changing a CLI command means updating the README table in the same
change, per the docs-travel-with-the-change invariant; no deterministic check will remind you.

## `//go:embed` silently skips `_`- and `.`-prefixed paths

The project-local base templates live at `templates/skills/_base/SKILL.md.tmpl` and
`templates/agents/_base.md.tmpl` (ADR-0068). Go's `//go:embed` walk **excludes** every file or
directory whose name begins with `_` or `.` unless the pattern is `all:`-prefixed — so the bare
`//go:embed skills agents …` form embeds neither `_base`, and `fs.ReadFile(templates.FS, …)` fails
at render with a bare `file does not exist`. `templates/embed.go` therefore uses
`all:skills all:agents`. Do not "simplify" it back: dropping `all:` silently drops the base
templates. `TestUnsetFallbackRenders` reads both base templates from `templates.FS`, so a
regression fails the gate — but the failure is a confusing missing-file error, not an obvious embed
bug, hence this note.
