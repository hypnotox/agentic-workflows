# awf

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

`awf` is a small Go CLI that projects repository-owned agent guidance, routes paths to current topics, and keeps optional local effort memory.

AWF deliberately does not manage Git, commits, reviews, project gates, migrations, or general documentation. Its runtime works without Git and does not require external agent skills.

## Install

Download the archive for your platform from the [latest release](https://github.com/hypnotox/agentic-workflows/releases/latest), extract `awf`, and run it once in the repository root:

```sh
/path/to/awf init
```

Initialization creates the source descriptor, fixed generated files, and a repository-local `./awf` wrapper pinned to that release. Commit the sources and generated files.

The wrapper supports Linux and macOS on amd64 and arm64. It verifies the release checksum and caches the downloaded binary under the user cache directory.

## Source model

```text
.awf/
├── project.md
└── topics/
    └── **/*.md
```

`.awf/project.md` combines the source-format marker with project-specific agent guidance:

```markdown
---
format: 1
---

# Project guidance

Repository-specific instructions go here.
```

AWF copies the Markdown body literally into `AGENTS.md`. It does not evaluate placeholders, merge sections, or read generated files back as source.

Each topic is one Markdown file with positive path patterns:

```markdown
---
paths:
  - 'internal/projector/**'
  - 'cmd/awf/*'
---

# Projection

Current guidance goes here.
```

`*` matches within one path component and `**` matches across directories. Multiple topics may match. Topic bodies are ordinary Markdown; AWF interprets only `paths`.

## Commands

```text
awf init
awf render
awf check
awf resolve <path>...
awf effort new <slug>
awf effort list
awf effort show <slug>
awf effort finish <slug>
awf version
```

After changing project or topic sources:

```sh
./awf render
./awf check
```

`resolve` is lexical and accepts paths that do not exist yet:

```sh
./awf resolve internal/projector/new-file.go
```

A successful no-match prints `none`.

## Generated ownership

AWF always generates:

- `AGENTS.md` and a thin `CLAUDE.md` pointer;
- Pi and Claude `awf-topics` skills;
- Pi and Claude `awf-effort` skills;
- `.awf/.gitignore`;
- the root `awf` wrapper and `.awf/bootstrap.sh`.

A leading AWF comment marks generated ownership. `render` replaces marked current outputs and refuses to overwrite an unmarked file. AWF never deletes a retired output.

When a marked file is no longer part of the fixed output set, `render` succeeds and reports it while `check` fails. Resolve it manually:

- delete the file, or
- remove the AWF ownership comment to keep the file under repository ownership.

There is no lock file.

## Effort memory

An effort is local ignored memory at `.awf/efforts/<slug>/memory.md`. `finish` moves the complete resident to `.awf/effort-archive/<slug>` without replacing an existing archive. AWF treats the memory and any extra resident files as opaque.

AWF does not create or inspect Git worktrees. When isolation helps, use native Git yourself:

```sh
git worktree add -b awf/<slug> .awf/worktrees/<slug>
```

Integrate and remove the worktree through the repository's normal Git workflow, then finish the effort.

## Updating

For a release using the same source format, run the target version explicitly:

```sh
AWF_VERSION=<target> ./awf render
./awf check
```

The target binary rewrites the generated bootstrap pin. Existing v0.50 adopters follow [the manual migration guide](MIGRATING-v0.50.md). If a future release changes `format`, follow that release's guide; AWF contains no migration runtime or upgrade command.

## Optional external skills

Generated `AGENTS.md` conditionally suggests [`agentic-artifact-design`](https://github.com/hypnotox/agentic-skills) for substantial prose and `agentic-code-design` for structural code judgment. AWF does not install, probe, pin, or require those skills.

## Development

Read [AGENTS.md](AGENTS.md). AWF development uses `./x` so commands execute the checkout source rather than the released wrapper:

```sh
./x test
./x gate
./x render
./x check
```

## Status

AWF is pre-1.0. Source formats and generated output may change between releases; incompatible changes use explicit manual migration guidance.

## License

[GNU Affero General Public License v3.0 only](LICENSE) © hypnotox.

AWF interoperates with third-party coding agents and is not affiliated with or endorsed by their vendors.
