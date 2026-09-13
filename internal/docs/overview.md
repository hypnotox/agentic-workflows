# AWF guide

AWF delivers fixed workflow skills and embedded adopter guides. It also routes repository paths to current topics, reports routing coverage for chosen paths, keeps optional local effort memory, and creates optional intent, specification, plan, ADR, and topic starters. It does not own Git operations, hooks, CI, repository gates, or the meaning of authored Markdown.

## Ownership

Authors edit `AGENTS.md` directly and may maintain an optional `CLAUDE.md`. AWF neither generates these files nor requires a Markdown layout. Current project knowledge belongs in `docs/topics/`, change definitions in `docs/changes/<slug>/`, plans in `docs/plans/`, and enduring decisions in `docs/decisions/`. These remain author-owned, without generated copies. `.awf/project.md` is retired; there is no replacement configuration.

AWF generates four substantive skills for Pi and Claude, the root `awf` wrapper, `.awf/bootstrap.sh`, `.awf/.gitignore`, and `.awf/VERSION` from embedded content. Workflow pages and skill bodies share one canonical instruction source. A leading AWF marker identifies generated ownership except for the exact reserved `.awf/VERSION` path. That record reports the renderer's version; the bootstrap, not the record, selects the binary.

Do not edit generated files as their source. Use the repository's documented AWF runner to refresh and check them; examples use `./awf`:

```sh
./awf render
./awf check
```

`init` is a first-install entrypoint to the same generation. Render replaces regular marked outputs and the reserved version record, refuses other unmarked collisions, and reports retired marked files without deleting them. Check validates topics and working-tree generated output; it does not prove overall repository integration, a staged snapshot, Git history, or project-specific behavior. Review and commit related authored and generated changes together.

## Discover guides

Read the workflow relevant to the task, through native AWF skills or the CLI. Reuse established guidance rather than reloading it before every action:

```text
awf docs integration  adopt AWF, update versions, connect repository automation, or repair integration
awf docs agents       author concise repository instructions and optional Claude support
awf docs topics       discover, read, and maintain path-routed current knowledge
awf docs effort       keep continuity, memory, notes, and worktree handoffs
awf docs changes      define substantial changes and use optional intent, spec, plan, and ADR documents
awf docs completion   verify and commit coherent units during implementation, review findings, integrate, and clean up
```

Use the completion workflow during implementation for verification and commit cadence, not only at final completion; it applies even without an effort. Small self-contained changes without worktree isolation can remain effort-free; continuity across stages, sessions, or handoffs and implementation worktrees use coordinating efforts.

Guides are embedded in the binary and work before installation, without Git, repository sources, external skills, or network access. Reading them does not initialize, render, repin, or otherwise modify a repository. Navigate with `awf docs ...`; their canonical Markdown lives under `internal/docs/` in AWF's repository, not in an adopter checkout.

Use `awf new` for create-only starters. Documents and their relationships are optional and author-owned, not derived or synchronized by AWF. Effort contents remain opaque local state. Existing installations should read `awf docs integration` before retiring old sources or transferring generated agent guides to author ownership.
