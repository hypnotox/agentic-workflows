# AWF guide

AWF projects a small repository-owned source tree into fixed agent guidance. It also routes repository paths to current topics, inspects routing coverage for explicitly chosen paths, keeps optional local effort memory, and creates optional plan, decision-record, and topic starters. AWF does not own Git hooks, Git operations, CI, repository gates, or the meaning of authored Markdown.

## Sources and generated files

Authors maintain `.awf/project.md` and `.awf/topics/**/*.md`. AWF renders a fixed set of agent entrypoints, skill entrypoints, and repository launch scripts from those sources. A leading AWF marker identifies generated ownership.

Never edit a marked generated file as its source. Edit `.awf/project.md` or the applicable topic, then run the repository's AWF command:

```sh
./awf render
./awf check
```

Commit the sources and generated outputs together. `render` replaces current marked outputs but refuses an unmarked collision. It reports retired marked files without deleting them. `check` validates the working-tree AWF sources and generated files; it does not validate the repository's complete integration, staged snapshot, Git history, or project-specific behavior.

## Guides

Read a guide when its workflow becomes relevant:

```text
awf docs integration  adopt AWF, integrate repository-owned hooks and CI, update versions, or repair integration
awf docs topics       discover, author, and maintain path-routed current project knowledge
awf docs effort       use optional effort memory, plans, ADRs, and manual worktree conventions
```

The guides are embedded in the binary and work before initialization and without Git, repository sources, external skills, services, or network access. Reading them does not initialize, render, repin, or otherwise modify a repository. Navigate between pages with `awf docs ...`; the Markdown source paths present in the AWF repository need not exist in an adopter's checkout.

Use `awf new` to create effort, plan, ADR, or topic starters. Plans and ADRs are independent tracked aids and require no effort; effort memory may reference them. See `awf docs effort` for their distinct lifecycles and `awf docs topics` for topic creation and informational coverage inspection.
