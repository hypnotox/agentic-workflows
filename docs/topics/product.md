---
type: Project Topic
title: Product and CLI
description: Product boundaries, CLI responsibilities, and stable command and report contracts.
paths:
  - '**'
---

# Product and CLI

AWF owns static projection of its fixed workflow skills and infrastructure, embedded adopter guides, read-only knowledge-bundle conformance checking, lexical path-to-topic routing with per-argument output and response-local source references, local effort memory, native Git worktree creation for active efforts, and create-only intent, specification, plan, ADR, and topic starters. Repository agent instructions remain author-owned; there is no project descriptor or replacement configuration. The completion workflow supplies default agent commit guidance with or without effort memory. The CLI uses Git only for worktree creation and effort-root discovery; agents retain commits, synchronization, integration, removal, and branch cleanup under repository conventions and overrides. Only worktree creation requires Git; other commands and memory-only use remain available without it. AWF does not own repository review, gates, hooks, CI, migrations, document meaning, or general documentation authoring.

The public commands are `init`, `render`, `check`, `resolve`, `docs`, `new`, `effort`, and `version`. `new` creates efforts, change definitions, independent plans, ADRs, and topics. `effort` offers `list`, `show`, `finish`, and add-only `worktree add`. Effort commands resolve the Git primary checkout; tracked-document starters remain caller-checkout local. `resolve` reports globals separately and, when lexical paths are supplied, reports every argument in order with matching non-global topics or `none`; gaps succeed. Command handling and report formatting remain thin adapters over filesystem, Git, and projection owners; business behavior does not belong in the CLI.

This topic is explicitly global because these product boundaries apply to every AWF change. A global topic uses the exact sole selector `paths: ['**']`.

Use plain, stable output that is useful to humans and scripts. Usage errors exit 2, operational errors exit 1 on stderr, and a completed check report uses stdout with a failing exit when findings exist. Embedded docs print to stdout and do not load or mutate repository state.
