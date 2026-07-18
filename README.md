# agentic-workflows

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![coverage: raw](https://img.shields.io/codecov/c/github/hypnotox/agentic-workflows?flag=raw&label=coverage%3A%20raw)](https://codecov.io/gh/hypnotox/agentic-workflows?flags%5B0%5D=raw)
[![coverage: accountable](https://img.shields.io/codecov/c/github/hypnotox/agentic-workflows?flag=covered&label=coverage%3A%20accountable)](https://codecov.io/gh/hypnotox/agentic-workflows?flags%5B0%5D=covered)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-pre--1.0-orange.svg)](#)

`awf` renders an opinionated agentic-development workflow into your repo: a chain of
skills that walk an agent from brainstorm through ADR, plan, implementation, review, and
retrospective; independent review agents that read each artifact with fresh context; and
the project docs both rely on. All of it is generated from a small `.awf/` config tree
you commit, rendered into the native layout of every coding-agent runtime you enable, and
`awf check` fails the moment a rendered file drifts from the config that produced it.

The tool is a single Go binary. The standard it renders is language-agnostic. Both are
pre-1.0: interfaces may still move before a tagged release.

## Why

Teams working with coding agents accumulate a folklore layer: prompt snippets,
per-developer tweaks to an agent-instruction file, rules that live in one person's head.
Nothing reviews it, nothing enforces it, and it quietly drifts away from how the project
actually works.

awf treats the workflow as a build artifact instead. The source of truth is a committed
config tree, so a change to how your agents work is a diff someone reviews, like any
other change. Rendering is deterministic, so every contributor and every agent session
reads the same skills and docs, with nothing to retype per session. And a set of
mechanical checks guards what the agent produces, not how it reasons: stale or
hand-edited output, invalid skill frontmatter, dead internal links, references to disabled
skills, and documented invariants with no backing comment in source all fail loudly
instead of rotting.

## What gets rendered

- **Workflow skills** (one tree per enabled runtime: `.pi/skills/<prefix>-*/`,
  `.claude/skills/<prefix>-*/`, and so on). The core chain: brainstorming,
  ADR proposal and review, planning and plan review, a plan↔ADR resync, two execution
  styles (inline or subagent-per-task), implementation review, and a closing
  retrospective that promotes recurring findings toward deterministic checks. Task
  skills are opt-in (TDD, bugfix, debugging, a refactor coupling audit, a
  roadmap-graduation pass), except `adr-lifecycle`, which is scaffolded on with the chain.
- **Review agents**, likewise per runtime: `adr-reviewer`, `plan-reviewer`,
  `code-reviewer`. Each is dispatched with fresh context, so the author never grades
  its own work. Agents are format-neutral: each runtime gets them in its own dialect
  (frontmatter Markdown for most, a TOML profile for Codex).
- **Docs**. An `AGENTS.md` agent guide (with a `CLAUDE.md` or `GEMINI.md` bridge where the
  runtime expects one), workflow and documentation standards, plus opt-in project docs:
  architecture, testing, development, debugging, pitfalls, releasing, glossary, roadmap.
- **Domain docs** (`docs/domains/<name>.md`). One page per freeform domain you
  declare (`awf enable domain rendering`): your hand-authored current-state narrative
  plus a generated index of that domain's ADRs. A domain's sidecar can declare
  `paths` globs (its code territory), and `awf audit` then warns when code in that
  territory changes without the narrative being refreshed.
- **Git-hook payloads** (`.awf/hooks/`): inert pre-commit / commit-msg / pre-push
  scripts. You wire them up; awf never touches your git config.
- **A pinned bootstrap** (`.awf/bootstrap.sh`): an optional installer that fetches the
  exact awf version the repo was rendered with, for hooks and CI.
- **A working-memory directory** (`.awf/memory/`): always rendered with a
  self-ignoring `.gitignore`; agents keep per-effort session notes there without ever
  committing them.

awf renders for six runtimes: Pi, [Claude Code](https://www.anthropic.com/claude-code),
Codex, GitHub Copilot, Cursor, and Gemini. Each gets skills and agents in its own native
paths and dialect: Codex agents are TOML profiles, while Claude Code and Gemini receive a
bridge file. `targets` defaults to `[claude]`; set it to whichever runtimes your team
uses.

Pi 0.80.9+ automatically receives a trusted project extension with `subagent_grounding`, `subagent_explore`,
`subagent_review`, and `subagent_implement`. Grounding, exploration, and review are no-mutation
prompt policy, not an OS sandbox; implementation shares the checkout, runs alone and sequentially,
and commits only when its orchestrator sets `allowCommits`. Every role shows bounded inline child
progress while intermediate activity stays outside parent model content.

## How it works

```
.awf/  (you commit this)            rendered output (awf writes & tracks this)
├── config.yaml   enable arrays     ├── AGENTS.md            agent guide
│                 + vars            ├── bridge file          imports AGENTS.md
├── <kind>/<name>.yaml  sidecars    ├── .claude/skills/...   workflow skills
├── <kind>/parts/.../...  overrides ├── .claude/agents/...   review agents
└── parts/<name>/...  singletons    └── docs/...             project docs
```

The rendered paths above show the default `claude` target; each enabled runtime gets its
own layout, and they are not uniform (Codex splits skills into `.agents/` and agents into
`.codex/`, Pi keeps both under `.pi/skills/`). `awf list target` shows the roster.

You change the config and run `awf sync`; you never hand-edit a rendered file.
`awf check` fails when a rendered file is stale or was edited by hand, so the two can't
silently diverge. To customise one section of an artifact, drop a *convention part*
under `.awf/`; it replaces that section's body and inherits the rest of the template.
For skills and agents the catalog doesn't have, `awf new skill <name> "<desc>"` (or
`agent`) scaffolds a project-local artifact that gets the same rendering, validation,
and drift tracking as the built-in ones.

## Install

Download a prebuilt binary for your platform from the
[latest release](https://github.com/hypnotox/agentic-workflows/releases/latest), extract
it, and put `awf` on your `PATH`. It is a single static binary with no runtime
dependencies.

<details>
<summary>Install from source (Go users)</summary>

Requires Go 1.26+.

    go install github.com/hypnotox/agentic-workflows/cmd/awf@latest

</details>

### Pinning with `.awf/bootstrap.sh`

Projects that enable the `bootstrap` artifact (on by default from `awf init`, or
`awf enable bootstrap`) get a small rendered shell script that resolves the exact awf
version the repo was rendered with: it uses an
already-matching `awf` from `PATH` when one exists, otherwise downloads the release
archive, verifies its SHA-256 against the release checksums, caches the binary under
`$XDG_CACHE_HOME/awf/<version>/` (defaulting to `~/.cache`), and prints its path. Hooks
and CI can then run the pinned version without anyone installing awf by hand:

    "$(bash .awf/bootstrap.sh)" check

It touches nothing outside its cache directory, and `awf disable bootstrap` deletes it.
The bootstrap and hook payloads are bash scripts targeting the linux/darwin archives; on
Windows, put `awf` on `PATH` and call it directly.

## Quickstart

    cd your-project
    awf init             # scaffold .awf/, render the workflow core
    awf check            # verify rendered output is in sync
    awf list             # see what's enabled vs available
    awf enable skill tdd    # opt a skill in
    awf enable doc pitfalls # opt a doc in
    awf enable target pi    # render Pi 0.80.9+ skills and trusted subagent extension

The Pi extension is executable project code loaded behind Pi's project-trust prompt. Its generated
files are drift-checked; use `awf sync` to restore missing or modified copies.

`awf init` enables a curated core by default: eleven skills (the ten-step workflow chain
plus `adr-lifecycle`) and the three review agents. The workflow, documentation, and agent-guide standards sit outside
the toggleable catalog and always render. Everything else is opt-in via
`awf enable <kind> <name>`, and `awf disable` opts back out.

## Worked example

A complete example adopter lives in [`examples/sundial/`](examples/sundial/README.md):
a small fictional Go CLI with every catalog artifact enabled (authored parts,
domains, ADRs, a plan) and every rendered file committed, kept in sync by this
repository's own checks. Browse it to see exactly what an adoption looks like on
disk.

## Commands

| Command | Purpose |
|---|---|
| `awf init` | Scaffold `.awf/` and render. Prompts for config values on a TTY; `--describe` prints them as JSON for agents, `--set k=v` / `--answers FILE` fill them non-interactively, and `--set skills=` / `--set docs=` trim the enabled set. `--force` overwrites colliding files, backing each up to `<path>.awf-bak`. |
| `awf sync` | Re-render after a config or template change. |
| `awf check` | Fail on stale or hand-edited rendered output, dead links, dead skill references, invalid frontmatter, and unbacked invariants. |
| `awf list [<kind>]` | Show enabled vs available artifacts (`awf list target` shows adapters). |
| `awf enable` / `awf disable <kind> <name>` | Toggle an artifact or adapter. `<kind>` ∈ `skill`, `agent`, `doc`, `domain`, `target`, `bootstrap`, `hooks`, `runner`. Enabling a reviewing skill pulls in the agent it dispatches. |
| `awf new adr "<title>"` | Scaffold the next ADR under `docs/decisions/`. |
| `awf new plan "<title>"` | Scaffold a dated plan under `docs/plans/`. |
| `awf new skill\|agent\|doc <name> "<desc>"` | Scaffold a project-local skill, agent, or doc and enable it. |
| `awf audit [--base <ref>]` | Report workflow-conformance findings over the branch's commits. Not part of any gate, but exits non-zero on error-severity findings. |
| `awf invariants` | Report documented invariants that lack a backing comment in source. |
| `awf config` | Describe every config key and var, with this project's live state when run inside one. |
| `awf context <paths>` | Report the owning domains, backed invariants, and related ADRs for the given paths. |
| `awf prose-gate` | Scan tracked text files for typographic punctuation substitutes; blocking, opt-in per project. |
| `awf commit-gate [FILE]` | Validate one commit message against Conventional Commits; built for a `commit-msg` hook. |
| `awf upgrade` | Migrate the `.awf/` tree to the current schema. |
| `awf uninstall` | Remove awf's generated files (keeps your `.awf/` config). |
| `awf changelog` | Print the embedded changelog (`--version`, `--since`, or `--range`). |
| `awf version` | Print the awf version. |

Run `awf help` for the full synopsis.

## Adopting into an existing repo

`awf init` never silently clobbers your files. If a path it would write (say, an
existing `AGENTS.md`) is present and not awf-managed, init refuses and lists the
collisions; `awf init --force` overwrites them after backing each original up to
`<path>.awf-bak`. Rendered skills are named `<prefix>-<skill>`, with the prefix derived
from the repo directory's basename; change it via `prefix` in `.awf/config.yaml`. And
you can back out anytime: `awf uninstall` removes everything awf generated, leaving your
config in place.

awf renders git-hook *content* but never installs or activates hooks; the wiring is
yours. With the `hooks` artifact enabled (default on init), three inert payload scripts
land under `.awf/hooks/`: `pre-commit.sh` (drift check, then your gate),
`commit-msg.sh` (`awf commit-gate`), and `pre-push.sh`. Invoke them from wiring you own,
e.g. an executable `.git/hooks/pre-commit` containing
`exec bash .awf/hooks/pre-commit.sh "$@"`, or a tracked `core.hooksPath` directory. If
you adopted an earlier awf that ran `awf setup`, your repo's `core.hooksPath` may still
point at the no-longer-rendered `.githooks/`; run `git config --unset core.hooksPath`
after upgrading.

Local hooks are per-clone, so back them with CI. A minimal GitHub Actions job, kept on
the exact awf version the repo was rendered with by the bootstrap:

```yaml
jobs:
  awf:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Drift check (pinned awf)
        run: '"$(bash .awf/bootstrap.sh)" check'
      - name: Gate
        run: make gate # your project's gate command
```

## Documentation

- [`AGENTS.md`](AGENTS.md): the (rendered) agent guide that orients an AI agent in this repo
- [`docs/working-with-awf.md`](docs/working-with-awf.md): day-to-day usage, commands, overrides, the sync/check loop
- [`docs/workflow.md`](docs/workflow.md): the brainstorm/ADR/plan chain and commit discipline
- [`docs/architecture.md`](docs/architecture.md): system shape, packages, key components
- [`docs/decisions/README.md`](docs/decisions/README.md): architecture decision records
- [`docs/development.md`](docs/development.md): local setup and the `./x` command runner

## Contributing

This project develops itself with the workflow it ships, so the rules above apply here
too: never hand-edit a rendered file; change `.awf/` (or a template) and run
`awf sync`, then `awf check`. The gate (`./x gate`) must pass before every commit. Read
[`AGENTS.md`](AGENTS.md) and [`docs/workflow.md`](docs/workflow.md) before non-trivial
work.

## License

[MIT](LICENSE) © hypnotox.

`awf` renders configuration for, and interoperates with, third-party coding agents. It is
an independent project, not affiliated with or endorsed by any of their vendors.
