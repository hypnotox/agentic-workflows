## Key dependencies

- **`gopkg.in/yaml.v3`** — strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast rather than rendering silently wrong output.
- **`text/template`** (standard library) — the rendering engine, always executed with
  `missingkey=zero` so an unset optional var collapses to empty instead of leaking a token.
- **`golangci-lint`** — pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.
