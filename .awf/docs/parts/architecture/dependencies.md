## Key dependencies

- **`gopkg.in/yaml.v3`**: strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast.
- **`encoding/json`, `crypto/sha256`, process execution, and filesystem primitives** (standard library): parse,
  fingerprint, and validate tracked configuration and local effort state without a database or background daemon.
- **`text/template`** (standard library): the rendering engine; ADR-0001 owns its
  publication-safety contract.
- **`github.com/go-git/go-git/v5`** (with `go-billy/v5`): the pure-Go implementation behind the
  seam's in-process object reads. Native `git` is a runtime and test prerequisite for repository
  control-root resolution, refs and worktree topology, and working-tree truth. Both are backends
  of `internal/git` and nothing else: no other production package may import the library or
  construct a git subprocess, and `internal/testsupport/gitfixture` is the single exception on
  the test side, which the zero-internal-deps rule forces rather than permits.
- **`golang.org/x/mod`**: semver comparison for the binary-version gate (ADR-0039).
- **`github.com/bmatcuk/doublestar/v4`**: the matcher behind `internal/pathglob`'s anchored
  full-path glob dialect: invariant source globs, dependency manifests, and domain `paths`
  all match through it (ADR-0077).
- **`github.com/BurntSushi/toml`**: encodes and decodes the Codex adapter's TOML agent profiles
  (`internal/project/agent.go`, the `codex` target's `TOMLAgentDialect`).
- **`golangci-lint`**: pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.
- **`deadcode`** (`golang.org/x/tools/cmd/deadcode`): pinned as a `go tool` dependency; the gate
  runs it (no `-test`) and `cmd/deadcodecheck` fails on any production function unreachable from a
  `main` outside `internal/testsupport/` (ADR-0063). This repo only, not part of the rendered standard.
- **Pi ai/TUI, coding-agent, Remote Pi, and TypeBox**: peer APIs used only by generated Pi extensions at runtime; adopters supply them and they are not awf-binary dependencies. Subagent and handoff factories retain their own guards. The selected `effort-workflow` Pi companion additionally capability-detects command-context `changeCwd` before any CWD, activity, or memory mutation; a missing capability visibly degrades. Capability presence is final authority for the companion, with no foreign package publication, installation topology, or version floor. Remote Pi name override is separately negotiated, and its absence keeps complete metadata publication advisory and metadata-only.
- **Docker, Node, TypeScript, and c8**: pinned repo-only test dependencies under
  `tools/pi-extension-test/`; no host npm installation is used.
- **`gremlins`** (`github.com/go-gremlins/gremlins`): pinned as a `go tool` dependency; `./x mutants`
  runs it under the deterministic `.gremlins.yaml` config and `cmd/mutants` reports survived mutants
  (ADR-0066). Advisory only; never part of the gate. This repo only, not part of the rendered standard.

`internal/manifest` owns schema-sensitive lock decoding, while `internal/migrate` owns generation-31 removal of retired ADR routing payload.
