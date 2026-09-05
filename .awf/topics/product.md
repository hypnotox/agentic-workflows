---
paths:
  - 'README.md'
  - '.awf/project.md'
  - 'cmd/awf/**'
---

# Product and CLI

AWF owns three small capabilities: fixed documentation projection, lexical path-to-topic routing, and local effort memory. It does not own repository review, commits, gates, Git worktrees, migrations, or general documentation authoring.

The public commands are `init`, `render`, `check`, `resolve`, `effort`, and `version`. Effort commands are `new`, `list`, `show`, and `finish`. Command handling remains a thin adapter over `internal/projector` and `internal/effortfs`; business behavior does not belong in the CLI.

Use plain, stable output that is useful to humans and scripts. Usage errors exit 2, operational errors exit 1 on stderr, and a completed check report uses stdout with a failing exit when findings exist.
