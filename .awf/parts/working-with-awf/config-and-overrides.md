Configuration keys are documented in the [configuration reference](config-reference.md); authorship rules live in the [documentation standard](doc-standard.md). A convention part replaces its section body. `awf:edit` names its source; the `sectionDefault` placeholder retains the default when extending it.

awf renders self-ignoring roots for efforts, worktrees, and archives. Their descendants are local, unmanaged state; rendering and uninstall preserve them.

Efforts need durable continuity. One fixed slug owns its memory and optional opaque `scratch/`; Git owns worktree topology. Finish moves the resident to the local archive, which awf never inventories, restores, prunes, analyzes, or retains.

Declare a local document only when no standard document owns its fact. Declare it in `localDocs` or run `./awf new doc <name> <description> [--title <title>]`; it creates `docs/<name>.md`, and `api-v2` becomes `Api V2` without `--title`. Edit only its body between `awf:edit-in-place` and `awf:end`; awf owns the heading and shell. Render and check after edits; removal or uninstall preserves a present body as a sibling `.awf-bak`. For generated guidance, `awf:edit` names the owning convention part and `awf:source` names reader authority. `render.templateSourceRoot` is maintainer configuration and ordinary adopters omit it.

Use `./awf new pitfall "<Title>"` to create an authored pitfall source. Render after editing it; never edit its generated index or leaf.

Domain `paths` use anchored repository-relative globs. Topic metadata declares scoped `paths`, `applies: global`, or both.
