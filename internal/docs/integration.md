# Integrating AWF

Use this guide to adopt AWF in an existing repository, connect its check to repository-owned automation, update the pinned release, or repair a partial integration. AWF owns its fixed marked outputs and implements topic resolution, local effort memory, and optional scaffolds. AWF supplies default agent commit guidance in the shared generated `AGENTS.md` frame. The repository owns authored guidance, local commit conventions and overrides, hooks, CI, gates, and every Git configuration change. Agents execute Git operations under those conventions; the CLI performs no Git operations.

## Read guidance before initialization

The public launcher for a published release downloads that release's archive and checksums, verifies the binary, caches it, forwards the requested arguments, and returns the binary's exit status:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/latest/download/awf.sh | bash -s -- docs integration
```

This documentation command does not change the current directory or repository. Run later commands from the repository root. The launcher mutates a repository only when you explicitly request a mutating command such as `init`.

## Adopt an existing repository

1. Inspect the repository before writing anything:
   - read existing agent instructions and contributor documentation;
   - inspect `.awf/project.md`, `docs/topics/`, any older `.awf` sources, and marked generated files;
   - inspect hook scripts, `git config --get core.hooksPath`, CI, and the repository's normal checks;
   - identify useful repository-specific guidance and current implementation knowledge.
2. Preserve useful authored guidance in `.awf/project.md` and applicable `docs/topics/**/*.md` sources before giving AWF ownership of a fixed generated destination. Do not copy obsolete generated boilerplate merely because it exists.
3. For a repository with no AWF sources, run the selected release binary with `init`. For a repository whose `.awf/project.md` and topics already use the accepted source format, run `render` instead. Follow the migration procedure below for format 1. `init` creates `.awf/project.md` and then renders; it is not a general retry command.
4. If either command stops after partial effects, inspect its output and the repository diff. Preserve every file already written, correct the reported collision or invalid source, then continue with `render` when `.awf/project.md` now exists. Do not blindly rerun `init` after it created that source.
5. Edit the sources, run `render` and `check`, and review the complete diff. Use `awf docs topics` for selector and maintenance rules.
6. Run the repository's existing tests or gate. Commit `.awf/project.md`, topic sources, and generated outputs together.

An unmarked file at a fixed destination is repository-owned and AWF refuses to overwrite it. Transfer ownership explicitly: first preserve useful content in AWF sources or another repository-owned file, then remove or move the destination and render the marked replacement. Never add an AWF marker to content that has not been reconciled with its source.

## Add the working-tree check to automation

Add `./awf check` to the repository's existing gate and CI after the checkout is available. Keep source edits, rendering, review, staging, and commits explicit; neither `check` nor a hook performs them. For agents, explicit commits are deliberate agent-run Git operations, not operations that require a separate user request.

An optional repository-owned pre-commit hook can be as small as:

```sh
#!/bin/sh
set -eu
exec ./awf check
```

Make the script executable and integrate it into the repository's existing hook dispatcher or configured hook directory. If the repository deliberately stores hooks in `.githooks` and has no conflicting arrangement, activation is a repository operation:

```sh
chmod +x .githooks/pre-commit
git config core.hooksPath .githooks
```

Do not replace an existing `core.hooksPath` or hook body without preserving its unrelated behavior. AWF does not create, activate, edit, or remove hooks. `check` examines AWF files in the current working tree; it does not validate an isolated staged commit and does not prove that hooks, CI, or the overall integration are complete.

## Repair ownership and integration

Use the reported path and `git diff` to distinguish these cases:

- **Current marked output is stale:** run the selected binary's `render`, then review it.
- **Unmarked fixed destination collides:** preserve its useful content, explicitly transfer or retain ownership, and only then render.
- **Marked file is retired:** `render` reports it and `check` fails. Delete it if obsolete, or remove the marker to retain it as repository-owned content. AWF never deletes it.
- **Generated output and source disagree:** edit the source, not the generated destination, and render again.
- **Integration is incomplete:** inspect the repository's gate, CI, hook scripts, executable modes, and effective `core.hooksPath`; AWF does not assess these.

For repositories converted from v0.50, follow `MIGRATING-v0.50.md` in the target release archive or the [repository copy](https://github.com/hypnotox/agentic-workflows/blob/main/MIGRATING-v0.50.md). Inspect every legacy hook and the effective `core.hooksPath`. Remove obsolete AWF calls while preserving unrelated checks, and change or unset hook configuration only through an explicit repository decision. AWF performs no legacy cleanup or Git configuration.

## Update the pinned release

Read the target release's guide before changing the repository pin. For a published exact version, use its launcher without invoking a mutating command:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/download/v<target>/awf.sh | bash -s -- docs integration
```

Reading guidance only populates the user cache; it does not repin the repository. For a compatible source format, explicitly select the target binary through the existing wrapper and render:

```sh
AWF_VERSION=<target> ./awf render
./awf check
```

The target binary rewrites the committed bootstrap pin. Review and commit the source and generated changes together. Never substitute the latest release silently for a repository pin. If the target changes the source format, follow its migration guide instead of assuming a same-format render.

## Migrate source format 1 to 2

Source format 2 moves canonical topics from `.awf/topics/` to `docs/topics/`. The topic Markdown and selector syntax are unchanged. `resolve`, `render`, and `check` reject format 1 rather than silently treating its old topic directory as an empty topic set.

Use the target format-2 binary directly from the repository root during this transition, not the old pinned repository wrapper:

1. Move the topics from `.awf/topics/` to `docs/topics/`, preserving nested paths and one authoritative copy. Reconcile any existing destination files deliberately without overwriting useful content. Repositories without topics need no directory move.
2. Repair references and relative Markdown links affected by the move. Selectors remain repository-relative; change only selectors that refer to relocated paths, not all selectors merely because their topic moved.
3. Change `.awf/project.md` frontmatter from `format: 1` to `format: 2`, preserving its body. Leave effort memory, effort-local plans, worktrees, and archives untouched. Tracked plans and ADRs remain in `docs/plans/` and `docs/decisions/`.
4. Render and check with the target binary, then inspect topic resolution for the paths the moved topics should cover:

   ```sh
   /path/to/new/awf render
   /path/to/new/awf check
   /path/to/new/awf resolve
   /path/to/new/awf resolve <relevant-path>...
   ```

5. Review the moved sources, repaired references, and generated diff; run the repository's normal checks and commit the transition together. Rendering updates the repository wrapper's pin for subsequent commands.

There is no old-location fallback or automatic conversion. In AWF's own source repository, the embedded CLI manuals also move to `internal/docs/`; adopting repositories do not need that package or copies of those manuals.

## Confirm the integrated result

Use `awf docs effort` for the authoritative completion and integration procedure. It applies whether or not the work uses effort memory or worktrees.

## Optional companion skills

External engineering skills can complement AWF, but AWF neither installs nor requires them. The optional [`agentic-skills`](https://github.com/hypnotox/agentic-skills) package provides `agentic-planning`, `agentic-implementing`, `agentic-reviewing`, `agentic-artifact-design`, and `agentic-code-design` for general engineering methods. If a repository requires particular skills, record that requirement in its own `.awf/project.md`; keep universal recommendations here rather than in every generated frame.
