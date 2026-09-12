# awf

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

`awf` is a small Go CLI that projects repository-owned agent guidance, routes paths to current topics, inspects routing coverage for chosen paths, keeps optional local effort memory, and creates optional intent, specification, plan, ADR, and topic starters.

AWF supplies default agent commit guidance in the shared generated `AGENTS.md` frame; agents execute Git operations under repository conventions. The CLI performs no Git operations and does not manage reviews, repository gates, hooks, CI, migrations, or document meaning. It works without Git or external agent skills once its pinned binary is available.

## Start

Starting with the first release that includes `awf.sh`, read the integration guide through the latest published launcher without installing AWF into `PATH`:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/latest/download/awf.sh | bash -s -- docs integration
```

The launcher selects its concrete release, downloads and verifies that release's platform archive and checksums, caches the binary under `${XDG_CACHE_HOME:-$HOME/.cache}/awf/<version>/awf`, and forwards the command. Reading documentation does not modify the repository. Follow the guide before explicitly running `init` in a repository.

Alternatively, download an archive from the [latest release](https://github.com/hypnotox/agentic-workflows/releases/latest), extract `awf`, and run the binary directly. AWF supports Linux and macOS on amd64 and arm64.

Initialization creates `.awf/project.md`, the fixed generated guidance, and a repository-local `./awf` wrapper pinned to that release. Commit sources and generated outputs together.

## Documentation

The authoritative adopter guides are ordinary Markdown in `internal/docs/`, embedded in every binary and readable here:

- [AWF guide](internal/docs/overview.md): purpose, source/generated ownership, render/check, and guide discovery;
- [Integration](internal/docs/integration.md): adoption, repository-owned hooks and CI, updates, and repair;
- [Topics](internal/docs/topics.md): path-routed current project knowledge, topic creation, and optional coverage inspection;
- [Efforts](internal/docs/effort.md): local continuity, change documents, durable ADRs, and worktrees;
- [Migrating from v0.50](MIGRATING-v0.50.md): one-time conversion and legacy cleanup.

Use `./awf docs`, `./awf docs integration`, `./awf docs topics`, and `./awf docs effort` inside an adopting repository. Generated `AGENTS.md` and the Pi and Claude skill entrypoints route agents to these guides without duplicating their runbooks. Repositories using source format 1 should follow the [format-2 migration](internal/docs/integration.md#migrate-source-format-1-to-2) before updating.

## Development

Contributors should read [AGENTS.md](AGENTS.md). Development uses `./x` so commands execute the checkout source rather than the released wrapper:

```sh
./x docs integration
./x test
./x gate
./x render
./x check
```

Project knowledge belongs in `docs/`: current implementation guidance in [`docs/topics/`](docs/topics/), change documents in `docs/changes/<slug>/`, and ADRs in `docs/decisions/`. Release history remains in [CHANGELOG.md](CHANGELOG.md), and the v0.50 conversion in [MIGRATING-v0.50.md](MIGRATING-v0.50.md).

## Status

AWF is pre-1.0. Source formats and generated output may change between releases; incompatible changes use explicit manual migration guidance.

## License

[GNU Affero General Public License v3.0 only](LICENSE) © hypnotox.

AWF interoperates with third-party coding agents and is not affiliated with or endorsed by their vendors.
