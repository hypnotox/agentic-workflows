## Key dependencies

- **`gopkg.in/yaml.v3`**: strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast.
- **`text/template`** (standard library): the rendering engine; ADR-0001 owns its
  publication-safety contract.
- **`github.com/go-git/go-git/v5`** (with `go-billy/v5`): pure-Go git access for `awf audit`'s
  history and working-tree reads; awf and its tests need no host `git` binary.
- **`golang.org/x/mod`**: semver comparison for the binary-version gate (ADR-0039).
- **`github.com/bmatcuk/doublestar/v4`**: the matcher behind `internal/pathglob`'s anchored
  full-path glob dialect: invariant source globs, dependency manifests, and domain `paths`
  all match through it (ADR-0077).
- **`golangci-lint`**: pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.
- **`deadcode`** (`golang.org/x/tools/cmd/deadcode`): pinned as a `go tool` dependency; the gate
  runs it (no `-test`) and `cmd/deadcodecheck` fails on any production function unreachable from a
  `main` outside `internal/testsupport/` (ADR-0063). This repo only, not part of the rendered standard.
- **Pi 0.80.9 and TypeBox 1.1.38**: peer APIs used only by the generated Pi extension at runtime;
  they are supplied by the adopter's Pi installation and are not dependencies of the awf binary.
- **Docker, Node, TypeScript, and c8**: pinned repo-only test dependencies under
  `tools/pi-extension-test/`; no host npm installation is used.
- **`gremlins`** (`github.com/go-gremlins/gremlins`): pinned as a `go tool` dependency; `./x mutants`
  runs it under the deterministic `.gremlins.yaml` config and `cmd/mutants` reports survived mutants
  (ADR-0066). Advisory only; never part of the gate. This repo only, not part of the rendered standard.
