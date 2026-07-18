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

Developer tools are pinned in `go.mod`'s `tool` block for reproducibility:
`golangci-lint` (lint and format), `deadcode` (the dead-code gate, ADR-0063), and
`gremlins` (advisory mutation testing, ADR-0066).

The Pi-extension test lane pins Node, TypeScript, Pi 0.80.9, TypeBox, and test dependencies in
`tools/pi-extension-test/`. Docker installs them into a repo-keyed persistent volume; they are
never awf binary dependencies and never create host npm state.
