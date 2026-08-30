Read-only authority queries expose current-state topics without owning code discovery or enforcement.

## Claims

### `invariant: authority-read-projections`

`awf read topic` exposes the active topic, references, coverage, proof sites, and selectors without historical-decision or plan projections.
Backing: test

### `invariant: path-topic-resolution`

`awf resolve topic` lexically normalizes repository-relative proposed or existing paths and deterministically reports owning domains and applicable topics, including explicit absence.
Backing: test

### `invariant: unowned-path-census`

`awf resolve topic --uncovered` is a whole-repository informational census that accepts no positional roots and collapses unowned paths to topmost directories.
Backing: test

### `invariant: authority-query-read-only`

Focused authority queries are read-only and leave enforcement to `awf check`.
Backing: test

### `invariant: codegraph-navigation-boundary`

CodeGraph owns structural source discovery, architecture, callers, dependencies, and impact analysis; Git selects changed paths; `awf resolve topic` and `awf read topic` expose focused normative authority.
Backing: test
