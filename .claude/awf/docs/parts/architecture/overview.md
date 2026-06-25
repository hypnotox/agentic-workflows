## Overview

awf ties a per-project `.claude/awf/` config tree to an embedded template catalog, renders the
standard's skills, agents, hooks, docs, and agent guide, and drift-checks the rendered output
against a lock file. awf is both the tool that publishes the standard and its own first adopter,
so this repo's `.claude/` is rendered by the same engine it ships.

The config tree (ADR-0009) lives under a single `.claude/awf/` root:

- **`config.yaml`** — the skeleton: `prefix`, `vars`, `invariants`, `docsDir`, and flat enable
  arrays (`skills`, `agents`, `docs`, `hooks` — a name's presence enables that target).
- **`<kind>/<target>.yaml`** — optional per-target sidecars holding a target's structured `data`,
  its `sections` overrides (`drop` / `replaceWith`), and its `local` flag.
- **`<kind>/parts/<target>/<section>.md`** — convention parts: if present, the file replaces that
  section's body, no `replaceWith` pointer needed. Per-section precedence is
  `drop > explicit replaceWith > convention part > template default`.
- **`agents-doc.yaml`** + **`parts/agents-doc/<section>.md`** — the always-on agent-guide singleton.
- **`awf.lock`** — the relocated, schema-versioned lock; each entry's `ConfigHash` is a per-target
  projection over exactly that file's inputs, so a sidecar or part edit reflags only that target.
