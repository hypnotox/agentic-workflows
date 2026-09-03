| Dependency | Role |
|---|---|
| `gopkg.in/yaml.v3` | Strict `KnownFields` parsing and comment-preserving `.awf/` and frontmatter mutation. |
| `github.com/go-git/go-git/v5` with `go-billy/v5` | In-process backend of `internal/git`; native Git is the other backend for control roots, refs, worktrees, and working-tree truth. Only `internal/git` may use either, with `internal/testsupport/gitfixture` as the test exception. |
| `golang.org/x/mod` | SemVer comparison for the binary-version gate (ADR-0039). |
| `github.com/bmatcuk/doublestar/v4` | Anchored path glob matching in `internal/pathglob` (ADR-0077). |

For volatile mechanisms, compose at the outer boundary that knows production and inject a consumer-owned semantic function or immutable value before adding an interface. See `code-design/dependency-composition`.

| Development dependency | Role |
|---|---|
| `golangci-lint` | Lint and format. |
| `deadcode` | Dead-code gate (ADR-0063). |

`agentic-skills` and `pi-tools` are operator-supplied harness packages, not AWF source, binary, or development dependencies. `agentic-skills` owns generic skills, canonical roles, and its Pi role adapter. `pi-tools` owns Pi role execution and runtime mechanics. AWF publishes only its four repository-local skills and does not vendor, install, update, pin, or probe either package.
