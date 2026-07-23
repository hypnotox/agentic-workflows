## Dependencies

Runtime dependencies are deliberately few (see `go.mod`):

- **`gopkg.in/yaml.v3`**: strict (`KnownFields`) parsing and comment-preserving mutation
  of the `.awf/` config tree, plus ADR and skill/agent frontmatter.
- **`github.com/go-git/go-git/v5`** (with `go-billy/v5`): pure-Go git access for
  `awf audit`'s history and working-tree reads; awf and its tests need no host `git` binary.
- **`golang.org/x/mod`**: semver comparison for the binary-version gate (ADR-0039).
- **`github.com/bmatcuk/doublestar/v4`**: anchored path-glob matching behind
  `internal/pathglob` (ADR-0077).
- **`github.com/BurntSushi/toml`**: encodes the Codex adapter's TOML agent profiles
  (`internal/project/agent.go`).

The repository-only dashboard fallback shells out to Git for exact commit materialization and ref compare-and-swap, and to the selected Go toolchain for normalized builds. Published cache entries contain no new linked runtime dependency; adopters using bootstrap or PATH resolution do not use this development fallback.

Developer tools are pinned in `go.mod`'s `tool` block for reproducibility:
`golangci-lint` (lint and format), `deadcode` (the dead-code gate, ADR-0063), and
`gremlins` (advisory mutation testing, ADR-0066).

The Pi-extension test lane pins Node, TypeScript, Pi ai/TUI 0.81.1, the checksummed compatible
coding-agent `fork-v0.81.1-awf.3` release, TypeBox, and test dependencies in
`tools/pi-extension-test/`. Docker installs them into a repo-keyed persistent volume; they are
never awf binary dependencies and never create host npm state.
