---
date: 2026-07-30
adrs: [178]
status: Implemented
---
# Plan: Explicit Dependency Composition Architecture

## Goal

Implement [ADR-0178](../decisions/0178-explicit-dependency-composition-architecture.md) by making its repository-wide code-design authority active, preserving reviewer defaults while adding awareness, and delivering `project.Loader` through the `runSync` composition root.

Non-goals: converting other filesystem, process, or package-global seams; adding cancellation, a dependency container, a universal dependency interface, or a second Loader consumer.

## Architecture summary

First freeze ADR-0178 without changing current authority. Then land one incremental authority/governance batch: six prospective dependency-composition claims, the `code-design` scope, default-preserving reviewer hints, workflow awareness, and the native-Git documentation correction. Finally refactor the existing `project.Open` policy into an injected `project.Loader`, compose its production dependencies in `cmd/awf` for the `runSync` family, retain `project.Open` only as the documented compatibility wrapper, add focused tests and foundation documentation, apply the final wiring claim, and freeze the ADR and plan.

The Loader owns project-opening policy. Its constructor receives exactly three dependencies: a config-tree load function, the standard catalog value, and a semantic invoking-root-to-resident-root function. The executable boundary selects production mechanisms; the Loader validates the returned config and owns target resolution, effective-catalog derivation, standard-catalog defense-in-depth validation, and effective-catalog conformance. One-operation dependencies remain functions or values, and no dependency bag, service locator, cancellation parameter, injected catalog-validator function, or unused adapter is introduced.

## File structure

- **Created:** `internal/project/loader_test.go`.
- **Modified authored authority/configuration:** `docs/decisions/0178-explicit-dependency-composition-architecture.md`, `.awf/config.yaml`, `.awf/topics/parts/code-design/dependency-composition/current-state.md`, `.awf/agents/adr-reviewer.yaml`, `.awf/agents/code-reviewer.yaml`, `.awf/agents/plan-reviewer.yaml`, `.awf/parts/workflow/chain.md`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/architecture/data-flow.md`, `.awf/docs/parts/architecture/dependencies.md`, `.awf/docs/parts/development/setup.md`, `.awf/docs/parts/development/dependencies.md`, `.awf/docs/glossary.yaml`, and this plan.
- **Modified production/tests:** `internal/project/project.go`, `internal/project/local.go`, `cmd/awf/sync.go`, and `cmd/awf/run_test.go`.
- **Modified generated outputs:** `.awf/awf.lock`, `docs/decisions/INDEX.md`, `docs/topics/code-design/dependency-composition.md`, `docs/config-reference.md`, `docs/workflow.md`, `docs/architecture.md`, `docs/development.md`, `docs/glossary.md`, `AGENTS.md`, `.claude/agents/adr-reviewer.md`, `.claude/agents/code-reviewer.md`, `.claude/agents/plan-reviewer.md`, `.pi/agents/adr-reviewer.md`, `.pi/agents/code-reviewer.md`, `.pi/agents/plan-reviewer.md`, `.claude/skills/awf-reviewing-adr/SKILL.md`, `.claude/skills/awf-reviewing-impl/SKILL.md`, `.claude/skills/awf-reviewing-plan/SKILL.md`, `.claude/skills/awf-reviewing-plan-resync/SKILL.md`, `.pi/skills/awf-reviewing-adr/SKILL.md`, `.pi/skills/awf-reviewing-impl/SKILL.md`, `.pi/skills/awf-reviewing-plan/SKILL.md`, and `.pi/skills/awf-reviewing-plan-resync/SKILL.md`.
- **Deleted:** none.

All file paths in this plan are exact repository-relative paths rooted at the checkout containing this plan. Run every command from that repository root; `/home/hypno/Projects/agentic-workflows` is the review-time checkout, not durable path authority.

## Phase 1: Freeze the approved decision

**Execution mode: inline.** This is an independently green lifecycle-only transaction. It applies no State-changes operation and leaves the empty dependency-composition topic shell unchanged.

- [ ] **Task 1.1: Accept ADR-0178.** In `docs/decisions/0178-explicit-dependency-composition-architecture.md`, change frontmatter `status: Proposed` to `status: Accepted` and append the dated Accepted entry to `## Status history` using the frozen content digest required by `awf-adr-lifecycle`. Do not alter the frozen Context, Decision, State changes, Consequences, or Alternatives text. Run `./x render`; inspect `git diff -- docs/decisions/0178-explicit-dependency-composition-architecture.md docs/decisions/INDEX.md .awf/awf.lock` and confirm the diff contains only the Accepted transition and its generated index/lock consequences. Run `./awf context --show pending docs/decisions/0178-explicit-dependency-composition-architecture.md .awf/topics/parts/code-design/dependency-composition/current-state.md`; expected terminal state is all seven add operations reported as pending and no active claim attributed to ADR-0178.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage only `docs/decisions/0178-explicit-dependency-composition-architecture.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock`; run `./awf check --staged` and expect zero findings, then `./x gate` and expect success; create the one phase-closing commit:

