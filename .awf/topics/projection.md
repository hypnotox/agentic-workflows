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

`.awf/project.md` has `format: 1` frontmatter and an opaque Markdown body. AWF copies every body byte literally into the fixed `AGENTS.md` frame. Generated output is never an input.

The output set is fixed in `internal/projector`: `AGENTS.md`, `CLAUDE.md`, Pi and Claude topic and effort skills, `.awf/.gitignore`, the root wrapper, and `.awf/bootstrap.sh`. Pi and Claude outputs are always present. The wrapper and bootstrap are always present.

An exact leading AWF comment marks generated ownership. `render` may create a missing destination or replace a regular marked destination, but refuses an unmarked collision. It writes complete files by temporary file and rename. It never deletes retired outputs. Instead, it succeeds and reports marked files outside the current output set. Removing the file or its AWF marker is the adopter's explicit cleanup.

`check` validates sources, compares the fixed output bytes and executable class, and fails for unmanaged marked files. Author-owned `.awf/efforts/<slug>/plan.md` and `docs/decisions/<slug>.md` files do not join the output inventory: `render` leaves them untouched and `check` does not interpret their contents. There is no lock, ownership history, migration state, or Git input.
