---
format: plan-v2
date: 2026-08-23
adrs: [0296, 0299, 0300, 0301]
status: Proposed
---
# Plan: Make cmd/awf a Thin Composition Root

## Goal

Reduce `cmd/awf` to command-spec lookup, parsing, concrete dependency composition, one focused application-operation invocation per resolved command leaf, renderer and stream selection, CLI wording, and exit mapping. Preserve every command universe, ordered mechanism, output byte contract, partial outcome, safety property, help surface, and exit behavior. Do not create a generic application layer, service locator, dependency bag, speculative interface, production fault hook, or unrelated or support-floor compatibility cleanup. Adapters made obsolete solely by RF-006 may retire with their replacement. Do not perform RF-007, RF-010 beyond the one named comment, RF-008B, or RF-014B work.

## Architecture summary

ADR-0296 supplies the approved direction: small operation-specific packages or existing cohesive owners sit between `cmd/awf` and immutable project state, domain services, and semantic mechanisms. Each resolved command leaf selects exactly one focused operation after CLI variant parsing; an operation may retain the ordered multi-stage mechanism that defines one use case. `Publisher`, `RepositoryChecker`, `currentstatecoord`, `internal/upgrade`, `internal/audit`, `internal/effort`, and `internal/worktree` retain their established meanings under ADRs 0299 through 0301. New upper operations may coordinate existing owners without moving lower policy upward or creating reverse imports.

Result owners map semantic results into `presentation.Document`; `cmd/awf` retains renderer or protocol-bypass selection, exact wording that is inherently CLI-owned, stdout and stderr choice, and Error-only or command-specific exit mapping. Touched mutable test seams become explicit operation inputs or CLI-selected values, never replacement package globals. Operation tests own business rules, universes, atomicity, partial outcomes, and sequencing; command tests retain parsing, help, goldens, streams, JSON or protocol bytes, bypass behavior, and exits. Production-source proofs enforce the composition boundary and existing single-home routes.

<!-- awf:template-source templates/partials/plan-flexibility.md -->
**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Extract initialization and publication use cases

**Execution mode: inline.**

Completes: ["init-publication-composition"]

### Task 1.1: Give initialization one focused application operation
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan"]
Paths: ["cmd/awf/init.go", "cmd/awf/init_test.go", "internal/initop", "internal/initspec", "internal/testsupport"]

Move initialization orchestration, collision probing, scaffold cleanup and rollback, gate invocation, `Publisher.Initialize`, advisory assembly, and immutable next-action policy behind one focused operation. Preserve describe mode, answers and profile parsing, force backup behavior, interactivity and prompt selection, JSON bypass, collision and recovery wording, gate and advisory paths, streams, and exits. Keep `initspec` as the specification and result owner rather than making it a filesystem coordinator. Convert touched mutable CLI seams into explicit values or operation dependencies without adding production fault hooks, and move business-rule and rollback or partial-result tests with the operation.

### Task 1.2: Make render a direct Publisher operation route
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination"]
Paths: ["cmd/awf/sync.go", "cmd/awf/sync_test.go", "cmd/awf/publishing.go", "internal/publisher", "internal/testsupport"]

Compose `Loader` and `Publisher`, invoke one Publisher sync operation, and render the Publisher-owned mutation result without retaining a parallel cmd-owned mutation assembler. Preserve Loader single-home, atomic publication, backup decisions, post-mutation routes, one operation-scoped output plan, lock behavior, output, streams, and exits. Keep renderer selection and command wording in `cmd/awf`; move only result semantics that belong to Publisher.

### Phase close

Close initialization and render composition only after focused behavior tests, publication ownership proofs, render and drift checks, and the full project gate preserve the pre-extraction contracts. Update and render any authority this phase makes stale in the same transaction. After the commit, send the orchestrator the required phase-completion report before beginning Phase 2.

```commit
refactor(code-design): extract initialization and render operations
```

## Phase 2: Extract repository-check use cases

**Execution mode: inline.**

Completes: ["check-composition"]