```commit
docs(adr): accept 0178 explicit dependency composition
```

## Phase 2: Activate code-design authority and governance

**Execution mode: inline.** Start from a clean working tree with Phase 1 committed and `./x check` and `./x gate` successful. This is ADR-0178's first incremental application transaction. It applies, in declaration order, `outer-composition`, `consumer-owned-contracts`, `mechanism-adapters`, `direct-injection-first`, `concrete-first-consumer`, and `dependency-composition-commit-classification`; `sync-project-loader-wiring` remains pending, so the Implementing state is legal.

- [ ] **Task 2.1: Author the six prospective code-design claims and the first application event.** In `.awf/topics/parts/code-design/dependency-composition/current-state.md`, replace the pending-shell sentence with this exact paragraph and append these exact blocks after `## Claims`, in this order:

```markdown
This topic governs dependencies introduced by new work and seams deliberately converted under its authority. Existing direct mechanism calls and package-global seams remain bounded future candidates until a concrete consumer brings them into scope; this authority does not require a wholesale conversion.

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

Every new composition capability lands in the same green transaction as exactly one named concrete first consumer; no adapter, constructor field, interface method, option, or helper is added only for anticipated reuse.
Origin: ADR-0178
Backing: unbacked
Verify: For each newly exported or shared composition symbol, trace its production callers in the same commit and confirm one concrete first consumer uses the whole introduced capability.

### `invariant: dependency-composition-commit-classification`

Dependency-composition and cross-package code-structure work uses the `code-design` scope, and a structural change uses the existing `refactor` type rather than a `refactor` scope.
Origin: ADR-0178
Backing: unbacked
Verify: Compare `.awf/config.yaml` with the rendered scope tables, confirm no `refactor` scope exists, and run `./awf check commit` against the planned dependency-composition subjects after the staged scope addition.
```

  In ADR-0178, change Accepted to Implementing, append the Implementing status event with the same frozen digest, then append one Applied event using the next `state-sequence` value reported at execution time and exactly those six operations. Do not hardcode or predict the repository-global sequence. Leave `sync-project-loader-wiring` unapplied.
- [ ] **Task 2.2: Add the code-design scope and concise awareness without duplicating claims.** In `.awf/config.yaml`, insert `{name: code-design, meaning: dependency composition and cross-package code structure}` into `audit.allowedScopes` in lexical order; do not add a `refactor` scope. Append one sentence to `.awf/parts/workflow/chain.md`: agents changing dependency selection, ownership, or wiring consult `code-design/dependency-composition` before design or implementation. The sentence points to the topic and does not restate its claims.

  In all three agent sidecars, add one concise focus item named `dependency-composition-authority` that directs structural dependency changes to `code-design/dependency-composition` and asks the reviewer to reject speculative capability or a missing concrete first consumer without copying the topic's normative prose. Treat every touched list as wholesale replacement and preserve catalog defaults explicitly:
  - `.awf/agents/adr-reviewer.yaml`: `focusItems` begins with exact catalog defaults `decision-clarity` and `consequences-honesty`, then retains `Schema stability` and `Template invariants`, then the new hint. `docCurrencyItems` retains the exact generic defaults for updating every behavior-stating document, regenerating the decision index, matching State-changes claims, and pre-Accepted topic metadata, followed by every existing project-specific update/remove/rationale check; equivalent project wording may replace a duplicate, but no default obligation may disappear.
  - `.awf/agents/plan-reviewer.yaml`: `focusItems` retains catalog defaults `step-exactness` and `dependency-order` plus every current local focus and the new hint. `docCurrencyItems` begins with the generic same-commit document-update default and retains both current local checks.
  - `.awf/agents/code-reviewer.yaml`: `correctnessTraps` begins with the exact defaults for checked/explicitly ignored errors and empty/zero/nil boundaries, then retains all current local traps; `focusItems` retains catalog defaults `plan-adherence` and `test-coverage`, every current local focus, and the new hint; `docCurrencyItems` begins with the generic same-commit document-update default and retains every current local check.

  Apply the sidecar changes as exact insertions, leaving every unmentioned mapping byte-for-byte unchanged and in its existing order. Insert these exact mappings:

