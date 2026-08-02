---
format: plan-v1
date: 2026-08-02
adrs:
  - revision-aware-historical-audit-pipeline
status: Proposed
---
# Plan: Revision-aware Historical Audit Pipeline

## Goal

Implement [ADR-revision-aware-historical-audit-pipeline](../decisions/revision-aware-historical-audit-pipeline.md): make `awf audit` collect and own one historical range, reuse revision state, load only transition-policy authority, propagate cancellation, and read only selected committed blobs while retaining historical finding behavior outside the approved projection refinement.

The plan does not change range membership, convert audit to first-parent-only history, add a persistent cache, weaken current or staged checks, or impose a wall-clock acceptance gate.

## Architecture summary

`internal/audit` becomes the single owner of an invocation-scoped history operation assembled from one `internal/git` range collection. It lazily derives immutable revision states and shares them between current-state transition replay and stale-merge authorization. `internal/currentstate` and `internal/topic` expose a reduced ADR/topic-claim projection that deliberately omits marker and domain-path validation. `internal/git` supplies separate first-parent merge paths plus committed entry and selected-blob evidence; `internal/snapshot.Selection` keeps selected content type-distinct from complete `snapshot.Tree`. `internal/project` resolves settings and invokes the operation without recollecting history. State changes apply in declaration order, and each implementation transaction updates its authored current-state and architecture sources before render.

All paths below are relative to `/home/hypno/Projects/agentic-workflows/.awf/worktrees/streamline-the-historical-audit-pipeline`. New production definitions land with their first consumer. Operation dependencies are direct functions or immutable values owned by one audit invocation; no package global, Project field, invalidation step, backend type leak, or test-only production switch is permitted.

## Phase 1: Unify historical audit ownership

**Execution mode: inline.**

### Task 1.1: Write the operation ownership regressions first
Paths: ["internal/audit/history_test.go", "internal/audit/audit_test.go"]

Create `internal/audit/history_test.go` in package `audit` with controlled direct function dependencies for range collection and revision loading. Add `TestHistoryOperationCollectsRangeOnceAndCachesStates` and make it prove one collection for the whole invocation, one derivation for each requested revision including an error result, no retry of a cached error, and direct loading of a first parent outside the selected range rather than recursive ancestry traversal. Add `TestHistoryOperationSharesStatesAcrossTransitionAndStaleReplay` with a qualifying merge whose result, first parent, and incoming parent are requested by both replay paths; assert each required revision is derived once and that transition findings remain after existing pure, stale-merge, and live findings in the current output order.

Add an audit-level fixture proving an empty range still runs the live cleanliness rule once and does not derive a revision. Run `go test ./internal/audit -run 'TestHistoryOperation|Test.*EmptyRange'`; the new tests must fail before the operation exists.

### Task 1.2: Introduce the invocation-owned history operation and one range walk
Latitude: exact
Paths: ["internal/audit/history.go", "internal/audit/audit.go", "internal/project/project.go", "internal/project/currentstate.go", "internal/project/staged_test.go", "internal/project/mergeaggregate_test.go", "internal/audit/audit_test.go", "internal/audit/history_test.go"]

Create `internal/audit/history.go`. Define one unexported history operation constructed with the immutable collected `[]git.Commit`, revision/parent lookup, and direct load functions selected by `Run`. Its revision cache stores both successful state and error identity and never outlives `Run`. In this phase a cache miss may use existing complete `snapshot.CommitTree` evidence and the existing broad historical loader; the sparse replacement belongs to Phase 4.

Refactor `audit.Run` so it opens the repository once, invokes `RangeCommits(base, head)` once, constructs the operation, evaluates existing commit-local and range-aggregate rules without changing `git.Commit.Changes`, replays stale merges through cached revision state, runs live cleanliness once, and appends transition findings last exactly as `Project.Audit` currently does. A root transition uses the empty universe. A merge transition uses its first parent and `currentstate.MergeAggregate`; stale authorization continues to use result, first parent, and every incoming parent with each tree's committed lock/schema boundary. Preserve the existing distinction that transition load failure becomes a per-commit Warning while stale-merge evidence failure aborts.