### Task 2.1: Move check capability preparation and sequencing to a focused operation owner
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination", "0300:owners-classify-results", "0300:repository-checker-aggregates-results"]
Paths: ["cmd/awf/check.go", "cmd/awf/checkrepo.go", "cmd/awf/checkstaged.go", "cmd/awf/check_presentation.go", "cmd/awf/check_test.go", "cmd/awf/checkrepo_test.go", "cmd/awf/checkstaged_test.go", "internal/checkop", "internal/repositorycheck", "internal/generatedcheck", "internal/referencecheck", "internal/plancheck", "internal/pitfallcheck", "internal/vocabularycheck", "internal/configcheck", "internal/currentstatecoord", "internal/prosegate", "internal/memorycite", "internal/commitpolicy", "internal/project", "internal/testsupport"]

Move the closed capability requirements and step table, immutable check inputs, working and staged preparation, bare-check composition, version information, failure classification, plan-note handling, and aggregate report assembly behind focused check operations. Preserve working, report, current-state, index, and staged universes; capability acquisition; direct and aggregate ordering; continuation and deduplication; typed severity and protected property; partial output; and Error-only exit behavior. Feed completed owner results to policy-free `RepositoryChecker`; do not let it prepare inputs, select capability policy, infer classification, rebuild a Publisher plan or corpus, or absorb an individual check owner.

### Task 2.2: Move check result presentation to its semantic owner
Applying: ["0296:boundary-values", "0300:owners-classify-results", "0300:repository-checker-aggregates-results"]
Paths: ["cmd/awf/check_presentation.go", "cmd/awf/check_presentation_test.go", "cmd/awf/check_report_test.go", "internal/checkop", "internal/presentation", "internal/testsupport"]

Give the focused result owner immutable success, finding, and produced-failure semantics plus its `presentation.Document` mapping. Keep central syntax rendering, renderer selection, CLI-only prefixes, stdout and stderr selection, and exit mapping in `cmd/awf`. Retain exact text, category multiplicity, direct-child suppression, and partial-report behavior. Move business-rule or universe or assembly tests to the operation owner while preserving end-to-end command goldens and stream or exit tests.

### Phase close

Close check extraction only after every check command leaf reaches exactly one focused operation, capability and owner-result mutation proofs pass, working and staged behavior remains unchanged, and the full project gate is green. Update and render any authority this phase makes stale in the same transaction. After the commit, send the orchestrator the required phase-completion report before beginning Phase 3.

```commit
refactor(code-design): extract repository check operations
```

## Phase 3: Extract upgrade and audit use cases

**Execution mode: inline.**

Completes: ["upgrade-audit-composition"]

### Task 3.1: Make internal/upgrade own complete upgrade and recovery operations
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan"]
Paths: ["cmd/awf/upgrade.go", "cmd/awf/upgrade_presentation.go", "cmd/awf/upgrade_test.go", "internal/upgrade", "internal/testsupport"]

Move normal upgrade and recovery sequencing, dependency selection inputs, journal and recovery coordination, migration and gate ordering, always-sync handling, partial-axis result assembly, and semantic presentation mapping to `internal/upgrade`. Preserve authority checks, migration safety, journal recovery, terminal Publisher sync, every partial outcome, exact mutation and remedy presentation, streams, and exits. `cmd/awf` keeps recover-versus-normal parsing, concrete dependency construction, renderer selection, and exit mapping. Use direct concrete dependencies and existing upgrade contracts rather than a generic bundle or a new test seam.

### Task 3.2: Give configured audit invocation an audit-owned operation
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["cmd/awf/audit.go", "cmd/awf/audit_test.go", "internal/audit", "internal/project", "internal/testsupport"]

Move configured audit input construction and application orchestration behind an `internal/audit` operation while preserving the existing analysis and report owner. Avoid a reverse Publisher dependency because Publisher already consumes audit-owned values. Preserve explicit range parsing, count and empty notices, warning-only zero exit, Error nonzero exit, current-binary behavior, exact report rendering, and failure propagation. Retain CLI range grammar, concrete composition, streams, and exit mapping in `cmd/awf`; remove or reduce the broad project compatibility route only where RF-006 makes it obsolete.

