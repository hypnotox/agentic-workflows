---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0194: Decompose internal project into core contextq and resident

## Context

ADR-0180's Consequences designated the `internal/project` package cohesion and decomposition
question as the next code-design decision, and stated its prerequisite: a `Project` whose fields
are all construction inputs. That prerequisite is met; the state-ownership conversion landed and
`TestProjectDerivedStateOwnership` mechanically enforces it. ADR-0180's Context also handed this
decision its evidence: the four synthetic partial `Project` literals and the fourteen zero-field
files. Decision item 2's core-side constructors dissolve the three context-side literals (they
become the staged load half), the remaining current-state literal stays inside core load
machinery under the same one-constructor discipline, and the fourteen zero-field files partition
between the carves and the core along their cluster lines. The 2026-07-30 full-repo code-design
audit (docs/research/code-design-audit-2026-07-30.md) fed five items into this decision: the
`internal/git` three-way split, export-surface minimization, template-ID ownership, a
path-anchoring type, and report-rendering ownership.

The roadmap's deferred-decomposition section states a third prerequisite, a package-cohesion
and boundary pattern, and instructs that the decomposition follow it. This decision reverses
that sequencing deliberately: the boundaries here were measured empirically (the cluster map,
the two verified cycles, and the per-symbol coupling census below), which grounds this
package's split more strongly than a generic pattern would, and the direction half of any such
pattern is already owned by `code-design/dependency-composition`. The deferred
`receiver-reads-owned-state` rule stays open in the roadmap rewrite this ADR obliges; a later
cohesion pattern generalizes from this decision's evidence rather than gating it.

The package was mapped before this decision was drafted (all figures verified 2026-07-31 against
main; line references are current as of that date and drift with the tree):

- 30 production files, 8080 lines (every non-test Go file counted whole), in thirteen
  responsibility clusters. The largest are the render pipeline (11 files, roughly 1900 lines),
  the context command (5 files, 1480 lines), drift check (789), output plan (678),
  current-state and topics (706), and the orchestration core (570).
- Two verified cycles bind the sync core. `check.go` calls `outputPlan` (check.go:34,411) while
  `output_plan.go` calls `validateArtifact` (defined check.go:364, called output_plan.go:706) and
  `localOutPaths` (defined check.go:372, called output_plan.go:606). And `output_plan.go` calls
  `renderAllBase` (output_plan.go:505) whose signature takes `targetOutputDeclaration`, a type
  defined in output_plan.go. Render, plan, and check cannot be layered into separate packages
  without relocating these coupling points first.
- The context cluster is a clean consumer of the rest: nothing outside `cmd/awf` calls its entry
  points, and no staying production file depends on its query logic (two small helpers,
  `pathMatchesAny` and `safelyMatchablePaths`, are the only staying-file references and are
  trivially relocatable).
- The exported surface is 128 top-level identifiers; 60 are never spelled outside the package.
  26 of 36 exported context result types are named only by `cmd/awf/context_test.go`'s field
  assertions. `cmd/awf` names 43 symbols; `cmd/releasecheck` names 2 (`Version`,
  `BridgeTrancheComplete`), both in staying files.
- `config` and `catalog` are foundational (imported by 15 and 13 of the 29 files); `topic` and
  `adr` are near-universal; `snapshot` is bounded to the current-state, context, and output-plan
  files; `git` is bounded to 3 files.
- Presentation of `internal/project` result models lives in `cmd/awf`: `cmd/awf/context.go`
  carries a bank of render functions over the context result types, and `cmd/awf/config.go`
  renders the config-reference model through `map[string]any` with discarded type assertions, so
  a renamed model field renders as an empty string under a green gate.
- Template identity is derived in parallel encodings: descriptor closures, constants, fallback
  helpers, inline construction (output_plan.go:169,190), singleton and hook and bootstrap
  literals, and a resident-root table duplicating IDs declared elsewhere; `internal/topic`
  re-reads the same embedded templates the project package hashes
  (internal/project/topics.go:43,47 reads them immediately before calling `topic.RenderTopic`).
  `validateDeclarationPlanParity` reconciles declaration-vs-render IDs at runtime, so divergence
  is detected drift, not silent drift.
