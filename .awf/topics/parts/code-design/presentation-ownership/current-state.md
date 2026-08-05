How rendering a result model for humans is owned: the package that owns the model owns its presentation, while command binaries keep argument parsing, renderer selection, and exit mapping. This topic governs renderings introduced by new work and command surfaces deliberately converted under its authority; the remaining cmd-side renderings (printPlan, printTopic) are recorded future candidates that convert as they are next touched (ADR-0195 item 4), not violations of a settled state.

## Claims

### `invariant: model-owner-renders`

The package that owns a result model maps its semantics into the central presentation representation; `internal/presentation` alone validates and renders syntax, while a command binary keeps argument parsing, presentation-versus-bypass selection, stream choice, and exit mapping.
Origin: ADR-0195
Revised-by: ADR-0234
Backing: unbacked
Verify: when touching a command surface, confirm the rendering of each result model it prints lives in the package owning that model.

### `invariant: closed-presentation-tree`

`internal/presentation` owns the closed Document, Field, Section, List, RecordGroup, Record, and Steps tree and is the sole syntax validator and text renderer; no raw-text node or alternate renderer bypasses that boundary.
Origin: ADR-0234
Backing: test
