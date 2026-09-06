---
paths:
  - '**'
---

# Product and CLI

AWF owns fixed documentation projection, embedded adopter guides, lexical path-to-topic routing, local effort memory, and create-only plan and ADR scaffolds. It supplies default agent commit guidance through the shared generated `AGENTS.md` frame; agents execute Git operations under repository conventions and overrides. The CLI performs no Git operations and does not own repository review, gates, hooks, CI, Git worktrees, migrations, document meaning, or general documentation authoring.

The public commands are `init`, `render`, `check`, `resolve`, `docs`, `effort`, `adr`, `plan`, and `version`. Effort commands are `new`, `list`, `show`, and `finish`; `adr` and `plan` each offer `new`. Command handling remains a thin adapter over the internal filesystem and projection owners; business behavior does not belong in the CLI.

This topic is explicitly global because these product boundaries apply to every AWF change. A global topic uses the exact sole selector `paths: ['**']`.

Use plain, stable output that is useful to humans and scripts. Usage errors exit 2, operational errors exit 1 on stderr, and a completed check report uses stdout with a failing exit when findings exist. Embedded docs print to stdout and do not load or mutate repository state.
