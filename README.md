# agentic-workflows

`awf` renders a standardised, opinionated agentic-development workflow into any project — a suite of
[Claude Code](https://www.anthropic.com/claude-code) skills, independent review agents, git hooks,
and documentation — from a small committed config tree, and wraps the probabilistic agent in
deterministic checks (drift, frontmatter, invariant backing, dead links).

The idea: the workflow your AI agents follow should be **version-controlled, reviewable, and
enforced** — not retyped into a prompt each session. You keep a `.awf/` config tree in your repo;
`awf` renders it into the files your agent reads (`.claude/`, `AGENTS.md`, `docs/`, `.githooks/`) and
tells you when the rendered output drifts from the config.

> **Status:** pre-1.0 and evolving; the rendered standard is language-agnostic, the `awf` tool is a
> Go binary. Interfaces may change before a tagged release.

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

Requires Go 1.26+.

    go install github.com/hypnotox/agentic-workflows/cmd/awf@latest

## Quickstart

    cd your-project
    awf init      # scaffold .awf/, render the workflow-core set, activate git hooks
    awf check     # verify rendered output is in sync
    awf list      # see which skills are enabled vs available
    awf add tdd   # opt into an additional skill

`awf init` enables a curated **workflow core** by default — the brainstorm → plan → ADR → implement →
review chain skills, the review agents, the workflow docs, and the gate hooks. Everything else in the
catalog is opt-in via `awf add` (skills) or the config arrays (docs).

## Commands

| Command | Purpose |
|---|---|
| `awf init` | Scaffold `.awf/`, render, and activate git hooks. `--force` overwrites colliding files (backing each up to `<path>.awf-bak`); `--force-hooks` takes over an existing `core.hooksPath`. |
| `awf sync` | Re-render after a template or config change. |
| `awf check` | Fail on stale or hand-edited rendered output. |
| `awf list` | Show catalog skills and their per-project state. |
| `awf add <skill>` | Enable a catalog skill. |
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
- **Trim to taste** — the curated default is small; add or remove targets in `.awf/config.yaml`.
- **Back out anytime** — `awf uninstall` removes everything awf generated and unsets its hook path,
  leaving your `.awf/` config in place.

## Documentation

- [`AGENTS.md`](AGENTS.md) — the agent guide (rendered) that orients an AI agent in this repo
- [`docs/architecture.md`](docs/architecture.md) — system shape, packages, key components
- [`docs/workflow.md`](docs/workflow.md) — the brainstorm/plan/ADR chain and commit discipline
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
