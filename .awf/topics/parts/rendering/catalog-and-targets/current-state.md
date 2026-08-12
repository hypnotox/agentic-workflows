The catalog package holds the compile-time descriptor set for every artifact kind and target adapter. The claims below capture the current catalog and target contracts.

## Claims

### `invariant: adr-singleton-section-parity`

Each ADR-system singleton's catalog section list equals the awf:section markers declared in its template, and the singleton renders with no unresolved-variable placeholder.
Origin: ADR-0021
Backing: test

### `invariant: built-in-runtime-targets`

The built-in runtime target registry contains exactly `claude` and `pi` in deterministic `KnownTargets` order. Descriptor-driven rendering remains generic rather than branching on the two names.
Origin: ADR-0214
Revised-by: ADR-0251
Backing: test

### `invariant: catalog-defaults-generic-denylist`

No default-data value carried by any catalog spec contains an awf-repo-specific token: neither the `./x` command prefix nor the `hypnotox/agentic-workflows` module path appears anywhere in the recursively walked default data.
Origin: ADR-0045
Backing: test

### `invariant: catalog-go-single-source`

The standard catalog exists only as the compile-time Go value in the catalog package: no catalog.yaml is embedded and no catalog is parsed at runtime, and that Go value is populated across every kind - skills, agents, docs, singletons, the domain-doc spec, and vars.
Origin: ADR-0060
Backing: test

### `invariant: claude-md-bridge`

The claude target's bridge file is `CLAUDE.md`: the adapter emits an awf-owned repo-root `CLAUDE.md` whose body is the `@AGENTS.md` import beneath the provenance banner, tracked as a rendered file.
Origin: ADR-0016
Backing: test

### `invariant: no-single-marker-init-descriptor`

The catalog exposes no invariants-marker or invariants-globs var descriptor; the comment-marker mapping reaches configuration only through currentState.sources.
Origin: ADR-0064
Revised-by: ADR-0140
Backing: test

### `invariant: requires-skills-exact`

Every standard skill has an empty `RequiresSkills`; workflow-profile neighbors are advisory only. Artifact requirements, including reviewing agents' structural `RequiresSkills`, remain exact catalog declarations rather than workflow edges.
Origin: ADR-0080
Revised-by: ADR-0167, ADR-0251
Backing: test

### `invariant: reviewing-skill-specs-paired`

Every catalog skill whose name begins with reviewing- carries a non-empty requiresAgent naming the reviewer agent it dispatches.
Origin: ADR-0050
Backing: test

### `invariant: skill-section-parity`

For every catalog skill and agent, the set of awf:section markers in its template source equals the sections list its catalog entry declares, as order-independent set equality, so a section rename cannot half-land with a blank-path provenance pointer.
Origin: ADR-0054
Backing: test

### `invariant: structured-agent-encoding`

Agent rendering consumes structured metadata - a literal name, a separately rendered description, and a rendered instruction body - before a target encoder emits its artifact. The Markdown encoder never parses another rendered agent artifact, and arbitrary target-owned outputs retain their separately declared encoding.
Origin: ADR-0122
Revised-by: ADR-0214
Backing: test

### `invariant: target-dialect-render`

Each built-in target renders every catalog skill and agent exactly once at that target's declared path and dialect, and the emitted artifact parses under that runtime's native format. A closed target descriptor may additionally declare a target-owned skill with a catalog predicate; it uses the same target path, prefix, dialect, provenance, and policy machinery, is absent from every other target, and is planned and rendered by one resolved declaration path.
Claude Code and Pi each render every skill and agent exactly once at that descriptor's declared path and encoding, and the emitted artifact parses under that runtime's native format. The built-in Claude Code and Pi targets emit Markdown agents while retaining independent descriptor-owned paths, suffixes, capabilities, bridges, wording, and additional outputs.
Origin: ADR-0122
Revised-by: ADR-0214, ADR-0218, ADR-0251
Backing: test

### `invariant: unified-doc-model`

Every doc and singleton projection is derived from the single catalog document collection rather than a separate hand-maintained list. The singleton kinds equal exactly the catalog entries declaring their own output paths, the plain singletons equal those non-agents-doc non-generated entries, and each such entry renders under the documentation root at its declared path.
Origin: ADR-0061
Revised-by: ADR-0251
Backing: test

### `invariant: var-descriptor-parity`

Every var referenced by any catalog template has a matching var descriptor in the catalog, and no var descriptor names a var that appears in no template.
Origin: ADR-0029
Backing: test

### `invariant: var-descriptor-set-pinned`

The catalog's value-carrying string var descriptor keys are exactly the pinned functional set (gateCmd, gateCmdFull, checkCmd, commitGateCmd, testCmd, commitScopes, activeMdRegenCmd, invariantTestPath); no multiselect descriptor controls catalog rendering.
Origin: ADR-0084
Revised-by: ADR-0156, ADR-0158, ADR-0210, ADR-0251, ADR-0271
Backing: test
