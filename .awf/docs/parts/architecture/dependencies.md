| Dependency | Role |
|---|---|
| `gopkg.in/yaml.v3` | Strict configuration and ADR frontmatter parsing. |
| `text/template` | Rendering. |
| `github.com/go-git/go-git/v5` and Git | Git seam backends. |
| `github.com/bmatcuk/doublestar/v4` | Anchored repository-path globs. |
| Pi APIs | Generated extension runtime APIs, not CLI dependencies. |

Repository-only gate tools are documented in [testing](testing.md).