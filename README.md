# awf

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

`awf` is a small Go CLI that delivers fixed workflow skills and embedded guides, routes paths to current topics, reports routing coverage for chosen paths, keeps optional local effort memory, and creates optional intent, specification, plan, ADR, and topic starters.

Repository instructions in `AGENTS.md` and optional `CLAUDE.md` remain author-owned. Five AWF skills share their substantive instructions with the CLI guides: topics, effort continuity, change definition, ADRs, and completion. The completion workflow supplies default agent commit guidance; agents execute Git operations under repository conventions. The CLI performs no Git operations and does not manage reviews, repository gates, hooks, CI, migrations, or document meaning. It works without Git or external agent skills once its pinned binary is available.

## Start

Starting with the first release that includes `awf.sh`, read the integration guide through the latest published launcher without installing AWF into `PATH`:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/latest/download/awf.sh | bash -s -- docs integration
```

The launcher selects its concrete release, downloads and verifies that release's platform archive and checksums, caches the binary under `${XDG_CACHE_HOME:-$HOME/.cache}/awf/<version>/awf`, and forwards the command. Reading documentation does not modify the repository. Follow the guide before explicitly running `init` in a repository.

Alternatively, download an archive from the [latest release](https://github.com/hypnotox/agentic-workflows/releases/latest), extract `awf`, and run the binary directly. AWF supports Linux and macOS on amd64 and arm64.

Initialization installs the fixed skills, supporting infrastructure, `.awf/VERSION` renderer record, and a repository-local `./awf` wrapper pinned to that release. It creates no project descriptor or agent instruction file. Commit related authored and generated changes together.

## Documentation

The authoritative adopter guides are ordinary Markdown in `internal/docs/`, embedded in every binary and readable here:

- [AWF guide](internal/docs/overview.md): purpose, source/generated ownership, render/check, and guide discovery;
- [Integration](internal/docs/integration.md): adoption, ownership transition, repository-owned automation, updates, and repair;
- [Agent guidance](internal/docs/agents.md): concise author-owned `AGENTS.md` and optional Claude support;
- [Topics](internal/docs/topics.md): path-routed current project knowledge, topic creation, and optional coverage inspection;
- [Efforts](internal/docs/effort.md): local continuity, notes, worktrees, and handoffs;
- [Changes](internal/docs/changes.md): define outcomes and routes, challenge premises, and review intent, specifications, and plans where complexity warrants it;
- [ADRs](internal/docs/adr.md): record consequential decisions and maintain their review, authority, and lifecycle;
- [Completion](internal/docs/completion.md): verification, commits, independent result review where warranted, retrospectives, integration, and cleanup, with or without an effort;
- [Migrating from v0.50](MIGRATING-v0.50.md): one-time conversion and legacy cleanup.

Use `./awf docs` to discover guides and `./awf docs <page>` to read one. Pi and Claude receive substantive skills derived from the same canonical workflows, not a second instruction set. Existing installations must follow the [ownership transition](internal/docs/integration.md#transition-existing-installations), including preservation of unrendered `.awf/project.md` edits and any old topic layout.

## Development

Contributors should read [AGENTS.md](AGENTS.md). Development uses `./x` so commands execute the checkout source rather than the released wrapper:

```sh
./x docs integration
./x test
./x gate
./x render
./x check
```

Project knowledge belongs in `docs/`: current implementation guidance in [`docs/topics/`](docs/topics/), change definitions in `docs/changes/<slug>/`, implementation plans in `docs/plans/`, and ADRs in `docs/decisions/`. Release history remains in [CHANGELOG.md](CHANGELOG.md), and the v0.50 conversion in [MIGRATING-v0.50.md](MIGRATING-v0.50.md).

## Status

AWF is pre-1.0. Authored topic contracts and generated output may change between releases; incompatible changes use explicit manual migration guidance.

## License

[GNU Affero General Public License v3.0 only](LICENSE) © hypnotox.

AWF interoperates with third-party coding agents and is not affiliated with or endorsed by their vendors.
