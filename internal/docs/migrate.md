# Migrating older installations

Use this guide when a repository already carries an older AWF installation: a `.awf/project.md` descriptor, a `.awf/topics/` tree, the v0.50 `.awf/agents-doc.yaml` layout, documents written before the knowledge-bundle contract, or output and scripts from a release with a since-changed contract. First-time adoption and compatible updates are in `awf docs integration`.

Every transition here is manual and bounded to known old layouts; no command converts a repository. `init`, `render`, `check`, and topic resolution stop with a migration error while `.awf/project.md`, `.awf/agents-doc.yaml`, `.awf/parts/agents-doc/`, or `.awf/topics/` remains, rather than silently ignoring that guidance. Documentation, starter, and effort commands keep working.

## Before you start

Convert each repository on an ordinary review branch. Invoke the target binary directly, for example a downloaded archive's `awf` or the target release's launcher, because the old repository wrapper still selects its old release:

```sh
curl -fsSL https://github.com/hypnotox/agentic-workflows/releases/download/v<target>/awf.sh | bash -s -- docs migrate
```

The examples below write `/path/to/new/awf` for that binary. Do not run an old `awf upgrade` flow against a new release.

Throughout, preserve tracked change documents, decisions, plans, local effort memory, effort archives, and existing Git worktrees. AWF does not adopt or relocate existing worktrees, and the conversion does not delete effort contents.

## Descriptor installations

For installations with `.awf/project.md` (former formats 1 and 2):

1. Reconcile its body with the current `AGENTS.md`, including useful edits in either file and **unrendered descriptor edits**. Keep repository-specific instructions in `AGENTS.md` and remove superseded shared workflow prose now delivered by the skills. Do not render the old source over authored edits or discard the descriptor before comparing.
2. Remove the generated ownership marker from the retained `AGENTS.md`. A retained `CLAUDE.md`, including an `@AGENTS.md` import, is now author-owned too; remove any old marker there. Neither file needs a particular layout; see `awf docs agents`.
3. Retire `.awf/project.md` after preserving its useful content. There is no replacement descriptor or format field.

Inspect `.awf/topics/` even when no descriptor remains. In the former format-1 layout each file is a complete topic: move it to `docs/topics/`, preserving nested paths and reconciling existing destinations without overwriting useful content. Selector semantics are unchanged and selectors stay repository-relative, so change only selectors that refer to relocated paths. Repair references and relative links, then remove the old location, including an empty directory.

## The v0.50 layout

v0.50 kept authored agent guidance in `.awf/agents-doc.yaml` with overrides under `.awf/parts/agents-doc/`, and split each topic into selectors under `.awf/topics/metadata/` and prose under `.awf/topics/parts/`. Do not move that split layout as though each file were a complete topic.

