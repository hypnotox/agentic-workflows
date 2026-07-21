The catalog package holds the compile-time descriptor set for every artifact kind and target adapter. The claims below capture the current catalog and target contracts.

## Claims

### `invariant: adr-singleton-section-parity`

Each ADR-system singleton's catalog section list equals the awf:section markers declared in its template, and the singleton renders with no unresolved-variable placeholder.
Origin: ADR-0021
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

### `invariant: enabled-set-closed`

Every enabled, non-local artifact's direct catalog requirements (required skills, required agent, required doc) must themselves be enabled; an unmet requirement fails project open with a repair hint.
Origin: ADR-0081
Backing: test

### `invariant: exploration-skill-closure`

Every standard skill that names exploring declares a one-way requirement on it, the default core scaffold includes exploring, and the skill dependency graph introduces no reciprocal edge back from exploring to a consumer.
Origin: ADR-0132
Backing: test

### `invariant: mandatory-doc-pool-exclusion`

Documents flagged as mandatory never appear in the toggleable doc pool that enable and disable operate on. The pool of addable and removable doc names is disjoint from both the mandatory docs and the singleton kinds.
Origin: ADR-0061
Backing: test

### `invariant: no-single-marker-init-descriptor`

The catalog exposes no invariants-marker or invariants-globs var descriptor; the comment-marker mapping reaches configuration only through currentState.sources.
Origin: ADR-0064
Revised-by: ADR-0140
Backing: test

### `invariant: pi-child-process-safety`

In the generated Pi subagent extension, every child exit path removes the temporary role prompt and its listeners, cancellation escalates from TERM to KILL based on the observed process exit, and child errors preserve bounded diagnostics.
Origin: ADR-0123
Backing: test

### `invariant: pi-child-tool-boundaries`

Pi subagent children use an explicitly selected validated model or inherit the parent, inherit the parent's thinking level, receive fixed role allowlists excluding extension tools, and enforce fixed retained-output limits with explicit truncation diagnostics.
Origin: ADR-0123
Revised-by: ADR-0141
Backing: test

### `invariant: pi-extension-target-render`

Enabling the Pi target renders exactly the two governed extension files with valid TypeScript provenance comments and target-sensitive config hashes; a target set without Pi renders neither file, and both files participate in ordinary check, sync, and manifest-cleanup semantics.
Origin: ADR-0123
Backing: test

### `invariant: pi-implementation-state-boundary`

Pi implementation subagent calls serialize against one another, enforce the caller-selected commit permission - reporting a changed HEAD under a no-commit permission as a policy violation without auto-reverting - and report starting and ending git state, marking commit verification unavailable outside a git checkout.
Origin: ADR-0123
Backing: test

### `invariant: pi-minimum-runtime`

The generated Pi extension supports Pi 0.80.9 or newer and emits a single actionable compatibility error on an older runtime instead of registering its tools partially.
Origin: ADR-0123
Backing: test

### `invariant: pi-real-runtime-smoke`

The containerized fixtures are the deterministic gate for the Pi extension, and release readiness additionally requires one real subagent child run on Pi 0.80.9 or newer.
Origin: ADR-0123
Backing: unbacked
Verify: Before a release, perform the documented real-Pi smoke check by running a subagent child against Pi 0.80.9 or newer and record any compatibility finding in the release work.

### `invariant: requires-skills-exact`

An artifact's declared unconditional skill references must match its rendered output exactly: an unconditional prefix-and-skill reference in the output that is not declared, and a declared reference that no longer appears in the output, both fail the template sweep.
Origin: ADR-0080
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

Target encoders consume structured agent metadata, a literal name plus a rendered description and a rendered instruction body, and the Codex TOML encoder never parses a rendered Markdown agent artifact to produce its output.
Origin: ADR-0122
Backing: test

### `invariant: target-dialect-render`

Each enabled target renders every skill and agent exactly once at that target's declared path and dialect, and the emitted artifact parses under that runtime's native format, for example a Codex agent rendering as valid TOML at .codex/agents/<name>.toml.
Origin: ADR-0122
Backing: test

### `invariant: unified-doc-model`

Every doc and singleton projection is derived from the single catalog document collection rather than a separate hand-maintained list. The singleton kinds equal exactly the mandatory catalog entries, the plain singletons equal the mandatory non-agents-doc non-generated entries, and each mandatory non-agents-doc entry renders under the documentation directory at its declared path.
Origin: ADR-0061
Backing: test

### `invariant: var-descriptor-parity`

Every var referenced by any catalog template has a matching var descriptor in the catalog, and no var descriptor names a var that appears in no template.
Origin: ADR-0029
Backing: test

### `invariant: var-descriptor-set-pinned`

The catalog's value-carrying string var descriptor keys are exactly the pinned functional set (gateCmd, gateCmdFull, checkCmd, commitGateCmd, proseGateCmd, testCmd, commitScopes, activeMdRegenCmd, invariantTestPath), and the only multiselect descriptors are the two catalog trims docs and skills.
Origin: ADR-0084
Backing: test
