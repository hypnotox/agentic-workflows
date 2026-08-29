Read-only authority queries expose current-state topics and ADR lifecycle progress without owning code discovery or enforcement.

## Claims

### `invariant: authority-read-projections`

`awf read topic` preserves the active topic, history, references, coverage, proof-site, and ADR parser projections, while `awf read adr` reports lifecycle status, canonical operation progress, and parsed linked plans.
Origin: ADR-0320
Backing: test

### `invariant: path-topic-resolution`

`awf resolve topic` lexically normalizes repository-relative proposed or existing paths and deterministically reports owning domains and applicable topics, including explicit absence.
Origin: ADR-0320
Backing: test

### `invariant: unowned-path-census`

`awf resolve topic --uncovered` is a whole-repository informational census that accepts no positional roots and collapses unowned paths to topmost directories.
Origin: ADR-0320
Backing: test

### `invariant: authority-query-read-only`

Focused authority queries are read-only and leave enforcement to `awf check`.
Origin: ADR-0320
Backing: test

### `invariant: authority-query-full-profile-only`

Focused authority queries require the Full governance profile.
Origin: ADR-0320
Backing: test

### `invariant: codegraph-navigation-boundary`

CodeGraph is the documented owner of structural source discovery, architecture, callers, dependencies, and impact analysis; Git selects changed paths. Full workflow guidance uses `awf resolve topic`, `awf read topic`, and `awf read adr` only for focused normative authority, without a parallel awf navigation fallback.
Origin: ADR-0320
Backing: test
