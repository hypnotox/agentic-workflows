# Authoring agent guidance

Edit `AGENTS.md` directly. It is the repository's author-owned entrypoint, not an AWF output, and AWF requires no particular Markdown structure. Use this guide when adopting AWF or cleaning up repository-wide agent instructions.

The guide should give an agent enough orientation and genuinely global direction to choose the right next action. Keep detailed workflows in skills and current subsystem knowledge in topics. Shared engineering principles belong in their own canonical source, not repeated throughout repository instructions.

## Choose what belongs

Include a brief present-tense orientation: what the project does, its important technology or module identity, maturity, and audience where these affect the work. State the agent's responsibility beyond the immediate edit when that expectation matters.

Keep only genuinely global requirements in the entrypoint. Distinguish hard rules, defaults, and options; state the conditions that change their applicability. Include essential build, test, and maintenance commands and a few useful canonical pointers. Prefer the repository's actual runner over an exhaustive CLI inventory. Explain a non-obvious runner distinction when choosing incorrectly would change what executes.

Route to AWF in one stable sentence: say that the repository uses the AWF workflow skills, which are also readable through the runner as `awf docs <page>` where native skills are unavailable, and tell agents to run `awf resolve` for the paths they will change at the start of work. The skill descriptions say when each workflow applies, so do not copy their trigger conditions or the skill catalog into `AGENTS.md`; copies drift from the skills they repeat. Point to `awf docs knowledge` when contributors author Markdown under `docs/`.

Write concrete instructions, preserve judgment for routine choices, and use examples only to clarify—not to add hidden requirements. Resolve contradictions and repeated rules across the instruction set. Remove stale guidance rather than accumulating exceptions. Keep rationale and detailed procedures with their most specific authoritative owner.

## Illustrative guide

The headings below are optional. Adapt the content to the repository rather than filling a template:

```markdown
# Project guidance

This repository builds a Go CLI for local dataset inspection. Own both the
requested change and the project's ongoing correctness and maintainability.

Preserve the documented data format. Use Conventional Commits.

This repository uses the AWF workflow skills, each also readable as
`./awf docs <page>`; at the start of work, run `./awf resolve` with the paths you
will change and read every returned topic. When authoring Markdown under
`docs/`, follow the shared contract in `./awf docs knowledge`.

## Commands

- `go test ./...`: run tests.
- `go build ./...`: build packages.
- `./awf render && ./awf check`: refresh fixed AWF outputs and check them plus the knowledge bundle.

## References

- `README.md`: public usage and development entrypoints.
- `docs/topics/`: current path-routed project knowledge.
```

Workflow triggers and document lifecycles belong in the skills, not the entrypoint.

## Optional Claude support

If Claude should read the same repository instructions, create an author-owned `CLAUDE.md` containing:

```text
@AGENTS.md
```

Preserve useful Claude-specific instructions alongside that import when needed. Neither file is created, replaced, or structurally validated by AWF. See `awf docs migrate` before transitioning previously generated agent guides to author ownership.
