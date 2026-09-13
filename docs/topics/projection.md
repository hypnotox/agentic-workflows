---
paths:
  - 'internal/projector/**'
  - 'AGENTS.md'
  - 'CLAUDE.md'
  - '.pi/skills/**'
  - '.claude/skills/**'
  - 'awf'
  - '.awf/bootstrap.sh'
  - '.awf/.gitignore'
---

# Projection and ownership

`.awf/project.md` has `format: 2` frontmatter and an opaque Markdown body. `init` creates it from the embedded starter at `internal/projector/templates/project.md`, and AWF copies every body byte literally into the fixed `AGENTS.md` frame. The source format also fixes canonical topic discovery at `docs/topics/`; old source formats are rejected before projection or resolution. The shared frame in `internal/projector/templates/AGENTS.md`, embedded and rendered by `internal/projector/build.go`, owns the effort-adoption rule and default agent commit cadence, independent of external skills; repository instructions may override them. Generated output is never an input.

The output set is fixed in `internal/projector`: `AGENTS.md`, `CLAUDE.md`, Pi and Claude topic and effort skills, `.awf/.gitignore`, the root wrapper, and `.awf/bootstrap.sh`. Pi and Claude outputs are always present. The wrapper and bootstrap are always present. `AGENTS.md` carries direct `docs` routes, while `CLAUDE.md` contains exactly `@AGENTS.md`; the skills keep their identities but are concise just-in-time entrypoints to the embedded topic and effort guides.

An exact leading AWF comment marks generated ownership except for the markerless `CLAUDE.md` import. `render` may create a missing destination or replace a regular marked destination; it also recognizes an existing `CLAUDE.md` only when its bytes exactly match `@AGENTS.md`. Any other unmarked collision is refused. It writes complete files by temporary file and rename. It never deletes retired outputs. Instead, it succeeds and reports marked files outside the current output set, except beneath non-root nested repository roots identified by their own `.git` directory or worktree-style file. This boundary is filesystem-only and does not invoke Git or consult ignore rules. Removing a reported file or its AWF marker is the adopter's explicit cleanup.

`check` validates sources, compares the fixed output bytes and executable class, accepts the exact markerless `CLAUDE.md` import, and fails for unmanaged marked files. Author-owned topics in `docs/topics/`, change definitions in `docs/changes/`, plans in `docs/plans/`, and decisions in `docs/decisions/` do not join the output inventory or ownership-marker scan. `render` leaves them untouched. Topic selectors are validated for routing; change definitions, plans, decisions, and effort residents remain opaque. The generated bootstrap and release-only public launcher are rendered from the same embedded downloader template, but `awf.sh` is not an adopter output. There is no lock, ownership history, migration state, or Git input.