```yaml
# .awf/agents/adr-reviewer.yaml focusItems, before Schema stability
- name: decision-clarity
  description: each Decision item is a discrete, implementable commitment a reader could act on without further consultation
- name: consequences-honesty
  description: trade-offs name real costs and operational implications, not straw men
# append to adr-reviewer focusItems
- name: dependency-composition-authority
  description: when the ADR changes dependency selection, ownership, or wiring, consult code-design/dependency-composition and flag a speculative capability or a capability without one concrete first consumer
# prepend to adr-reviewer docCurrencyItems; its other three catalog defaults already have stricter project-specific equivalents and remain in place
- check: every document that states the behaviour this ADR changes is updated in the same commit

# append to plan-reviewer focusItems
- name: dependency-composition-authority
  description: when a plan changes dependency selection, ownership, or wiring, consult code-design/dependency-composition and reject speculative capability or a capability without one concrete first consumer
# prepend to plan-reviewer docCurrencyItems
- check: the plan schedules updates for every document its changes invalidate, in the same commits

# prepend to code-reviewer correctnessTraps
- description: error paths; every returned error is checked or explicitly ignored with a stated reason
- description: boundary conditions at empty, zero, and null/nil inputs
# append to code-reviewer focusItems
- name: dependency-composition-authority
  description: when the diff changes dependency selection, ownership, or wiring, consult code-design/dependency-composition and flag speculative capability or a capability without one concrete first consumer
# prepend to code-reviewer docCurrencyItems
- check: the change updates every document that states the old behaviour, in the same commit
```

  The ADR reviewer keeps its existing decision-index, claim-handshake, topic-shell, update/remove, and rationale checks; the plan and code reviewers keep every current local item. Compare the resulting arrays with `internal/catalog/standard.go`; `git diff -- .awf/agents/*.yaml` must show only these insertions and no deletion.
- [ ] **Task 2.3: Correct Git documentation and define the new jargon.** In `.awf/docs/parts/architecture/dependencies.md` and `.awf/docs/parts/development/dependencies.md`, replace the stale claim that awf and its tests need no host Git binary. State that `go-git` remains the pure-Go audit implementation, while native `git` is a runtime and test prerequisite for repository control-root resolution, efforts, and managed-worktree operations. In `.awf/docs/parts/development/setup.md`, add native Git to the working-checkout prerequisites. In `.awf/docs/glossary.yaml`, add these exact mappings under `data.terms`:

```yaml
"composition root": "The executable or application boundary that has enough production knowledge to select volatile mechanisms and construct their policy consumers explicitly. It is wiring, not a service locator, universal dependency bag, or owner of the consumer's policy."
"project Loader": "The project-opening policy object established by ADR-0178: it receives config-tree loading, the standard catalog value, and semantic resident-root resolution, then validates and derives one Project. The runSync family is its first explicitly composed consumer; project.Open remains transitional compatibility rather than the new composition path."
```
- [ ] **Task 2.4: Render and verify the governance transaction.** Run `./x render`, then `./x check`; both must finish clean. Run `git diff --name-only` and compare it with the exact Phase 2 authored paths plus the generated paths in File structure; any undeclared path is a plan deviation that must be recorded and settled before staging, not an open-ended derived-file discovery step. In particular, `audit.allowedScopes` must update all eight declared `.claude/skills/awf-reviewing-*` and `.pi/skills/awf-reviewing-*` outputs. Run `go test ./internal/project -run 'TestUnsetFallbackRenders|TestV2ADRTemplateEmptyDataFallback'`; expect success. Run `rg -n '<no value>|\{\{[^}]+\}\}' .claude/agents/adr-reviewer.md .claude/agents/code-reviewer.md .claude/agents/plan-reviewer.md .pi/agents/adr-reviewer.md .pi/agents/code-reviewer.md .pi/agents/plan-reviewer.md`; expect no output. Run `./awf context --show invariants --show all-rules --show evidence --show pending .awf/config.yaml .awf/agents/adr-reviewer.yaml .awf/agents/code-reviewer.yaml .awf/agents/plan-reviewer.yaml .awf/topics/parts/code-design/dependency-composition/current-state.md`; expect the six claims active with ADR-0178 origin and only `sync-project-loader-wiring` pending.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete authored and rendered Phase 2 transaction explicitly, including ADR-0178, the topic part/output, configuration/agent/workflow/documentation/glossary sources and outputs, `AGENTS.md`, all six agent outputs, all eight reviewing-skill outputs, `docs/decisions/INDEX.md`, and `.awf/awf.lock`; run `./awf check --staged` and expect zero findings, then `./x gate` and expect success; create the one phase-closing commit:

```commit
feat(code-design): establish composition authority (applies 0178)
```

## Phase 3: Compose project loading at runSync

**Execution mode: inline.** Start from a clean working tree with Phase 2 committed and `./x check` and `./x gate` successful. This is ADR-0178's final application transaction: it applies only `sync-project-loader-wiring`, then flips ADR-0178 and this plan to Implemented in the same green commit.

- [ ] **Task 3.1: Refactor project-opening policy into `project.Loader`.** In `internal/project/project.go`, define consumer-owned function types for `LoadConfigTree func(string) (*config.Config, error)` and `ResolveResidentRoot func(string) string`, plus a `Loader` holding those functions and a `*catalog.Catalog` standard value. Add `NewLoader(loadConfigTree LoadConfigTree, standard *catalog.Catalog, resolveResidentRoot ResolveResidentRoot) *Loader`; treat nil dependencies as programmer errors and panic with a message naming the missing dependency, rather than silently selecting production globals. Add `(*Loader).Open(root string) (*Project, error)` and move the existing `Open` body into it with this exact order:
  1. call `loadConfigTree(config.RootDir(root))` and return its error unchanged;
  2. call `cfg.Validate()` and return its error;
  3. retain the fixed `catalog.ValidateWorkflowProfiles(catalog.Standard)` defense-in-depth check and its existing genuinely-unreachable coverage exclusion; do not inject that validator and do not redirect it to a test catalog;
  4. resolve targets;
  5. construct `Project` with invoking `Root`, injected `standard`, and `residentRoot: resolveResidentRoot(root)`;
  6. derive the effective catalog from the injected standard value, assign `Cat`, then validate conformance.

  In `internal/project/local.go`, make `effectiveCatalog` clone `p.standard` and its maps rather than `catalog.Standard`; do not mutate the injected catalog. Keep `project.Open(root)` as a documented transitional wrapper with no new caller: it constructs the same production config loader and a best-effort semantic resident-root resolver using `awfgit.ResolveControlRoots(context.Background(), root)`, falling back to `root` on control-root or resident-root failure, and delegates to `NewLoader(...).Open(root)`. Do not add context to the Loader contract, an interface, options, a dependency bag, or a second policy implementation.
- [ ] **Task 3.2: Prove reachable Loader behavior through per-instance dependencies.** Create `internal/project/loader_test.go` in package `project` with these exact tests and fixture shapes:
  - `TestNewLoaderRejectsMissingDependencies`: table cases `load config tree`, `standard catalog`, and `resolve resident root`; pass one nil dependency per case, recover the panic, and require its text to contain the case name.
  - `TestLoaderOpenReturnsLoadError`: root is `t.TempDir()`, loader callback records its argument then returns sentinel `errors.New("load sentinel")`, resolver fails the test if called; require the argument equals `config.RootDir(root)` and `errors.Is(err, sentinel)`.
  - `TestLoaderOpenValidatesBeforeResolvingResidentRoot`: loader returns `&config.Config{}` so `Validate` returns `prefix must not be empty`; resolver fails the test if called; require that exact error text.
  - `TestLoaderOpenUsesSemanticResidentRoot`: write `testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\ntargets: [claude]\n")`; injected load delegates to `config.Load`, resolver records `root` and returns `filepath.Join(root, "resident")`; require `p.Root == root`, `p.residentRoot` equals the injected value, `len(p.Targets) == 1`, `p.Targets[0].Name == "claude"`, `p.Cat != injectedStandard`, and `reflect.DeepEqual(p.Cat.Skills["tdd"], injectedStandard.Skills["tdd"])` is true.
  - `TestLoaderOpenReturnsEffectiveCatalogError`: use config `prefix: example\nskills: [local]\nagents: []\n` and `testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "local.yaml"), "local: [bad\n")`; delegate config load and require a non-nil error naming `skills/local.yaml` before conformance.
  - `TestLoaderOpenReturnsConformanceError`: use config `prefix: example\nskills: [unknown]\nagents: []\n` with no local sidecar; require the existing unknown-skill conformance error naming `unknown`.
  - `TestLoaderOpenDoesNotMutateStandardCatalog`: shallow-copy `*catalog.Standard`, clone its Skills, Agents, and Docs maps, retain a second deep-equal snapshot, open the valid minimal fixture, and require `reflect.DeepEqual(injected, snapshot)` afterward.

  Use per-test Loader instances and `t.Parallel` only where no process or shared filesystem state is mutated. Mark `TestLoaderOpenUsesSemanticResidentRoot` with `// invariant: code-design/dependency-composition:sync-project-loader-wiring` for the policy layer. Do not add a fake/injected standard-catalog validator or a test for its unreachable failure. Run `go test ./internal/project -run 'TestLoader|TestOpen'`; expect success.