### Phase close

Close upgrade and audit extraction after safety, recovery, partial-outcome, report, stream, and exit tests pass with source-level import-direction and owner-route proofs plus the full project gate. Update and render any authority this phase makes stale in the same transaction. After the commit, send the orchestrator the required phase-completion report before beginning Phase 4.

```commit
refactor(code-design): extract upgrade and audit operations
```

## Phase 4: Extract current-state command use cases

**Execution mode: inline.**

Completes: ["current-state-composition"]

### Task 4.1: Keep ADR lifecycle coordination in currentstatecoord and move its result presentation
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0299:publisher-constructs-operation-plan"]
Paths: ["cmd/awf/adr.go", "cmd/awf/adr_test.go", "internal/currentstatecoord", "internal/testsupport"]

Make every resolved ADR command leaf invoke one focused currentstatecoord operation. Move numbering-result semantic presentation to its result owner while preserving deterministic numbering, pre-mutation and post-mutation universes, partial assignments, post-sync behavior, exact wording, streams, and exits. Keep command grammar, concrete state and Publisher callback composition, renderer selection, and exit mapping in `cmd/awf`. Do not expand currentstatecoord beyond ADR, topic, plan, and current-state authority coordination.

### Task 4.2: Extract context operation selection without reversing the neutral input boundary
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0301:neutral-context-query-input"]
Paths: ["cmd/awf/context.go", "cmd/awf/context_test.go", "cmd/awf/publishing.go", "internal/contextop", "internal/currentstatecoord", "internal/contextinput", "internal/contextq", "internal/publisher", "internal/testsupport"]

Move syntax-validated static, live, working, staged, range, and uncovered selection; Full-footprint gating; changed-path resolution; Publisher preparation; and one-universe context-query coordination behind focused operations. Preserve the static fallback, Publisher reuse, neutral immutable `contextinput`, contextq classification, 8192-byte spill boundary, exact packet or spill output, and failure behavior. Keep CLI argument parsing, delivery implementation selection, stream choice, and exits in `cmd/awf`. Convert `deliverContext` only through an explicit real delivery dependency and add no replacement global.

### Task 4.3: Extract topic operation selection while retaining topic ownership
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["cmd/awf/topic.go", "cmd/awf/topic_test.go", "internal/topicop", "internal/currentstatecoord", "internal/currentstate", "internal/presentation", "internal/testsupport"]

Move syntax-before-state validation, static fallback, Full-footprint gate, live option assembly, and semantic result construction behind focused topic operations. Preserve every topic option, coverage and static behavior, exact presentation, streams, and exits. Keep CLI grammar, concrete composition, renderer selection, and exit mapping in `cmd/awf`; do not collapse context and topic universes or create a generic read operation.

### Phase close

Close current-state extraction only after ADR, context, and topic command behavior passes, neutral input and Publisher reuse proofs remain mutation-sensitive, currentstatecoord and contextq import direction stays intact, and the full project gate is green. Update and render any authority this phase makes stale in the same transaction. After the commit, send the orchestrator the required phase-completion report before beginning Phase 5.

```commit
refactor(code-design): extract current state command operations
```

## Phase 5: Extract effort use cases

**Execution mode: inline.**

Completes: ["effort-composition"]

### Task 5.1: Coordinate effort and worktree owners from a cycle-safe focused package
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/effort_worktree_test.go", "internal/effortop", "internal/effort", "internal/worktree", "internal/testsupport"]

Place new, list, show, finish, worktree, integrate, memory, and activity application orchestration above both `internal/effort` and `internal/worktree` so the existing worktree-to-effort dependency does not cycle. Preserve grammar, root and repository identity, topology and ancestry safety, gates, memory single-writer protocol, activity input bounds, integrate partial outcomes, worktree lifecycle, and exact readable and machine-protocol results. Keep flag parsing, stdin JSON decoding and CLI diagnostics, protocol-versus-readable selection, byte-exact protocol serialization, streams, and exits in `cmd/awf`. Compose `effort.Service` and `worktree.Manager` directly; introduce no all-purpose lifecycle facade or provider interface.

