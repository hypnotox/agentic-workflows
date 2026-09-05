---
paths:
  - '.awf/topics/**'
  - 'internal/projector/source.go'
  - 'internal/projector/resolve.go'
  - 'internal/frontmatter/**'
  - 'internal/pathglob/**'
---

# Topic routing

Each `.awf/topics/**/*.md` file is one authoritative topic. Its relative path without `.md` is its ID. The required `paths` frontmatter contains one or more positive repository-relative patterns; unknown fields are ignored and the Markdown body is opaque.

`*` matches within one path component and `**` matches across directories. The exact sole selector `paths: ['**']` additionally declares an explicit global topic. A standalone `**` is invalid in a mixed or duplicate list; `*`, `src/**`, `**/*.go`, and other patterns remain ordinary selectors. Patterns have no negation or priority. Multiple topics may match the same path, with no hierarchy or exclusive owner.

Bare `resolve` returns explicit globals only. `resolve <path>...` returns globals plus each topic matching any argument, once per topic, in deterministic order. Arguments are lexical repository-relative paths: normalize separators, refuse absolute or escaping paths, validate them even when globals exist, and do not require targets to exist. No match is successful and prints `none`; resolution returns source locations rather than bodies.
