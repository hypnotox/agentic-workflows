| Dependency | Role |
|---|---|
| `gopkg.in/yaml.v3` | Strict `KnownFields` parsing and comment-preserving `.awf/`, ADR, and frontmatter mutation. |
| `github.com/go-git/go-git/v5` with `go-billy/v5` | In-process backend of `internal/git`; native Git is the other backend for control roots, refs, worktrees, and working-tree truth. Only `internal/git` may use either, with `internal/testsupport/gitfixture` as the test exception. |
| `golang.org/x/mod` | SemVer comparison for the binary-version gate (ADR-0039). |
| `github.com/bmatcuk/doublestar/v4` | Anchored path glob matching in `internal/pathglob` (ADR-0077). |

For volatile mechanisms, compose at the outer boundary that knows production and inject a consumer-owned semantic function or immutable value before adding an interface. See `code-design/dependency-composition`.

| Development dependency | Role |
|---|---|
| `golangci-lint` | Lint and format. |
| `deadcode` | Dead-code gate (ADR-0063). |
| `gremlins` | Advisory mutation testing (ADR-0066). |
| Pi lane dependencies | Pinned Node, TypeScript, Pi ai/TUI 0.81.1, TypeBox, and checksummed `fork-v0.81.1-awf.3` in `tools/pi-extension-test/`. Docker builds a content-keyed shared image and uses throwaway copies, with no host npm state or volume. |

Pi is adopter-supplied, not an awf binary dependency. The checked test artifact retains the numeric 0.81.1 floor and lock-pinned integrity.
