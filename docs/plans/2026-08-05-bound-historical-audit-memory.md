---
format: plan-v2
date: 2026-08-05
adrs:
  - lifetime-bounded-historical-audit-replay
status: Proposed
---
# Plan: Bound historical audit memory

## Goal

Make one `awf audit <base>..<head>` invocation retain only incremental range-rule state, compact
replay metadata, and the live unique historical-authority frontier while preserving normal findings,
ordering, attribution, severity, exit behavior, immediate cancellation, and at-most-once revision
derivation.

Do not add public batching, a persistent or disk-backed cache, repeated boundary parsing, a second
range walk, a generic cache framework, structural sharing across parser packages, or changes to
current-state transition and stale-merge qualification policy.

## Architecture summary

The existing Git seam changes from returning a materialized rich range to visiting one rich commit at
a time during its single walk. `internal/audit` owns incremental accumulators for the ordinary rule
groups and immediately projects each visited commit into compact replay metadata. It constructs a
child-before-parent dependency schedule from revision identities and ordered parents, validating the
graph independently from backend traversal order and using revision identity as the deterministic
tie-breaker; the original stream ordinal remains only for compatible finding order.

Historical replay resolves relevance aliases through lightweight committed controls before loading
heavy authority. A revision store distinguishes revision keys from shared state entries, caches each
load outcome once, counts the scheduled universe and source consumers of each canonical entry, and
clears source evidence and then the full heavy projection at their respective final uses. Stale-merge
and transition work executes in one graph schedule with separate result buffers. The live cleanliness
read follows replay internally and its findings are inserted between stale and transition buffers, so
normal output stays range rules, stale merge, live cleanliness, then transitions. The linked ADR
accepts that deterministic graph execution can change which coexisting fatal or cancellation failure
surfaces first.

Phase 1 authorizes the reviewed ADR. Phase 2 streams the rich range through incremental ordinary
rules. Phase 3 lands compact graph scheduling and interleaved replay while retaining the existing
revision cache. Phase 4 splits light and heavy state, resolves aliases, and releases final-consumer
evidence. Phase 5 proves the bound, applies the current-state claim, and publishes architecture and
operator-facing documentation. The terminal review flow later owns the status-only ADR and plan
completion transitions.

## Phase 1: Authorize lifetime-bounded replay

**Execution mode: inline.**

Advances: ["repository-green"]
Completes: ["adr-authorized"]

