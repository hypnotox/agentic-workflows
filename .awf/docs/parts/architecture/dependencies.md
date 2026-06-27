## Key dependencies

- **`gopkg.in/yaml.v3`** — strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast.
- **`text/template`** (standard library) — the rendering engine; ADR-0001 owns its
  publication-safety contract.
- **`golangci-lint`** — pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.