### Task 5.2: Move effort orchestration tests to the operation owner without broad cleanup
Applying: ["0296:extraction-owners", "0296:boundary-values"]
Paths: ["cmd/awf/effort_test.go", "cmd/awf/effort_worktree_test.go", "internal/effortop", "internal/effort", "internal/worktree", "internal/testsupport"]

Move only business-rule, topology, gate, memory, activity, atomicity, and partial-result oracles whose behavior relocates. Retain command grammar, input diagnostics, help, protocol bytes, bypass, stream, and exit tests in `cmd/awf`. Preserve strong production-source and runtime proofs for the semantic Git seam, effort/worktree owner split, and one focused operation per resolved leaf. Leave residual giant-test census and unrelated helper cleanup to RF-007.

### Phase close

Close effort extraction only after topology and destructive-safety tests, protocol goldens, partial-outcome evidence, owner-route mutation proofs, and the full project gate pass unchanged. Update and render any authority this phase makes stale in the same transaction. After the commit, send the orchestrator the required phase-completion report before beginning Phase 6.

```commit
refactor(code-design): extract effort command operations
```

## Phase 6: Enforce the thin composition boundary

**Execution mode: inline.**

Completes: ["thin-command-root", "verification-evidence"]

### Task 6.1: Remove residual application policy from cmd and enforce leaf cardinality
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:proportional-operations", "0300:repository-checker-aggregates-results", "0301:neutral-context-query-input"]
Paths: ["cmd/awf", "internal/initop", "internal/checkop", "internal/contextop", "internal/topicop", "internal/effortop", "internal/upgrade", "internal/audit", "internal/currentstatecoord", "internal/publisher", "internal/repositorycheck", "internal/contextinput", "internal/contextq", "internal/effort", "internal/worktree", "internal/testsupport"]
Post-check: Run the source probes and mutation-sensitive tests over production `cmd/awf/**/*.go` excluding `_test.go`, every registered and dynamically resolved command leaf, new operation owners, protected lower owners, and the package-global seams touched by RF-006. The terminal set has no application-policy symbol, no new mutable test seam, and no retained mutable seam at a deliberately converted site; each leaf invokes exactly one focused operation after variant selection; no forbidden reverse import or generic application type exists; and Loader, Publisher, RepositoryChecker, currentstatecoord, contextinput/contextq, audit, effort, and worktree retain one route and home.

Add production-source proofs for exact handler-to-operation cardinality at the resolved leaf level, not merely the top-level dispatch registry. Prove application policy is absent from cmd, import direction holds, no all-purpose `Application`, service locator, generic dependency bag, speculative interface, or new mutable production seam exists, and protected single-home routes remain intact. Convert existing package-global seams only where RF-006 deliberately touches their operation path; leave unrelated census seams outside this issue. Remove obsolete cmd coordinators and compatibility adapters made unused by RF-006, but do not perform unrelated compatibility deletion or RF-007 cleanup.

### Task 6.2: Replace the historical main guard comment with its current invariant
Latitude: exact
Applying: ["0296:dependency-direction"]
Paths: ["cmd/awf/main.go"]

Rewrite only the `Plan 2 Task 3.3` comment as a present-tense invariant explaining why guards own process-level interruption while handlers remain independently invocable. Preserve the guarded mechanism and leave broad historical-comment cleanup to RF-010.

