Agent-guide and documentation template contracts: section parity, glossary rendering, scope derivation, and the document map.

## Claims

### `invariant: agents-doc-section-parity`

The agents-doc template's awf:section marker names match its catalog-declared section list exactly and in order, so a guide section added to one but not the other fails rather than half-landing with a broken override path.
Origin: ADR-0148
Backing: test

### `invariant: agentsdoc-parts`

The agent-guide's you-and-this-project and identity section bodies can be overridden by convention parts placed under parts/agents-doc/, and with no override and empty invariants and doc-map data the guide still renders complete adopter-neutral prose with no <no value> token.
Origin: ADR-0148
Backing: test

### `invariant: docs-section-parity`

For every non-mandatory catalog doc, the section names declared in the catalog exactly match the set of awf:section marker blocks in that doc's template, and each doc renders from its template defaults with no leaked no-value token.
Origin: ADR-0148
Backing: test

### `invariant: document-map-lists-mandatory-docs`

The document-map section of the rendered `AGENTS.md` always cites every mandatory document-map doc (including the workflow guide, the documentation standard, and the agent-guide authoring standard) with its full title, link, and catalog description, regardless of the project's `docs:` array contents.
Origin: ADR-0148
Backing: test

### `invariant: glossary-table-forced`

No convention part can replace the rendered glossary terms table; the only part-override surfaces on the glossary doc are the prepend and append sections.
Origin: ADR-0148
Backing: test

### `invariant: glossary-terms-sorted`

The rendered glossary table orders its rows case-insensitively by term regardless of the authored map order, and two sidecars carrying the same entries in different order render byte-identically.
Origin: ADR-0148
Backing: test

### `invariant: glossary-terms-validated`

An empty term, an empty, null, or non-string meaning, an interior newline in a term or meaning, a non-string map key, or a case-insensitive duplicate term in the glossary sidecar fails the render, naming the sidecar path and the offending key.
Origin: ADR-0148
Backing: test

### `invariant: guide-scopes-derived`

The agent-guide template renders its commit-scope mention from the root commit-scopes render key rather than any hand-written scope list in the agents-doc data, and the mention degrades to generic Conventional Commits prose when no scopes are configured.
Origin: ADR-0148
Backing: test

### `invariant: no-doc-path-vars`

No template under templates/ references any of the removed doc-path or project-specific vars (workflowDoc, debuggingDoc, pitfallsDoc, roadmapDoc, stateDocsPath, oracleStateDoc, autonomousAdrRef, hostGitAdrRef, keyInvariantAdrRef, noDivingAdrRef, perTaskReviewAdrRef); doc references are supplied through the layout instead.
Origin: ADR-0148
Backing: test

### `invariant: guide-entry-point-routing`

The rendered guide's workflow section is a catalog-derived entry-skill trigger table: every catalog entry and task skill appears iff enabled, and none of the evicted prose classes renders (chain diagram, warrant definitions, plan-form contract, V2 batch semantics, exploration/subagent policy, duplicated gate sentence).
Origin: ADR-0157
Backing: test

### `invariant: working-memory-single-home`

The file skeleton, ground rules, and just-in-time retrieval prose render canonically in the workflow doc's working-memory section; the guide, the shared checkpoint partials, and the chain section point to that content rather than carrying copies of it.
Origin: ADR-0157
Backing: test
