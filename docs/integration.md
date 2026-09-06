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
   - inspect any `.awf` sources or marked generated files;
   - inspect hook scripts, `git config --get core.hooksPath`, CI, and the repository's normal checks;
   - identify useful repository-specific guidance and current implementation knowledge.
2. Preserve useful authored guidance in `.awf/project.md` and applicable `.awf/topics/**/*.md` sources before giving AWF ownership of a fixed generated destination. Do not copy obsolete generated boilerplate merely because it exists.
3. For a repository with no AWF sources, run the selected release binary with `init`. For a repository whose `.awf/project.md` and topics already exist, run `render` instead. `init` creates `.awf/project.md` and then renders; it is not a general retry command.
4. If either command stops after partial effects, inspect its output and the repository diff. Preserve every file already written, correct the reported collision or invalid source, then continue with `render` when `.awf/project.md` now exists. Do not blindly rerun `init` after it created that source.
5. Edit the sources, run `render` and `check`, and review the complete diff. Use `awf docs topics` for selector and maintenance rules.
6. Run the repository's existing tests or gate. Commit `.awf` sources and generated outputs together.

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

## Confirm the integrated result

After integrating implementation work, confirm that available evidence covers the actual combined result in the target checkout. Reuse checks whose evidence still applies, but rerun checks affected by divergence, conflict resolution, or changed integration context. Request additional review only when material uncertainty warrants it.

Before cleanup, reconcile affected active ADRs and topic links with the combined implementation, review whether any tracked plan still has a concrete execution, verification, review, or handoff use, and preserve any warranted reusable lesson in a focused fix, test, or topic. If a completed branch removes a formerly guiding plan, preserve both its committed form and later deletion through ordinary non-squash integration. Update any local memory with completion evidence and understandable references before archival. Use ordinary Git for integration and cleanup; `awf effort finish` remains only an archive move and does not assess readiness.

## Optional companion skills

External engineering skills can complement AWF, but AWF neither installs nor requires them. The optional [`agentic-skills`](https://github.com/hypnotox/agentic-skills) package provides `agentic-planning`, `agentic-implementing`, `agentic-reviewing`, `agentic-artifact-design`, and `agentic-code-design` for general engineering methods. If a repository requires particular skills, record that requirement in its own `.awf/project.md`; keep universal recommendations here rather than in every generated frame.
