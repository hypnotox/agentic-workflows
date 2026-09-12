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

`.awf/project.md` has `format: 1` frontmatter and an opaque Markdown body. AWF copies every body byte literally into the fixed `AGENTS.md` frame. The shared frame in `internal/projector/build.go` owns the effort-adoption rule and default agent commit cadence, independent of external skills; repository instructions may override them. Generated output is never an input.

The output set is fixed in `internal/projector`: `AGENTS.md`, `CLAUDE.md`, Pi and Claude topic and effort skills, `.awf/.gitignore`, the root wrapper, and `.awf/bootstrap.sh`. Pi and Claude outputs are always present. The wrapper and bootstrap are always present. `AGENTS.md` carries direct `docs` routes; the skills keep their identities but are concise just-in-time entrypoints to the embedded topic and effort guides.

An exact leading AWF comment marks generated ownership. `render` may create a missing destination or replace a regular marked destination, but refuses an unmarked collision. It writes complete files by temporary file and rename. It never deletes retired outputs. Instead, it succeeds and reports marked files outside the current output set, except beneath non-root nested repository roots identified by their own `.git` directory or worktree-style file. This boundary is filesystem-only and does not invoke Git or consult ignore rules. Removing a reported file or its AWF marker is the adopter's explicit cleanup.

`check` validates sources, compares the fixed output bytes and executable class, and fails for unmanaged marked files. Author-owned `docs/plans/<slug>.md` and `docs/decisions/<slug>.md` files do not join the output inventory; effort residents remain ignored opaque state. `render` leaves all of them untouched and `check` does not interpret their contents. The generated bootstrap and release-only public launcher are rendered from the same embedded downloader template, but `awf.sh` is not an adopter output. There is no lock, ownership history, migration state, or Git input.
