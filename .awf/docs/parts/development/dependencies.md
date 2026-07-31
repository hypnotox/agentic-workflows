## Dependencies

Runtime dependencies are deliberately few (see `go.mod`):

- **`gopkg.in/yaml.v3`**: strict (`KnownFields`) parsing and comment-preserving mutation
  of the `.awf/` config tree, plus ADR and skill/agent frontmatter.
- **`github.com/go-git/go-git/v5`** (with `go-billy/v5`): the pure-Go implementation behind the
  seam's in-process object reads. Native `git` is a runtime and test prerequisite for repository
  control-root resolution, refs and worktree topology, and working-tree truth. Both are
  backends of `internal/git` alone: no other production package may import the library or
  construct a git subprocess, and `internal/testsupport/gitfixture` is the single test-side
  exception, forced by the zero-internal-deps rule rather than chosen.
- **`golang.org/x/mod`**: semver comparison for the binary-version gate (ADR-0039).
- **`github.com/bmatcuk/doublestar/v4`**: anchored path-glob matching behind
  `internal/pathglob` (ADR-0077).
- **`github.com/BurntSushi/toml`**: encodes the Codex adapter's TOML agent profiles
  (`internal/project/agent.go`).


When introducing or deliberately converting a volatile mechanism, compose it at the outer boundary that owns production knowledge and inject a consumer-owned semantic function or immutable value before considering an interface. Consult `code-design/dependency-composition` for the complete authority.

Developer tools are pinned in `go.mod`'s `tool` block for reproducibility:
`golangci-lint` (lint and format), `deadcode` (the dead-code gate, ADR-0063), and
`gremlins` (advisory mutation testing, ADR-0066).

The Pi-extension test lane pins Node, TypeScript, Pi ai/TUI 0.81.1, the checksummed compatible
coding-agent `fork-v0.81.1-awf.3` release, TypeBox, and test dependencies in
`tools/pi-extension-test/`. Docker bakes them into an image keyed by their own content, which
every checkout and worktree shares, and each run reaches them through a symlink from its
throwaway working copy, so the lane creates no volume. They are never awf binary dependencies
and never create host npm state.