- On the `internal/git` feed item: the git-seam decision (ADR-0193, Implemented; landed with
  the single-home integration while this ADR was Proposed) makes the `internal/git`
  package root the single semantic seam with one `Repo` handle spanning object reads, commit
  walking, and worktree lifecycle, and its plan names a package split an explicit non-goal.
  Verified against the landed tree: no `Repo` method body mixes the go-git backend with the
  native runner, but the files interleave them, and the root package imports go-git regardless
  because `Repo` holds a `*gogit.Repository`. A backend subpackage split would therefore change
  no consumer's dependency footprint.

### Coupling audit

Move set contextq (context query cluster plus the `cmd/awf` render bank):

- Top-level callers: internal/project/currentstate.go:571 (`pathMatchesAny`),
  internal/project/topics.go:38,53 (`safelyMatchablePaths`), internal/project/output_plan.go:32
  and internal/project/target.go:40 (`ArtifactRole` as a field type); all dissolve by relocating
  those symbols into staying files rather than moving them.
- Sibling tests: N=21, M=~179 (sibling tests of the five moving files travel with them:
  context_test.go 26, context_paths_test.go 47, context_projection_test.go 25,
  context_artifacts_test.go 42, context_adr_test.go 9; plus output_plan_test.go:234,245,
  prose references in stateownership_test.go:143,305, and ~17-20 sites in
  cmd/awf/context_test.go).
- Subpackage imports: only cmd/awf/context.go and cmd/awf/context_test.go add the new import;
  cmd/releasecheck and internal/evals are untouched.
- Cross-package methods / init(): `ContextForOptions` (cmd/awf/context.go:73) and `Uncovered`
  (cmd/awf/context.go:106) become free functions or methods on the new package's own type, since
  Go forbids methods on a foreign type; zero init() functions exist in internal/project or
  cmd/awf.

Move set resident (resident-root policy, anchoring, collisions, uninstall):

- Top-level callers: ~15 staying-core sites, including the syncReport body (project.go, 9
  sites), `outputPath` callers (check.go:487,512, output_plan.go:701), `isResidentPath` callers
  (currentstate.go:534, sweep.go:205), and iteration of the resident-root table for template
  rendering (render.go:651, output_plan.go:327); all become ordinary core-to-resident imports.
- Sibling tests: N=3, M=~30 direct sites, plus 27 `SyncReport`/`InitializeReport` call sites
  across 10 files that stay source-compatible because the sync vocabulary stays in core.
- Subpackage imports: cmd/awf/init.go, cmd/awf/uninstall.go, and cmd/awf/sync.go add the new
  import; no other importer changes.
- Cross-package methods / init(): `BackupFile` has no caller outside project.go (project.go:321,381)
  and stays in core; `CollisionsAt` and `Uninstall` are already free functions; zero init().

The audit also surfaced the two facts that reshaped scope: `Uninstall` calls the unexported
`isResidentPath` while core calls `residentRootNames` and `preserveResidentRemoval`, a
bidirectional coupling that dissolves only if the resident-path predicate moves together with
the resident-root table; and `Backup`/`Change` are defined and consumed inside project.go's
syncReport, so moving them would make core import the new package to type its own return values.

Two worktrees were in flight when this design began; the defect-cleanup branch integrated to
main (merge 44d99ebe) before this ADR was drafted, leaving the single-home branch as the only
outstanding gate. This decision was designed against that effort's landed Phase 4 state and
its Phase 5 plan; that branch has since integrated to main (merge 556185fc, its seam decision
landing as ADR-0193, Implemented), clearing the execution gate. The design premises this ADR
took from the in-flight branch were re-verified against the landed tree, with drift absorbed by
amendment while this ADR remains pre-terminal (ADR-0188).

## Decision

