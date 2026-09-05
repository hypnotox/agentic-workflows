# Migrating from AWF v0.50

AWF's next source format is a manual simplification, not an in-place upgrade. Convert each repository on an ordinary review branch with the new binary available directly. Do not run the old `awf upgrade` flow against the new release.

## 1. Preserve repository-owned content

Before removing the v0.50 machinery, identify content the repository still owns:

- project-specific guidance in `AGENTS.md`, `.awf/agents-doc.yaml`, and `.awf/parts/agents-doc/`;
- topic selectors under `.awf/topics/metadata/` and topic prose under `.awf/topics/parts/`;
- authored parts or overrides elsewhere beneath `.awf/`;
- editable bodies inside generated documents;
- ordinary decisions, roadmaps, glossaries, pitfalls, or project documentation the repository wants to retain;
- ignored effort memory and any existing Git worktrees.

Do not copy AWF's generated governance boilerplate merely because it appears in the rendered guide. Preserve repository-specific instructions and useful current facts. Executable custom behavior belongs in a repository-owned script, not in Markdown.

## 2. Create the new sources

Create `.awf/project.md`:

```markdown
---
format: 1
---

# Project guidance

You are a coding agent responsible for developing and maintaining this project. Own both the immediate task and the project's long-term health.

Add the repository's identity, invariants, workflow, routine commands, and documentation pointers here.
```

Create one file per retained topic at `.awf/topics/<id>.md`:

```markdown
---
paths:
  - 'path/to/area/**'
---

# Topic title

Current guidance goes here.
```

Move the old selector paths and useful prose into that file. Remove claim IDs, `Backing:`, `Verify:`, coverage metadata, domains, and other old structural fields. AWF now interprets only `paths`.

## 3. Retire the old representation

After preservation, remove the v0.50 configuration and generated-source machinery, including the old config, lock, parts, metadata, catalogs, hooks, and upgrade scripts. Also remove or unmark the old generated `.awf/efforts/.gitignore`, `.awf/worktrees/.gitignore`, and `.awf/effort-archive/.gitignore`; the new projector replaces them with `.awf/.gitignore`. Keep detached project documents as ordinary files with AWF ownership and edit-control comments removed.

Do not delete ignored effort contents or native Git worktrees as part of this source cleanup. New AWF effort commands use `.awf/efforts/<slug>/memory.md` when present and treat extra resident files as opaque. Git worktrees are now entirely user-managed.

## 4. Render with the new binary

Invoke the downloaded new binary directly because the old repository wrapper still selects v0.50:

```sh
/path/to/new/awf render
/path/to/new/awf check
```

The first render replaces the fixed v0.50 outputs whose legacy AWF marker is still present and creates the new fixed outputs. It does not delete anything else.

`render` reports every marked file outside the new output set. Review each reported path and either:

- delete it when it is obsolete generated output, or
- remove the AWF ownership comment when it should remain repository-owned.

Repeat until `check` succeeds. An unmarked file at a fixed destination is an explicit collision; preserve or move its content, then delete the destination if AWF should generate it.

## 5. Verify the repository

Review the complete Git diff, especially the new project guidance, every retained topic, and every detached document. Then run:

```sh
/path/to/new/awf check
```

Run the repository's normal tests or gate. Commit the new `.awf` sources and fixed generated outputs together.

## Peer-agent handoff

Assign one repository per agent or clearly partition repositories. Give the agent the new release binary and require a short report containing:

- project guidance preserved in `.awf/project.md`;
- topics converted;
- documents detached and retained;
- obsolete generated files deleted;
- existing effort memory or Git worktrees left for manual ownership;
- unresolved content choices;
- final `awf check` and repository-gate results.

Repository-local review decides what content remains useful. There is no universal converter or fleet-wide compatibility state.