Move `currentStateTransitionRule`, transition orchestration, and the historical universe/lock helpers needed by both replay paths from `internal/project/currentstate.go` into `internal/audit`. Remove `Project.auditTransitions` and `Project.rangePairUniverses`; `Project.Audit` resolves inputs and returns the single `audit.Run` result. Relocate or rewrite their project fixtures under `internal/audit` without weakening authored-commit, merge-aggregate, malformed-history, historical-schema, root, and missing-parent coverage. Keep working-tree, staged, and live commit-authorization loaders in `internal/project` unchanged.

Use direct function injection only at the unexported operation constructor so tests count semantic productions without introducing a repository interface or universal dependency bag. Run `rg -n 'RangeCommits\(' internal/audit internal/project`; the audit command path must have one production collection site and no transition recollection in `internal/project`.

### Task 1.3: Apply operation-owned history authority and documentation
Latitude: exact
Paths: ["docs/decisions/revision-aware-historical-audit-pipeline.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/topics/tooling/audit-and-snapshots.md", "docs/domains/tooling.md", "docs/architecture.md"]

Change the linked ADR from `Proposed` to `Implementing`. Append the required Implementing status event with the canonical content stamp, then an Applied event for operation `add tooling/audit-and-snapshots:audit-history-operation-owned`. Add this exact first claim to the authored topic part:

```markdown
### `invariant: audit-history-operation-owned`

One `awf audit` invocation collects its requested commit range exactly once and owns one immutable historical operation for that range. Transition replay and stale-merge replay share the operation's revision states and cached load errors, each required revision is derived at most once, and no cache survives the invocation or lives on Project.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test
```

Place `// invariant: tooling/audit-and-snapshots:audit-history-operation-owned (TestHistoryOperationCollectsRangeOnceAndCachesStates)` immediately above that test and a second marker naming `TestHistoryOperationSharesStatesAcrossTransitionAndStaleReplay` above the sharing test. The tests must exercise the complete claim, including cache lifetime through separate operation instances.

Update the tooling domain and architecture authored sources to place historical orchestration in `internal/audit`, describe Project as outer composition only, and show one range flowing through pure rules, live state, transition replay, and stale merge replay. State that the first implementation still uses complete snapshots and that later phases narrow the representation. Run `./x render`; inspect and stage only generated outputs attributable to these authored changes. `./x check` must finish clean and `docs/decisions/INDEX.md` must show the ADR as Implementing with the first operation Applied.

### Phase close

Run `gofmt` on changed Go files, `go test ./internal/audit ./internal/project`, `go test ./...`, `git diff --check`, `./x render`, and `./x check`; every command must succeed or finish clean. Stage the complete phase transaction explicitly, run `./awf check staged` and `./x gate`, and create:

```commit
refactor(tooling): unify historical audit ownership
```

## Phase 2: Reduce historical authority and reuse irrelevant revisions

**Execution mode: inline.**

### Task 2.1: Write reduced-projection regressions first
Paths: ["internal/topic/tree_test.go", "internal/currentstate/load_test.go", "internal/audit/history_test.go", "internal/audit/audit_test.go"]

In `internal/topic/tree_test.go`, add `TestLoadAuthorityCorpusFromTreeOmitsMarkersAndDomainPaths`. Construct valid ADR/topic authority with a malformed configured domain sidecar and a malformed or incomplete proof marker source. Require the reduced corpus to preserve exactly `Corpus.All()` topic and claim content while leaving domain paths and marker sites absent; require the existing full `LoadCorpusFromTree` to retain its current validation behavior on the same broad snapshot.

In `internal/currentstate/load_test.go`, add `TestLoadUniverseFromTreeMatchesPolicyProjection`. Compare ADRs, ADR source bytes, and topics from the reduced Universe with `LoadFromTree(...).Universe()` for a fully valid tree, then prove malformed marker-only and domain-sidecar-only bytes do not affect the reduced result. Keep malformed config, ADR, topic metadata, topic part, claim provenance, and claim reference failures blocking.

