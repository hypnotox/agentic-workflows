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
  the test side, which the zero-internal-deps rule forces rather than permits. The seam walks audit
  ranges through one rich-commit visitor without audit policy. Historical audit enumerates committed
  path and mode metadata through this seam without reading blobs, then its light load reads only
  configuration and schema controls. A one-shot heavy loader reads the exact ADR and topic authority
  selection at first use, releases its captured selection immediately, and the audit-owned alias-aware
  store releases sources and parsed universes at their separately counted final consumers. Exact selected reads reject unsafe, duplicate, missing, outside-root,
  and unsupported paths. First-parent changed paths are separate merge relevance evidence and never
  populate ordinary commit changes; current and staged checks retain complete snapshots and their full
  marker, coverage, and domain-sidecar projection.
- **`golang.org/x/mod`**: semver comparison for the binary-version gate (ADR-0039).
- **`github.com/bmatcuk/doublestar/v4`**: the matcher behind `internal/pathglob`'s anchored
  full-path glob dialect: invariant source globs, dependency manifests, and domain `paths`
  all match through it (ADR-0077).
- **`golangci-lint`**: pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.
- **`deadcode`** (`golang.org/x/tools/cmd/deadcode`): pinned as a `go tool` dependency; the gate
  runs it (no `-test`) and `cmd/deadcodecheck` fails on any production function unreachable from a
  `main` outside `internal/testsupport/` (ADR-0063). This repo only, not part of the rendered standard.
- **Pi ai/TUI, coding-agent, Remote Pi, and TypeBox**: peer APIs used only by generated Pi extensions at runtime; adopters supply them and they are not awf-binary dependencies. Subagent and handoff factories retain their own guards. The selected `effort-workflow` Pi companion invokes the binary from the repository root, keeps its association process-local, and exposes fixed relative effort paths without changing CWD. The optional Remote Pi display-suffix event surface is consumed through awf-owned structural types: complete capability snapshots, payload-free requests, and string-or-null publication remain independent from complete advisory metadata. Missing or failing suffix integration degrades to metadata-only behavior and never changes routing identity.
- **Docker, Node, TypeScript, and c8**: pinned repo-only test dependencies under
  `tools/pi-extension-test/`; no host npm installation is used.
- **`gremlins`** (`github.com/go-gremlins/gremlins`): pinned as a `go tool` dependency; `./x mutants`
  runs it under the deterministic `.gremlins.yaml` config and `cmd/mutants` reports survived mutants
  (ADR-0066). Advisory only; never part of the gate. This repo only, not part of the rendered standard.

`internal/manifest` owns schema-sensitive lock decoding, while `internal/migrate` owns generation-31 removal of retired ADR routing payload.

- The generated context-usage extension uses the same pinned Pi runtime floor as handoff and subagents, but owns its local formatter rather than sharing subagent presentation code.
