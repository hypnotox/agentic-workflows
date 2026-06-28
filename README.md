# agentic-workflows

> **An opinionated agentic-development workflow, wrapped in deterministic checks so it actually holds.**

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-pre--1.0-orange.svg)](#)
[![Claude Code](https://img.shields.io/badge/Claude_Code-skills_%26_agents-D97757)](https://www.anthropic.com/claude-code)

`awf` renders a standardised, opinionated agentic-development workflow into any project — a suite of
[Claude Code](https://www.anthropic.com/claude-code) skills, independent review agents, git hooks,
and documentation — from a small committed config tree, and wraps the probabilistic agent in
deterministic checks (drift, frontmatter, invariant backing, dead links).

You keep a `.awf/` config tree in your repo; `awf` renders it into the files your agent reads
(`.claude/`, `AGENTS.md`, `docs/`, `.githooks/`), and tells you the moment the rendered output drifts
from the config that produced it.

> **Status:** pre-1.0 and evolving; the rendered standard is language-agnostic, the `awf` tool is a
> Go binary. Interfaces may change before a tagged release.

## Why

The instructions your AI agent follows — how to brainstorm, when to write an ADR, what a review must
check, which gate blocks a commit — are usually scattered across prompts, retyped per session, and
impossible to review. They drift from how the project actually works, and nothing tells you when.

`awf` makes that workflow a **version-controlled artifact**:

- **Reviewable** — the workflow lives in a committed `.awf/` config tree, so changes to *how your
  agents work* go through the same diff-and-review as changes to your code.
- **Consistent** — every contributor (and every agent session) reads the same rendered skills,
  agents, and docs; there is no per-developer prompt folklore.
- **Enforced** — a deterministic gate wraps the probabilistic agent: drift detection, frontmatter
  validation, invariant backing, and dead-link checks fail loudly instead of rotting silently.
- **Portable** — one small config tree renders a whole standard into any repo, in any language, and
  `awf check` keeps the rendered output honest forever after.

## How it works

```
.awf/  (you commit this)          rendered output (awf writes & tracks this)
├── config.yaml   enable arrays   ├── AGENTS.md            agent guide
│                 + vars          ├── CLAUDE.md            imports AGENTS.md
├── <kind>/<name>.yaml  sidecars  ├── .claude/skills/…     workflow skills
└── <kind>/parts/…/…    overrides ├── .claude/agents/…     review agents
                                  ├── docs/…               project docs
                                  └── .githooks/…          gate hooks
```

You change the config and re-render; you never hand-edit a rendered file. `awf check` fails if a
rendered file is stale (config changed) or hand-edited, so the two never silently diverge. To
customise a section, drop a *convention part* under `.awf/` that overrides just that section and
inherits the rest of the template.

## Install

Download a prebuilt binary for your platform from the
[latest release](https://github.com/hypnotox/agentic-workflows/releases/latest),
extract it, and put `awf` on your `PATH`. awf is a single static binary with no
runtime dependencies — no Go toolchain required.

<details>
<summary>Install from source (Go users)</summary>

Requires Go 1.26+.

    go install github.com/hypnotox/agentic-workflows/cmd/awf@latest

</details>

## Quickstart

    cd your-project
    awf init             # scaffold .awf/, render the workflow-core set, activate git hooks
    awf check            # verify rendered output is in sync
    awf list             # see which targets are enabled vs available
    awf add skill tdd    # opt a skill in
    awf add doc pitfalls # opt a doc in

`awf init` enables a curated **workflow core** by default — the brainstorm → ADR → plan → implement →
review chain skills, the review agents, the workflow docs, and the gate hooks. Everything else in the
catalog is opt-in with `awf add <kind> <name>` (and `awf remove <kind> <name>` to opt back out).

## Commands

| Command | Purpose |
|---|---|
| `awf init` | Scaffold `.awf/`, render, and activate git hooks. `--force` overwrites colliding files (backing each up to `<path>.awf-bak`); `--force-hooks` takes over an existing `core.hooksPath`. Prompts for config values on a TTY; `--describe` prints the fillable values as JSON (for agents), and `--set k=v` / `--answers FILE` supply them non-interactively. `--set skills=`/`--set docs=` trim which catalog skills/docs are enabled (core pre-selected). |
| `awf sync` | Re-render after a template or config change. |
| `awf check` | Fail on stale or hand-edited rendered output. |
| `awf list [<kind>]` | Show targets and their per-project state (all kinds, or one). |
| `awf add <kind> <name>` | Enable a target — `<kind>` ∈ `skill`, `agent`, `doc`, `domain`. |
| `awf remove <kind> <name>` | Disable a target (a catalog target, or a freeform domain). |
| `awf setup` | Activate git hooks (`core.hooksPath`); `--force-hooks` to override an existing value. |
| `awf audit` | Report workflow-conformance findings over the branch (advisory). |
| `awf invariants` | Report Implemented-ADR invariants lacking a backing comment. |
| `awf upgrade` | Migrate the `.awf/` config tree to the current schema. |
| `awf uninstall` | Remove awf's generated files and unset `core.hooksPath` (keeps your `.awf/` config). |
| `awf version` | Print the awf version. |

Run `awf help` for the full synopsis.

## Adopting into an existing repo

`awf init` never silently clobbers your files. If a path it would write (e.g. an existing
`AGENTS.md`) is already present and not awf-managed, init refuses and lists the collisions. Then:

- **`awf init --force`** overwrites them, backing each original up to `<path>.awf-bak` first.
- **`awf setup`** refuses to repoint a `core.hooksPath` that already belongs to another hooks manager
  (husky, lefthook) unless you pass `--force-hooks`.
- **Trim to taste** — the curated default is small; grow or shrink it with `awf add`/`remove <kind> <name>` (or edit `.awf/config.yaml` directly).
- **Back out anytime** — `awf uninstall` removes everything awf generated and unsets its hook path,
  leaving your `.awf/` config in place.

## Documentation

- [`AGENTS.md`](AGENTS.md) — the agent guide (rendered) that orients an AI agent in this repo
- [`docs/architecture.md`](docs/architecture.md) — system shape, packages, key components
- [`docs/workflow.md`](docs/workflow.md) — the brainstorm/ADR/plan chain and commit discipline
- [`docs/decisions/README.md`](docs/decisions/README.md) — architecture decision records
- [`docs/development.md`](docs/development.md) — local setup and the `./x` command runner

## Contributing

This project develops itself with the workflow it ships. Before non-trivial work, read
[`AGENTS.md`](AGENTS.md) and [`docs/workflow.md`](docs/workflow.md). The core rule: **never hand-edit
a rendered file** — change `.awf/` (or a template) and run `awf sync`, then `awf check`. The gate
(`./x gate`) must pass before every commit.

## License

[MIT](LICENSE) © hypnotox.

`awf` renders configuration for, and interoperates with, Anthropic's Claude Code, but is an
independent project — not affiliated with or endorsed by Anthropic.