In `internal/audit/history_test.go`, add `TestHistoricalStateUsesPolicyProjectionAndReusesIrrelevantCommits`. Cover a code-only commit, a malformed marker-only Go change, a malformed domain-sidecar change, `.awf/config.yaml` change, topic metadata/part change, default and custom decisions-directory ADR changes, deletion/rename paths, a root, a first parent outside the range, and a merge. Assert only inputs to the approved historical projection cause state derivation and that ordinary commit findings are unchanged. The tests must fail against the broad loader and no-reuse operation.

### Task 2.2: Separate topic authority assembly from marker and domain-path loading
Latitude: exact
Paths: ["internal/topic/tree.go", "internal/topic/tree_test.go", "internal/currentstate/load.go", "internal/currentstate/load_test.go"]

Refactor `internal/topic/tree.go` so metadata/part discovery and `assembleCorpus` remain single-homed. Add documented `LoadAuthorityCorpusFromTree(tree *snapshot.Tree, cfg *config.Config, adrs adr.Corpus) (Corpus, error)` as the concrete first consumer of the split: it parses topic metadata and current-state parts, validates configured domain membership, claim identity, provenance, references, and operation application, but supplies empty domain ownership and does not call `markerIndexFromTreeFiles`. Keep `LoadCorpusFromTree` behavior byte-for-byte compatible by adding domain sidecars and marker indexing after the shared authority assembly.

Add documented `currentstate.LoadUniverseFromTree(tree *snapshot.Tree, cfg *config.Config) (Universe, error)`. Share ADR discovery, corpus construction, source cloning, and error formatting with `LoadFromTree`; do not rebuild ADR or topic grammar. The reduced function returns only the exact Universe projection and never constructs a discarded `Loaded.Markers` or `Corpus.DomainPaths`. Replace historical state derivation in `internal/audit` with this loader. Working-tree, index, staged, context, and topic-query callers continue using the full loader.

### Task 2.3: Specify first-parent merge paths and relevance with failing contracts
Latitude: exact
Paths: ["internal/git/walk_test.go", "internal/git/git_test.go", "internal/git/entrypoints_test.go", "internal/audit/history_test.go"]

Add contract cases for a new `(*git.Repo).FirstParentChangedPaths(ctx, rev) ([]string, error)`. Require sorted, unique, repository-handle-relative slash paths for insertions, modifications, deletions, and both sides of renames; a root compares with the empty tree; a merge compares only with its first parent; a subdirectory handle excludes outside paths; cancellation and a shallow missing parent preserve seam error identity. Prove explicitly that `RangeCommits` still leaves merge `Commit.Changes` empty.

Register `FirstParentChangedPaths` in `internal/git/entrypoints_test.go` against the semantic contract suite. Run the focused Git tests and the history relevance test; they must fail before production implementation.

### Task 2.4: Implement conservative relevance and parent-state reuse
Latitude: exact
Paths: ["internal/git/walk.go", "internal/git/git.go", "internal/audit/history.go", "internal/audit/history_test.go", "internal/audit/audit.go"]

Implement `FirstParentChangedPaths` inside the Git seam with go-git tree comparison, not one native Git subprocess per commit. Do not populate `Commit.Changes` for merges. Reuse the seam's rerooting, cancellation, safe-path, symlink/gitlink, and opaque-error conventions.

Extend each historical state with the committed config/layout used to derive it. For a non-root in-range commit, obtain changed paths from its existing non-merge `Commit.Changes` or the separate merge entrypoint. Reuse the first-parent state only when every old/new path is outside `.awf/**` and outside the parent's configured top-level `<docsDir>/decisions/*.md` set. Any ambiguous path, `.awf` change, config/layout change, relevant rename/deletion, or unavailable first-parent evidence reloads. Because `.awf/config.yaml` itself is conservatively relevant, the child cannot acquire a different decisions directory on a reuse path. A boundary parent outside the collected range loads directly and is cached, without walking its ancestry. An irrelevant child aliases the immutable parent value or cached error; it must not mutate parent-owned slices or maps.

