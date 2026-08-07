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

For every name-derived catalog doc (neither the root agent guide nor a document declaring `Path`), the section names declared in the catalog exactly match the set of awf:section marker blocks in that doc's template, and each doc renders from its template defaults with no leaked no-value token.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: document-map-lists-mandatory-docs`

The document-map section of the rendered `AGENTS.md` always cites every catalog document-map doc (including the workflow guide, the documentation standard, and the agent-guide authoring standard) with its full title, link, and catalog description.
Origin: ADR-0148
Revised-by: ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: glossary-table-forced`

No convention part can replace the rendered glossary terms table; the only part-override surfaces on the glossary doc are the prepend and append sections.
Origin: ADR-0148
Backing: test

### `invariant: glossary-terms-sorted`

The rendered glossary table orders its rows case-insensitively by term regardless of the authored order, and two sidecars carrying the same entries in different order render byte-identically.
Origin: ADR-0148
Revised-by: ADR-0207
Backing: test

### `invariant: glossary-terms-validated`

An empty, missing, or non-string term, an empty, null, or non-string meaning, an interior newline in a term or meaning, a malformed record, an unknown record key, or a case-insensitive duplicate term within a single layer of the glossary sidecar fails the render, naming the sidecar path, and the offending term where the term itself parsed.
Origin: ADR-0148
Revised-by: ADR-0207, ADR-0208
Backing: test

### `invariant: glossary-standard-vocabulary`

The rendered glossary merges the catalog's shipped standard vocabulary with the project's authored terms into one sorted table, a project term overriding a shipped term of the same case-insensitive name.
Origin: ADR-0207
Backing: test

### `invariant: glossary-standard-terms-portable`

Every shipped standard term carries exactly a string term and a string meaning, with no domains key, no ADR reference, and no meaning exceeding the terseness threshold, so the shipped layer is portable into any adopter tree.
Origin: ADR-0207
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

The rendered guide routes agents to native catalog skills whose exposed descriptions fit the work, without duplicating standard or local skill names, purposes, triggers, kinds, relationships, or a fallback catalog. Empty and missing render data remains coherent, and skill selection stays advisory.
Origin: ADR-0157
Revised-by: ADR-0167, ADR-0241, ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: working-memory-single-home`

Working-memory protocol has one canonical workflow-document home. The root guide carries only slim native-skill routing and states that effort creation depends on durable continuity, while `effort-workflow` alone owns creation through finish. It preserves the one-user-managed-writer boundary without duplicating protocol, topology, or memory-path procedure. Resume verification remains procedurally homed in orienting.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0157
Revised-by: ADR-0160, ADR-0161, ADR-0164, ADR-0167, ADR-0175, ADR-0187, ADR-0189, ADR-0222, ADR-0226, ADR-0241, ADR-0243
Backing: test

### `invariant: agent-guide-size-budgets`

The direct default `AGENTS.md` render is at most 8 KiB and this repository's self-hosted `AGENTS.md` is at most 10 KiB. These fixed regression bounds diagnose failures with observed and allowed bytes plus test-only largest-section contributions; production rendering has no section attribution.
Origin: ADR-0241
Backing: test

### `invariant: maintainable-code-design-guide`

The standard catalog renders `docs/maintainable-code-design.md` as a mandatory document-map singleton with ordered convention-part sections for decision posture, contextual heuristics, semantic modeling, readability, boundaries and dependencies, an illustrative pattern toolbox, preparatory refactoring, and failure modes. Its canonical language-agnostic, adopter-neutral decision posture makes the simplest sufficient solution the default and permits added abstraction, indirection, validation, test machinery, tooling, cleanup, or process only for requested behavior, a reproduced defect, an existing documented contract, or a clearly applicable project invariant; the agent guide projects this rule concisely. Workflow plan selection is proportionate: plans are for sequencing, coordination, or resumability that materially helps and record approved choices rather than invent speculative work. Empty project data remains coherent and free of repository-specific content.
Origin: ADR-0168
Revised-by: ADR-0200, ADR-0232
Backing: test