1. **Identify what the repository owns.** Look for project-specific guidance in `AGENTS.md`, `.awf/agents-doc.yaml`, and `.awf/parts/agents-doc/`; topic selectors and prose; authored parts or overrides elsewhere beneath `.awf/`; editable bodies inside generated documents; and decisions, roadmaps, glossaries, or other project documents worth keeping. Do not copy AWF's generated governance boilerplate merely because it appears in the rendered guide. Executable custom behavior belongs in a repository-owned script, not in Markdown.
2. **Write the guidance directly.** Reconcile useful instructions into author-owned `AGENTS.md`, remove its generated marker and superseded shared workflow prose, and keep useful Claude instructions in an optional `CLAUDE.md`. Use `awf docs agents`.
3. **Combine each topic.** Create one `docs/topics/<id>.md` per retained topic, with its selectors as `paths` and its prose as the body, following `awf docs topics` and `awf docs knowledge`. Remove claim IDs, `Backing:`, `Verify:`, coverage metadata, domains, and other old structural fields; routing reads only `paths`.
4. **Keep decisions deliberately.** Keep still-governing decisions as active ADRs in `docs/decisions/` and link them from applicable topics; retire only withdrawn or superseded records after preserving any still-binding substance. A legacy use of the word accepted does not by itself establish activation. Convert their metadata as described in [Documentation metadata](#documentation-metadata). Existing effort-local plans stay where they are; move a still-useful one into `docs/plans/` only deliberately, keeping one authoritative copy.
5. **Clean up hooks.** Inspect hook scripts and the effective hook location (`git config --get core.hooksPath`). Remove obsolete calls to old AWF hook commands while preserving unrelated checks and dispatch. If `core.hooksPath` points to a retired AWF-owned directory, replace or unset it only after retaining any repository-owned hooks it still serves. AWF does not edit hooks or Git configuration.
6. **Retire the old machinery.** Remove the old config, lock, parts, metadata, catalogs, obsolete AWF hook files, upgrade scripts, and the `.awf/topics/` tree once every retained topic has a complete home in `docs/topics/`. Do not create a `.awf/project.md` replacement. Remove or unmark the old generated `.awf/efforts/.gitignore`, `.awf/worktrees/.gitignore`, and `.awf/effort-archive/.gitignore`; `.awf/.gitignore` replaces them. Keep detached project documents as ordinary files with AWF ownership and edit-control comments removed.

When several repositories are converted by separate agents, give each one repository and ask for a short report: guidance preserved, topics converted, documents detached and retained, generated files deleted, effort memory or worktrees left for manual ownership, unresolved content choices, and the final `check` and repository-gate results. Repository-local review decides what content remains useful.

The original v0.50 conversion guide is preserved at the [v0.62.0 release](https://github.com/hypnotox/agentic-workflows/blob/v0.62.0/MIGRATING-v0.50.md).

## Documentation metadata

Ordinary Markdown recursively under `docs/` forms the knowledge bundle described in `awf docs knowledge`. Documents written before that contract need these edits; no command makes them:

1. Add a string `type` and a meaningful `description` to every concept, preserving existing fields, bodies, links, and topic selectors. `title` is optional.
2. For each ADR in `docs/decisions/`, move legacy `status: pending|accepted|active` to `decision_status` and add the OKF `status` mapped in `awf docs adr`: pending and accepted map to draft, active maps to stable. Renaming a field does not establish new agreement or activation; active predecessors keep governing until their supersession and retirement conditions are met.
3. If both status fields already exist, or metadata conflicts with the record's documented authority, reconcile that record explicitly from established agreement and implementation evidence. Do not guess, overwrite a value, or treat a previously absent state as approval.
4. A legacy concept named `index.md` or `log.md` collides with a reserved name. Rename it to a non-reserved basename and repair references, preserving its body and selectors.

## Render, review, and commit

Inspect `.awf/VERSION` before rendering: that exact path is reserved for AWF's version record and may be replaced without a marker, so move any unrelated content first. Then run:

```sh
/path/to/new/awf render
/path/to/new/awf check
/path/to/new/awf resolve <relevant-path>...
```

The first render replaces fixed outputs that still carry an AWF marker and creates the new ones. It reports every marked file outside the current output set without deleting it: delete obsolete generated files, or remove the marker from files the repository should keep. Repeat until `check` succeeds. Resolve the topics for representative paths to confirm converted selectors route where intended.

Review the complete diff, especially agent guidance, every retained topic, and detached documents, and confirm that the topics keep all current knowledge the repository still needs. Run the repository's tests or gate, follow `awf docs completion`, and commit the guidance, `docs/` changes, and generated outputs together. Rendering also updates the bootstrap pin, so later `./awf` commands run the target release.

## Release-specific changes

### Removed `.pi/skills` output

Installations rendered before this output was removed keep marked `.pi/skills/awf-*` files, which `check` reports as unmanaged until removed. Pi and omp read `.agents/skills`. Remove the old copies with `rm -r .pi/skills/awf-*`.

### Resolve output in 0.60

AWF 0.60 replaced the flat topic list and the separate `resolve --coverage` report. Update scripts, prompts, hooks, and documentation that parsed the old TSV output or passed `--coverage` to call `resolve <path>...` and read its `globals`, per-argument `paths`, and `references` sections. Gaps remain nonfatal, and option-like arguments are now usage errors rather than paths.