Preserve commit evaluation order and exact finding attribution. Add a regression proving a merge's separate first-parent paths trigger reload without exposing merge changes to dependency, diff-threshold, punctuation, or other ordinary rules.

### Task 2.5: Apply the historical policy projection claim and documentation
Latitude: exact
Paths: ["docs/decisions/revision-aware-historical-audit-pipeline.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/topics/tooling/audit-and-snapshots.md", "docs/domains/tooling.md", "docs/architecture.md"]

Append one Applied event for operation `add tooling/audit-and-snapshots:audit-history-policy-projection`. Add this exact second claim:

```markdown
### `invariant: audit-history-policy-projection`

Historical transition and stale-merge replay derive only committed configuration and schema boundaries, ADR records and source bytes, and topic definitions and claims. Marker indexes, test-glob backing, coverage paths, and domain ownership sidecars stay owned by repository and staged checks; malformed bytes exclusive to those omitted projections do not create historical findings or failures. An in-range revision proven not to change this authority reuses its first-parent state, with merge relevance derived separately from first-parent paths and ambiguous evidence forcing a reload.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test
```

Add exact proof markers naming `TestLoadAuthorityCorpusFromTreeOmitsMarkersAndDomainPaths`, `TestLoadUniverseFromTreeMatchesPolicyProjection`, and `TestHistoricalStateUsesPolicyProjectionAndReusesIrrelevantCommits` above their respective tests. Each named test must mutation-prove its layer; no single easy unit test stands in for all producer and consumer clauses.

Update authored tooling and architecture prose with the reduced historical policy boundary, conservative path relevance, boundary-parent behavior, and the fact that current/staged checks retain full marker, coverage, and domain-sidecar validation. Run `./x render && ./x check`; both must finish clean.

### Phase close

Run `gofmt` on changed Go files, `go test ./internal/topic ./internal/currentstate ./internal/git ./internal/audit ./internal/project`, `go test ./...`, `git diff --check`, `./x render`, and `./x check`. Stage the complete phase explicitly, require `./awf check staged` and `./x gate`, then create:

```commit
refactor(tooling): narrow historical audit authority
```

## Phase 3: Propagate historical cancellation

**Execution mode: inline.**

### Task 3.1: Write cancellation regressions before the fix
Paths: ["internal/audit/history_test.go", "internal/audit/git_context_test.go"]

Add `TestAuditPropagatesHistoricalCancellation` with subtests for `context.Canceled` and `context.DeadlineExceeded` returned by range collection, transition result derivation, first-parent derivation, merge changed paths, a state shared with stale-merge replay, stale-merge-only evidence, and live cleanliness. Assert `errors.Is` identity, no conversion to a `current-state-transition` Warning, no later state derivation or rule/live call after cancellation is observed, and no retry of the canceled cached result. Retain tests showing an ordinary non-context parse/load error is still a transition Warning and a stale-merge evidence error is still fatal. Run the focused test and require it to fail because transition replay currently converts cancellation into a finding. Phase 4 extends this same named regression with committed-entry enumeration and selected-blob read cancellation before the claim receives terminal backing.

### Task 3.2: Make context termination an operation failure
Latitude: exact
Paths: ["internal/audit/history.go", "internal/audit/audit.go", "internal/audit/history_test.go", "internal/audit/git_context_test.go"]

At each historical orchestration boundary, detect cancellation and deadline errors with `errors.Is` and return them immediately. Do not string-match, wrap them into findings, or continue to later commits/rules. Preserve useful operation context with `%w` wrapping. Keep all non-context transition load errors advisory and all non-context stale-merge evidence errors fatal as before. Ensure the Git seam remains the sole mechanism boundary and every lower-level cancellation is reachable through the operation result.

### Task 3.3: Apply cancellation authority and current documentation
Latitude: exact
Paths: ["docs/decisions/revision-aware-historical-audit-pipeline.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/topics/tooling/audit-and-snapshots.md", "docs/domains/tooling.md", "docs/architecture.md"]

Append one Applied event for operation `add tooling/audit-and-snapshots:audit-cancellation-propagates`. Add this exact third claim:

