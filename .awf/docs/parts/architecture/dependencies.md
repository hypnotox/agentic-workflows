## Key dependencies

- **`gopkg.in/yaml.v3`**: strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast.
- **`encoding/json`, `crypto/sha256`, process execution, and filesystem primitives** (standard library): parse,
  fingerprint, and project the normative telemetry descriptor, implement its confined durable
  append-only resident ledger, produce canonical metrics, normalized exports, and diagnostic
  results, and publish the locked immutable dashboard development cache without a database,
  background daemon, or TypeScript aggregation engine.
- **Git and the Go toolchain**: repository-only dashboard fallback inputs. Git resolves, materializes,
  and compare-and-swap advances the local pinned ref; the selected Go binary performs two normalized
  builds from that committed tree. They are development requirements, not runtime dependencies of an
  installed adopter binary.
- **`text/template`** (standard library): the rendering engine; ADR-0001 owns its
  publication-safety contract.
- **`github.com/go-git/go-git/v5`** (with `go-billy/v5`): pure-Go git access for `awf audit`'s
  history and working-tree reads; awf and its tests need no host `git` binary.
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
- **Pi ai/TUI 0.81.1, compatible coding-agent 0.81.1, and TypeBox 1.1.38**: peer APIs used only by the generated Pi
  extensions at runtime; they are supplied by the adopter's Pi installation and are not dependencies
  of the awf binary. The dashboard additionally requires custom session entries, widget and overlay
  APIs, lifecycle events, and shutdown notification; all three extension factories fail closed before
  functional registration when their required minimum surface is absent. The test package pins pi-ai
  and pi-tui directly at 0.81.1, TypeBox directly at 1.1.38, and coding-agent to the checksummed
  `hypnotox/pi` `fork-v0.81.1-awf.3` release URL because the official coding-agent 0.81.1 artifact
  lacks `ExtensionAPI.queueCommand`. Its lockfile SRI is
  `sha512-Xk34jkheEgNwBPMfT00+jmhY3YHcMkq5xL3C+a1Cr9yR0hsN76J5am6RJkZVQSxwAdHS2GKgzREElp0awve/sQ==`.
- **Docker, Node, TypeScript, and c8**: pinned repo-only test dependencies under
  `tools/pi-extension-test/`; no host npm installation is used.
- **`gremlins`** (`github.com/go-gremlins/gremlins`): pinned as a `go tool` dependency; `./x mutants`
  runs it under the deterministic `.gremlins.yaml` config and `cmd/mutants` reports survived mutants
  (ADR-0066). Advisory only; never part of the gate. This repo only, not part of the rendered standard.
