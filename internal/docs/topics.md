# Working with topics

Use this workflow at the start of work in a repository and when work moves to new paths, to find and read the knowledge that applies, and when implementation changes facts a topic states. Once the context is known, reuse it for the task instead of resolving before every edit.

Commands use `./awf`; substitute the repository's AWF runner. Other workflows are cited as `awf docs <page>`, the same text as the skill `awf-<page>`.

Each non-reserved `docs/topics/**/*.md` file is a topic: one body of current project guidance plus the path selectors that make it relevant. Other Markdown under `docs/`, such as plans and decisions, is not a topic. Follow `awf docs knowledge` for shared metadata.

## Find applicable context

```sh
./awf resolve                                   # global topics only
./awf resolve internal/projector/new-file.go docs/future.md
```

Pass the repository-relative paths you will work on; they need not exist. The report lists global topics, then each path's matches as numbered references, then each number's source file with its description beneath it. Read every referenced source once. A path with no specific topic reports `none`, which is not an error; `awf help resolve` gives the output and argument rules. Routing shows matches for the paths you chose, not that documentation is complete, accurate, or fresh.

## Create and author a topic

Create a topic with an ID, which may be nested, and at least one selector. Quote globs so the shell does not expand them:

```sh
./awf new topic projection 'internal/projector/**' 'cmd/awf/*'
```

You may also write the file directly:

```markdown
---
type: Project Topic
title: Projection
description: Current projection behavior, ownership boundaries, and change guidance.
paths:
  - 'internal/projector/**'
  - 'cmd/awf/*'
---

# Projection
```

Selectors are relative to the repository root, not the topic's directory. `*` matches within one path component and `**` across directories. Selectors are positive only; several topics may match one path. The exact sole selector `paths: ['**']` makes a global topic that every `resolve` reports. `awf help new` gives the ID and selector rules.

`resolve` prints the description under each reference, so write one line that tells a reader whether the topic matters to their change. Begin the body with a focused purpose, then current behavior, ownership boundaries, and relationships that matter to future changes, then constraints and practical implications. Prefer useful orientation to an exhaustive inventory, keep change and verification guidance near its facts, and link active ADRs instead of repeating their rationale. Remove prompts and anything that does not serve the topic.

## Maintain current knowledge

Update affected topics when implementation changes their facts or instructions; active ADRs keep enduring choices and rationale. Add source, test, or verification references beside important claims when they materially reduce lookup effort, stating what they actually show; a link alone proves nothing.

Remove obsolete advice and consolidate stale or redundant guidance instead of accumulating chronology, claim IDs, proof markers, backlink registries, or a pitfalls list. Topics keep the resulting current guidance, not implementation findings.

Topics are author-owned and are not copied into generated files. After editing one, run `./awf check`, which validates topic selectors and the knowledge bundle, and commit the topic with the change it describes.