```markdown
### `invariant: audit-cancellation-propagates`

When range collection, committed evidence, revision derivation, transition replay, stale-merge replay, or the live cleanliness read returns context cancellation or deadline expiry, `awf audit` aborts and preserves that error identity. It never converts context termination into a finding, retries the canceled derivation, or continues to later audit work; non-context transition load failures remain advisory.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test
```

Place `// invariant: tooling/audit-and-snapshots:audit-cancellation-propagates (TestAuditPropagatesHistoricalCancellation)` above the complete regression. Update the tooling domain and architecture data flow to name cancellation as an operation outcome rather than an advisory finding. Render and require clean drift/state checks.

### Phase close

Run `gofmt` on changed Go files, `go test ./internal/audit -run 'TestAuditPropagatesHistoricalCancellation|Test.*Context|TestHistoryOperation'`, `go test ./internal/audit ./internal/git ./internal/project`, `git diff --check`, `./x render`, and `./x check`. Stage explicitly, require `./awf check staged` and `./x gate`, then create:

```commit
fix(tooling): propagate historical audit cancellation
```

## Phase 4: Select committed authority blobs and measure the result

**Execution mode: inline.**

### Task 4.1: Specify committed entry and selected-blob contracts first
Latitude: exact
Paths: ["internal/git/git_test.go", "internal/git/walk_test.go", "internal/git/entrypoints_test.go"]

Add backend-neutral contract coverage for two new methods:

- `(*git.Repo).CommitEntries(ctx, rev) ([]git.TreeEntry, error)` returns sorted project-relative entries with `Path` and existing `BlobMode` for regular, executable, and symlink files; it skips gitlinks, reads no blob contents, honors a nested-project handle prefix, and propagates cancellation and missing/corrupt revision errors.
- `(*git.Repo).CommitBlobsAt(ctx, rev, paths []string) ([]git.IndexBlob, error)` accepts a duplicate-free set of canonical project-relative paths, returns sorted immutable bytes and modes for exactly those paths, rejects unsafe or duplicate requests, and errors rather than silently omitting a requested missing, gitlink, outside-prefix, or unsupported entry.

Cover empty selection, regular/executable/symlink modes, nested directories, subdirectory rerooting, missing selection, duplicate and unsafe input, and cancellation before and during reads. Make the no-eager-read clause observable with a loose-object fixture: commit one selected and one unselected blob, remove only the unselected blob object from the isolated fixture after the commit, and prove `CommitEntries` plus `CommitBlobsAt` for the selected path still succeed while selecting the removed path fails. Register both entrypoints in `internal/git/entrypoints_test.go` against the semantic suite. Run focused tests and require failure before production definitions exist.

### Task 4.2: Add an explicit immutable snapshot selection
Latitude: exact
Paths: ["internal/snapshot/selection.go", "internal/snapshot/selection_test.go", "internal/snapshot/snapshot.go"]

Create `snapshot.Selection` as a distinct immutable type, not an alias or wrapper exposing conversion to `*snapshot.Tree`. Factor one unexported snapshot file-set construction core that validates modes and canonical path safety, rejects duplicates, clones bytes, and sorts paths; both `NewTree` and `NewSelection` must consume that one core while returning distinct public types. `Selection.Lookup` and `Selection.List` return cloned bytes. Keep complete `Tree` consumers unchanged; do not add a method that materializes a Selection as a Tree or makes absence in a Selection mean repository absence.

Add `TestSelectionOwnsExplicitFileSet` for construction, ordering, lookup/list clone isolation, modes, duplicate/unsafe rejection, and compile-time-distinct API use through helpers accepting only `*Selection` or only `*Tree`. The proof marker is deferred to Phase 5. Run `go test ./internal/snapshot -run TestSelectionOwnsExplicitFileSet`; it must fail before `Selection` exists.

### Task 4.3: Implement selected Git evidence and the sparse historical loader
Latitude: exact
Paths: ["internal/git/git.go", "internal/git/handle.go", "internal/git/git_test.go", "internal/git/walk_test.go", "internal/git/entrypoints_test.go", "internal/snapshot/index.go", "internal/snapshot/selection.go", "internal/snapshot/selection_test.go", "internal/audit/history.go", "internal/audit/history_test.go", "internal/audit/git_context_test.go", "internal/currentstate/load.go", "internal/currentstate/load_test.go"]