### Task 6.3: Prove protected behavior and record current-state documentation
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0300:owners-classify-results", "0300:repository-checker-aggregates-results", "0301:neutral-context-query-input"]
Paths: ["cmd/awf", "internal/initop", "internal/checkop", "internal/contextop", "internal/topicop", "internal/effortop", "internal/upgrade", "internal/audit", "internal/currentstatecoord", "internal/publisher", "internal/repositorycheck", "internal/contextinput", "internal/contextq", "internal/effort", "internal/worktree", "internal/testsupport", ".awf/topics/parts/code-design/dependency-composition/current-state.md", ".awf/docs/parts/architecture", "docs/architecture.md", "docs/topics/code-design/dependency-composition.md", ".awf/awf.lock", "docs/plans/2026-08-23-make-cmd-awf-a-thin-composition-root.md"]
Post-check: Before the closing commit, run focused owner and command suites, `go test ./...`, render and drift checks, staged checks, the unmodified `./x gate`, lint, and ordinary check. Review representative human, JSON, and protocol outputs for exact wording, stdout or stderr, bypass, and exit behavior. The precommit terminal state has only the intentional staged transaction and lifecycle-authorized advisory findings recorded in Notes.

Update only residual authority whose current wording remains false after the phase-local currency updates, at its `.awf/` source and rendered projection. Preserve help and clispec single-source ownership. Reconcile authority-preserving route deviations and residual debt in Notes. Clean-tip audit and assurance follow the closing commit because their exact range and clean-worktree preconditions do not exist earlier.

### Phase close

Create the closing commit after the precommit evidence above.

```commit
refactor(code-design): enforce thin command composition
```

At the clean committed tip, run the current-binary repository audit and range-local audit over the exact implementation range, then obtain independent implementation assurance over that range. Any review-driven mutation lands as an explicit focused settlement transaction with staged checks and the full gate, followed by renewed clean-tip audits and assurance as required by the review workflow. Report blockers or authority deviations immediately. When assurance settles, send the complete integration-ready report and stop mutation with the effort, branch, worktree, ADRs, and Proposed plan intact; do not integrate, number ADRs, terminally close the plan, edit the audit program, remove topology, run retrospective, finish the effort, or begin RF-007.

## Definition of done

- `dod: init-publication-composition` Initialization and render command leaves compose and invoke one focused operation while Publisher and initspec retain their established ownership and all collision, rollback, gate, advisory, atomic-publication, output, and exit behavior.
- `dod: check-composition` Check command leaves invoke focused operations that own capability preparation and ordered use cases while completed owner results remain policy-free inputs to RepositoryChecker and every working, staged, partial-output, severity, and exit contract is preserved.
- `dod: upgrade-audit-composition` Upgrade, recovery, and audit leaves invoke their focused owner operations with unchanged migration, journal, recovery, sync, report, output, safety, and exit behavior.
- `dod: current-state-composition` ADR, context, and topic leaves invoke focused operations without expanding currentstatecoord, reversing contextinput/contextq dependencies, rebuilding Publisher values, collapsing universes, or changing output and exit behavior.
- `dod: effort-composition` Every effort leaf invokes one cycle-safe focused operation while effort/worktree semantics, topology safety, memory and activity protocols, partial outcomes, machine bytes, streams, and exits remain unchanged.
- `dod: thin-command-root` Production `cmd/awf` contains only command-spec lookup, parsing, concrete composition, one focused invocation per resolved leaf, renderer or bypass selection, CLI wording and streams, and exit mapping, backed by mutation-sensitive source proofs with no generic application object, service locator, dependency bag, speculative interface, new global test seam, or retained mutable seam at an RF-006-converted site.
- `dod: verification-evidence` Exact CLI help, goldens, wording, stdout and stderr, JSON and protocol bypass, and exits remain proven; owner tests prove moved policy, universes, atomicity, and partial results; the unmodified gate, lint, current-binary audit, local audit, render and check commands pass at the reviewed clean tip.

## Notes

Apply the plan-flexibility rule above when recording deviations. The sole writer reports each committed phase, any blocker or authority deviation immediately, and the complete integration-ready evidence before stopping mutation. Record review dispositions, cross-owner route adjustments, representative output review, verification results, residual debt, and any lifecycle-authorized advisories here.

Plan-review dispositions:

