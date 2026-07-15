## Components

- **`cmd/sundial/`:** argument parsing and output; exits 2 on usage errors.
- **`internal/almanac/`:** the cosine day-length model (ADR-0001): `Sun(Location,
  date)` returns clamped, polar-safe sunrise/sunset pairs.
- **`internal/schedule/`:** formats seven `almanac.Day` values as the plain-text
  sun table.
