---
date: 2026-07-31
adrs: [0194]
status: Proposed
---
# Plan: Decompose internal project

## Goal

Execute ADR-0194: carve `internal/contextq` and `internal/resident` out of `internal/project`,
repair the core's internal straddles, land the presentation-ownership rule with its two in-scope
conversions, consolidate template-ID derivation, widen kind dispatch to the whole module, and land
all seven state operations with their backing. Non-goals: any `internal/git` change, the
slash-space/OS-space discipline, fixture-builder consolidation, and conversions of command
surfaces beyond context and config-reference.

**Sequencing gate:** implementation starts only after the single-home effort's branch
(`awf/single-home-and-git-seam-decisions`) integrates to main and this effort's branch is
rebased or fast-forwarded onto that state. Baseline before Phase 1: `git merge --ff-only main`
(or rebase) succeeds, `./x check` clean, `./x gate` exit 0. Satisfied 2026-07-31: the
single-home branch merged to main at 556185fc and this branch rebased onto main 373f14f2.

**Claim application (V2):** the ADR flips to `Implementing` in Phase 1's closing commit (first
batch). Batches follow the ADR's declaration order exactly: Phase 1 applies op 1, Phase 2 op 2,
Phase 3 op 3, Phase 4 op 4, Phase 5 op 5, Phase 6 op 6. Operation 7 is the final batch and
rides the terminal-review transaction together with the `Implemented` flip (an all-applied
`Implementing` state is illegal, so the last Applied event and the flip must land together);
that transaction is owned by terminal review, not a plan phase, and its exact contents are
listed after Phase 6. Each phase-closing commit that applies a batch appends its Applied event
to the ADR Status history and lands the claim prose given in that phase.

## Architecture summary

Six phases, each one green transaction: (1) in-core straddle repairs plus module-wide kind
dispatch; (2) the `internal/resident` carve below core; (3) the `internal/contextq` carve above
core behind the `ContextState` seam, with the render bank descending from `cmd/awf`; (4) the
presentation-ownership topic anchored and the config-reference rendering typed into core; (5)
template-ID derivation consolidated into the descriptor and declaration tables; (6) the
state-ownership scanner widened and the roadmap and glossary obligations closed. The
package-ownership guard and operation 7 land in the terminal transaction after Phase 6.
Design and rationale: ADR-0194.

## File structure

- **Created:** `internal/resident/` (production files for the table, predicate, `Roots`,
  lifecycle operations, plus tests), `internal/contextq/` (the five moved query files, the
  render bank, plus moved and new tests), structural test files named per phase.
- **Modified:** `internal/project/` (check.go, validate.go, output_plan.go, render.go,
  project.go, currentstate.go, sweep.go, topics.go, scaffold.go, local.go, kind.go,
  context files until they move, configreference.go, stateownership_test.go and sibling tests),
  `cmd/awf/` (context.go, config.go, init.go, uninstall.go, sync.go, list_add.go, new.go and
  their tests), `internal/topic/` (render entry signature), `.awf/domains/rendering.yaml`,
  `.awf/domains/tooling.yaml`, `.awf/topics/metadata/tooling/context-and-topic.yaml`,
  `.awf/topics/metadata/rendering/project-output-plan.yaml`, topic part files for the five
  claim-prose landings, `.awf/agents/adr-reviewer.yaml`, `.awf/agents/code-reviewer.yaml`,
  `.awf/agents/plan-reviewer.yaml`, `.awf/parts/workflow/chain.md`,
  `.awf/docs/parts/architecture/` and `.awf/docs/parts/roadmap/deferred.md` and
  `.awf/docs/parts/glossary/` source parts with their rendered outputs,
  `docs/decisions/0194-...md` (status events), and the render outputs every phase-closing
  commit stages: `docs/decisions/INDEX.md`, `.awf/awf.lock`,
  `docs/topics/rendering/project-output-plan.md`, `docs/topics/tooling/context-and-topic.md`,
  `docs/topics/code-design/presentation-ownership.md`,
  `docs/topics/code-design/state-ownership.md`.
