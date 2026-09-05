# awf

[![CI](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml/badge.svg)](https://github.com/hypnotox/agentic-workflows/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

`awf` is a small Go CLI that projects repository-owned agent guidance, routes paths to current topics, keeps optional local effort memory, and creates optional plan and ADR scaffolds.

AWF deliberately does not manage Git, commits, reviews, project gates, migrations, or document meaning. Its scaffolds are conveniences rather than a general authoring platform. The runtime works without Git and does not require external agent skills.

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

`*` matches within one path component and `**` matches across directories. Multiple topics may match, with no hierarchy, priority, or exclusive owner. Topic bodies are ordinary Markdown; AWF interprets only `paths`.

A topic is explicitly global only when `**` is its exact sole selector:

```yaml
paths:
  - '**'
```

A standalone `**` is invalid in mixed or duplicate selector lists. `*`, `src/**`, and `**/*.go` remain ordinary matching patterns.

## Commands

```text
awf init
awf render
awf check
awf resolve [<path>...]
awf effort new <slug>
awf effort list
awf effort show <slug>
awf effort finish <slug>
awf plan new <effort-slug>
awf adr new <slug>
awf version
```

After changing project or topic sources:

```sh
./awf render
./awf check
```

Use `resolve` when repository context is needed. Without arguments it returns explicit global topics only. With paths it returns globals plus topics matching any supplied lexical path, once per topic:

```sh
./awf resolve
./awf resolve internal/projector/new-file.go docs/future.md
```

Paths need not exist. Invalid absolute or escaping paths are rejected even when a global topic exists. Output remains the topic ID and source path; a successful empty result prints `none`.

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

## Efforts, plans, and decisions

An effort is local ignored memory at `.awf/efforts/<slug>/memory.md`. Record its outcome and success criteria, current state, next actions, artifact references, selected attributed decision evidence, and completion evidence. Detailed criteria may instead live in a plan referenced from memory. Do not reconstruct dialogue or retain complete sessions.

Plans and ADRs are independent and optional: complexity can warrant a plan, a material choice can warrant an ADR, and an effort can need either, both, or neither. The create-only scaffold commands write author-owned Markdown and never replace an existing destination:

```sh
./awf plan new <effort-slug> # .awf/efforts/<effort-slug>/plan.md
./awf adr new <slug>         # docs/decisions/<slug>.md
```

A plan requires an existing effort. AWF does not interpret either document, include it in projection, or perform Git actions.

Without a worktree, effort and implementation files share one checkout. With a worktree, keep memory and plans in the primary checkout and the ADR and implementation in the implementation checkout. Record both locations in memory and handoffs. Run memory and plan commands from the primary root, and ADR and implementation Git commands from the implementation root. Run worktree creation, integration, removal, and branch cleanup from the primary root; an absolute wrapper path does not substitute for the correct working directory.

AWF does not create or inspect Git worktrees. When isolation helps, use native Git yourself:

```sh
git worktree add -b awf/<slug> .awf/worktrees/<slug>
```

An ADR has one working copy. Commit the chosen decision and rationale during the effort. After verifying the result and incorporating the durable decision and useful rationale into applicable topics, remove the ADR in a later implementation/topic-update commit. Preserve both commits through ordinary non-squash integration. Historical content remains available through Git:

```sh
git log --full-history -- docs/decisions/<slug>.md
git show <decision-commit>:docs/decisions/<slug>.md
```

Before closing, compare the actual result with the success criteria, report unmet criteria and deviations, surface a changed goal, and keep topics accurate. `effort finish` enforces none of this meaning; it only moves the complete resident to `.awf/effort-archive/<slug>` without replacement. After integration, remove any worktree through native Git and finish the effort.

## Updating

For a release using the same source format, run the target version explicitly:

```sh
AWF_VERSION=<target> ./awf render
./awf check
```

The target binary rewrites the generated bootstrap pin. Existing v0.50 adopters follow [the manual migration guide](MIGRATING-v0.50.md). If a future release changes `format`, follow that release's guide; AWF contains no migration runtime or upgrade command.

## Optional external skills

Generated guidance conditionally references [`agentic-planning`, `agentic-implementing`, and `agentic-reviewing`](https://github.com/hypnotox/agentic-skills) for general engineering methods, plus `agentic-artifact-design` for substantial prose and `agentic-code-design` for structural code judgment. AWF owns only repository-specific discovery, locations, continuity, and completion conventions; it does not install, probe, pin, or require external skills.

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
