Agent-guide and documentation template contracts: section parity, glossary rendering, scope derivation, and the document map.

## Claims

### `invariant: guide-awf-invocation`

The rendered agent guide directs repository-local executable awf commands through `./awf`, including generated-tree rendering and staged verification, while retaining bare product and CLI grammar where no local execution is prescribed.
Backing: test


### `invariant: agents-doc-section-parity`

The agents-doc template's awf:section marker names match its catalog-declared section list exactly and in order, so a guide section added to one but not the other fails rather than half-landing with a broken override path.
Backing: test

### `invariant: agentsdoc-parts`

The agent-guide's you-and-this-project and identity section bodies can be overridden by convention parts placed under parts/agents-doc/, and with no override and empty invariants and doc-map data the guide still renders complete adopter-neutral prose with no <no value> token.
Backing: test

### `invariant: docs-section-parity`

For every name-derived catalog doc (neither the root agent guide nor a document declaring `Path`), the section names declared in the catalog exactly match the set of awf:section marker blocks in that doc's template, and each doc renders from its template defaults with no leaked no-value token.
Backing: test

### `invariant: document-map-lists-mandatory-docs`

The default document-map body of rendered `AGENTS.md` cites every selected catalog document-map doc with its full title, link, and catalog description. The normalized name-sorted local-document projection is a non-replaceable suffix; it remains outside the catalog and `Layout.Docs`.
Backing: test

### `invariant: glossary-table-forced`

No convention part can replace the rendered glossary terms table; the only part-override surfaces on the glossary doc are the prepend and append sections.
Backing: test

### `invariant: glossary-terms-sorted`

The rendered glossary table orders its rows case-insensitively by term regardless of the authored order, and two sidecars carrying the same entries in different order render byte-identically.
Backing: test

### `invariant: glossary-terms-validated`

An empty, missing, or non-string term, an empty, null, or non-string meaning, an interior newline in a term or meaning, a malformed record, an unknown record key, or a case-insensitive duplicate term within a single layer of the glossary sidecar fails the render, naming the sidecar path, and the offending term where the term itself parsed.
Backing: test

### `invariant: glossary-standard-vocabulary`

The rendered glossary merges the catalog's shipped standard vocabulary with the project's authored terms into one sorted table, a project term overriding a shipped term of the same case-insensitive name.
Backing: test

### `invariant: glossary-standard-terms-portable`

Every shipped standard term carries exactly a string term and a string meaning, with no domains key, no ADR reference, and no meaning exceeding the terseness threshold, so the shipped layer is portable into any adopter tree.
Backing: test

### `invariant: guide-scopes-derived`

The agent-guide template renders its commit-scope mention from the root commit-scopes render key rather than any hand-written scope list in the agents-doc data, and the mention degrades to generic Conventional Commits prose when no scopes are configured.
Backing: test

### `invariant: no-doc-path-vars`

No template under templates/ references any of the removed doc-path or project-specific vars (workflowDoc, debuggingDoc, pitfallsDoc, roadmapDoc, stateDocsPath, oracleStateDoc, autonomousAdrRef, hostGitAdrRef, keyInvariantAdrRef, noDivingAdrRef, perTaskReviewAdrRef); doc references are supplied through the layout instead.
Backing: test

### `invariant: guide-entry-point-routing`

The rendered guide treats exposed native-skill descriptions as routing metadata and selects only bodies governing the next concrete action. Core routes the operational workflow without Full-only governance bodies; Full adds its selected governance bodies without changing the workflow's correctness, autonomy, maintainability, or review-quality bar. Empty and missing render data remains coherent, and selection stays advisory.
Backing: test

### `invariant: working-memory-single-home`

Working-memory protocol has one canonical capability-neutral workflow-document home in both governance footprints. The root guide carries only slim native-skill routing, while `effort-workflow` alone owns creation through finish and preserves the one-user-managed-writer boundary.
Backing: test

### `invariant: agent-guide-size-budgets`

The direct default `AGENTS.md` render is at most 8 KiB and this repository's self-hosted `AGENTS.md` is at most 10 KiB. These fixed regression bounds diagnose failures with observed and allowed bytes plus test-only largest-section contributions; production rendering has no section attribution.
Backing: test

### `invariant: maintainable-code-design-guide`

Both governance footprints render `docs/maintainable-code-design.md` as a mandatory document-map singleton with its ordered convention-part sections. The guide is the canonical adopter-neutral maintainable-design doctrine and makes the simplest sufficient solution the default; the separate shared clean-integration rule applies that doctrine proportionally through workflow consumers without duplicating it. Full additionally supplies plan routing where useful for sequencing, coordination, or resumability without changing the maintainability bar.
Backing: test
