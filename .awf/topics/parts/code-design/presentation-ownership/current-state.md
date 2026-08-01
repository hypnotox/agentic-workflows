How rendering a result model for humans is owned: the package that owns the model owns its presentation, while command binaries keep argument parsing, renderer selection, and exit mapping. This topic governs renderings introduced by new work and command surfaces deliberately converted under its authority; the remaining cmd-side renderings (printPlan, printTopic) are recorded future candidates that convert as they are next touched (ADR-0195 item 4), not violations of a settled state.

## Claims

### `invariant: model-owner-renders`

The package that owns a result model owns its human rendering; a command binary keeps argument parsing, renderer selection, and exit mapping.
Origin: ADR-0195
Backing: unbacked
Verify: when touching a command surface, confirm the rendering of each result model it prints lives in the package owning that model.
