# Working with topics

Topics are AWF's current project-knowledge layer. Each `.awf/topics/**/*.md` file owns one body of current guidance and the positive path selectors that make it relevant. AWF returns source locations so readers use the authored files rather than generated copies.

## Discover applicable context

Use the repository's documented AWF runner when repository context is needed:

```sh
./awf resolve
./awf resolve internal/projector/new-file.go docs/future.md
```

Bare `resolve` returns explicit global topics only. A path query returns globals plus every topic matching any supplied path, once per topic, in deterministic order. Read every returned source. Once the applicable context is known, reuse it during the task rather than resolving again before every edit.

Arguments are lexical repository-relative paths. They need not exist. AWF normalizes separators and rejects absolute paths or paths that escape the repository. A successful query with no matches prints `none`.

## Author a topic

Create one ordinary Markdown file under `.awf/topics/`. Its relative path without `.md` is the topic ID:

```markdown
---
paths:
  - 'internal/projector/**'
  - 'cmd/awf/*'
---

# Projection

Current implementation facts and guidance go here.
```

`*` matches within one path component and `**` matches across directories. Selectors are positive; there is no negation, priority, or exclusive owner. Several topics may match the same path.

The exact sole selector `paths: ['**']` declares an explicit global topic:

```yaml
paths:
  - '**'
```

A standalone `**` is invalid in a mixed or duplicate selector list. Patterns such as `*`, `src/**`, and `**/*.go` remain ordinary path selectors rather than global declarations.

Choose selectors for the paths whose work needs the knowledge. Keep repository-wide guidance global only when it genuinely applies to every change. AWF interprets the `paths` field but treats the Markdown body and unknown frontmatter fields as opaque authored content.

## Maintain current knowledge

Update affected topics when implementation changes their facts or instructions. Preserve useful decision rationale in the most specific current topic after a decision is implemented; future agents should not need an active ADR or historical transcript to understand the current state. Remove obsolete claims instead of accumulating chronology.

After source edits:

```sh
./awf render
./awf check
```

Review the source and generated diff and commit them together. Topic sources remain authoritative even though generated AGENTS and skills provide discovery cues. Use `awf docs integration` for ownership collisions, CI, and version updates.
