Generated documentation outputs: domain and topic documents, docs layout, pitfall data validation, unauthored-content stubs, and skill-reference hygiene.

## Claims

### `invariant: domain-doc-regenerated`

awf check regenerates each enabled domain document from current state and reports it stale when the on-disk copy diverges, so adding a topic to a domain without re-syncing is detected rather than passing silently.
Origin: ADR-0148
Backing: test

### `invariant: domains-dir-given`

The layout's domains directory is computed as `docs/domains` beneath the fixed documentation root.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: layout-derivation`

The decisions directory, ADR index file, and plans directory derive structurally from the fixed `docs` root as `docs/decisions`, `docs/decisions/INDEX.md`, and `docs/plans`, rather than being independently configurable.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: docs-root-fixed`

The documentation root is exactly `docs`, fixed in the binary rather than read from configuration.
Origin: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: layout-docs-full-catalog`

The layout docs map contains exactly every catalog document name, each mapping to `docs/<name>.md`, and no other keys.
Origin: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: pitfall-adr-link-resolved`

check fails a pitfall entry whose related list names an ADR number with no matching file under docs/decisions/.
Origin: ADR-0148
Backing: test

### `invariant: pitfall-data-validated`

check fails on unparseable docs/pitfalls.yaml data and on any entry with a non-string or empty or newline-bearing title, a missing or non-string or empty body, or a malformed domains, related, or tags field; the transform that renders docs/pitfalls.md is a hard error on the same malformed data.
Origin: ADR-0148
Backing: test

### `invariant: pitfall-domains-resolved`

check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid.
Origin: ADR-0148
Revised-by: ADR-0207
Backing: test

### `invariant: glossary-domains-resolved`

check fails a glossary record whose domains list names a domain not configured in the project; a record with no domains is valid.
Origin: ADR-0207
Backing: test

### `invariant: glossary-terseness-advisory`

check reports one non-failing note per glossary term whose meaning exceeds the terseness threshold, naming the sidecar path, the term, and its length. The evaluated set is the merged one, so the shipped standard vocabulary is bound by the threshold alongside the project's authored terms.
Origin: ADR-0207
Backing: test

### `invariant: skill-ref-unknown-ignored`

A prefix-anchored token whose trailing word matches no catalog skill name produces no dead-skill-reference finding.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: stub-notes-path-keyed`

The unauthored-content advisory reports one entry per rendered output path, so artifacts that share a template id, including per-target artifacts and domain docs, each report independently.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: topic-output-complete`

Every valid topic input has one rendered topic document and participates in its domain's generated topic index, output plan, lock manifest, drift check, and prune behaviour.
Origin: ADR-0148
Revised-by: ADR-0159
Backing: unbacked
Verify: Creating and removing a topic in a render fixture changes awf render, awf check, the output plan, the lock, the index, and stale-output pruning consistently.
### `invariant: working-with-awf-mandatory`

The working-with-awf doc renders as an always-on singleton for every project, present in the plain-singleton set and the catalog's singleton kinds; during the schema-compatible intermediate it remains suppressible only by a local: true sidecar.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test
