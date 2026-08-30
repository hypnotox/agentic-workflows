Generated documentation outputs: domain and topic documents, docs layout, the pitfall corpus and generated family, unauthored-content stubs, and skill-reference hygiene.

## Claims

### `invariant: domain-doc-regenerated`

awf check regenerates each enabled domain document from current state and reports it stale when the on-disk copy diverges, so adding a topic to a domain without re-syncing is detected rather than passing silently.
Backing: test

### `invariant: domains-dir-given`

The layout's domains directory is computed as `docs/domains` beneath the fixed documentation root.
Backing: test

### `invariant: layout-derivation`

For Full, the decisions directory, ADR index file, and plans directory derive structurally from the fixed `docs` root as `docs/decisions`, `docs/decisions/INDEX.md`, and `docs/plans`; Core does not emit those managed structural outputs.
Backing: test

### `invariant: docs-root-fixed`

The documentation root is exactly `docs`, fixed in the binary rather than read from configuration.
Backing: test

### `invariant: local-doc-output-complete`

Every valid local-document declaration produces one separately configured output through normalized metadata, shared-shell render, lock membership, working-tree regeneration drift, Markdown-link and skill-reference scans, and agent-guide discovery. Its body remains in-place preserved, and this ordinary working-tree coverage does not add staged semantics.
Backing: test

### `invariant: pitfall-adr-link-resolved`

check fails a pitfall entry whose related list names an ADR number with no matching file under docs/decisions/.
Backing: test

### `invariant: pitfall-corpus-validated`

The pitfall source loader accepts only direct regular lowercase-kebab `.md` leaves under `.awf/docs/pitfalls`, reserves `index`, strictly validates a required single-line title plus optional duplicate-free domains and related ADRs, rejects retired tag metadata, requires a nonblank body, and enforces corpus-wide title uniqueness for render and check.
Backing: test

### `invariant: pitfall-output-complete`

Every valid pitfall source produces exactly one metadata row and one generated leaf through matching working and staged output declarations, with full-source leaf hashes, metadata-only index hashes, lock and drift membership, ordinary backup, and deletion pruning.
Backing: test

### `invariant: pitfall-domains-resolved`

check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid.
Backing: test

### `invariant: glossary-domains-resolved`

check fails a glossary record whose domains list names a domain not configured in the project; a record with no domains is valid.
Backing: test

### `invariant: glossary-terseness-advisory`

check reports one Warning per glossary term whose meaning exceeds the terseness threshold, naming the sidecar path, the term, and its length, with successful exit. The evaluated set is the merged one, so the shipped standard vocabulary is bound by the threshold alongside the project's authored terms.
Backing: test

### `invariant: skill-ref-unknown-ignored`

A prefix-anchored token whose trailing word matches no catalog skill name produces no dead-skill-reference finding.
Backing: test

### `invariant: stub-notes-path-keyed`

The unauthored-content advisory reports one entry per rendered output path, so artifacts that share a template id, including per-target artifacts and domain docs, each report independently.
Backing: test

### `invariant: topic-output-complete`

Every valid topic input has one rendered topic document and participates in its domain's generated topic index, output plan, lock manifest, drift check, and prune behaviour.
Backing: unbacked
Verify: Creating and removing a topic in a render fixture changes awf render, awf check, the output plan, the lock, the index, and stale-output pruning consistently.

### `invariant: working-with-awf-mandatory`

The working-with-awf doc renders as an always-on singleton for every project, present in the plain-singleton set and the catalog's singleton kinds.
Backing: test

### `invariant: opaque-doc-source-guidance`

Opaque generated documentation carries one compact reader-facing `awf:source` marker for topic pages and indexes, domain navigation, glossary, the pitfall index and each exact-source pitfall leaf, the ADR index, config reference, and target bridges. Section-overridable standard docs and AGENTS.md retain their `awf:edit` guidance without duplication; authored ADRs and plans remain banner-free. Marker payloads guide readers and are not exhaustive machine dependencies.
Backing: test

### `invariant: layout-docs-profile-projection`

Layout and document-map derivation expose only documents emitted by the selected governance footprint.
Backing: test

### `invariant: pi-runtime-reference-output`

The standard Pi runtime reference renders as one lock-listed, drift-checked, link-scanned catalog document, and the generated AGENTS.md document map reaches it directly. Its unconditional publication states that it applies only to Pi adopters.
Backing: test