1. `internal/project` remains the sync core: render pipeline, output plan, drift check,
   current-state and topics, sweep, validate, the kind and target tables, layout,
   config-reference, scaffold, and sync orchestration. The measured cycles are repaired as
   in-package moves, not package boundaries: `validateArtifact` and `localOutPaths` relocate
   from check.go to validate.go, and declaration types live plan-side with a documented one-way
   direction (the plan orchestrates rendering; render files never call plan functions).
   `ScaffoldVarRefs` relocates from local.go to scaffold.go. `ArtifactRole` relocates from
   context_artifacts.go to a staying file; it is core declaration vocabulary. The dependency
   directions asserted here and in items 2 and 3 follow `code-design/dependency-composition`:
   each new seam value is a dependency received whole by a named first consumer, not a
   speculative capability.
2. A new package `internal/contextq` owns the context query: path classification, request
   assembly, universe assembly, topic and claim and pending projection, artifact records, the
   context and uncovered result vocabulary, and the human rendering of those results. The split
   line is load versus project: core keeps the loading machinery (working and staged
   current-state loads, lock reads, eligible-path selection) and exposes it through one exported
   assembled-state value with exactly two core-side constructors, one from an opened `Project`
   and one from a staged tree. Because the constructors are core-side, no additional core
   internals are exported. `contextq` has one constructor over that value. The staged context
   and uncovered entry points split accordingly: their load halves stay, their query halves
   move. The value's working name is `ContextState`, giving the plan a stable referent; the
   final name is settled at implementation and is not `Universe`, which already has two other
   meanings in the repository. `contextq` is the value's first and only consumer.
3. A new package `internal/resident` owns resident-root policy and path anchoring: the
   resident-root table, the resident-path predicate, a `Roots` anchoring value (tracked root
   plus resident root, constructed once at project open) owning the output-path resolution
   policy, and the resident lifecycle operations `CollisionsAt`, `Uninstall`,
   `inspectResidentRoots`, and `preserveResidentRemoval`. The dependency direction is core to
   resident, and core's output-path resolution is the `Roots` value's first consumer.
   `InitCollisions` (a wrapper over the output plan), `BackupFile`, and the sync
   vocabulary `Backup` and `Change` stay in core. The ~11 raw root joins that bypass the
   anchoring policy convert to the `Roots` value during execution.
4. Presentation ownership becomes a standing rule: the package that owns a result model owns
   its human rendering; a command binary keeps argument parsing, renderer selection (text
   versus JSON), and exit mapping. Two conversions are in scope: the context render bank moves
   from cmd/awf/context.go into `contextq`, and the config-reference rendering moves from
   cmd/awf/config.go into core as typed rendering over the model, removing the
   `map[string]any` seam. Other command surfaces convert as they are next touched.
5. Template identity has one derivation point: the kind-descriptor and declaration tables.
   The inline template-ID constructions, the duplicate hook and singleton and bootstrap
   literals, and the resident-root table's re-spelled IDs fold into it, with descriptor facets
   added as needed. `internal/topic` receives template identity and content as parameters from
   the caller that already holds them instead of re-reading the embedded template tree.
   `validateDeclarationPlanParity` would then compare the derivation with itself; it is retired
   with the consolidation, and Consequences records the loss of its runtime detection.
6. The kind-dispatch single table widens from the project package to the whole module: the four
   kind facts currently hard-coded in cmd/awf (the graph-kind predicate at list_add.go:110, the
   kind switch at new.go:31, and the domain-kind branches at list_add.go:235,248) resolve
   through the descriptor table, with descriptor facets added as needed, and the corresponding
   claim is updated to state module-wide reach. The widened claim keeps its proof inside
   `internal/project` as a source-scanning structural test over the cmd/awf sources (the shape
   the state-ownership scanner already uses), so the proof marker stays within the topic's
   selectors and the rendering domain's paths.
7. The `internal/git` feed item is decided in the negative: no package split. The git-seam
   decision's (ADR-0193) one-seam-package shape is the end state; the clean method-level backend
   separation is preserved as file discipline inside the package. This ADR records the two
   reasons: consumers gain nothing (the root package imports go-git regardless), and the seam
   claims and walker are built around a single package surface.
8. Export trim rides each carve: symbols move and unexport in the same execution phase, never
   renaming twice. The 26 test-only context result types become unexported as their assertions
   move into the new package's tests; the remaining command-facing context surface is the
   result and options types, the facet vocabulary, and the path normalization helper.