### Task 1.1: Accept the reviewed successor ADR
Latitude: exact
Paths: ["docs/decisions/lifetime-bounded-historical-audit-replay.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Start from the committed, review-settled plan with `git status --short` producing no output and
`./x check` finishing clean. Apply the `awf-adr-lifecycle` procedure without changing the reviewed
Context, Decision, State changes, Consequences, or Alternatives. Change the ADR frontmatter from
`Proposed` to `Accepted` and append the execution-day Accepted status event with its canonical
`content-sha256`. Obtain the stamp mechanically by placing a 64-lowercase-hex placeholder, running
`./awf check`, and replacing it with the digest reported by the mismatch; repeat until the check is
clean rather than hashing file bytes independently.

Run `./x render`. Inspect the ADR, generated decision index, and lock diff; the transaction must
contain only the Accepted lifecycle event and deterministic render consequences. Run
`./awf context --show pending docs/decisions/lifetime-bounded-historical-audit-replay.md` and require
`tooling/audit-and-snapshots:audit-history-operation-owned` to be Remaining, with no operation
Applied or Canceled.

### Phase close

Stage the ADR, `docs/decisions/INDEX.md`, and `.awf/awf.lock` explicitly. Require
`./awf check staged` and `./x gate` to pass, then create the closing commit.

```commit
docs(adr): accept lifetime-bounded audit replay
```

## Phase 2: Stream rich range evidence through ordinary rules

**Execution mode: subagent-driven.**

Advances: ["repository-green", "compatible-audit"]
Completes: ["stream-bounded-range"]

### Task 2.1: Specify the streaming Git seam and incremental rule oracle
Latitude: exact
Applying: ["lifetime-bounded-historical-audit-replay:compact-replay-projection", "lifetime-bounded-historical-audit-replay:invocation-local-boundary"]
Paths: ["internal/git/walk_test.go", "internal/git/entrypoints_test.go", "internal/audit/audit_test.go", "internal/audit/history_test.go"]

Start from a clean managed worktree whose latest commit is the Phase 1 Accepted transition.
`git status --short` must produce no output, `./x check` must finish clean, and `./x gate` must pass.

Write failing tests before production changes. Replace the materializing `RangeCommits` contract with
one production-reachable `WalkRangeCommits(ctx, base, head, visit)` contract that returns the number
of included commits and visits each rich `git.Commit` exactly once. Preserve current base/head
resolution, unrelated-history error, empty-range result, prefix rerooting and filtering, merge
inclusion, Markdown text, line statistics, parent order, callback order, and context identity.
Exercise linear, root, merge, nested-prefix, shallow-boundary, corrupt-tree, callback-error, and
cancellation paths. A visitor failure must stop the walk immediately and preserve the visitor error
identity; the returned count must describe only successfully visited included commits. Update the
mechanically checked Git entrypoint registry to name the new seam and its contract test. Do not keep
a production `RangeCommits` compatibility wrapper that would become unreachable outside tests and
fail the dead-code gate.

Add an audit differential fixture that feeds the same rich commits to the existing whole-range rule
functions and to a new incremental evaluator oracle. Assert exact `Finding` slices per rule group and
for final grouping, including disabled and empty cases. Cover conventional subjects, ADR
frontmatter and status/index co-change, dependency-manifest versus ADR branch aggregation,
non-generated changed-line threshold versus plan presence, documented and undocumented domains,
domain code and current-state-part co-change, generated-path exclusions, and punctuation rises,
falls, and stable counts. The new tests must fail because no streaming seam or incremental evaluator
exists; record that expected red state before production edits.

### Task 2.2: Replace whole-range evaluation with one-pass accumulators
Applying: ["lifetime-bounded-historical-audit-replay:compact-replay-projection", "lifetime-bounded-historical-audit-replay:invocation-local-boundary"]
Paths: ["internal/git/walk.go", "internal/git/walk_test.go", "internal/git/entrypoints_test.go", "internal/audit/audit.go", "internal/audit/audit_test.go", "internal/audit/history.go", "internal/audit/history_test.go"]

In `internal/git/walk.go`, extract the existing range resolution and traversal into
`Repo.WalkRangeCommits`. Invoke the visitor only after prefix inclusion is decided, stop on its
error, and avoid appending rich commits anywhere in the Git implementation. Remove the old
materializing entrypoint and adapt its contract coverage to collect inside tests only when a test
needs the complete sequence.

In `internal/audit`, replace `evaluate([]awfgit.Commit, Inputs)` with one invocation-local evaluator
whose `observe(awfgit.Commit)` method offers a commit to every ordinary rule accumulator and whose
`findings()` finalizes buffers in the existing rule order. Keep `CheckConventionalCommit` as the
single commit-policy implementation. Translate the current rules without changing their policies:
per-commit groups append to their own buffers; branch aggregates retain only booleans, canonical
sets/maps, accumulated line totals, and the minimal ADR/domain evidence required to finalize their
existing sorted findings. Never retain `FileChange.OldText`, `NewText`, or the rich commit after
`observe` returns.

During the same visitor call, append one audit-owned compact replay record carrying original ordinal,
hash, full revision, subject, ordered parents, merge flag, merge message only for merges, and sorted
unique old/new changed paths needed by relevance. Do not retain actions, line counts, or Markdown
text in replay metadata. Change `historyOperation` construction and `Run` to receive the stream
collector, compact records, grouped ordinary findings, and exact visited count. At this phase the
existing stale and transition implementations may still iterate compact records through a temporary
adapter, but no later consumer may require the rich range.

Require all existing audit finding tests to remain byte-for-byte compatible. Run
`go test ./internal/git ./internal/audit` and verify the streaming tests establish visitor failure
and cancellation without a second walk.

### Phase close

Run `go test ./internal/git ./internal/audit`, `git diff --check`, `./x render`, and `./x check`; each
must exit zero. Stage the complete seam, accumulator, compact projection, and test transaction
explicitly. Require `./awf check staged` and `./x gate` to pass, then create the closing commit.

```commit
refactor(tooling): stream audit range evaluation
```

## Phase 3: Schedule deterministic interleaved replay

**Execution mode: subagent-driven.**

Advances: ["repository-green", "compatible-audit"]
Completes: ["deterministic-replay"]

### Task 3.1: Specify graph integrity, scheduling, and compatibility
Latitude: exact
Applying: ["lifetime-bounded-historical-audit-replay:explicit-dependency-schedule", "lifetime-bounded-historical-audit-replay:deterministic-interleaved-replay"]
Paths: ["internal/audit/history_test.go"]

Start from a clean managed worktree whose latest commit is the Phase 2 streaming transaction.
`git status --short` must produce no output, `./x check` must finish clean, and `./x gate` must pass.

Add failing table tests around an audit-owned replay graph built from compact records. Prove it
rejects duplicate revision identities, self-parent evidence, and cycles among in-range nodes;
treats every parent absent from the selected compact range as an explicit boundary dependency rather
than an error; preserves ordered parent roles; and produces the same schedule for every permutation
of the input records. Cover linear, fork, ordinary merge, octopus merge, disconnected selected
components caused by prefix filtering, and shared boundary-parent graphs.

Define the schedule exactly: every in-range child precedes its in-range parents; among currently
ready nodes choose the lexicographically smallest full revision, independent of stream order. Keep
the original stream ordinal only on findings so compatible grouping never depends on execution
order. Assert the scheduler registers result, first-parent transition, and every ordered merge-parent
uses and does not walk ancestry beyond compact nodes and their named boundary revisions.

Add end-to-end operation tests proving graph replay returns ordinary, stale, live, and transition
finding buffers in the established output order even when execution order differs. Cover a
transition warning with a stale authorization finding, a live cleanliness finding, root comparison
against the empty universe, first-parent merge transition, all-parent stale qualification, cached
load errors, and cancellation at each consumer. Pin the approved change: when coexisting failures
are reachable, the deterministic graph's first encountered failure wins, while context termination
still aborts immediately and is never converted to a finding.

### Task 3.2: Replace phase-wide history loops with one graph replay
Applying: ["lifetime-bounded-historical-audit-replay:explicit-dependency-schedule", "lifetime-bounded-historical-audit-replay:deterministic-interleaved-replay"]
Paths: ["internal/audit/history.go", "internal/audit/history_test.go"]

Implement the compact replay graph and deterministic child-before-parent scheduler in
`internal/audit`. Validate the complete compact graph before loading committed authority. Boundary
parents remain direct revision keys for the existing loader and never become recursively expanded
nodes. Keep graph and scheduling types unexported and cohesive with `historyOperation`.

Replace separate operation-wide `staleMergeFindings` and `transitionFindings` traversal from
`historyOperation.run` with one scheduled replay step per compact commit. A step performs the merge
schema/authorization/qualification work and the before/after transition work required by that
commit, using the existing state cache and current-state functions unchanged in this phase. Store
stale and transition findings separately and attach original commit attribution; call live
cleanliness after scheduled replay, then assemble range-rule, stale, live, and transition buffers in
the established external order. Remove obsolete phase-loop production functions rather than
retaining parallel implementations.

Preserve exact fatal/advisory boundaries: malformed or unavailable stale-merge evidence is fatal;
non-context transition projection failure is a warning for its commit; context cancellation and
deadline expiry remain matchable operation errors; cached errors are not retried. Run
`go test ./internal/audit` and require graph permutation tests and all existing historical policy
fixtures to pass.

### Phase close

Run `go test ./internal/audit`, `git diff --check`, `./x render`, and `./x check`; each must exit zero.
Stage the graph, replay, and tests explicitly. Require `./awf check staged` and `./x gate` to pass,
then create the closing commit.

```commit
refactor(tooling): schedule historical audit replay
```

## Phase 4: Release heavy revision evidence at final use

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["lifetime-bounded-replay", "compatible-audit"]

### Task 4.1: Specify light controls, alias ownership, and final-use release
Latitude: exact
Applying: ["lifetime-bounded-historical-audit-replay:final-consumer-release", "lifetime-bounded-historical-audit-replay:alias-aware-ownership"]
Paths: ["internal/audit/history_test.go"]

Start from a clean managed worktree whose latest commit is the Phase 3 graph-replay transaction.
`git status --short` must produce no output, `./x check` must finish clean, and `./x gate` must pass.

Write failing tests before changing revision loading. Instrument the unexported operation model with
logical current and high-water heavy-entry counters owned by the revision store; tests inspect those
values directly, with no runtime finalizer, heap profile, GC timing, or observer callback retaining
payloads.

Prove light controls and cached errors derive once per revision; authority bytes and parsing remain
lazy until the first heavy consumer; loader functions release captured selections after execution;
and a non-context parent-control failure conservatively derives the child instead of creating a
false alias. Resolve in-range relevance recursively from compact changed paths and the parent's
committed docs layout before heavy loading. A `.awf/**` change, a top-level ADR change under the
historically applicable decisions directory, a relevant merge first-parent path, or ambiguous
evidence keeps a distinct state. Boundary revisions without compact change evidence remain distinct.

Cover chains and forks of irrelevant aliases, a relevant child beside irrelevant siblings, shared
boundary parents, ordinary and octopus merges, pre-schema merges, roots, malformed controls,
cached heavy-load failures, and cancellation. For every fixture, derive the expected live frontier
from its declared consumer graph and assert the store's high-water result equals that frontier,
every scheduled universe/source consumer is discharged, every heavy payload is cleared at terminal
state, and no canonical load outcome is retried. Assert `Universe.Sources` clears after the final
stale consumer even when ADR/topic transition data remains live, and the full universe clears only
after its final consumer.

### Task 4.2: Split revision controls from heavy payload and account for shared consumers
Applying: ["lifetime-bounded-historical-audit-replay:final-consumer-release", "lifetime-bounded-historical-audit-replay:alias-aware-ownership"]
Paths: ["internal/audit/history.go", "internal/audit/history_test.go"]

Refactor `revisionState` and `loadSelectedRevision` into explicit light and heavy ownership. The
light load enumerates committed metadata once, reads and validates config plus the optional lock,
derives docs layout and exact authority paths, and retains only compact controls, path selection,
and cached errors. Its heavy loader reads that exact selection, validates scannability, parses the
existing reduced `currentstate.Universe`, caches the outcome once, and nils its loader and any
captured selection immediately after materialization. Do not reimplement config, ADR, topic, sparse
selection, transition, or qualification grammar in audit.

Resolve every compact in-range revision to a canonical state entry before scheduling heavy
consumers. Store revision-key aliases separately from entry ownership. Aggregate universe and source
consumer totals across every key resolving to the same entry, including result, first-parent,
ordered incoming-parent, and boundary uses. For each scheduled use, decrement the matching logical
consumer exactly once; clear `Universe.Sources` when the source total reaches zero, and clear the
whole universe plus heavy error payload after the final heavy consumer. Keep compact controls and
cached error identity only through their last light consumer, then remove the revision key and clear
the shared entry when no key remains.

Preserve fallback semantics when parent controls, merge first-parent paths, or committed docs layout
cannot prove irrelevance. Preserve cancellation identity at every light and heavy read. A nil state
from the injected loader remains fail-closed. Remove lazy closures whose only effect was
operation-lifetime selection retention.

Run `go test ./internal/audit` and require the deterministic logical frontier tests, exact load-call
assertions, and all policy compatibility fixtures to pass.

### Phase close

Run `go test ./internal/audit`, `git diff --check`, `./x render`, and `./x check`; each must exit zero.
Stage the state ownership, release accounting, and regression tests explicitly. Require
`./awf check staged` and `./x gate` to pass, then create the closing commit.

```commit
fix(tooling): release consumed audit revision state
```

## Phase 5: Prove and publish bounded audit ownership

**Execution mode: subagent-driven.**

Completes: ["authority-current", "repository-green"]

### Task 5.1: Measure streaming and heavy-frontier scaling
Latitude: exact
Applying: ["lifetime-bounded-historical-audit-replay:bounded-memory-contract"]
Paths: ["internal/audit/history_benchmark_test.go", "internal/audit/history_test.go", "internal/git/walk_test.go"]

Start from a clean managed worktree whose latest commit is the Phase 4 final-use transaction.
`git status --short` must produce no output, `./x check` must finish clean, and `./x gate` must pass.

Extend the existing code-only, authority-heavy, and merge-heavy synthetic benchmarks so setup is
outside the timed body and benchmark output reports allocations and work without retaining prior
iterations. Add fixtures whose history length grows while the live dependency frontier remains the
same, plus fork/merge and irrelevant-alias fixtures whose declared graph changes the frontier.
Reuse the revision store's logical high-water state in deterministic tests; do not infer correctness
from `runtime.MemStats`, RSS, allocation totals, or garbage collection.

Prove streaming collection never has more than the visitor's current rich commit logically live and
that heavy high-water follows the fixture's dependency frontier rather than its number of
historically relevant revisions. Run each bounded benchmark once with
`timeout 90s env GOMEMLIMIT=512MiB go test ./internal/audit -run '^$' -bench '^BenchmarkAuditHistory' -benchtime=1x -benchmem`;
the command must exit zero. Record diagnostic benchmark results in the plan Notes during execution,
without turning machine-specific bytes or duration into a gate. Do not run the reported full
`v0.22.0..HEAD` audit in the unisolated development host.

### Task 5.2: Apply the operation-owned claim and authored documentation
Kind: batch
Latitude: exact
Applying: ["lifetime-bounded-historical-audit-replay:compact-replay-projection", "lifetime-bounded-historical-audit-replay:explicit-dependency-schedule", "lifetime-bounded-historical-audit-replay:final-consumer-release", "lifetime-bounded-historical-audit-replay:alias-aware-ownership", "lifetime-bounded-historical-audit-replay:deterministic-interleaved-replay", "lifetime-bounded-historical-audit-replay:invocation-local-boundary", "lifetime-bounded-historical-audit-replay:bounded-memory-contract"]
Paths: [".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", "docs/decisions/lifetime-bounded-historical-audit-replay.md", "internal/audit/history_test.go", "changelog/CHANGELOG.md", "docs/topics/tooling/audit-and-snapshots.md", "docs/architecture.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Update `audit-history-operation-owned` so one streamed range operation owns incremental rule state, compact graph metadata, at-most-once revision outcomes, alias-aware shared entries, and final-consumer release bounded by the live unique dependency frontier."
Edge: "Preserve immediate cancellation and normal finding order while stating that deterministic interleaved replay, not the former stale-live-transition execution sequence, owns coexisting failure precedence."
Post-check: "Run ./x render && ./x check, then run ./awf context --show pending docs/decisions/lifetime-bounded-historical-audit-replay.md; require the one update operation Applied, none Remaining or Canceled, and the ADR still Implementing. Run rg -n 'materializ|cache.*lifetime|stale.*live.*transition' .awf/topics/parts/tooling/audit-and-snapshots/current-state.md .awf/docs/parts/architecture/components.md .awf/docs/parts/architecture/data-flow.md .awf/docs/parts/architecture/dependencies.md and inspect every match so no active prose retains the superseded whole-range or operation-lifetime-heavy-cache model."

Apply the linked ADR's one declared operation using `awf-adr-lifecycle`. Change its frontmatter from
`Accepted` to `Implementing`, append the canonical Implementing status event repeating the latest
content stamp, then append one Applied event naming exactly
`update tooling/audit-and-snapshots:audit-history-operation-owned`. Do not append Implemented; the
terminal review flow owns that later status-only transaction.

Rewrite the authored claim to retain one invocation, one streamed range walk, normal finding
compatibility, at-most-once revision outcomes, and no Project/global cache while adding incremental
range-rule state, compact replay graph ownership, deterministic interleaving, alias-aware shared
entries, and final-consumer source/universe release. Preserve `Origin: ADR-0221`, add
`Revised-by: ADR-lifetime-bounded-historical-audit-replay`, retain `Backing: test`, and place the
exact invariant proof marker on the live test whose named unit proves streaming, shared ownership,
and terminal release. Keep cancellation and sparse-policy claims unchanged unless their prose is
mechanically cross-referenced; this ADR declares no operation on them.

Update only authored architecture sources to replace the materialized range and operation-lifetime
heavy cache with the streaming Git callback boundary, audit-owned incremental accumulators, compact
graph scheduling, light controls, and final-consumer heavy ownership. Keep `internal/git` free of
audit policy and keep current-state parsing/qualification with their existing packages. Add an
Unreleased changelog fix describing bounded audit memory without promising a fixed byte or duration.
Run `./x render`; include every generated topic, architecture, decision index, and lock consequence,
and never edit generated files directly.

### Task 5.3: Verify the complete bounded-memory transaction
Applying: ["lifetime-bounded-historical-audit-replay:bounded-memory-contract"]
Paths: ["internal/audit", "internal/git", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", "docs/decisions/lifetime-bounded-historical-audit-replay.md", "changelog/CHANGELOG.md"]

Run `go test ./internal/git ./internal/audit`, the bounded one-iteration benchmark command from Task
5.1, `git diff --check`, `./x render`, and `./x check`; every command must exit zero. Inspect
`git status --short` and stage the complete code, benchmark, ADR application, authored claim,
authored architecture, changelog, and deterministic rendered outputs explicitly. Run
`./awf check staged` and `./x gate`; both must pass with the invariant proof marker resolving to its
named live test and no dead production seam.

### Phase close

After Task 5.3's staged checks pass, create the one closing application commit.

```commit
fix(tooling): publish bounded historical audit ownership
```

## Definition of done

- `dod: adr-authorized` The reviewed successor ADR is Accepted before production execution, with its one claim update Remaining and no premature authority mutation.
- `dod: stream-bounded-range` One Git range walk visits rich commits incrementally, ordinary audit rules retain only group-specific summaries/findings, and replay retains only compact commit metadata.
- `dod: deterministic-replay` Audit constructs and validates a backend-order-independent child-before-parent graph schedule and preserves externally grouped findings while using accepted graph-order failure precedence.
- `dod: lifetime-bounded-replay` Each revision outcome derives at most once, aliases share canonical ownership safely, and source and universe payloads release after their final scheduled consumers so logical high-water follows the live unique dependency frontier.
- `dod: compatible-audit` Existing normal rule names, ordering, severity, attribution, transition warnings, stale fatal errors, live findings, and immediate context termination remain covered and green.
- `dod: authority-current` The ADR's one operation is Applied, the active claim and authored architecture describe streaming and final-consumer ownership, generated docs agree, and the ADR remains Implementing for terminal review.
- `dod: repository-green` Every phase closes with explicit staged conformance, 100 percent coverage, no production dead code, rendered drift clean, and the full repository gate passing.

## Notes

Record diagnostic benchmark output, review-driven plan adjustments, implementation deviations, and
follow-up measurements here. Do not record the unsafe full-range command as required verification;
run it only later inside operator-provided hard memory isolation.
