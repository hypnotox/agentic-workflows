# Migrating from AWF v0.50

AWF's source format 2 is a manual simplification, not an in-place upgrade. Convert each repository on an ordinary review branch with the new binary available directly. Do not run the old `awf upgrade` flow against the new release. Repositories already using source format 1 should instead follow `awf docs integration` from the target binary.

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
format: 2
---

# Project guidance

You are a coding agent responsible for developing and maintaining this project. Own both the immediate task and the project's long-term health.

Add the repository's identity, invariants, workflow, routine commands, and documentation pointers here.
```

Create one file per retained topic at `docs/topics/<id>.md`:

```markdown
---
paths:
  - 'path/to/area/**'
---

# Topic title

Current guidance goes here.
```

Move the old selector paths and useful prose into that file. Remove claim IDs, `Backing:`, `Verify:`, coverage metadata, domains, and other old structural fields. AWF now interprets only `paths`. Repair references and relative links affected by the move; selectors remain repository-relative.

For genuinely repository-wide current knowledge, use the exact sole global declaration:

```yaml
paths:
  - '**'
```

The standalone `**` entry is invalid in a mixed or duplicate list. A standalone `*` is still an ordinary root-level wildcard, including in mixed lists; `src/**` and `**/*.go` are also ordinary path selectors.

Review retained decision documents deliberately. Keep accepted decisions that still govern the repository as active ADRs in `docs/decisions/`, link them from applicable topics, and retire only withdrawn or superseded records after preserving any still-binding substance. Current topics describe implemented behavior and practical implications; active ADRs own the enduring choices and rationale.

New implementation plans belong in `docs/changes/<slug>/plan.md`, alongside optional intent and specification documents and independent of effort memory. Do not move or delete existing effort-local plans automatically. When older work resumes, an agent may deliberately move a still-useful plan into the tracked location and update references while preserving one authoritative copy.

## 3. Retire the old representation

Before removing the v0.50 machinery, inspect repository hook scripts and the effective hook location:

```sh
git config --get core.hooksPath
```

Remove obsolete calls to old AWF hook commands while preserving unrelated repository checks and dispatch behavior. If `core.hooksPath` points to a retired AWF-owned directory, deliberately replace or unset it only after retaining any repository-owned hooks it still serves. The new AWF does not edit hook scripts, activate hooks, change Git configuration, or perform this cleanup.

After preservation, remove the v0.50 configuration and generated-source machinery, including the old config, lock, parts, metadata, catalogs, obsolete AWF hook files, and upgrade scripts. Also remove or unmark the old generated `.awf/efforts/.gitignore`, `.awf/worktrees/.gitignore`, and `.awf/effort-archive/.gitignore`; the new projector replaces them with `.awf/.gitignore`. Keep detached project documents as ordinary files with AWF ownership and edit-control comments removed.

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

Use the new binary's context query when reviewing converted topics:

```sh
/path/to/new/awf resolve                              # explicit globals only
/path/to/new/awf resolve path/to/file.go              # globals plus path matches
/path/to/new/awf resolve --coverage path/to/file.go   # globals and per-path specific coverage
```

Resolution returns source locations rather than topic bodies. Coverage is informational and operates only on the explicit lexical paths supplied by the caller; it is not a documentation-completeness gate.

## 5. Verify the repository

Review the complete Git diff, especially the new project guidance, every retained topic, and every detached document. Confirm that current topics retain all current knowledge the repository still needs. Then run:

```sh
/path/to/new/awf check
```

Run the repository's normal tests or gate. Commit `.awf/project.md`, `docs/topics/`, and fixed generated outputs together.

## Peer-agent handoff

Assign one repository per agent or clearly partition repositories. Give the agent the new release binary and require a short report containing:

- project guidance preserved in `.awf/project.md`;
- topics converted into `docs/topics/`;
- documents detached and retained;
- obsolete generated files deleted;
- existing effort memory or Git worktrees left for manual ownership;
- unresolved content choices;
- final `awf check` and repository-gate results.

Repository-local review decides what content remains useful. There is no universal converter or fleet-wide compatibility state.