9. Execution sequencing and migration mechanics: implementation starts after the single-home
   effort's branch integrates (satisfied at merge 556185fc). Every file move lands in the same transaction as the domain-path
   and topic-selector widening that keeps the moved files owned and their `touches-state`
   markers in scope, because an omitted new package degrades silently to unowned rather than
   failing loudly. The destinations are named now: the tooling domain gains
   `internal/contextq/**` and `.awf/topics/metadata/tooling/context-and-topic.yaml` gains the
   same selector; the rendering domain gains `internal/resident/**` and
   `.awf/topics/metadata/rendering/project-output-plan.yaml` gains the same selector. The
   updated `project-derived-state-ownership` claim asserts that no production function in
   `internal/project`, `internal/contextq`, or `internal/resident` writes a field of that
   package's constructed long-lived values outside the constructing function; the scanner and
   its rule comment widen accordingly in the same transaction as the moves they name.
   Package-internal tests that would need the new packages convert to external test packages.
   Render-touching moves are verified by the byte-identity oracle: `awf render` reports no
   change, or the diff is a defect. The same-transaction documentation obligations include
   `docs/architecture.md` via its source parts (the internal/project role description, the
   cmd/awf presentation sentence, and the resident-root-table paragraph), the roadmap's
   deferred-decomposition section via its source part (retired or rewritten to point at this
   ADR, with the `receiver-reads-owned-state` deferral explicitly dispositioned), and the
   glossary additions of item 11.
10. Claim forms and backing. `rendering/project-output-plan:resident-policy-single-home` is an
   invariant, Backing: test, proven by a structural source-scanning test in the resident
   package asserting no file under internal/project or cmd redeclares or re-derives the
   resident-root table or the resident-path predicate. `rendering/project-output-plan:template-id-single-derivation` is an
   invariant, Backing: test, proven by a structural test asserting no production file outside
   the sanctioned descriptor and declaration table files contains a template-ID string
   literal. `tooling/context-and-topic:context-query-boundary` is an invariant, Backing: test,
   proven by a structural test asserting core's exported surface carries no context result
   vocabulary and the context package reaches core only through the assembled-state seam.
   `code-design/presentation-ownership:model-owner-renders` is a reasoned contract, Backing:
   unbacked, whose Verify instruction is to confirm, when touching a command surface, that the
   rendering of each result model lives in the package owning that model. The two updated
   claims keep Backing: test, with proof locations as stated in items 6 and 9.
11. The new global topic gains the enforcement anchors a reasoned-claim topic needs, mirroring
   ADR-0180: a presentation-ownership focus item is added to the adr-reviewer, code-reviewer,
   and plan-reviewer sidecars (backfilling catalog defaults, since focusItems replaces them
   wholesale), the workflow chain part directs an agent changing where a result model is
   rendered to consult `code-design/presentation-ownership`, and the glossary gains the new
   vocabulary (presentation ownership, the assembled context state, the `Roots` anchoring
   value, resident-root policy), all rendered in the same commit that introduces them.
12. Package ownership becomes loudly enforced: a structural test asserts that every production
   package under `internal/` and `cmd/` is matched by at least one domain's paths, so a future
   package omitted from domain ownership fails the suite instead of degrading silently to
   unowned. The claim is `tooling/context-and-topic:production-packages-domain-owned`, an
   invariant, Backing: test, proven by that structural test. This retires the residual risk
   item 9 otherwise accepts.

## State changes

- update `rendering/project-output-plan:kind-dispatch-single-table`
- add `rendering/project-output-plan:resident-policy-single-home`
- add `tooling/context-and-topic:context-query-boundary`
- add `code-design/presentation-ownership:model-owner-renders`
- add `rendering/project-output-plan:template-id-single-derivation`
- update `code-design/state-ownership:project-derived-state-ownership`
- add `tooling/context-and-topic:production-packages-domain-owned`

## Consequences

