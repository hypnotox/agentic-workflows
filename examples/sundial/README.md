# sundial — the awf example adopter

`sundial` is a small fictional Go CLI that prints a week of approximate sunrise and
sunset times:

    go run ./cmd/sundial 52.5 13.4

The fiction is scenery. This directory's real purpose is to be a **complete,
committed example of an awf adoption** — every catalog skill, agent, and doc
enabled, domains with declared territories, authored convention parts, three ADRs,
a plan, and every rendered file checked in. It doubles as the awf repository's
rendered-output quality oracle (its ADR-0090): the enclosing `./x sync` re-renders
this directory from awf's source on every template change, and the enclosing
`./x check` fails on drift, invariant findings, or any advisory note — so what you
see here is provably what awf produces.

## What is what

- `.awf/` — the authored config tree: `config.yaml`, sidecars, convention parts.
  This is the input; everything marked GENERATED is rendered from it.
- `AGENTS.md`, `CLAUDE.md`, `.claude/`, most of `docs/` — rendered output. Never
  edit these; change `.awf/` and run `./x sync`.
- `docs/decisions/`, `docs/plans/` — the fiction's hand-written workflow artifacts
  (`ACTIVE.md` is generated).
- `cmd/`, `internal/` — the fictional Go module (`example.com/sundial`),
  deliberately separate from the awf module.
- `.githooks/` — illustrative hook wiring. This directory is not a git repository,
  so the stubs can never fire here; in a real adoption, activate them with
  `git config core.hooksPath .githooks`.
- `x` — the fiction's command runner. Its awf verbs fetch the release pinned in
  `.awf/bootstrap.sh`; inside this repository, the enclosing `./x sync` and
  `./x check` use awf built from source instead.

## Regenerating

From the repository root: `./x sync` re-renders this directory along with the
repo's own tree, and `./x check` gates it — drift, invariants, zero advisory
notes, and this module's `go test ./...`.