- **Deleted:** the five context production files (`internal/project/context.go`,
  `context_paths.go`, `context_projection.go`, `context_artifacts.go`, `context_adr.go`) and
  their five sibling tests (`context_test.go`, `context_paths_test.go`,
  `context_projection_test.go`, `context_artifacts_test.go`, `context_adr_test.go`) - moved,
  not lost; `internal/project/context_wrapper_test.go` is NOT among them and stays (it drives
  the `./x context` wrapper by exec and carries the context-spill-observability proof marker);
  `validateDeclarationPlanParity` and its direct tests (Phase 5).

## Phase 1: Core straddle repairs and module-wide kind dispatch

**Execution mode: inline.**

- [ ] **Task 1.1: Relocate the cycle edges.** Move `validateArtifact` (a free function) and
  `localOutPaths` (a `*Project` method), both currently in `internal/project/check.go`, into
  `internal/project/validate.go` verbatim (no behavior change). Post-check:
  `grep -n "func validateArtifact\|func (p \*Project) localOutPaths" internal/project/check.go`
  returns no output (exit 1) and the same grep over `internal/project/validate.go` returns two
  lines; `go test ./internal/project/` passes.
- [ ] **Task 1.2: Relocate the small straddles.** Move `ScaffoldVarRefs` from
  `internal/project/local.go` to `internal/project/scaffold.go`; move the `ArtifactRole` type
  and its constants from `internal/project/context_artifacts.go` into
  `internal/project/output_plan.go`. For the two helpers with callers on both sides of the
  Phase 3 split: hoist `pathMatchesAny` (context.go) into `internal/pathglob` as exported
  `MatchAny` (it is a small loop over `pathglob.Match`; both future sides import pathglob) and
  update its callers (`currentstate.go:571` and the context files); leave
  `safelyMatchablePaths` in `internal/project/topics.go` for the staying callers, and Phase 3
  gives `contextq` its own private copy (it is a small filter over `snapshot.Tree.List`).
  Post-check: `go build ./...` clean; `go test ./internal/project/ ./internal/pathglob/`
  passes.
- [ ] **Task 1.3: Add the missing descriptor facets.** In `internal/project/kind.go`, extend
  `kindDescriptor` with the two boolean facets the cmd-side facts need: `graphKind` (true for
  skill, agent, doc) and `freeformDomain` (true for the domains kind), populated in the ordered
  table. Extend the existing table-completeness test in `internal/project/kind_test.go` to
  assert the new facets are set for the kinds named above and unset otherwise.
- [ ] **Task 1.4: Fold the cmd/awf kind facts into the table.** Replace every cmd-side kind
  fact with calls through narrow exported descriptor lookups (for example
  `project.IsGraphKind(kind string) bool`, `project.IsFreeformDomainKind(kind string) bool`,
  `project.IsDocKind(kind string) bool`, implemented via the table). Affected sites
  (exhaustive): `cmd/awf/list_add.go` (the `isGraphKind` function body, both
  `kind == "domain"` branches, and the `op.Node.Kind == "skill"` comparison at ~:297) and
  `cmd/awf/new.go` (the catalog-kind switch arm at ~:31 and the `isDoc := kind == "doc"`
  comparison at ~:218). The `case "adr"` / `"plan"` / `"topic"` arms and the usage string in
  new.go are command routing over non-catalog artifact names, not kind facts; they stay.
  Post-check: `go test ./cmd/awf/` passes and the Task 1.5 structural test passes.
