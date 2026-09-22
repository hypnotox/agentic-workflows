# Authoring agent guidance

Edit `AGENTS.md` directly. It is the repository's author-owned entrypoint, not an AWF output, and AWF requires no particular Markdown structure. Use this guide when adopting AWF or cleaning up repository-wide agent instructions.

The guide should give an agent enough orientation and genuinely global direction to choose the right next action. Keep detailed workflows in skills and current subsystem knowledge in topics. Shared engineering principles belong in their own canonical source, not repeated throughout repository instructions.

## Choose what belongs

Include a brief present-tense orientation: what the project does, its important technology or module identity, maturity, and audience where these affect the work. State the agent's responsibility beyond the immediate edit when that expectation matters.

Keep only genuinely global requirements in the entrypoint. Distinguish hard rules, defaults, and options; state the conditions that change their applicability. Include essential build, test, and maintenance commands and a few useful canonical pointers. Prefer the repository's actual runner over an exhaustive CLI inventory. Explain a non-obvious runner distinction when choosing incorrectly would change what executes.

Include the discovery cues agents need before selecting detailed guidance: how to find applicable topics and the shared knowledge-document contract, when brainstorming or definition needs the change workflow, when enduring decisions need the ADR workflow, when continuity needs an effort, and how to reach verification and commit guidance during implementation as well as at completion, even without an effort. Point to the AWF CLI workflow when native skills are unavailable. Native skill descriptions own the skill catalog; do not reproduce it in `AGENTS.md`.

Write concrete instructions, preserve judgment for routine choices, and use examples only to clarify—not to add hidden requirements. Resolve contradictions and repeated rules across the instruction set. Remove stale guidance rather than accumulating exceptions. Keep rationale and detailed procedures with their most specific authoritative owner.

## Illustrative guide

The headings below are optional. Adapt the content to the repository rather than filling a template:

```markdown
# Project guidance

This repository builds a Go CLI for local dataset inspection. Own both the
requested change and the project's ongoing correctness and maintainability.

Preserve the documented data format. Use Conventional Commits.

Use `./awf resolve` for global topics, adding repository-relative paths for
matching knowledge. Read the returned sources and keep affected topics current;
use the topic workflow, available through `./awf docs topics`. When authoring
Markdown under `docs/`, follow the shared contract in `./awf docs knowledge`.

Use the change workflow (`./awf docs changes`) when brainstorming a material
choice or defining a change's outcome or route, including when no change document
is needed. Use the ADR workflow (`./awf docs adr`) when recording or changing
enduring decisions.

Use an effort for continuity across stages, sessions, or handoffs and for
implementation worktrees; check for an existing effort first. Follow the effort
workflow (`./awf docs effort`). Use the completion workflow
(`./awf docs completion`) during implementation for verification and commit
cadence, and before final completion or integration, even without an effort.
Native AWF skills supply the same workflows.

## Commands

- `go test ./...`: run tests.
- `go build ./...`: build packages.
- `./awf render && ./awf check`: refresh fixed AWF outputs and check them plus the knowledge bundle.

## References

- `README.md`: public usage and development entrypoints.
- `docs/topics/`: current path-routed project knowledge.
```

Change and ADR workflow cues belong in the entrypoint; detailed document lifecycles belong in their workflows.

## Optional Claude support

If Claude should read the same repository instructions, create an author-owned `CLAUDE.md` containing:

```text
@AGENTS.md
```

Preserve useful Claude-specific instructions alongside that import when needed. Neither file is created, replaced, or structurally validated by AWF. See `awf docs integration` before transitioning previously generated agent guides to author ownership.
