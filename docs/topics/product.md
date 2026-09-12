---
paths:
  - '**'
---

# Product and CLI

AWF owns fixed documentation projection, embedded adopter guides, lexical path-to-topic routing and optional coverage inspection, local effort memory, and create-only intent, specification, plan, ADR, and topic starters. It supplies default agent commit guidance through the shared generated `AGENTS.md` frame; agents execute Git operations under repository conventions and overrides. The CLI performs no Git operations and does not own repository review, gates, hooks, CI, Git worktrees, migrations, document meaning, or general documentation authoring.

The public commands are `init`, `render`, `check`, `resolve`, `docs`, `new`, `effort`, and `version`. `new` creates efforts, change definitions, independent plans, ADRs, and topics. `effort` retains only `list`, `show`, and `finish`. `resolve --coverage` reports globals once and specific matches for each explicit normalized input path; gaps succeed. Command handling and report formatting remain thin adapters over filesystem and projection owners; business behavior does not belong in the CLI.

This topic is explicitly global because these product boundaries apply to every AWF change. A global topic uses the exact sole selector `paths: ['**']`.

Use plain, stable output that is useful to humans and scripts. Usage errors exit 2, operational errors exit 1 on stderr, and a completed check report uses stdout with a failing exit when findings exist. Embedded docs print to stdout and do not load or mutate repository state.
