## Key dependencies

- **`gopkg.in/yaml.v3`** — strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast.
- **`text/template`** (standard library) — the rendering engine; ADR-0001 owns its
  publication-safety contract.
- **`golangci-lint`** — pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.
- **`deadcode`** (`golang.org/x/tools/cmd/deadcode`) — pinned as a `go tool` dependency; the gate
  runs it (no `-test`) and `cmd/deadcodecheck` fails on any production function unreachable from a
  `main` outside `internal/testsupport/` (ADR-0063). This repo only, not part of the rendered standard.