Implement `TreeEntry`, `CommitEntries`, and `CommitBlobsAt` inside `internal/git` using go-git tree metadata and exact blob lookup. No backend representation crosses the exported signatures. Reuse `BlobMode`, rerooting, context checks, and opaque error translation. Enumeration must not call a blob reader. Selected loading must read each requested blob once and return owned bytes. In `internal/snapshot/index.go`, factor the existing `git.IndexBlob` to `snapshot.File` mode/byte translation into one unexported helper used by both complete `treeFromBlobs` and sparse-selection construction; `internal/audit` must not duplicate the mode switch.

Refactor the reduced parser core in `internal/currentstate` to accept an exact `[]snapshot.File` authority set without duplicating parsing. Existing `LoadUniverseFromTree` delegates with `tree.List()`; add `LoadUniverseFromSelection(selection *snapshot.Selection, cfg *config.Config)` as the historical consumer and keep full `LoadFromTree` unchanged.

On a historical cache miss, `internal/audit` performs this fixed two-stage selection:

1. Enumerate committed entries without bytes. If `.awf/config.yaml` is absent, return the empty historical state. Select existing `.awf/config.yaml` and `.awf/awf.lock`, load those exact blobs, parse each revision's schema-migrated config and optional lock, and validate only config fields.
2. From the already enumerated entries, select `.awf/topics/metadata/**/*.yaml`, `.awf/topics/parts/**/current-state.md`, and top-level Markdown records under that config's `<docsDir>/decisions/`, excluding no candidate beyond the existing ADR reserved-basename parser policy. Load only additional selected blobs, combine them with the control files in one `snapshot.Selection`, and derive `currentstate.Universe` from that selection.

Do not select domain sidecars, marker-source Go/templates, generated topic/domain documents, unrelated Markdown, lock-generated outputs, or arbitrary repository files. A symlink at a required authority path remains non-scannable and produces the same policy-relevant error. Missing optional lock remains supported. A selected blob disappearing or changing between enumeration and read is a committed-object error, never a working-tree race.

Add `TestHistoricalStateSelectsOnlyAuthorityBlobs` using direct counting dependencies to assert the exact selected path set for default/custom docs directories, no read of an unrelated blob or malformed marker source, one config/lock read per derived revision, and reuse across irrelevant commits. Cover absent config, absent lock, symlinked authority, nested adopted-project paths that are outside the root project's authority, and historical schema migration. Extend `TestAuditPropagatesHistoricalCancellation` with committed-entry enumeration and selected-blob read cancellation cases, proving immediate abort and no later read or rule call. Keep complete snapshot APIs in use by current/staged checks.

### Task 4.4: Add deterministic benchmarks and report real-repository measurements
Latitude: exact
Paths: ["internal/audit/history_benchmark_test.go", "docs/plans/2026-08-02-revision-aware-historical-audit-pipeline.md"]

Create `internal/audit/history_benchmark_test.go` with `BenchmarkAuditHistoryCodeOnly50`, `BenchmarkAuditHistoryAuthorityHeavy50`, and `BenchmarkAuditHistoryMergeHeavy50`. Build deterministic Git fixtures before `b.ResetTimer`, use a prebuilt/adopted project configuration, call the production `audit.Run`, use `b.ReportAllocs`, and assert setup validity outside the timed loop. The code-only shape changes unrelated Go files; authority-heavy changes ADR/topic authority legally; merge-heavy contains integrated side-branch commits and qualifying merge evidence. Do not assert elapsed time in tests.

Run:

```text
go test ./internal/audit -run '^$' -bench 'BenchmarkAuditHistory' -benchmem
go build -o /tmp/awf-audit ./cmd/awf
base=$(git rev-parse HEAD~20)
test "$(git rev-list --count "$base..HEAD")" -ge 50
TIMEFORMAT='%3R seconds'; time /tmp/awf-audit audit "$base..HEAD"
```

