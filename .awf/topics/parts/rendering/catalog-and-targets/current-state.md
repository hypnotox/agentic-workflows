The catalog package holds the compile-time descriptor set for every artifact kind and target adapter. The claims below capture the current catalog and target contracts.

## Claims

### `invariant: built-in-runtime-targets`

The built-in runtime target registry contains exactly `claude` and `pi` in deterministic `KnownTargets` order. Descriptor-driven rendering remains generic rather than branching on the two names.
Backing: test

### `invariant: catalog-defaults-generic-denylist`

No default-data value carried by any catalog spec contains an awf-repo-specific token: neither the `./x` command prefix nor the `hypnotox/agentic-workflows` module path appears anywhere in the recursively walked default data.
Backing: test

### `invariant: catalog-go-single-source`

The standard catalog exists only as the compile-time Go value in the catalog package: no catalog.yaml is embedded and no catalog is parsed at runtime, and that Go value is populated across every kind - skills, agents, docs, singletons, the domain-doc spec, and vars.
Backing: test

### `invariant: claude-md-bridge`

The claude target's bridge file is `CLAUDE.md`: the adapter emits an awf-owned repo-root `CLAUDE.md` whose body is the `@AGENTS.md` import beneath the provenance banner, tracked as a rendered file.
Backing: test

### `invariant: no-single-marker-init-descriptor`

The catalog exposes no invariants-marker or invariants-globs var descriptor; the comment-marker mapping reaches configuration only through currentState.sources.
Backing: test

### `invariant: skill-section-parity`

For every catalog skill and agent, the set of awf:section markers in its template source equals the sections list its catalog entry declares, as order-independent set equality, so a section rename cannot half-land with a blank-path provenance pointer.
Backing: test

### `invariant: structured-agent-encoding`

Agent rendering consumes structured metadata - a literal name, a separately rendered description, and a rendered instruction body - before a target encoder emits its artifact. The Markdown encoder never parses another rendered agent artifact, and arbitrary target-owned outputs retain their separately declared encoding.
Backing: test

### `invariant: target-dialect-render`

Each built-in target renders every standard skill and agent exactly once at that target's declared path and dialect, and the emitted artifact parses under that runtime's native format. A target-owned derived skill uses the same declaration path and is emitted only for its owning target.
Backing: test

### `invariant: unified-doc-model`

Every doc and singleton projection derives from the one standard catalog document collection rather than a separate hand-maintained list. Its singleton kinds equal exactly the entries declaring output paths, and each such entry renders under the documentation root at its declared path.
Backing: test

### `invariant: var-descriptor-parity`

Every var referenced by any catalog template has a matching var descriptor in the catalog, and no var descriptor names a var that appears in no template.
Backing: test

### `invariant: var-descriptor-set-pinned`

The catalog's value-carrying string var descriptor keys are exactly the pinned functional set (`gateCmd`, `checkCmd`, and `testCmd`); no selector or multiselect descriptor controls catalog rendering.
Backing: test
