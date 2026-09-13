# Integrating AWF

Use this guide to adopt AWF, connect its check to repository-owned automation, update a pinned release, or repair an integration. AWF generates its own fixed skills and launch infrastructure. Repository agent instructions and project knowledge remain author-owned. The repository owns hooks, CI, gates, commit conventions, and every Git operation or configuration change; AWF does not manage them.

## Read before installation

The public launcher downloads its concrete release's archive and checksums, verifies and caches the binary, forwards arguments, and returns the binary's exit status:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/latest/download/awf.sh | bash -s -- docs integration
```

Reading documentation does not modify or repin a repository. Run mutating commands from the repository root. Once the pinned binary is available, AWF needs no Git, external skills, services, or network access.

## Adopt a repository

1. Inspect existing agent and contributor instructions, topics, older `.awf` sources, and fixed generated destinations. Inspect repository checks, hook scripts, CI, and the effective hook location (`git config --get core.hooksPath` in Git repositories).
2. Keep useful repository instructions in author-owned `AGENTS.md` and applicable current knowledge in `docs/topics/**/*.md`. Use `awf docs agents` for concise authoring guidance and optional Claude support. Neither `AGENTS.md` nor `CLAUDE.md` is created or replaced by AWF. If older AWF sources or generated guides exist, perform the ownership transition below before rendering.
3. Inspect `.awf/VERSION`: this exact path is reserved for AWF's renderer-version record and may be replaced even without a marker. Preserve or move any unrelated existing content before giving AWF that destination. It is not configuration, a binary pin, or a prerequisite for commands.
4. Run the selected binary's `init`. It performs the same generation as `render`, creates no descriptor or agent guide, and can be repeated under ordinary render ownership rules.
5. Run `render` and `check`, review the full diff, and run the repository's tests or gate. Commit the author-owned guidance, topics, and generated outputs together.

The fixed outputs are four skills under both `.pi/skills/` and `.claude/skills/`, the root `awf` wrapper, `.awf/bootstrap.sh`, `.awf/.gitignore`, and `.awf/VERSION`. A leading AWF comment marks ownership except for the reserved version record. Both harnesses receive the same canonical workflow instructions; the CLI guides remain available without native skills.

Rendering refuses non-regular destinations and unmarked collisions other than `.awf/VERSION`. To give AWF a colliding destination, first preserve useful content elsewhere, then remove or move the destination deliberately. Do not add an AWF marker to unreconciled authored content. If generation stops after partial writes, inspect the output and diff, retain valid files, correct the reported problem, and rerun `render`. AWF never automatically deletes retired output.

## Transition existing installations

Use the target binary directly during an incompatible transition; the old repository wrapper still selects its old release. Read its guides first. Conversion is manual and bounded to the known old layouts, not an automatic migration.

For installations with `.awf/project.md`:

1. Reconcile its body with the current `AGENTS.md`, including useful edits in either file and **unrendered descriptor edits**. Keep repository-specific instructions in `AGENTS.md`; remove superseded shared workflow prose now delivered by skills and CLI guides. Do not blindly render the old source over authored edits or discard the descriptor before comparison.
2. Remove the generated ownership marker from the retained `AGENTS.md`. Preserve useful Claude instructions; a retained `CLAUDE.md`, including an existing `@AGENTS.md` import, is now author-owned. Remove any old ownership marker there too. Neither file needs a particular layout.
3. Retire `.awf/project.md` after preserving its useful content. There is no replacement descriptor or format field.

Inspect `.awf/topics/` even if no descriptor remains. For the former format-1 layout, move its topic Markdown to `docs/topics/`, preserving nested paths and reconciling existing destinations without overwriting useful content. Topic selector semantics are unchanged. Repair references and relative links; selectors remain repository-relative, so change only selectors that actually refer to relocated paths. Remove the old location after reconciliation, including an empty retired directory.

The older v0.50 layout split topic selectors and prose under `.awf/topics/metadata/` and `.awf/topics/parts/`. Follow `MIGRATING-v0.50.md` in the release archive or the [repository copy](https://github.com/hypnotox/agentic-workflows/blob/main/MIGRATING-v0.50.md) to combine them and preserve other authored overrides. Do not move that split layout as though each file were a complete topic. Inspect legacy hooks and remove obsolete AWF calls while retaining unrelated behavior. Change or unset hook configuration only through an explicit repository decision.

`init`, `render`, `check`, and topic resolution report an actionable migration error while `.awf/project.md` or `.awf/topics/` remains; they never silently ignore that guidance. Documentation and artifact/effort commands remain available. There is no old-location fallback or format parser.

Inspect the reserved `.awf/VERSION` destination, then render and check with the target binary:

```sh
/path/to/new/awf render
/path/to/new/awf check
/path/to/new/awf resolve
/path/to/new/awf resolve <relevant-path>...
```

Review reported retired marked files. Delete obsolete ones deliberately or remove their marker to retain them as author-owned content. Preserve tracked change documents, decisions, plans, local effort memory, archives, and worktrees throughout the transition. Run the repository's checks and commit the reconciled result. Rendering updates the bootstrap pin for later wrapper commands. In AWF's own checkout, use `./x` to execute the target checkout source.

## Connect checks to automation

Add `./awf check` to the repository's existing gate and CI after checkout. It validates topic sources, generated bytes, executable state, and retired ownership markers in the working tree. It does not validate an isolated staged snapshot or prove overall integration. Keep source edits, rendering, review, staging, and commits explicit.

An optional repository-owned pre-commit hook can be:

```sh
#!/bin/sh
set -eu
exec ./awf check
```

Make it executable and integrate it into the existing hook dispatcher or configured directory. For a repository deliberately using `.githooks` with no conflicting arrangement:

```sh
chmod +x .githooks/pre-commit
git config core.hooksPath .githooks
```

Preserve unrelated hook behavior and existing `core.hooksPath` configuration. AWF does not install, activate, edit, or remove hooks. Agent commits are deliberate Git operations under repository conventions; they do not normally require a separate user request. The completion workflow owns their default cadence.

## Update the pinned release

Read the target release's guide before changing the pin:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/download/v<target>/awf.sh | bash -s -- docs integration
```

For a compatible update, explicitly select the target binary through the existing wrapper:

```sh
AWF_VERSION=<target> ./awf render
./awf check
```

The target renderer rewrites the committed bootstrap pin and `.awf/VERSION`. The version record contains the running renderer's embedded version and is checked for drift; neither its presence nor its value controls binary selection, resolution, documentation, or artifact creation. The bootstrap selects the pinned binary. Never silently substitute the latest release. For an incompatible change, follow the target's migration guide instead of assuming render is sufficient.

## Repair and confirm

Use the reported path and diff to distinguish stale marked output (render it), unmarked collisions (reconcile ownership), retired marked files (delete or unmark deliberately), and invalid topics (fix their authored source). A missing or stale version record is generated drift, repaired by render—not a request to configure AWF.

For incomplete integration, inspect repository gates, CI, hook scripts, executable modes, and effective hook configuration. AWF does not assess them. Before reporting completion, follow `awf docs completion`, including when the work uses no effort or worktree.

External engineering skills may complement AWF but are neither installed nor required by it. The optional [agentic-skills](https://github.com/hypnotox/agentic-skills) package supplies general engineering methods. If a repository requires external skills or shared doctrine, state that in its author-owned guidance rather than duplicating those methods in AWF workflows.