- [ ] **Task 3.3: Make the runSync family the single explicit Loader consumer.** In `cmd/awf/sync.go`, compose one production Loader per sync invocation at the outer command boundary: pass `config.Load`, `catalog.Standard`, and a command-owned `resolveProjectResidentRoot(root string) string` adapter to `project.NewLoader`. The adapter closes over `context.Background()`, calls `awfgit.ResolveControlRoots`, translates a successful primary resident root to the worktree root with the existing two-parent `filepath.Dir` calculation, and returns the invoking root on either error. Change the shared printing helper to `runSyncPrinting(loader *project.Loader, root string, seed *project.InitAuthority, stdout io.Writer) error` and call `loader.Open(root)`. Route both `runSync` and `runSyncInitialized` through that helper with their one composed production Loader.

  Preserve this exhaustive production caller set without converting any caller into a Loader consumer: `cmd/awf/dispatch.go`'s `render` handler calls `runSync`; `cmd/awf/init.go:runInit` calls `runSync` or `runSyncInitialized`; `cmd/awf/list_add.go:enableDisableSingleton`, `enableDisableTarget`, and both terminal branches of `toggle` call `runSync`; `cmd/awf/new.go:newLocal` calls `runSync`; and `cmd/awf/upgrade.go:runUpgrade` calls `runSync`.

  In `cmd/awf/run_test.go`, add:
  - `TestResolveProjectResidentRoot`: create and commit a base file in `gitfixture.InitRepo(t)`, run native `git -C <primary> worktree add -b linked <linked>` into a sibling temp path, invoke the resolver from `<linked>`, and require the distinct `<primary>` root. An implementation that always returns the invoking root must fail.
  - `TestResolveProjectResidentRootFallsBackOutsideGit`: `t.TempDir()` must return unchanged.
  - `TestResolveProjectResidentRootFallsBackOnUnsafeResident`: create a gitfixture repo, make `<root>/.awf` a symlink to another temp directory before resolution, and require unchanged root because `ResidentRoot` refuses the symlink.
  - `TestRunSyncPrintingUsesInjectedLoader`: create a gitfixture repo with a base commit, call `testsupport.WriteAwfConfig(t, root, minimalYAML)`, require `os.Stat(config.LockPath(root))` to return `os.ErrNotExist`, inject a Loader whose config callback delegates to `config.Load` while recording the exact config-root argument and whose semantic resolver returns the invoking root, call `runSyncPrinting(loader, root, &project.InitAuthority{InitializedWithVersion: project.Version}, io.Discard)`, and require success plus exactly the expected argument. Mutating the helper back to `project.Open` must make this test fail.
  - `TestSyncCompositionAndCallers`: parse non-test `cmd/awf/*.go` with `go/parser`; require `runSync` and `runSyncInitialized` to construct a Loader and pass it to `runSyncPrinting`, require `runSyncPrinting` to call its Loader's `Open`, require the caller set above by file and enclosing symbol, and reject any other production call to `project.NewLoader`, `runSync`, or `runSyncInitialized`. Mark this test with `// invariant: code-design/dependency-composition:sync-project-loader-wiring`; a bypass at either command entry point or any added post-mutation caller must fail it.

  Use real per-test Git fixtures or per-instance `project.Loader` values, never a mutable package-global factory. Run `go test ./cmd/awf -run 'TestRunSync|TestResolveProjectResidentRoot|TestSyncCompositionAndCallers'`; expect success. Run `rg -n 'project\.NewLoader|runSync(Initialized)?\(' cmd/awf --glob '*.go' --glob '!**/*_test.go'`; require only `sync.go` Loader construction plus the enumerated definitions/callers above. Run `rg -n 'func Open|NewLoader.*Open' internal/project/project.go`; require `project.Open` to remain the sole documented compatibility construction.