- Reasoned: bounded operation homes are `internal/initop`, `internal/checkop`, `internal/contextop`, `internal/topicop`, and cycle-safe `internal/effortop`; existing-owner populations are explicitly enumerated in task paths, with terminal source proofs covering the complete resolved command-leaf and protected-owner populations. This removes ambiguous package placement without freezing non-load-bearing filenames.
- Reasoned: the mutable-seam requirement applies only to new seams and existing seams deliberately converted by RF-006. Unrelated census seams remain outside this issue, avoiding RF-007 scope and test-shaped production changes.
- Reasoned: the compatibility exclusion now permits only adapter retirement caused directly by RF-006 while preserving RF-008B and RF-014B support-floor cleanup boundaries.
- Phase 1 route: `cmd/awf/sync_test.go` does not exist; render route and partial-output proofs remain in `cmd/awf/run_test.go` and `cmd/awf/upgrade_test.go`. The obsolete command-owned `operationPlan` seam retired when initialization collision planning moved to `internal/initop`.
- Phase 1 review settlement: initialization now binds collision checking, publication, and advisories to one Publisher `Preparation`. Publisher retains ordinary convenience routes while a prepared route reuses the exact operation plan and rejects an unbound preparation. The ownership proof traces the final initialization route and distinguishes its temporary pre-prompt collision universe.
- Phase 2 route: `internal/checkop` owns capability preparation, working and staged sequencing, plan-warning deduplication, produced-report classification, and semantic presentation for every check leaf. Command-side business tests moved with those owners; command goldens and the renderer and exit boundary remain in `cmd/awf`. The new package is registered to the tooling domain and CLI topic, and check-only Publisher preparation retired from the command package while context and effort composition remain there.
- Phase 1 review: close `216e24a66`; settlements `e9682e32a` and `60314b808`; cumulative review was clear through `60314b808`. Semantic summaries, rather than retained verbatim review text, record that collision checking, publication, and advisories bind to one Publisher Preparation, prepared routes are traced, and the temporary pre-prompt universe is distinguished.
- Phase 2 review: close `104451a6f`; settlement `6bc5fea8c`; cumulative review was clear through `6bc5fea8c`. The surviving semantic summary is that `internal/checkop` owns preparation, sequencing, deduplication, failure classification, and presentation while the command retains its boundary.
- Phase 3 review: close `323899e20325b0ad653e639fb33b7d85476304b2`; settlement `33615dfce`; final mechanical comment fix `5bbd22a29`. Six original finding texts were not retained verbatim. Their semantic dispositions were to correct the checkpoint full SHA; replace relocated-lock save coverage ignore with a real unwritable-destination oracle; restore the real Publisher partial-result command oracle; strengthen audit clean, warning, and error scope proof; strengthen exact empty-range output proof; and centralize report-status and Failed exit classification in one audit-owned pass. A fresh Phase 3 verify pass found only the exported dependency comment issue, otherwise finding the original review settled; that comment was fixed in `5bbd22a29`.
- Phase 4 review: close `5a1dfc1cc`. At `cmd/awf/publishing_test.go:21-46,102-147,173-207`, test-local working and staged context routes and a prepared Publisher copy reproduced the removed production route; their invariant and business tests move to `internal/contextop` to exercise private `workingState`, `stagedState`, and `complete` directly, leaving command-only output, delivery, stream, and exit tests in `cmd`. At `internal/testsupport/publishing_ownership_test.go:94-101,248-276`, the ownership proof accepted any `New(...).Prepare()` selector and counted any Prepare; it now requires the actual package-qualified Publisher constructor, counts that route accordingly, and has a negative fixture rejecting another package's `New(...).Prepare()`. Both findings are settled by this follow-up transaction. Review text is semantically summarized here rather than quoted verbatim.
- Phase 5 review: close `0b8739569`. Direct focused `internal/effortop` functions replaced the rejected pre-commit generic operation facade. The claimed newline-bearing Show and Finish presentation routes are unreachable through the sole production composition path because `ResolveControlRoots` rejects multiline Git path output; the existing coverage exclusions now state that actual precondition. The command ownership proof now parses Go syntax, resolves owner import and composed-receiver aliases, associates calls with each resolved effort leaf, and rejects comment, dead-helper, renamed-import, and renamed-receiver false evidence through negative fixtures. No material deviation or residual debt remains.
