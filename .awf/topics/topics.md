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

`*` matches within one path component and `**` matches across directories. Patterns have no negation or priority. Multiple topics may match the same path.

`resolve` treats arguments as lexical repository-relative paths. It normalizes separators, refuses absolute or escaping paths, does not require targets to exist, and returns the matching topic IDs and source paths in deterministic order. No match is successful.
