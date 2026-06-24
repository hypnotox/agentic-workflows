# awf — Agent Guide

This document is the authoritative reference for AI agents working in the
`awf` repository. Read it before taking any action; keep it current
as decisions evolve.

This repository contains `awf` — the **Agentic Workflows** CLI and standard. `awf` scaffolds, renders, and drift-checks a suite of Claude Code skills, review agents, and git hooks into any Go project from a single `.claude/awf.yaml` config file. It is both the tool that publishes the standard and the first adopter of it.

The full workflow chain (brainstorming → ADR → plan → execution → review) is expressed as project-owned skill files under `.claude/skills/awf-*/` and review agents under `.claude/agents/`. Hooks under `.githooks/` enforce the gate before commits and pushes land.


## Build & Test

Run the test suite:

```
go test ./...
```

Run the full gate before committing or handing off:

```
./x gate
```

The gate must be green before any commit lands on the main branch.

## Workflow Chain

Follow this canonical chain for feature and fix work:

1. **awf-brainstorming** — explore intent, surface constraints, pick approach
2. **awf-proposing-adr** — record significant decisions; ADRs live under `docs/decisions`
3. **awf-reviewing-adr** — review the proposed ADR before acceptance
4. **awf-writing-plans** — write a bite-sized implementation plan; plans live under `docs/the skills framework/plans`
5. **awf-reviewing-plan** — review the plan before execution starts
6. **awf-reviewing-plan-resync** — re-review after plan adjustments
7. **awf-executing-plans** / **awf-subagent-driven-development** — execute tasks one-by-one, gate after each
8. **awf-reviewing-impl** — review the finished implementation before merge

Never skip steps; each gate keeps later steps cheap.

## Repository Layout

Key directories:

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add` subcommands.
- **`internal/config/`** — parses and validates `.claude/awf.yaml`; owns the config schema.
- **`internal/catalog/`** — reads `templates/catalog.yaml`; describes which skills/agents/hooks/sections are available.
- **`internal/render/`** — Go `text/template` rendering with `missingkey=zero`; applies `data`, `sections` (drop / replaceWith), and per-template part injection.
- **`internal/manifest/`** — writes and reads `.claude/awf.lock`; drives drift detection for `awf check`.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and `Check()`; golden tests live here.
- **`internal/adrtools/`** — regenerates `docs/decisions/ACTIVE.md` from ADR frontmatter; run via `go test ./internal/adrtools/`.
- **`templates/`** — embedded skill, agent, hook, and agents-doc templates; catalog lives at `templates/catalog.yaml`.
- **`docs/decisions/`** — ADRs for this repository; `ACTIVE.md` is auto-generated; `README.md` is the human index.
- **`docs/the skills framework/plans/`** — implementation plans written by `awf-writing-plans`.
- **`.claude/skills/awf-*/`** — rendered skill files (committed; do not hand-edit).
- **`.claude/agents/`** — rendered agent files (committed; do not hand-edit).
- **`.githooks/`** — rendered pre-commit and pre-push hooks (committed; run `awf setup` — or `./x setup` in this repo — to activate via `core.hooksPath`).


## Conventions & Invariants

Conventions every agent must respect:

- **TDD first.** Write a failing test before the implementation change. Use `go test -run TestX -v ./...` to confirm the test fails for the right reason before fixing.
- **Gate before every commit.** Run `./x gate` (≈15s; runs tests, vet, and golangci-lint). Never commit with a failing gate; never skip with `--no-verify`.
- **Publication-safe templates.** All templates render with `missingkey=zero`. A template must never produce a no-value token when a var is an empty string — wrap optional content in a conditional block. Run `awf check` after any sync to verify.
- **`awf check` is the drift oracle.** After editing `.claude/awf.yaml` or any part file, run `./x sync && ./x check`. A clean check means on-disk rendered files match the lock. Commit rendered files alongside config changes.
- **ADRs live under `docs/decisions/`.** Follow the template at `docs/decisions/template.md`. Regenerate `docs/decisions/ACTIVE.md` via `go test ./internal/adrtools/` after any ADR status change. Never hand-edit `ACTIVE.md`.
- **Plans live under `docs/the skills framework/plans/`.** Use `awf-writing-plans` to write them; `awf-reviewing-plan` to review them before execution.
- **Conventional Commits with `awf` scope** for tool and workflow changes: `feat(awf):`, `fix(awf):`, `docs(adr):`, `refactor(awf):`. Subject lines ≤72 chars, imperative mood.
- **One concern per commit.** No incidental refactors riding alongside a feature. No `git add -A` — stage files explicitly.
- **Do not hand-edit rendered files.** Edit `.claude/awf.yaml` or part files under `.claude/awf/parts/`, then re-sync. Rendered files are committed as generated artifacts.
- **`.the skills framework/` files are never committed.** They are session-local tooling artifacts.

