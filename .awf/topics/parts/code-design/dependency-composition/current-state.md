This topic governs dependencies introduced by new work and seams deliberately converted under its authority. Existing direct mechanism calls and package-global seams remain bounded future candidates until a concrete consumer brings them into scope; this authority does not require a wholesale conversion.

## Claims

### `invariant: outer-composition`

A new or deliberately converted volatile dependency is selected explicitly at the outermost layer with enough production knowledge; its policy consumer receives the selected semantic dependency and does not discover it through a service locator, universal dependency bag, or mutable package global.
Origin: ADR-0178
Backing: unbacked
Verify: For each changed constructor and executable wiring site, trace production selection to the outermost knowledgeable layer and confirm no prohibited discovery mechanism supplies the dependency.

### `invariant: consumer-owned-contracts`

Each new or deliberately converted seam is the narrowest contract owned by its consumer and is named for the semantic operation rather than a filesystem, process, Git-command, or other mechanism representation.
Origin: ADR-0178
Backing: unbacked
Verify: For each changed seam, identify its consumer, required operations, and mechanism boundary; confirm the contract contains no operation or representation the consumer does not need.

### `invariant: mechanism-adapters`

A mechanism adapter remains outside the policy package it serves, translates mechanism-specific values and errors at that boundary, and does not absorb policy owned by the consumer.
Origin: ADR-0178
Backing: unbacked
Verify: Inspect every changed adapter's package direction, returned values, errors, and decisions; confirm mechanism representation stops at the boundary and policy stays with the consumer.

### `invariant: direct-injection-first`

A one-operation dependency is injected as a function and an immutable input as a value; an interface is introduced only for a cohesive multi-operation behavioral contract with domain meaning, and a required dependency never silently defaults.
Origin: ADR-0178
Backing: unbacked
Verify: Inspect changed constructor parameters and nil handling, and reject an interface, hidden production default, or test-only production indirection that is not required by the consumer contract.

### `invariant: concrete-first-consumer`

Every new production composition capability lands in the same green transaction as exactly one named concrete production first consumer. A composition capability exported by a dedicated shared test-support package under `internal/testsupport/**` instead lands with exactly one named outside-package test first consumer. In either case the consumer uses the whole introduced capability, and no adapter, constructor field, interface method, option, helper, fault operation, or other composition surface is added only for anticipated reuse.
Origin: ADR-0178
Revised-by: ADR-test-support-exports-earn-test-consumers
Backing: unbacked
Verify: For each newly exported or shared composition symbol, classify its declaring package, trace the corresponding production or outside-package test caller in the same commit, confirm exactly one named first consumer uses the whole introduced capability, and reject every introduced member without that consumer use.

### `invariant: dependency-composition-commit-classification`

Code-design authority and cross-package code-structure work uses the `code-design` scope, and a structural change uses the existing `refactor` type rather than a `refactor` scope.
Origin: ADR-0178
Revised-by: ADR-0180, ADR-0210
Backing: unbacked
Verify: Compare `.awf/config.yaml` with the rendered scope tables, confirm no `refactor` scope exists, and run `./awf check staged commit` against the planned code-design subjects.

### `invariant: sync-project-loader-wiring`

Top-level render, initialized render, and every existing post-mutation render reach project opening through the one Loader composed by the `runSync` family; `project.Open` remains a transitional compatibility wrapper with no new caller.
Origin: ADR-0178
Backing: test
