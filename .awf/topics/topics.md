---
paths:
  - '.awf/topics/**'
  - 'internal/artifactfs/**'
  - 'internal/projector/source.go'
  - 'internal/projector/resolve.go'
  - 'internal/frontmatter/**'
  - 'internal/pathglob/**'
  - 'docs/topics.md'
---

# Topic routing

Each `.awf/topics/**/*.md` file is one authoritative topic. Its relative path without `.md` is its ID. The required `paths` frontmatter contains one or more positive repository-relative patterns; unknown fields are ignored and the Markdown body is opaque.

`*` matches within one path component and `**` matches across directories. The exact sole selector `paths: ['**']` additionally declares an explicit global topic. A standalone `**` is invalid in a mixed or duplicate list; `*`, `./**`, `src/**`, `**/*.go`, and other patterns remain ordinary selectors. Patterns have no negation or priority. Multiple topics may match the same path, with no hierarchy or exclusive owner.

`internal/projector.NormalizeTopicPatterns` is the shared validation and matching-normalization contract used by source loading and `internal/artifactfs` topic creation. Creation serializes the authored selector spelling, so `./**` retains its ordinary non-global meaning even though its normalized matching form is `**`. Nested topic IDs are constrained beneath `.awf/topics` and omit `.md`.

Bare `resolve` returns explicit globals only. `resolve <path>...` returns globals plus each topic matching any argument, once per topic, in deterministic order. `resolve --coverage <path>...` requires explicit lexical paths, deduplicates and sorts their normalized forms, reports globals separately, and reports every matching non-global topic or `none` per path. Coverage does not discover paths, judge documentation quality, or fail for gaps.

Resolve arguments are lexical repository-relative paths: normalize separators, refuse absolute or escaping paths, validate them even when globals exist, and do not require targets to exist. The embedded `docs topics` page owns adopter authoring, coverage limits, and maintenance procedures; generated skills only route to it.
