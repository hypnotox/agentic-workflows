# AWF guide

AWF delivers fixed workflow skills and embedded adopter guides. It also routes repository paths to current topics, reports globals and per-argument matches with response-local source references, keeps optional local effort memory, creates native Git worktrees for active efforts, and creates optional intent, specification, plan, ADR, and topic starters. Git use is limited to worktree creation and effort-root discovery. Agents retain synchronization, integration, removal, and branch cleanup; AWF does not own hooks, CI, repository gates, or the meaning of authored Markdown.

## Ownership

Authors edit `AGENTS.md` directly and may maintain an optional `CLAUDE.md`. AWF neither generates these files nor requires a Markdown layout. Current project knowledge belongs in `docs/topics/`, change definitions in `docs/changes/<slug>/`, plans in `docs/plans/`, and enduring decisions in `docs/decisions/`. These remain author-owned, without generated copies. `.awf/project.md` is retired; there is no replacement configuration.

AWF generates five workflow skills for `.agents/skills/` (serving Codex, Pi, and omp) and Claude (`.claude/skills/`), the root `awf` wrapper, `.awf/bootstrap.sh`, `.awf/.gitignore`, and `.awf/VERSION` from embedded content. Workflow pages and both skill directories' bodies share one canonical instruction source. A leading AWF marker identifies generated ownership except for the exact reserved `.awf/VERSION` path. That record reports the renderer's version; the bootstrap, not the record, selects the binary.

Do not edit generated files as their source. Use the repository's documented AWF runner to refresh and check them; examples use `./awf`:

```sh
./awf render
./awf check
```

`init` is a first-install entrypoint to the same generation. Render replaces regular marked outputs and the reserved version record, refuses other unmarked collisions, and reports retired marked files without deleting them. Check validates the `docs/` knowledge bundle, topic selectors, and working-tree generated output; it does not prove overall repository integration, a staged snapshot, Git history, or project-specific behavior. Review and commit related authored and generated changes together.

## Discover guides

Read the workflow relevant to the task, through native AWF skills or the CLI; the skill `awf-<page>` carries the same text as `awf docs <page>`, and its description says when to use it. Reuse established guidance rather than reloading it before every action:

```text
awf docs integration  adopt AWF, update versions, connect repository automation, or repair integration
awf docs knowledge    author the OKF bundle, metadata, indexes, and logs under docs/
awf docs agents       author concise repository instructions and optional Claude support
awf docs topics       find, read, and maintain the topics that apply to the paths you work on
awf docs effort       keep continuity across stages and sessions, decide on a worktree, and hand off
awf docs changes      settle a material choice with the user before planning, and write intent, specification, or plan
awf docs adr          record consequential decisions and maintain their authority and lifecycle
awf docs completion   verify and commit each unit, review the result, confirm before publishing, and finish work
```

Use the completion workflow during implementation for verification and commit cadence, not only at final completion; it applies even without an effort. Small self-contained changes without worktree isolation can remain effort-free; continuity across stages, sessions, or handoffs and implementation worktrees use coordinating efforts.

Guides are embedded in the binary and work before installation, without Git, repository sources, external skills, or network access. Reading them does not initialize, render, repin, or otherwise modify a repository. Navigate with `awf docs ...`; their canonical Markdown lives under `internal/docs/` in AWF's repository, not in an adopter checkout.

Use `awf new` for create-only starters. Tracked documents are created in the caller's checkout; effort commands share primary-checkout memory when invoked from linked Git checkouts. Documents and their relationships are optional and author-owned, not derived or synchronized by AWF. Effort contents remain opaque local state. Only worktree creation requires Git; other commands and memory-only use remain available without it. See `awf docs effort` for worktree naming and continuity. Existing installations should read `awf docs integration` before retiring old sources or transferring generated agent guides to author ownership.
