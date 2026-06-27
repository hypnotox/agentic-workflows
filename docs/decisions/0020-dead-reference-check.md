---
status: Proposed
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [check, drift, docs]
related: [0011, 0013, 0019]
domains: [rendering]
---
# ADR-0020: Dead-Reference Check in `awf check`

## Context

`awf check` verifies that rendered output matches the lock, that skill/agent frontmatter parses,
and that Implemented-ADR invariants are backed. It does not verify that links *within* a rendered
doc resolve. A managed doc can ship a dead internal markdown link — a renamed or deleted target, or
a path authored at the wrong relativity — and nothing catches it.

A grounding-check found a live instance: `docs/workflow.md` emits `[docs/decisions/README.md](docs/decisions/README.md)`,
but a markdown link resolves relative to its own file's directory (`docs/`), so the target is
`docs/docs/decisions/README.md`, which does not exist — the link 404s in any renderer. The root
cause is that `.layout` paths (ADR-0013) are **repo-root-relative**: correct as a link target from a
root file (`AGENTS.md`'s document map resolves), broken when the same string is emitted as a
markdown link from a file under `docsDir`.

ADR-0019 set aside content reference-scanning for plans and ADRs because they are append-only
historical records where a dead reference may be intentional. The tractable, low-false-positive
subset is checking **markdown links** in awf's **managed rendered docs**: link syntax is
unambiguous (unlike bare backtick path-mentions, which are mostly patterns, commands, or package
dirs), and a dead internal link in awf's own output is a deterministic defect — the kind awf gates.

## Decision

1. **Add a pure `internal/refs` package.** `Links(content string) []string` extracts inline markdown
   links `[..](target)` and returns the relative-path targets, after: skipping `http(s)`/`mailto`/`tel`
   targets and bare `#`-anchors; stripping a trailing `#anchor` and a `(target "title")` title; and
   skipping fenced code blocks (```` ``` ```` and `~~~`). It is stdlib-only and does no I/O.

2. **`internal/project.Check` runs the scan and gates.** For each awf-managed rendered markdown file,
   resolve every `refs.Links` target relative to that file's own directory, `os.Stat` it against the
   repo, and emit `manifest.Drift{Kind: "dead-reference", Path: file, Detail: target}` for each miss.
   A `dead-reference` drift fails `awf check` and the pre-commit hook, exactly like a hash drift.

3. **Scope is awf-managed rendered markdown only:** the in-scope `RenderAll` outputs (skills, agents,
   `AGENTS.md`, docs) plus the regenerated `ACTIVE.md` and domain docs; the `CLAUDE.md` bridge and
   `.githooks/*` are excluded (filter by template id, not by `.md` suffix). Hand-authored ADRs
   (`docs/decisions/NNNN-*.md`) and plans (`docs/plans/*.md`) are out of scope — they are append-only
   historical records (ADR-0019) and not awf-rendered. Links *to* ADR files from the generated
   `ACTIVE.md` / domain docs are in scope, so a deleted ADR is still caught.

4. **Resolution is file-relative and assumes a synced tree.** Targets resolve relative to the
   containing file's directory (standard markdown semantics) and are `os.Stat`-ed on disk, so a link
   to an enabled-but-unsynced managed file resolves once `sync` has run; before sync it also surfaces
   as ordinary missing-file drift.

5. **Fix the link the check surfaces.** `templates/docs/workflow.md.tmpl` cites `.layout.adrReadme`
   (root-relative) as a markdown link from under `docsDir`; make it a `docsDir`-relative link so the
   gate is green on introduction. `.layout.*` paths are root-relative and must not be markdown-linked
   from a file under `docsDir` — this check now enforces that.

## Invariants

- `inv: dead-reference-gated` — `awf check` emits a `dead-reference` drift, and fails, when a managed
  rendered markdown file contains an inline markdown link whose relative target does not resolve
  (file-relative) on disk; it is silent for resolving targets, external/anchor targets, and links
  inside fenced code blocks.
- The scan covers only awf-managed rendered markdown (skills, agents, `AGENTS.md`, docs, domain docs,
  `ACTIVE.md`), never hand-authored ADRs or plans (textual).
- `internal/refs` performs no I/O; path resolution and `os.Stat` live in `internal/project` (textual).

## Consequences

- `awf check` now guarantees awf's managed rendered docs carry no internal markdown link whose
  target file is missing — a deterministic doc-integrity property enforced on every commit, for awf
  and every adopter. (Anchor fragments and reference-style links are out of scope; see Decision 1.)
- It catches an existing defect (the `workflow.md` README link), fixed on introduction (Decision 5).
- It surfaces the ADR-0013 layout-relativity gotcha and enforces against re-introducing it; a fuller
  fix (a `docsDir`-relative link helper in `.layout`) is left as deferred follow-up.
- Per-check cost is a markdown scan of the rendered set — negligible.
- The defensive edge handling (reference-style omitted, code-fence skip, title/anchor strip) is not
  exercised by awf's current docs; it is covered by synthetic `internal/refs` unit tests, so no
  `// coverage-ignore` is needed.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- The `workflow.md.tmpl` link fix re-renders `docs/workflow.md`.
- Adding the check materially shifts the `rendering` domain's current state, so
  `.awf/domains/parts/rendering/current-state.md` is refreshed in the implementing range.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Advisory (warn, never fail) | A dead internal link is a deterministic defect; awf's identity is deterministic gates. Gating gives the strong "no dead links" guarantee. |
| Also check bare backtick path-mentions | Backtick tokens are mostly patterns, commands, or package dirs, not files — false-positive-prone and incompatible with gating. |
| Also scan ADRs and plans | Hand-authored append-only historical records (ADR-0019); not awf-rendered, and may intentionally reference since-moved files. |
| Repo-root-relative resolution | Would let root-relative `.layout` links pass, but breaks genuinely file-relative links (domain docs' `../decisions/*`). File-relative is correct markdown semantics, and the `workflow.md` link is genuinely broken. |
| A separate `awf lint-docs` subcommand | Rendered-output integrity is what `awf check` already owns; a separate command duplicates the render-and-iterate. |
