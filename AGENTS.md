# awf — Agent Guide

This document is the authoritative reference for AI agents working in the `awf`
repository. Read it before taking any action; keep it current as decisions evolve.

## You and this project

You are a developer on `awf` — the Agentic Workflows CLI and standard. You are responsible for its long-term health as well as the task in front of you. Bugs you notice in passing are yours; coverage gaps are yours; documentation drift is yours to fix in the same commit that caused it. awf is both the tool that publishes the standard and the first adopter of it, so its own setup must model what it generates.


## Identity

`awf` scaffolds, renders, and drift-checks a suite of Claude Code skills, review agents, git hooks, docs, and this agent guide into any Go project from a single `.claude/awf.yaml` config file. The full workflow chain is expressed as project-owned skill files under `.claude/skills/awf-*/` and review agents under `.claude/agents/`; hooks under `.githooks/` enforce the gate. Module path `agentic-workflows`; Go 1.26; private, pre-1.0, no external API stability.


## Invariants

Hard rules every change must respect:

- **Append-only ADRs.** Decision rationale lives under `docs/decisions/`; `docs/decisions/ACTIVE.md` is generated — never hand-edited.
- **Docs travel with the change.** Reality and its documentation update in the same commit.
- **Green gate before every commit.** `./x gate` must pass before any commit lands.
- **Publication-safe templates.** Every template renders with `missingkey=zero`; never emit a no-value token for an empty var — wrap optional output in a conditional. Run `awf check` after any sync to verify. (ADR-0001)
- **`awf check` is the drift oracle.** After editing `.claude/awf.yaml` or any part, run `./x sync && ./x check`. Commit rendered files alongside config changes; never hand-edit a rendered file.
- **Conventional Commits, `awf` scope.** One concern per commit; stage files explicitly (no `git add -A`).
- **Valid skill/agent frontmatter.** Rendered skills and agents carry parseable YAML frontmatter with non-empty `name`/`description`; `awf sync` fails fast and `awf check` reports `invalid-frontmatter` otherwise. (ADR-0006)

## Workflow

Canonical chain for non-trivial work:

```
brainstorming → planning (if warranted) → ADR (if warranted) → review → implementation → review
```

Brainstorming is the hard prerequisite. **Planning** is warranted by *complexity* (multi-commit, interdependent steps); an **ADR** is warranted by *load-bearing-ness* (a design decision the project must remember). Many tasks need neither. Reviews are lightweight: the grounding-check inside `awf-brainstorming` subsumes plan/ADR review, and `awf-reviewing-impl` is the single terminal review.

**Chain skills** (invoke in order): `awf-brainstorming`, `awf-writing-plans`, `awf-proposing-adr`, `awf-executing-plans` / `awf-subagent-driven-development`, `awf-reviewing-impl`. **Task skills** (as needed): `awf-tdd`, `awf-bugfix`, `awf-debugging`, `awf-adr-lifecycle`.

Run `./x gate` before every commit; `./x gate full` is the full tier. Conventional Commits; one concern per commit. Full rules: [AGENTS.md](AGENTS.md).

## Commands

- `go test ./...` — run the test suite
- `./x gate` — run the gate before committing
- `./x check` — check rendered files for drift


## Document map

- **ADR index:** [docs/decisions/README.md](docs/decisions/README.md) — architecture decisions and lifecycle.
- **Active ADRs:** [docs/decisions/ACTIVE.md](docs/decisions/ACTIVE.md) — generated status index; do not hand-edit.
- **Plans:** [docs/plans](docs/plans) — implementation plans for complex work.
- **Architecture:** [docs/architecture.md](docs/architecture.md) — system shape, packages, key components, dependencies