- [ ] **Task 1.5: The widened kind proof.** Add a source-scanning structural test in
  `internal/project/kind_test.go` (same AST-walking shape as
  `internal/project/stateownership_test.go`) that parses the `cmd/awf` production sources and
  fails on any equality or switch-case comparison whose string-literal operand is one of the
  descriptor table's kind names (singular or plural), with the exported accessor bodies as the
  only allowlist. Multiple proofs on one claim are legal (`internal/topic/markers.go` requires
  only proofs > 0, and the corpus already carries multi-proof claims), so keep the existing
  marker on the table-completeness test and add a second
  `// invariant: rendering/project-output-plan:kind-dispatch-single-table` marker on this scan.
  Post-check: the new test passes (`go test ./internal/project/ -run` its name, PASS).
- [ ] **Task 1.6: Land the updated claim prose and the Implementing flip.** In
  `.awf/topics/parts/rendering/project-output-plan/current-state.md`, replace the
  `kind-dispatch-single-table` claim body with: "Every per-kind facet - the config enable array,
  catalog pool, declared sections, output path, singular and plural labels, graph membership,
  and freeform-domain membership - is defined once in the single ordered kind-descriptor table
  in the project package, and cmd/awf decides no kind fact outside the table's exported
  accessors; a test asserts the table's kind set equals the catalog's kinds plus the freeform
  domains kind, and a source-scanning test over the cmd/awf sources asserts no kind-name
  equality or switch-case comparison remains there." (Phase 1 review reworded this from a
  module-wide universal: in-core kind-literal dispatch beside the table falsified it; see
  Notes.) Keep `Origin: ADR-0027`, add
  `Revised-by: ADR-0194`, keep `Backing: test`. Append to the ADR's Status history an
  `Implementing` event and an `Applied` event for operation 1 per the
  status-event format in `docs/decisions/template.md` (`- YYYY-MM-DD: Applied; operations:
  <operation-list>`; ADR-0191 removed the state sequence, so no sequence is written).
  Obtaining the stamp is mechanical, never guessed: write 64 zeros as the digest, run
  `./awf check state`, and copy the computed digest from
  the mismatch message; the same content stamp repeats on every later status event of this
  ADR unless an Amended event changes it. Run `./x render` and stage every rendered output
  (`docs/decisions/INDEX.md`, `.awf/awf.lock`, and the rendered topic doc). Post-check:
  `./awf check --staged` clean.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): repair core straddles and widen kind dispatch
