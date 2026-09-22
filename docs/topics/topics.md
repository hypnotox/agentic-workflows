---
type: Project Topic
title: Topic routing
description: Topic source loading, selector semantics, routing coverage, and create-only topic ownership.
paths:
  - 'docs/topics/**'
  - 'internal/artifactfs/**'
  - 'internal/projector/source.go'
  - 'internal/projector/resolve.go'
  - 'internal/frontmatter/**'
  - 'internal/pathglob/**'
  - 'internal/docs/topics.md'
---

# Topic routing

Each non-reserved `docs/topics/**/*.md` file is one authoritative topic. `index.md` and `log.md` are excluded case-insensitively at every depth, using `internal/knowledge.IsReserved`; they never require selectors. Its relative path without `.md` is its ID. The required `paths` frontmatter contains one or more positive repository-relative patterns; other fields are ignored by routing and the Markdown body is opaque. `check` separately validates the shared knowledge contract described by `awf docs knowledge`; neither type nor description changes routing. Other Markdown under `docs/` is not topic input, and there is no fallback to `.awf/topics/`.

`*` matches within one path component and `**` matches across directories. The exact sole selector `paths: ['**']` additionally declares an explicit global topic. A standalone `**` is invalid in a mixed or duplicate list; `*`, `./**`, `src/**`, `**/*.go`, and other patterns remain ordinary selectors. Patterns have no negation or priority. Multiple topics may match the same path, with no hierarchy or exclusive owner.

`internal/projector.TopicsPath` owns the topic directory used by direct loading and creation. `LoadTopics` requires neither a project descriptor nor a generated version record. It reports a manual migration diagnostic for retired `.awf/project.md`, `.awf/agents-doc.yaml`, `.awf/parts/agents-doc/`, or `.awf/topics/` locations rather than silently omitting their guidance. `internal/projector.NormalizeTopicPatterns` is the shared validation and matching-normalization contract used by source loading and `internal/artifactfs` topic creation. Creation serializes the authored selector spelling, so `./**` retains its ordinary non-global meaning even though its normalized matching form is `**`. Nested topic IDs are constrained beneath `docs/topics`, omit `.md`, and cannot end in reserved index/log basenames. Selectors remain relative to the repository root, not to the topic directory.

Bare `resolve` returns explicit globals only. `resolve <path>...` returns globals plus each topic matching any argument, once per topic, in deterministic order. `resolve --coverage <path>...` requires explicit lexical paths, deduplicates and sorts their normalized forms, reports globals separately, and reports every matching non-global topic or `none` per path. Coverage does not discover paths, judge documentation quality, or fail for gaps.

Resolve arguments are lexical repository-relative paths: normalize separators, refuse absolute or escaping paths, validate them even when globals exist, and do not require targets to exist. The embedded `docs topics` page owns adopter authoring, coverage limits, and maintenance procedures; the generated topic skills derive their substantive bodies from that same Markdown.
