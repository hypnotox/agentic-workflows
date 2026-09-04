# agentic-workflows

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-pre--1.0-orange.svg)](#status)

`awf` is a language-agnostic Go CLI for installing a governed agentic-development
workflow. A committed `.awf/` tree renders consistent, reviewable guidance, and `awf
check` detects drift.

## Highlights

- One standard governance footprint with durable repository truth and risk-based routing
- Independent capability triggers for decisions, context, planning, implementation, and review
- Durable decisions for load-bearing choices and deliberately effort-backed operational plans when continuity helps
- Four fixed AWF skills: `awf-effort`, `awf-topics`, `awf-decisions`, and `awf-maintenance`
- Operator-installed generic skills and roles from `agentic-skills`
- CodeGraph for structural source discovery, architecture, callers, dependencies, and impact analysis; Git for changed-path selection
- Generated agent guides, skills, documentation, and optional Git hook payloads
- Current-state topics that separate live project authority from historical decisions
- Backed invariants that connect documented claims to tests or explicit verification
- Repository-local efforts and managed worktrees for work that needs continuity
- Deterministic rendering and drift checks in local development and CI

## Supported coding agents

awf renders four fixed repository-local skills for both [Pi](https://github.com/badlogic/pi-mono)
and [Claude Code](https://www.anthropic.com/claude-code). Generic engineering skills and the
`agentic-explorer`, `agentic-premise-checker`, `agentic-implementer`, and `agentic-reviewer` roles
come from the separately installed
[`agentic-skills`](https://github.com/hypnotox/agentic-skills) package. AWF does not install,
update, vendor, configure, or probe global harness packages.

For Claude Code harness use, optionally install `agentic-skills` to add its generic skills and roles:

```sh
claude plugin marketplace add hypnotox/agentic-skills
claude plugin install agentic-skills@agentic-skills
```

For Pi harness use, optionally install [`pi-tools`](https://github.com/hypnotox/pi-tools) first, then `agentic-skills`:

```sh
pi install git:github.com/hypnotox/pi-tools
pi install git:github.com/hypnotox/agentic-skills
```

`agentic-skills` supplies the thin Pi role adapter, while `pi-tools` owns role execution and runtime
mechanics. AWF emits no role prompts, adapter, model router, policy layer, or preference store. Its
binary remains offline and functional when those optional operator-managed capabilities are absent.
CodeGraph is the expected source-navigation tool for structural discovery, architecture, callers,
dependencies, and impact analysis, while Git selects changed paths. AWF does not check that
CodeGraph is installed; its focused read and resolve commands supply normative project authority
that structural navigation cannot infer.

## Install

Download a binary from the
[latest awf release](https://github.com/hypnotox/agentic-workflows/releases/latest),
extract it, and place `awf` on your `PATH`. Linux tarballs carry portable
`root:root` ownership and ordinary executable and regular-file modes, so a
restricted rootless user namespace can extract them without mapping the release
builder's account.

To install from source with Go 1.27 or later:

```sh
go install github.com/hypnotox/agentic-workflows/cmd/awf@latest
```

## Quickstart

From the repository you want awf to manage:

```sh
awf init
awf check
awf list
```

`awf init` creates the standard `.awf/` tree and renders the workflow, including durable
decision guidance and current-state authority. Commit both the source tree and its rendered outputs.
After changing `.awf/`, render and check again:

```sh
awf render
awf check
```

If initialization finds an existing file it would replace, it stops before mutation and reports the collision.

## How it works

```text
.awf/                              rendered output
├── config.yaml                    ├── AGENTS.md
├── <kind>/<name>.yaml             ├── .pi/skills/awf-*/
├── <kind>/parts/...               ├── .claude/skills/awf-*/
└── parts/...                      └── docs/
```

The standard footprint includes only AWF-specific effort, current-state topic, durable-decision,
and maintenance skills plus governed documentation, checks, and managed worktrees. Generic
brainstorming, context, debugging, code design, planning, implementation, review, and role prompts
come from `agentic-skills`.

| Capability | Independent trigger | Boundary |
|---|---|---|
| Brainstorming | A material choice needs resolution. | Stops only while that choice remains unresolved. |
| Context | Orientation, exploration, or premise challenge adds value. | Produces evidence, not project authority. |
| Debugging | Unexpected behavior has an unknown cause. | Establishes an oracle and root cause before correction. |
| Code design | Agreed behavior raises a structural question. | Preserves the agreed outcome while resolving ownership or dependency shape. |
| Planning | Sequencing, coordination, or resumability materially helps. | Remains interaction-local by default; only a deliberately effort-backed operational plan enters effort scratch. |
| Decision record | An accepted load-bearing choice should outlive implementation. | Records rationale, not current state or implementation choreography. |
| Effort | Durable continuity materially helps. | Owns optional repository-local memory and managed topology. |
| Implementation | The outcome and protected contract are settled. | Executes and verifies without reopening settled choices. |
| Review | Material risk or uncertainty warrants independent assurance. | Reports findings without owning implementation. |

Evaluate these triggers independently at intake and again when relevant facts change. No capability
is an automatic predecessor or successor of another. See [the workflow guide](docs/workflow.md) for
the full boundaries.

## Commands

Punctuation findings are advisory Warnings with zero exit.

<!-- awf:clispec-commands:start -->
| Command | Purpose |
|---|---|
| `awf init [flags]` | Scaffold .awf/ and render the standard workflow footprint |
| `awf render` | Re-render after a template or config change |
| `awf edit <kind> <name> <part> --content <text>` | Replace one semantically identified artifact part |
| `awf reset <kind> <name> <part>` | Restore one semantically identified artifact part |
| `awf check` | Verify working-tree or explicitly staged repository state |
| `awf read <subcommand>` | Read a focused current-state authority projection |
| `awf resolve topic <path>...` | Resolve lexical paths to current-state authority |
| `awf audit <base>\|<a>..<b>` | Report workflow-conformance findings over a commit range (advisory) |
| `awf effort <subcommand>` | Manage slugged repository-local efforts |
| `awf list [<kind>]` | Show the catalog and configured domain inventory |
| `awf config [<key-or-var>]` | Describe config keys and vars (live state inside a project) |
| `awf new <kind> <args>` | Scaffold a new artifact: kind in {topic, domain, pitfall, doc} |
| `awf remove domain <name>` | Remove a configured domain |
| `awf upgrade` | Migrate the .awf/ config tree to the current schema version |
| `awf uninstall` | Remove awf's generated files (keeps .awf/) |
| `awf changelog [--version <v> \| --since <v> \| --range <from>..<to>]` | Print the embedded changelog, or one version/range of it |
| `awf version` | Print the awf version |
<!-- awf:clispec-commands:end -->

Use `awf read topic <domain>/<topic>[:<claim>]` for focused authority. Use `awf resolve topic <path>...` for lexical path ownership and `awf resolve topic --uncovered` for the whole-repository unowned-path census. A path with no authority reports `none` successfully; `awf check` remains the enforcement oracle.

Run `awf help` for complete usage. See
[Working with awf](docs/working-with-awf.md) for configuration, overrides, upgrades,
hooks, efforts, and day-to-day commands.

## Git hooks and CI

Wire the generated `.awf/hooks/` payloads through your own hook mechanism and run `awf
check` in CI. Enable `.awf/bootstrap.sh` to use the repository-pinned awf release.

## Documentation

- [Working with awf](docs/working-with-awf.md): commands, configuration, and daily use
- [Workflow](docs/workflow.md): the development workflow and commit discipline
- [Configuration reference](docs/config-reference.md): all configuration keys and variables
- [Architecture](docs/architecture.md): system structure and dependencies
- `docs/decisions/`: append-only historical decision records
- [Development](docs/development.md): contributing setup and project commands

## Status

awf is pre-1.0. Interfaces and generated formats may change before a stable release.
Use `awf upgrade` when moving an existing project to a newer schema.

Current limitations remain explicit:

- Pi role delegation requires separately installed `pi-tools` and `agentic-skills`; AWF does not probe their availability or compatibility.
- Effort-backed implementation children align with an explicit managed checkout, but parent-session mutation still relies on explicit path targeting and deliberate outside writes are not confined. See [Known Issues](docs/known-issues.md).
- Release archives and checksums share one publication channel. Exact-revision rulesets and workflow gates do not provide independent provenance; immutable releases and attestations remain deferred.
- Linux/amd64 runs exhaustive CI; macOS/arm64 runs native Go behavior. Releases contain Linux and Darwin artifacts for amd64 and arm64; Windows is unsupported.

Every open repository issue and its completion criteria remains listed in [Known Issues](docs/known-issues.md).

## Contributing

Read [AGENTS.md](AGENTS.md) and [the development guide](docs/development.md).

## License

[GNU Affero General Public License v3.0 only](LICENSE) © hypnotox.

awf interoperates with third-party coding agents and is not affiliated with or endorsed
by their vendors.