```

## Phase 2: The internal/resident carve

**Execution mode: subagent-driven.** Baseline: `git status --short` empty, `./x check` clean,
`./x gate` exit 0, in the effort's managed worktree on
`awf/decompose-the-internal-project-package`.

- [ ] **Task 2.1: Create the package.** New `internal/resident/resident.go` holding, moved
  verbatim from `internal/project/install.go` and `internal/project/project.go` and
  `internal/project/currentstate.go`: the `residentRoots` table (exported as needed by core's
  render iteration), `residentRootNames`, the resident-path predicate (exported
  `IsResidentPath`), `inspectResidentRoots`, `preserveResidentRemoval`, `CollisionsAt`,
  `Uninstall`, and a new `Roots` value type `{Tracked, Resident string}` constructed by
  `NewRoots(tracked, resident string) Roots` with a `ResolveOutput(path string) string` method
  carrying the body of `project.outputPath`. `InitCollisions`, `BackupFile`, `Backup`, and
  `Change` stay in `internal/project`. Every moved symbol keeps its behavior; no signature
  changes beyond receiver-to-parameter conversion (`*Project` methods become functions or
  `Roots` methods taking what they read).
- [ ] **Task 2.2: Point core and cmd at the package.** Batch task. Representative: in
  `internal/project/project.go`, construct `resident.NewRoots(root, residentRoot)` once in
  `Open`, store it as a `Project` field (a construction input, per the state-ownership claim),
  and replace each `p.outputPath(x)` call with `p.roots.ResolveOutput(x)`. Edge: the
  `render.go` and `output_plan.go` iterations over the resident-root table import
  `internal/resident` and range its exported table; no other file may re-declare the table.
  Exhaustive affected sites: `internal/project/{project.go,check.go,output_plan.go,render.go,currentstate.go,sweep.go,install.go}`
  and `cmd/awf/{init.go,uninstall.go}` (rewire `project.CollisionsAt` and `project.Uninstall`
  to `resident.CollisionsAt`/`resident.Uninstall`) and `cmd/awf/sync.go` (imports unchanged;
  `Backup`/`Change` stay project types). Post-check:
  `grep -rn "func (p \*Project) outputPath\|residentRoots\b" $(ls internal/project/*.go | grep -v _test)`
  returns no output (exit 1), and `go build ./... && go test ./internal/project/ ./internal/resident/ ./cmd/awf/`
  passes.
- [ ] **Task 2.3: Move the tests and markers.** Move `internal/project/install_test.go` content
  that exercises moved symbols into `internal/resident/resident_test.go` (external test package
  if it needs project); keep tests of `InitCollisions`/`BackupFile` in project. The
  `touches-state` markers currently in `install.go` move with their functions; they stay valid
  because Task 2.5 widens the owning selectors in this same transaction.
- [ ] **Task 2.4: Route the render kind comparison and prove single home.** First, route
  `internal/project/render.go`'s `kind != "efforts" && kind != "worktrees"` comparison
  (~:755) through the resident package (an exported name-set predicate, for example
  `resident.IsResidentKind(kind string) bool`). Then add the structural test
  `internal/resident/singlehome_test.go`: AST-scan production files under `internal/project`
  and `cmd/` and fail if any re-declares the resident-root table shape or string-compares
  against the resident root names outside the resident package. `internal/git`'s
  `ResidentName` constants are deliberately out of the predicate's scope: they are the git
  seam's own spelling, decided untouched (ADR-0194 item 7), and the ADR Consequences records
  the tolerated parallel. Carry the proof marker for
  `rendering/project-output-plan:resident-policy-single-home`.
- [ ] **Task 2.5: Ownership, claim prose, Applied event.** Add `internal/resident/**` to
  `.awf/domains/rendering.yaml` paths and to
  `.awf/topics/metadata/rendering/project-output-plan.yaml` paths. Land the claim in
  `.awf/topics/parts/rendering/project-output-plan/current-state.md`: slug
  `resident-policy-single-home`, body: "The resident-root table, the resident-path predicate,
  and anchored output-path resolution have exactly one production home in internal/resident;
  core consumes them through the Roots value constructed once at project open, and no file
  under internal/project or cmd redeclares or re-derives the table or predicate
  (internal/git's seam-owned ResidentName spelling is the recorded tolerated parallel)."
  `Origin: ADR-0194`, `Backing: test`. Update the architecture source part sentence describing the resident-root
  table (`.awf/docs/parts/architecture/`) to name `internal/resident`. Append the Applied event
  for operation 2 to the ADR. `./x render`; post-check `./awf check --staged` clean and
  `./awf context internal/resident` reports the package covered, not unowned.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): carve internal/resident below the core
```

## Phase 3: The internal/contextq carve

**Execution mode: subagent-driven.** Baseline: `git status --short` empty, `./x check` clean,
`./x gate` exit 0.

- [ ] **Task 3.1: The ContextState seam.** In `internal/project` (new file
  `internal/project/contextstate.go`): define exported `ContextState` carrying what the moved
  query code consumes and core will not export separately: the effective catalog, `Layout`, the
  `Roots` value, `Cfg`, `Targets`, the loaded working or staged current-state
  (`currentstate.Loaded`), the lock, the snapshot tree, and the output declarations. Two
  constructors, both core-side: `(p *Project) ContextState() (ContextState, error)` (working
  tree, wrapping `workingCurrentState` and friends) and
  `StagedContextState(root string) (ContextState, error)` (staged tree, wrapping the load half
  of today's `StagedContextRootOptions`). All fields are construction inputs set inside the
  constructor; no other core internal becomes exported for this seam.
- [ ] **Task 3.2: Move the query.** Move `context.go`, `context_paths.go`,
  `context_projection.go`, `context_artifacts.go`, `context_adr.go` (minus the symbols already
  relocated in Phase 1 and minus the load halves absorbed into Task 3.1) to
  `internal/contextq/`, package `contextq`, with one constructor
  `New(state project.ContextState) *Query` and every moved entry point a method on `*Query`
  (one construction path, full construction invariant; no free-function alternative) replacing
  the `*Project` receivers. `ContextForOptions` and `Uncovered` and the staged query entry
  points become `*Query` methods. `ArtifactRole` references become `project.ArtifactRole`.
  Unlisted unexported helpers of those files move with them; `contextq` receives its own
  private copy of `safelyMatchablePaths` per Task 1.2. Post-check: `go build ./...` clean;
  `grep -rn "classifyContextPath\|projectTopicImpact\|assembleContextUniverse" internal/project/*.go`
  returns no output (exit 1).
- [ ] **Task 3.3: Descend the render bank.** Move the `render*` functions from
  `cmd/awf/context.go` into `internal/contextq` (exported entry per rendered report, for
  example `RenderContextText(result) string`); `cmd/awf/context.go` keeps flag parsing, the
  text-vs-JSON switch, and exit mapping, and calls `contextq` for both the query and the text.
  The eight functions to descend: `renderUncovered`, `renderContext`, `renderPathImpact`,
  `renderRelationships`, `renderTopicImpact`, `renderClaimCategory`, `renderEvidence`,
  `renderList`. Post-check: `grep -n "func render" cmd/awf/context.go` returns no output
  (exit 1).
- [ ] **Task 3.4: Move tests, convert packages, unexport.** Move the five sibling test files
  and the field-assertion content of `cmd/awf/context_test.go` into `internal/contextq` tests;
  convert `internal/project/output_plan_test.go`'s two context-calling cases and any
  `stateownership_test.go` prose references to the new entry points (external test package
  where needed). Then unexport every context result type not named by `cmd/awf` production
  after the descent (the command-facing surface stays: `ContextResult`, `ContextOptions`,
  `ContextFacet`, `UncoveredResult`, `NormalizeContextPaths`, plus the selection/status
  constants the JSON output needs). Post-check: `go build ./... && go vet ./...` clean;
  `go test ./internal/contextq/ ./internal/project/ ./cmd/awf/` passes.
- [ ] **Task 3.5: The boundary proof.** New structural test in
  `internal/contextq/boundary_test.go`: fail if `internal/project`'s exported surface (AST
  scan) contains any of the moved result-vocabulary type names, or if `internal/contextq`
  production imports any `internal/project` symbol other than `ContextState`, its
  constructors, and the deliberately shared vocabulary (`ArtifactRole`, `Kinds`). Carry the
  proof marker for `tooling/context-and-topic:context-query-boundary`.
- [ ] **Task 3.6: Ownership, claim prose, Applied event.** Add `internal/contextq/**` to
  `.awf/domains/tooling.yaml` paths and to
  `.awf/topics/metadata/tooling/context-and-topic.yaml` paths. Land the claim in
  `.awf/topics/parts/tooling/context-and-topic/current-state.md`: slug
  `context-query-boundary`, body: "Context assembly, classification, projection, and result
  rendering live in internal/contextq; internal/project's exported surface carries no context
  result vocabulary, and contextq reaches core state only through the assembled context-state
  value and its two core-side constructors." `Origin: ADR-0194`, `Backing: test`. Update the
  architecture source part passages (the internal/project role description and the cmd/awf
  presentation sentence). Append the Applied event for operation 3. `./x render`; post-check
  `./awf check --staged` clean and `./awf context internal/contextq` reports covered.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): carve contextq behind the ContextState seam
```

## Phase 4: Presentation ownership anchored and the config-reference conversion

**Execution mode: inline.**

- [ ] **Task 4.1: Typed config-reference rendering in core.** Today
  `ConfigReferenceModel` returns `map[string]any` (built by `configReferenceData`, which the
  doc generator `generateConfigReference` shares), and `cmd/awf/config.go` renders it plus a
  cmd-local `staticModel()` map. The change: introduce typed row and section types in
  `internal/project/configreference.go` as a typed projection beside `configReferenceData`
  (the doc-generator path keeps its current shape so `docs/config-reference.md` stays
  byte-identical); change `ConfigReferenceModel` to return the typed model; move the
  rendering (`printConfigReference` and helpers) AND the `staticModel` builder into core as
  typed rendering with no discarded type assertions; `cmd/awf/config.go` keeps flag parsing,
  selection, and exit mapping. The three config-reference invariants
  (`config/configspec-and-reference:config-reference-regen-drift`,
  `:config-reference-no-bare-vars`, `:config-reference-data-rejected`) must stay green.
  Post-check: `grep -n "map\[string\]any" cmd/awf/config.go` returns no output (exit 1);
  `go test ./cmd/awf/ ./internal/project/` passes with byte-identical `awf config` output and
  `./x render` reporting no change to `docs/config-reference.md` (any diff is a defect in the
  renderer, not the tests).
- [ ] **Task 4.2: Anchor the topic.** Add a presentation-ownership focus item to
  `.awf/agents/adr-reviewer.yaml`, `.awf/agents/code-reviewer.yaml`, and
  `.awf/agents/plan-reviewer.yaml` (these sidecars replace catalog defaults wholesale:
  first copy the catalog's current default focus items into any sidecar not already carrying
  them, then append the new item, mirroring how ADR-0180 item 8 landed its focus item). Add
  one clause to `.awf/parts/workflow/chain.md` directing an agent changing where a result
  model is rendered to consult `code-design/presentation-ownership`. Add glossary entries
  (`.awf/docs/parts/glossary/`) for: presentation ownership, context state, `Roots`
  anchoring value, resident-root policy.
- [ ] **Task 4.3: Claim prose, Applied event.** Land in
  `.awf/topics/parts/code-design/presentation-ownership/current-state.md`: slug
  `model-owner-renders`, body: "The package that owns a result model owns its human rendering;
  a command binary keeps argument parsing, renderer selection, and exit mapping."
  `Origin: ADR-0194`, `Backing: unbacked`,
  `Verify: when touching a command surface, confirm the rendering of each result model it prints lives in the package owning that model.`
  Append the Applied event for operation 4. `./x render`; `./awf check --staged` clean.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(code-design): anchor the presentation-ownership rule
```

## Phase 5: Template-ID single derivation

**Execution mode: inline.**

- [ ] **Task 5.1: Consolidate derivation.** The sanctioned declaration files are
  `internal/catalog/standard.go` (the catalog's own TID declarations),
  `internal/project/kind.go`, and the declaration tables in
  `internal/project/{output_plan.go,target.go,singleton.go}`. Rerun
  `grep -rn '\.tmpl"' --include=*.go internal/ cmd/ | grep -v _test` and disposition every
  hit outside the sanctioned files, by category: the inline skill/base tid builders in
  `output_plan.go` and the fallback TIDs in `local.go` (`baseSkillTID`/`baseAgentTID`/
  `baseDocTID`) fold into descriptor-table closures; `project.go`'s `coOwnedRunnerTID` and
  the bootstrap/runner literals in `output_plan.go` move into the declaration tables; the
  hook tids spelled in both `render.go` and `output_plan.go` collapse to one table entry; the
  resident table sheds its `TemplateID` field entirely (resident carries only names; core
  keeps `effortsTID`/`worktreesTID` as the single derivation, and `render.go`'s and
  `output_plan.go`'s iterations pair a resident name with its core-derived ID); the
  `scaffold.go` and `configreference.go` literals route through descriptor lookups; the
  `topics.go` re-read sites die in Task 5.2; the bare `.tmpl` suffix-trim literals in
  `check.go` are string operations, not IDs, and are exempt by the Task 5.3 predicate.
  Byte-identity is the oracle: after this task `./x render` reports no changed files and
  `git diff --stat` over rendered outputs is empty.
- [ ] **Task 5.2: Stop re-reading in topic.** Change `internal/topic`'s render entry
  (`RenderTopic` and the doc-generation path called from `internal/project/topics.go:43,47`)
  to accept template identity and content as parameters; delete the embedded re-read inside
  `internal/topic`. `internal/project` passes the bytes it already holds. Post-check:
  `grep -rn "embed\|ReadFile" internal/topic/render.go` shows no template re-read;
  `./x render` byte-identical.
- [ ] **Task 5.3: Retire the parity check.** Delete `validateDeclarationPlanParity`
  (`internal/project/output_plan.go`) and its direct tests; its callers drop the call. Add the
  structural test for the claim: new `internal/project/templateid_test.go` scanning production
  files under `internal/` and `cmd/` and failing on any string literal that contains `/` and
  ends `.tmpl` (a full template-ID path, distinguishing IDs from bare suffix-trim literals)
  outside the sanctioned declaration files named in Task 5.1. Carry the proof marker for
  `rendering/project-output-plan:template-id-single-derivation`.
- [ ] **Task 5.4: Claim prose, Applied event.** Land in
  `.awf/topics/parts/rendering/project-output-plan/current-state.md`: slug
  `template-id-single-derivation`, body: "Template identity derives from the catalog and
  kind-descriptor declaration tables alone; no production file outside those declaration
  files spells a full template-ID path literal, and internal/topic receives template identity
  and content from its caller rather than re-reading the embedded tree." `Origin: ADR-0194`,
  `Backing: test`. Append the Applied
  event for operation 5. `./x render`; `./awf check --staged` clean.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(rendering): derive every template ID from the tables
```

## Phase 6: Scanner widening and the record

**Execution mode: inline.**

- [ ] **Task 6.1: Widen the state-ownership scanner.** Extend
  `internal/project/stateownership_test.go` to load `./internal/project`,
  `./internal/contextq`, and `./internal/resident`, asserting no production function in any of
  the three writes a field of that package's constructed long-lived values outside its
  constructing function; update the rule comment to name the current conforming constructions
  (`Loader.Open`, the two `ContextState` constructors, `contextq.New`, `resident.NewRoots`).
  Keep the existing proof marker for
  `code-design/state-ownership:project-derived-state-ownership` on the widened test.
- [ ] **Task 6.2: Update the claim prose.** In
  `.awf/topics/parts/code-design/state-ownership/current-state.md`, replace the
  `project-derived-state-ownership` body with: "No production function in internal/project,
  internal/contextq, or internal/resident writes a field of that package's constructed
  long-lived values outside the function that constructs the value: the ADR corpus, topic
  corpus, effective skill set, context state, and Roots are derived by the operation that
  needs them and threaded to their consumers." Keep `Origin: ADR-0180`, add
  `Revised-by: ADR-0194`, keep `Backing: test`.
- [ ] **Task 6.3: Close the record.** Rewrite the roadmap's deferred-decomposition section
  (`.awf/docs/parts/roadmap/deferred.md`) to point at ADR-0194, recording the sequencing
  reversal and keeping `receiver-reads-owned-state` explicitly open for a future cohesion
  pattern. Verify the architecture part passages from Phases 2 and 3 are current. Append the
  Applied event for operation 6. `./x render`; `./awf check --staged` clean.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(code-design): widen state ownership and close the record
```

## Terminal transaction (owned by terminal review, not a plan phase)

After the terminal implementation review settles, the reviewing-impl flow lands the final
Applied batch and the status flip in one transaction (`awf-adr-lifecycle` mechanics). Its
contents, so the reviewer executes rather than designs:

- The ownership guard: new structural test in `internal/topic`'s test files (domain-coverage
  machinery lives there): resolve the repository root by walking upward from the test's
  working directory to the directory containing `go.mod`, enumerate every production package
  under `internal/` and `cmd/` (directories containing non-test `.go` files, excluding
  `internal/testsupport/testdata`, whose Go files are fixtures), and fail if any package's
  path is matched by no domain's `paths` selectors (load the domain metadata the same way
  production coverage code does). Carry the proof marker for
  `tooling/context-and-topic:production-packages-domain-owned`.
- The claim prose in `.awf/topics/parts/tooling/context-and-topic/current-state.md`, slug
  `production-packages-domain-owned`, body: "Every production package under internal/ and
  cmd/ is matched by at least one domain's paths; a package omitted from domain ownership
  fails the structural test rather than degrading silently to unowned." `Origin: ADR-0194`,
  `Backing: test`.
- The final Applied event for operation 7 and, immediately after it, the `Implemented` status
  event (same content stamp; no state sequence exists post-ADR-0191), plus this plan's
  `status: Implemented` flip. An all-applied `Implementing` state is illegal, which is why
  operation 7 and the flip travel together.

## Verification

- `./x check` and `./x gate` clean at every phase close; the whole-effort acceptance is the
  final gate plus: `./awf context internal/contextq internal/resident` reports both packages
  covered; `awf check` reports ADR-0194 operations 1-6 Applied with operation 7 remaining
  for the terminal transaction; rendered outputs are
  byte-identical across Phases 1-3 and 5 (`./x render` reports no change at each close after
  the phase's own config edits are accounted for); the export baseline shrank
  (`go doc ./internal/project | grep -c "^func\|^type"` is indicative only, not a gate).
- The ADR remains `Proposed`-lineage (`Implementing` + Applied events) until terminal review
  flips it; the plan stays `status: Proposed` until the same transaction.

## Notes

- Deferred by decision: `internal/git` untouched (ADR-0194 item 7); slash/OS-space discipline;
  fixture-builder consolidation; `printTopic`/`printPlan` conversions (future presentation-rule
  applications); further core decomposition is future-effort territory (user note at approval).
- Deferred by Phase 1 review ruling (2026-07-31): the in-core kind-literal dispatch that
  re-decides table facets beside the table (`nodeEnabled` in validate.go, `enableArray`,
  `hasLocalArtifact`, the count rows and sidecar-data lookup in configreference.go, the closure
  bucketing and base-TID switch in scaffold.go) stays as found; the claim wording was narrowed
  to the proven scope instead (ADR-0194 item 6 records the reasoning). The scaffold base-TID
  switch may still fold into Phase 5's template-ID consolidation.
- The single-home branch may land shapes that shift Phase 5's topic-render entry points; if so,
  amend the ADR (pre-terminal) and adjust Task 5.2's touched symbols here, recording the
  finding in this section.
- Re-verified 2026-07-31 after the single-home integration (main merge 556185fc, seam ADR
  landed as ADR-0193): Phase 5's topic-render entry points are unchanged
  (`internal/project/topics.go:43,47` still reads the two embedded templates in
  `generateTopicDocs` before `topic.RenderTopic`; `internal/topic` still does not import
  `internal/project`), so Task 5.2's touched symbols stand. In the same reconciliation this
  effort's ADR renumbered 0191 to 0194 (main took 0191-0193), and the phase-close status-event
  mechanics above were updated for main's ADR-0191 state-sequence removal.
- Indicative magnitudes (not gates): the contextq move surface is five production files plus
  roughly 180 sibling-test call sites; the resident surface is one production file plus
  roughly 30 test sites.
