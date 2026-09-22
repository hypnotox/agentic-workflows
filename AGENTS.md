# Agent guide

`awf` is a public pre-1.0 Go CLI at `github.com/hypnotox/agentic-workflows`. It delivers fixed workflow skills and embedded guides, routes repository paths to current topics, keeps local effort memory, and offers create-only document starters. Own both the requested change and the project's long-term health.

Keep the product direct and small: no policy engines, replacement configuration layers, or speculative abstractions. Preserve author ownership; never overwrite unmarked repository files except the reserved `.awf/VERSION` record, or automatically delete retired generated files. Keep AWF usable without Git, external skills, services, or network access once its pinned binary is available.

## Working here

Use `./x` during development: it runs the checkout source. The root `./awf` wrapper intentionally exercises the released bootstrap path and may run a different version.

Use `./x resolve` for explicit global topics, adding repository-relative paths for matching knowledge. Read every returned source, reuse established context, and keep affected topics current. Follow the topic workflow, available through `./x docs topics`. When authoring Markdown under `docs/`, follow the shared contract in `./x docs knowledge`.

Use the change workflow (`./x docs changes`) when brainstorming a material choice or defining a change's outcome or route, including when no change document is needed. Use the ADR workflow (`./x docs adr`) when recording or changing enduring decisions.

Use an effort for continuity across stages, sessions, or handoffs and for implementation worktrees; check for a matching active effort first. Follow the effort workflow (`./x docs effort`). Use the completion workflow (`./x docs completion`) during implementation for verification and commit cadence, and before final completion or integration, even without an effort. Native AWF skills supply the same workflows.

Use Conventional Commits with one concern per commit. Keep related implementation, tests, and documentation together. All projected content belongs in embedded template/source files, not Go string literals. Edit the owning sources and run `./x render && ./x check`; do not edit generated skills or infrastructure directly. `AGENTS.md`, optional `CLAUDE.md`, and topics are author-owned.

## Commands

- `./x test`: run the complete Go test suite.
- `./x gate`: format-check, test, and build.
- `./x render && ./x check`: refresh and check fixed outputs, validate the knowledge bundle, and check topic selectors.
- `./x resolve [<path>...]`: find applicable topics.
- `./x docs`: discover embedded workflow and adoption guides.
- `./x build`: build `bin/awf`.

## Canonical references

- `README.md`: public entrypoint and documentation index.
- `internal/docs/`: canonical adopter guides and shared workflow instructions.
- `docs/topics/`: path-routed current implementation knowledge.
- `docs/changes/`, `docs/plans/`, `docs/decisions/`: authored change definitions, implementation routes, and enduring decisions when useful; see `./x docs changes` and `./x docs adr`.
- `MIGRATING-v0.50.md`: conversion of older installations.
- `CHANGELOG.md`: release history.
