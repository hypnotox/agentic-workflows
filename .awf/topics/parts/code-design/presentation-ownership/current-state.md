<!-- awf:comment Remaining ADR-0194 operations land as their phases apply. -->
How rendering a result model for humans is owned: the package that owns the model owns its presentation, while command binaries keep argument parsing, renderer selection, and exit mapping.

## Claims

### `invariant: model-owner-renders`

The package that owns a result model owns its human rendering; a command binary keeps argument parsing, renderer selection, and exit mapping.
Origin: ADR-0194
Backing: unbacked
Verify: when touching a command surface, confirm the rendering of each result model it prints lives in the package owning that model.
