# awf

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

AWF gives coding agents a consistent way of working in your repository. Its workflow skills tell an agent when to settle a direction with you before planning, when to record a decision, how to verify and commit its work, and when to stop and confirm before publishing. Its topic routing tells the agent which of your project's current documentation applies to the files it is about to change.

Installing AWF in a repository gives you:

- five workflow skills, for topics, efforts, changes, ADRs, and completion, under `.agents/skills/` (Codex, Pi, and omp) and `.claude/skills/`, each also readable as `awf docs <page>`;
- `awf resolve <path>...`, which reports the topics in `docs/topics/` whose path selectors match the files an agent will change;
- `awf check`, which validates the Markdown under `docs/` as an [OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md) knowledge bundle, topic selectors, and AWF's generated files, for use in your gate and CI;
- optional local effort memory for work that spans sessions or handoffs, native Git worktrees for active efforts, and create-only starters for intent, specification, plan, ADR, and topic documents;
- a repository-local `./awf` wrapper pinned to one verified release.

AWF requires the [agentic-skills](https://github.com/hypnotox/agentic-skills) package, installed in each agent harness you use. AWF's workflows say where in a change a checkpoint such as brainstorming, review, or a retrospective belongs and which documents result, and rely on agentic-skills methods for how to carry it out. Once the pinned binary is available, the CLI itself works offline, and every command except worktree creation works without Git.

## Start

Read the integration guide through the latest published launcher, without installing anything or modifying the repository:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/latest/download/awf.sh | bash -s -- docs integration
```

The launcher downloads and verifies its release's platform archive, caches the binary, and runs the command. AWF supports Linux and macOS on amd64 and arm64; you can also download an archive from the [latest release](https://github.com/hypnotox/agentic-workflows/releases/latest) and run `awf` directly.

**If the repository already has a `docs/` directory**, every Markdown file in it becomes part of the knowledge bundle, and `check` fails until each one has `type` and `description` frontmatter or is moved out of `docs/`. The integration guide describes the options.

Then, from the repository root, install the fixed files:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/latest/download/awf.sh | bash -s -- init
```

Initialization creates the skills, the pinned `./awf` wrapper, and its supporting files under `.awf/`. It creates no configuration file and never writes `AGENTS.md` or `CLAUDE.md`; add a one-sentence route to AWF there yourself (`./awf docs agents`). Commit the authored and generated changes together. Repositories with an older AWF installation read the migration guide through the launcher instead, with `docs migrate` in place of `init`; their own `./awf` wrapper still runs the old release, which may not have that page.

## Limits

Repository instructions in `AGENTS.md` and optional `CLAUDE.md`, and everything under `docs/`, remain author-owned; AWF generates only its fixed files and never deletes retired ones automatically. `check` validates structure, not whether documentation is true, complete, or approved. The completion workflow supplies default commit guidance, but agents perform commits, synchronization, integration, and cleanup with native Git under your conventions. AWF does not manage reviews, repository gates, hooks, CI, or the meaning of your documents.

## Documentation

The authoritative adopter guides are ordinary Markdown in `internal/docs/`, embedded in every binary. Use `./awf docs` to list them and `./awf docs <page>` to read one:

- [AWF guide](internal/docs/overview.md): purpose, source/generated ownership, render/check, and guide discovery;
- [Integration](internal/docs/integration.md): first-time adoption, repository-owned automation, updates, and repair;
- [Migration](internal/docs/migrate.md): converting older installations and adapting to changed contracts;
- [Knowledge](internal/docs/knowledge.md): shared bundle metadata, reserved indexes/logs, and structural checking;
- [Agent guidance](internal/docs/agents.md): concise author-owned `AGENTS.md` and optional Claude support;
- [Topics](internal/docs/topics.md): finding, creating, and maintaining path-routed current project knowledge;
- [Efforts](internal/docs/effort.md): local continuity, notes, the choice of a worktree, and handoffs;
- [Changes](internal/docs/changes.md): settle the direction with the user before planning, write intent, specifications, and plans, and review them where warranted;
- [ADRs](internal/docs/adr.md): record consequential decisions and maintain their authority and lifecycle;
- [Completion](internal/docs/completion.md): verification, commits, result review where warranted, confirmation before publishing, retrospectives, integration, and cleanup, with or without an effort.

## Development

Contributors start with [AGENTS.md](AGENTS.md), which states the development commands and where project knowledge lives. Development runs the checkout source through `./x`, not the released `./awf` wrapper. Project knowledge is in [`docs/`](docs/), and release history in [CHANGELOG.md](CHANGELOG.md).

## Status

AWF is pre-1.0. Authored topic contracts and generated output may change between releases; incompatible changes are described in the changelog and in `awf docs migrate`.

## License

[GNU Affero General Public License v3.0 only](LICENSE) © hypnotox.

AWF interoperates with third-party coding agents and is not affiliated with or endorsed by their vendors.