Record the benchmark summaries, representative commit count, prebuilt real elapsed result, and any profile-driven follow-up in this plan's Notes. The measurement is diagnostic: less than 10 seconds is substantial and less than 2 seconds aspirational, but neither gates Phase close. If the result remains material, capture a CPU/allocation profile and record a concrete follow-up recommendation; do not add structural sharing or incremental corpus parsing in this transaction.

### Task 4.5: Update architecture and tooling documentation without applying the final claim
Latitude: exact
Paths: [".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/awf.lock", "docs/domains/tooling.md", "docs/architecture.md", "docs/plans/2026-08-02-revision-aware-historical-audit-pipeline.md"]

Document the backend-neutral Git entry/selection seam, the explicit `snapshot.Selection` boundary, the two-stage historical selection flow, and the remaining complete-snapshot consumers. State that committed entry enumeration reads metadata only and that exact selected reads fail closed. Update the plan Notes with actual measurements. Run `./x render && ./x check` and inspect all rendered changes.

Keep the linked ADR `Implementing`, operation `add tooling/audit-and-snapshots:sparse-snapshot-explicit-selection` Remaining, and the authored topic part unchanged for that claim. Do not add any proof marker for the final claim before terminal implementation review.

### Phase close

Run `gofmt` on changed Go files, `go test ./internal/git ./internal/snapshot ./internal/currentstate ./internal/audit ./internal/project`, `go test ./...`, the benchmark command from Task 4.4, `git diff --check`, `./x render`, and `./x check`. Stage the complete implementation and documentation transaction explicitly, require `./awf check staged` and `./x gate`, then create:

```commit
perf(tooling): select historical audit authority blobs
```

## Phase 5: Govern the reviewed sparse pipeline and freeze the records

**Execution mode: inline.**

### Task 5.1: Apply the final claim and terminal lifecycle events
Latitude: exact
Paths: ["glob:docs/decisions/[0-9][0-9][0-9][0-9]-revision-aware-historical-audit-pipeline.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", "internal/snapshot/selection_test.go", "internal/git/git_test.go", "internal/audit/history_test.go", "docs/plans/2026-08-02-revision-aware-historical-audit-pipeline.md", ".awf/awf.lock", "docs/topics/tooling/audit-and-snapshots.md", "docs/decisions/INDEX.md"]
Post-check: `set -- docs/decisions/[0-9][0-9][0-9][0-9]-revision-aware-historical-audit-pipeline.md; test "$#" -eq 1 && test -f "$1" && ./x check`

This phase's path base is the integrated primary checkout `/home/hypno/Projects/agentic-workflows`, not the managed-worktree base used by Phases 1 through 4. After terminal implementation review settles, integration completes, and ADR numbering commits, resolve the numbered ADR deterministically with `set -- docs/decisions/[0-9][0-9][0-9][0-9]-revision-aware-historical-audit-pipeline.md; test "$#" -eq 1 && test -f "$1"; adr_path=$1`; use only `$adr_path` for lifecycle edits. Then add this exact final claim:

```markdown
### `invariant: sparse-snapshot-explicit-selection`

Historical audit enumerates committed path and mode metadata without reading blob contents, then reads only its exact authority selection into an immutable snapshot Selection that is type-distinct from a complete snapshot Tree. Selected reads fail on an unsafe, duplicate, missing, outside-project, or unsupported requested path; full-tree consumers cannot treat an unselected path as repository absence, and current and staged checks retain complete snapshots.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test
```

Add these exact proof markers immediately above the already reviewed tests:

```text
// invariant: tooling/audit-and-snapshots:sparse-snapshot-explicit-selection (TestSelectionOwnsExplicitFileSet)
// invariant: tooling/audit-and-snapshots:sparse-snapshot-explicit-selection (TestCommitEntriesAndBlobsAtContracts)
// invariant: tooling/audit-and-snapshots:sparse-snapshot-explicit-selection (TestHistoricalStateSelectsOnlyAuthorityBlobs)
```