The core drops from 8080 to roughly 6400 production lines by shedding the two clusters with the
least coupling to it, then gains the typed config-reference rendering from cmd/awf (net roughly
6500): item 4's rule combined with the rejected config-reference carve deliberately gives core a
presentation responsibility for the one model it keeps. The compiler starts enforcing the two
boundaries that were previously prose. The context vocabulary stops being public API that
exists only for one test file, and the config-reference rendering hazard (a renamed field
silently rendering empty) is removed by construction. The presentation rule and the template-ID
single home create conversion obligations that later work inherits when touching other command
surfaces. Retiring `validateDeclarationPlanParity` removes the divergence it guarded against
and the guard itself: a future re-divergence of template identity has no runtime detector, only
the single derivation point. Extending two existing topics' selectors to cover the new packages
is the second and third occurrence of the selector-stretch pain point the roadmap records
against ADR-0183; the deferred path-owning-topic idea gains evidence rather than a silent
workaround.

Costs accepted: the test-move surface is large and known (M of roughly 179 and 30 for the two
carves against N of 21 and 3), dominated by sibling tests that travel with their files; the
`SyncReport` call sites stay source-compatible because the sync vocabulary stays in core. The
core remains around 6500 lines, bound by its real cycles; this decision documents its internal
direction (plan orchestrates render, check consumes both) rather than pretending a package
boundary would hold there. The new packages must be added to domain ownership in the same
transactions as the moves, because the failure mode of omission is silent unownership, not a
loud gate failure; item 12's guard then makes that failure mode loud for every future package
as well.

Risks: item 7's premises were taken from the then-unlanded git-seam branch; they were
re-verified against the landed ADR-0193 tree after integration (no shift found), and while this
ADR is pre-terminal any residual drift is absorbed by amendment (ADR-0188). The decision
deliberately creates no
claim about the git-access surface, which belongs to the git-seam decision. For the same
reason, `internal/git`'s `ResidentName` constants remain a knowingly-tolerated parallel
spelling of the resident-root names: the seam owns its own vocabulary, and the resident
single-home claim scopes itself to internal/project and cmd rather than reaching into the
package item 7 leaves untouched.

Downstream: a written plan is warranted (multi-phase, two package carves, claim migrations, a
claim update reaching cmd/awf); the export-surface baseline for later trims moves from the
audit's stale 86/47 to the verified 128/60.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Layer the core into plan, render, and check packages over a shared types package | Fights both verified cycles; the render/plan type coupling is evidence they are one concern; the shared types package tends toward a dumping ground. |
| Keep presentation in the command binary (status quo) | The config-reference rendering already shows the failure: the model crosses as `map[string]any` with discarded type assertions, so a renamed field renders empty under a green gate. |
| Record the presentation rule as a maintainable-code-design section instead of a topic | That document's sections are a fixed list and its value is generic guidance; a topic is specific and mechanically reviewable where the guide cannot be (ADR-0180's grounds). |
| Keep one package and reorganize into service types | Go visibility is package-scoped, so no boundary becomes enforceable; the designated decomposition decision would decide not to decompose. |
| Carve scaffold into its own package | It reads the kind-descriptor, singleton, and hook tables at six call sites; carving it means exporting three core tables to move 198 lines. |
| Carve config-reference | Its rendering path calls renderTarget, outputPlan, and deriveOperationState; carving it exports exactly the machinery this decision encapsulates. |
| Split internal/git into three packages along the audit's responsibility groups | The shared repo-state cohort spans consumers, one seam handle spans backends by design, the root imports go-git regardless, and the in-flight seam plan names the split a non-goal. |
| Name the assembled-state value Universe | Collides with the existing current-state transition Universe and a third glossary sense. |
| Name the second package install | Core's render and output-plan files iterate its resident-root table; a rendering dependency on an install package misstates the layering that a resident-policy package states plainly. |
| Move Backup, Change, and BackupFile with the lifecycle operations | They are sync-pipeline vocabulary defined and consumed inside syncReport; moving them makes core import the new package to type its own return values and breaks 27 otherwise-compatible call sites. |
| Move the whole staged context entry points | They call unexported loading machinery; moving them would export roughly eight core internals, defeating the export-trim goal the seam exists to serve. |
| Wait for the git-seam branch to land before deciding anything | The design is grounded in its settled Phase 4 state and its plan; only execution needs the landed tree, and pre-terminal amendment absorbs drift. |

## Status history

- 2026-07-31: Proposed
