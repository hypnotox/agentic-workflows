## Components

- **`cmd/sundial/`:** argument parsing and output; exits 2 on usage errors.
- **`internal/almanac/`:** the cosine day-length model (ADR-0001): `Sun(Location,
  date)` returns clamped, polar-safe sunrise/sunset pairs.
- **`internal/schedule/`:** formats seven `almanac.Day` values as the plain-text
  sun table.
- **Workflow configuration:** the optional `commitPolicy` mapping is parsed and structurally validated by awf; absence preserves existing behavior and does not activate a hook or runtime policy.
- **Workflow context:** `awf context` provides tier-0 directory orientation and
  tier-1 marker relationships for exact or Git-selected files; named facets
  expand topic authority without changing the application dependency graph.