The Git contract test created in Phase 4 must use the exact name `TestCommitEntriesAndBlobsAtContracts`. Each marker proves its own layer and all three together prove the claim.

Append the final Applied event for operation `add tooling/audit-and-snapshots:sparse-snapshot-explicit-selection`, then append the linked ADR's Implemented status event with its current content stamp. Change this plan's `status:` to `Implemented` and record implementation deviations and final measurements in Notes. Run `./x render`; require the ADR to move from In flight to History and no operation to remain.

### Phase close

Run `go test ./internal/git -run TestCommitEntriesAndBlobsAtContracts`, `go test ./internal/snapshot -run TestSelectionOwnsExplicitFileSet`, `go test ./internal/audit -run TestHistoricalStateSelectsOnlyAuthorityBlobs`, `git diff --check`, `./x render`, and `./x check`. Stage only the final claim, proof markers, lifecycle records, plan Notes/status, lock, and rendered outputs. Require `./awf check staged` and `./x gate`, then create:

```commit
feat(tooling): govern sparse historical audit
```

## Definition of done

- `awf audit` collects the requested DAG range once and `internal/project` performs no second historical walk.
- Transition and stale-merge replay share operation-owned revision states and cached errors; no cache survives an invocation or requires invalidation.
- Historical replay parses ADR/topic-claim authority without marker, coverage, or domain-sidecar validation, while current and staged checks retain their full behavior.
- Irrelevant ordinary and merge commits reuse first-parent authority safely, and ambiguous or authority-changing paths reload without exposing merge diffs to ordinary audit rules.
- Context cancellation and deadline expiry abort with matchable identity and never become findings.
- Committed entry enumeration avoids blob reads, selected reads fail closed, and historical audit reads only its explicit authority Selection while complete snapshot consumers remain type-separated.
- Existing audit finding names, ranks, ordering, commit attribution, range membership, ordinary merge-change behavior, and non-context error routing pass their regression suites outside the approved projection refinement.
- All new Git entrypoints are registered in the backend-neutral contract registry; all four ADR claims exist with named, mutation-capable proof markers and no operation remains.
- `go test ./...`, `./x render`, `./x check`, `./awf check staged`, and `./x gate` finish successfully at every phase close, with 100 percent statement coverage and no production dead code.
- The synthetic benchmarks and a representative prebuilt 50-plus-commit real audit are reported in Notes without turning elapsed time into a gate.
- Authored architecture and tooling-domain sources describe the final ownership, reduced projection, cancellation outcome, Git selection seam, and sparse/full snapshot boundary; rendered outputs are clean.
- After terminal review and integration, the ADR and this plan are Implemented, the managed worktree and branch are removed without force, and retrospective runs last.

## Notes

- Before implementation, a prebuilt binary took about 68 seconds over an indicative 69-commit range and about 85 seconds over an indicative 85-commit range in this repository. These observations motivate the work but are not verification counts or acceptance thresholds.
- The plan and linked ADR remain Proposed until implementation is authorized. The first phase moves the ADR directly to Implementing with its first Applied batch. Phases 2 and 3 apply middle batches. Phase 4 implements the final code and tests but leaves its operation Remaining until terminal review.
- After Phase 4 closes, merge the current integration branch into the managed worktree and resolve or abort any conflict. Number the pending ADR with `./awf adr number revision-aware-historical-audit-pipeline`, render, stage only numbering/render output, require `./awf check staged && ./x gate`, and commit the numbering transaction. Invoke `awf-reviewing-impl` over every effort commit and settle findings in new green commits.
- Integrate through `./awf effort integrate streamline-the-historical-audit-pipeline` from the clean primary checkout. Accept only an already-integrated or fast-forward result, or explicitly finish a clean staged divergent merge after `./awf check staged && ./x gate`. Re-run terminal implementation review over any target-side merge/fix history until no findings remain. Phase 5 runs only in the integrated primary checkout after that review settles.
- After Phase 5, run `./awf effort worktree remove streamline-the-historical-audit-pipeline` without force. Require the managed worktree path, worktree registration, and effort branch to be absent before invoking `awf-retrospective`.