- [ ] **Task 3.4: Document the implemented foundation at its authored sources.** In `.awf/docs/parts/architecture/components.md`, identify `cmd/awf` as the composition root for render/post-mutation sync and `internal/project.Loader` as the project-opening policy boundary. In `.awf/docs/parts/architecture/data-flow.md`, describe production selection flowing from `runSync` through injected config-tree load, standard catalog, and semantic resident-root resolution into Loader validation/derivation. In `.awf/docs/parts/development/dependencies.md`, add a concise contributor rule to compose new volatile mechanisms at the outer boundary, use consumer-owned semantic functions/values first, and consult `code-design/dependency-composition`; do not duplicate the claim text. Rendered `docs/architecture.md` and `docs/development.md` must state the new wiring and retain the corrected native-Git prerequisite from Phase 2.
- [ ] **Task 3.5: Apply the wiring claim and freeze the records.** Append this exact block to `.awf/topics/parts/code-design/dependency-composition/current-state.md`, matching the policy-layer marker in `internal/project/loader_test.go` and the command/caller-layer marker in `cmd/awf/run_test.go`:

```markdown
### `invariant: sync-project-loader-wiring`

Top-level render, initialized render, and every existing post-mutation render reach project opening through the one Loader composed by the `runSync` family; `project.Open` remains a transitional compatibility wrapper with no new caller.
Origin: ADR-0178
Backing: test
```

  In ADR-0178, append the final Applied event using the next state-sequence reported at execution time and only `add code-design/dependency-composition:sync-project-loader-wiring`, then append the Implemented status event with the frozen digest and change frontmatter to Implemented. In this plan, record any execution deviation under Notes and change `status: Proposed` to `status: Implemented`; do not edit it after this commit.
- [ ] **Task 3.6: Render and verify the terminal transaction.** Run `gofmt -w internal/project/project.go internal/project/local.go internal/project/loader_test.go cmd/awf/sync.go cmd/awf/run_test.go`. Run `go test ./internal/project ./cmd/awf`; expect success. Run `./x render` and `./x check`; expect clean completion and generated `docs/decisions/INDEX.md`, topic, architecture/development, and lock output matching their authored inputs. Run `./awf topic code-design/dependency-composition --coverage`; expect all seven claims present, with `sync-project-loader-wiring` test-backed and no pending operation. Run `./awf context --show invariants --show all-rules --show evidence --show pending internal/project/project.go internal/project/loader_test.go cmd/awf/sync.go`; expect no ADR-0178 operation pending and the Loader proof marker reported.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete Phase 3 code, tests, authored docs/topic, ADR/plan lifecycle changes, and rendered outputs explicitly; run `./awf check --staged` and expect zero findings, then `./x gate` and expect success; create the one phase-closing commit:

```commit
refactor(code-design): compose project loader (implements 0178)
```

## Verification

- `git status --short` prints no output after the final commit.
- `./x check` exits successfully with zero drift, authority, frontmatter, or advisory findings.
- `./x gate` exits successfully, including 100 percent statement coverage and dead-code checks.
- `./awf context --show invariants --show all-rules --show evidence --show pending internal/project/project.go cmd/awf/sync.go` reports the global dependency-composition authority, the Loader test evidence, and no pending ADR-0178 operation.
- `git log --format='%s' -3` shows one Accepted transition, one first application batch, and one final implementation commit with allowed scopes and subjects no longer than 72 characters.
- `git diff HEAD~3..HEAD -- .awf/efforts` prints no output: implementation and review never modify effort memory.

## Notes

- During Phase 2 verification, the broad unresolved-token `rg` matched four pre-existing intentional literal template examples and no `<no value>` output; inspection confirmed the generated reviewer prompts introduced no unresolved value.
- Phase 3 also updates `internal/project/context.go`: its working-tree snapshot must carry the Loader-selected standard catalog into the snapshot Project, while the standalone staged-context entry selects `catalog.Standard` explicitly. This necessary propagation was omitted from the planned file list and prevents snapshot context assembly from dereferencing an absent standard catalog after `effectiveCatalog` stops reading the package global.
- The paused filesystem/global-seam refactor remains a downstream consumer after this plan; it is not resumed or edited here.
- Existing direct mechanism calls and package-global seams outside the Loader slice are not converted by this plan.
- The Loader contract deliberately has no cancellation input. Production resident-root resolution closes over `context.Background()` until a real signal-aware or embedded consumer exists.
- Implementation findings and deviations are appended here before the Phase 3 status flip; after `status: Implemented`, this plan is frozen.
