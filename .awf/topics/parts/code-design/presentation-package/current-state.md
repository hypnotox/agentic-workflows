The presentation package is the code-design-owned representation boundary for ordinary CLI output. Its scoped package contract remains active for `internal/presentation/**` and complements, rather than narrows or replaces, the repository-wide applicability and bounded ownership held by the global presentation-ownership topic.

## Claims

### `rule: presentation-package-boundary`

`internal/presentation` depends only on the Go standard library and owns presentation representation, syntax validation, and rendering without domain result semantics.
