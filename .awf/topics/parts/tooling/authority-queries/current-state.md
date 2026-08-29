Read-only authority queries expose current-state topics and ADR lifecycle progress without owning code discovery or enforcement.

## Claims

### `invariant: authority-read-projections`

`awf read topic` preserves the active topic, history, references, coverage, proof-site, and ADR parser projections, while `awf read adr` reports lifecycle status, canonical operation progress, and parsed linked plans.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test

### `invariant: path-topic-resolution`

`awf resolve topic` lexically normalizes repository-relative proposed or existing paths and deterministically reports owning domains and applicable topics, including explicit absence.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test

### `invariant: unowned-path-census`

`awf resolve topic --uncovered` is a whole-repository informational census that accepts no positional roots and collapses unowned paths to topmost directories.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test

### `invariant: authority-query-read-only`

Focused authority queries are read-only and leave enforcement to `awf check`.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test

### `invariant: authority-query-full-profile-only`

Focused authority queries require the Full governance profile.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test
