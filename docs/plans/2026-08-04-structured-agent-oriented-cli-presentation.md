---
format: plan-v2
date: 2026-08-04
adrs: [structured-agent-oriented-cli-presentation]
status: Proposed
---
# Plan: Structured Agent-Oriented CLI Presentation

## Goal

Give every ordinary `awf` command one deterministic, readable text presentation contract while
retaining byte-exact authored payloads and required machine protocols, removing convenience JSON
modes, and preserving domain ownership, continuation policy, mutation safety, and process outcomes.

## Architecture summary

A new standard-library-only `internal/presentation` package owns the closed node tree, value
normalization, validation, standard presentation shapes, prompt mode, and sole text renderer. Domain
packages map typed results into those shapes; `cmd/awf` retains argument parsing, bypass selection,
stream selection, and exit mapping through one typed command boundary. Checks and migrations collect
typed outcomes before presentation. The rollout establishes the syntax authority first, converts
context and command-boundary seams, migrates domain-owned result families in dependency order, then
closes structural adoption and exact public-contract coverage. Current-state operations apply only
with their matching implementation; the three whole-interface claims remain deferred until settled
terminal implementation review.

## Phase 1: Establish the presentation core

**Execution mode: subagent-driven.**

Advances: ["closed-presentation-tree", "ordinary-output-contract", "typed-output-boundary"]
Completes: ["presentation-core"]

### Task 1.1: Specify and implement the live presentation core
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:closed-presentation-tree", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:package-and-authority-home"]
Paths: ["internal/presentation/tree.go", "internal/presentation/values.go", "internal/presentation/render.go", "internal/presentation/presentation_test.go", "internal/presentation/testdata/"]

Before editing, require the parallel Sundial-removal effort to be integrated, rebase this branch onto
that main, require `test ! -e examples/sundial` and `git status --short` to print nothing, then
require `./x check` and `go test ./internal/... ./cmd/awf` to exit zero at the accepted-plan baseline.

Create `internal/presentation` with no dependency outside the Go standard library. Add a one-sentence
package comment stating that it owns the closed CLI presentation grammar, and document every exported
type, constructor, and function. Export only `Document`, `Field`, their value constructors, and the
renderer APIs exercised by the version consumer in this phase. Enforce lowercase label grammar,
nonempty scalar values, CR/LF rejection, prose whitespace normalization, literal horizontal-space
preservation, root Field ordering, no leading or inter-field blank line, one terminal newline, and
complete validation into a buffer before exactly one destination write. Do not add Section, List,
RecordGroup, Record, Steps, a standard shape, or the prompt operation before its first production
consumer in a later phase; do not expose a raw node, alternate renderer, visitor, domain field, or
generic map.

Drive implementation through table tests covering every admitted and rejected core transition,
label/value failures, both whitespace modes, atomic-write failure, root Field ordering, blank-line
behavior, and tree rendering through the live version consumer. Long successful examples belong in
checked-in `testdata`; no update flag or snapshot generator may rewrite them. Name the core unit
`TestPresentationCore`; the complete `TestPresentationTreeContract` proof arrives with the remaining
live node APIs and its claim in Phase 3.

Run `gofmt -w internal/presentation/*.go` and
`go test ./internal/presentation -run TestPresentationCore`; both must succeed.

### Task 1.2: Convert version output as the first production consumer
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:explicit-bypasses", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["cmd/awf/version.go", "cmd/awf/version_test.go", ".github/workflows/release.yml", "templates/bootstrap/awf-bootstrap.sh.tmpl", "internal/project/bootstrap_test.go"]
Representative: "Render version as one `version: <value>` Field through `internal/presentation`, then make release and bootstrap consumers require that exact label boundary instead of positional whitespace."
Edge: "Reject a missing, duplicated, or malformed `version:` label in consumer tests; retain stderr suppression and bootstrap fallback behavior without accepting legacy unlabeled output."
Post-check: "`go test ./cmd/awf ./internal/project -run 'Test.*(Version|Bootstrap)'` exits zero; the release workflow and bootstrap template contain no positional `awk '{print $2}'` parser; generic bootstrap projection tests validate the label-aware template."

Use the tree directly rather than introducing a speculative standard shape. Update the release
workflow and bootstrap template in the same transaction as the public label, and extend generic
bootstrap projection tests to pin label-aware parsing.
This live command and its consumers make the new package reachable before the Phase 1 gate.

### Task 1.3: Apply presentation ownership and enter Implementing
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:package-and-authority-home", "structured-agent-oriented-cli-presentation:claim-backing"]
Paths: ["docs/decisions/structured-agent-oriented-cli-presentation.md", ".awf/domains/code-design.yaml", ".awf/topics/parts/code-design/presentation-ownership/current-state.md", ".awf/topics/parts/code-design/presentation-package/current-state.md", "docs/topics/code-design/presentation-ownership.md", "docs/topics/code-design/presentation-package.md", "docs/domains/code-design.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Revise `model-owner-renders` so result owners map semantics into the central representation while `internal/presentation` alone validates and renders syntax, preserving its existing Origin/Revised-by chain before appending this ADR; add the scoped `presentation-package-boundary` rule with this ADR as Origin."
Edge: "Keep the package rule local to standard-library dependency direction and representation-only ownership; do not duplicate the global syntax grammar, add `closed-presentation-tree`, or add any tooling claim before its complete live API and proof exist."
Post-check: "After `./x render`, `./x check` is clean and `./awf context docs/decisions/structured-agent-oriented-cli-presentation.md` reports `model-owner-renders` and `presentation-package-boundary` Applied with the other ten operations Remaining."

Add `internal/presentation/**` to `.awf/domains/code-design.yaml` so the new production package has
its ADR-assigned domain owner before render and the staged ownership check. In the scoped
presentation-package topic, add `rule: presentation-package-boundary`: `internal/presentation`
depends only on the Go standard library and owns presentation representation, syntax validation,
and rendering without domain result semantics. Give the rule this ADR as Origin. Use
`awf-adr-lifecycle`: move the ADR from Proposed to Implementing, append the Implementing content
stamp, then one Applied event for the first two State changes together. Mutate
`model-owner-renders` in the global authoring topic, preserve its unbacked verification contract,
and leave `closed-presentation-tree` absent. Run `./x render`; never hand-edit generated topic,
domain, index, or lock output.

### Phase close

Stage the complete Phase 1 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass with clean render drift, no dead code, and full coverage. Create one commit:

```commit
feat(code-design): add presentation core (applies presentation)
```

## Phase 2: Convert context detail and preserve protocols

**Execution mode: subagent-driven.**

Advances: ["closed-presentation-tree", "ordinary-output-contract", "explicit-bypass-contract", "standard-presentation-shapes"]
Completes: ["context-presentation-boundary"]

### Task 2.1: Map context and topic results into the common tree
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:closed-presentation-tree", "structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:explicit-bypasses"]
Paths: ["internal/contextq/render.go", "internal/contextq/render_test.go", "internal/contextq/boundary_test.go", "internal/contextq/testdata/", "internal/topic/query.go", "internal/topic/query_test.go", "cmd/awf/context.go", "cmd/awf/context_test.go", "cmd/awf/topic.go", "cmd/awf/topic_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "README.md", "templates/docs/working-with-awf.md.tmpl", "docs/working-with-awf.md"]
Representative: "Replace Markdown-like context headings, padded facts, and semicolon chains with a `Detail` mapping owned by `internal/contextq`, preserving query classification and deterministic semantic ordering while delegating syntax to `internal/presentation`."
Edge: "Keep the exact two-line `AWF_CONTEXT_SPILL_V1` notice byte-for-byte, keep source-like context packet bytes out of that notice, remove topic `--json`, and ensure section-first context output has no leading blank line."
Post-check: "`go test ./internal/contextq ./internal/topic ./cmd/awf -run 'Test.*(Context|Topic|Spill)'` exits zero; `git grep -n -- '--json' internal/clispec cmd/awf/topic.go cmd/awf/topic_test.go` reports no topic JSON option or branch; spill golden assertions remain byte-exact."

Before editing, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/presentation ./internal/contextq ./internal/topic ./cmd/awf` to exit zero at the
Phase 1 commit.

Introduce the Section, List, RecordGroup, and fixed-arity Record APIs plus `presentation.Detail` with
these first production consumers. Enforce their admitted children, record arity, compact-field
escaping, nested indentation, deterministic grouping, root Field-to-Section blank line, root Section
separation, and the three-level section bound. Do not add Steps or the other standard shapes yet. Keep query assembly and semantic mapping in
`internal/contextq`; replace only its syntax renderer with a mapping into `presentation.Detail`. Map topic query results in their model-owning package rather
than constructing presentation records in `cmd/awf`. Update public labels, grouping, order, and
escaping through exact tests. Remove the topic convenience JSON flag from parser specification,
help and dispatch. In the same phase, update the topic command entries in `README.md` and the
working-with-awf template, run `./x render`, and stage the generated working guide. Do not change topic
selection, static-reference fallback, context cap calculation, spill-file lifetime, or exit behavior.

### Task 2.2: Prove the payload and spill paths remain isolated
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:explicit-bypasses", "structured-agent-oriented-cli-presentation:typed-command-boundary"]
Paths: ["cmd/contextspilllog/main.go", "cmd/contextspilllog/main_test.go", "x", "cmd/awf/read.go", "cmd/awf/read_test.go", "internal/plan/projection.go", "internal/plan/projection_test.go"]

Retain `read plan` as authored payload bytes and the spill notice as a machine protocol. Extend
existing tests so payload/protocol branches compare complete bytes, reject presentation prefixes or
suffixes, and prove the `x` consumer distinguishes the spill sentinel from ordinary context text.
Do not route payload bytes through a `Field`, normalize their whitespace, or weaken spill parsing.
Run `go test ./cmd/contextspilllog ./internal/plan ./cmd/awf -run 'Test.*(Context|Read|Projection|Spill)'`;
it must succeed.

### Task 2.3: Apply the context-query ownership update
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:package-and-authority-home"]
Paths: ["docs/decisions/structured-agent-oriented-cli-presentation.md", ".awf/topics/parts/tooling/context-and-topic/current-state.md", "docs/topics/tooling/context-and-topic.md", "docs/domains/tooling.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Revise `context-query-boundary` to retain query assembly and semantic mapping in `internal/contextq` while assigning node validation and syntax rendering to `internal/presentation`."
Edge: "Preserve `Origin`, append this ADR to the prior `Revised-by` sequence, retain `Backing: test`, and keep the existing `TestContextQueryBoundary` proof marker valid after the dependency change."
Post-check: "`./x render && ./x check` is clean and ADR context reports operations one, two, and four Applied, with operation three and operations five through twelve Remaining."

Append one middle Applied event for only the fourth declared operation and mutate its claim in the
same transaction. Render generated outputs rather than editing them.

### Phase close

Stage the complete Phase 2 transaction explicitly, including the ADR application and rendered
outputs. Run `awf check staged` and `./x gate`; both must pass. Create one commit:

```commit
feat(tooling): migrate context output (applies presentation)
```

## Phase 3: Introduce the typed command boundary and structured help

**Execution mode: subagent-driven.**

Advances: ["ordinary-output-contract", "standard-presentation-shapes", "typed-output-boundary"]
Completes: ["closed-presentation-tree", "structured-help-and-prompts"]

### Task 3.1: Make help specifications structured model data
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/main.go", "cmd/awf/dispatch.go", "cmd/awf/help_test.go", "cmd/awf/main_test.go", "cmd/awf/run_test.go", "cmd/awf/testdata/"]
Representative: "Replace pre-rendered `HelpBody` prose with structured usage forms, descriptions, details, positionals, options, examples, and related commands, then lower global and command help through `internal/presentation`."
Edge: "Preserve command ordering, nested help selection, unknown-command and usage classification, gated-command discovery, and one complete stderr diagnostic without Markdown headings, padding, or duplicated returned prose."
Post-check: "`go test ./internal/clispec ./cmd/awf -run 'Test.*(Help|Usage|Dispatch|Main)'` exits zero and `git grep -n 'HelpBody' -- internal/clispec cmd/awf` returns no output."

Before editing, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/presentation ./internal/clispec ./internal/commitpolicy ./cmd/awf` to exit zero
at the Phase 2 commit.

Keep `internal/clispec.Commands` as the single command registry. Do not introduce a second command
framework. Replace `globalHelp`, direct `HelpBody` writes, and prose-only `dispatchErr` with typed
presentation/exit outcomes at the command boundary. A produced failing report must remain distinct
from an operational or usage failure; the boundary selects stdout versus stderr and process exit
without printing one failure twice. Retain handler argument parsing and project/version gating.

### Task 3.2: Convert actionable diagnostics and interactive init prompts
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:closed-presentation-tree", "structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:interactive-presentation"]
Paths: ["internal/presentation/shapes.go", "internal/presentation/prompt.go", "internal/commitpolicy/render.go", "internal/commitpolicy/render_test.go", "internal/project/currentstate.go", "internal/project/currentstate_test.go", "cmd/awf/commitpolicy.go", "cmd/awf/commitpolicy_test.go", "cmd/awf/commitgate.go", "cmd/awf/commitgate_test.go", "cmd/awf/presentation_boundary_test.go", "internal/initspec/initspec.go", "internal/initspec/initspec_test.go", "cmd/awf/init.go", "cmd/awf/init_test.go", "cmd/awf/run_test.go"]
Representative: "Map commit-policy and current-state refusals into `Diagnostic` with condition, state, changed axes, cause, and `Steps`; carry produced-report exit status separately from renderer errors."
Edge: "A partial mutation lists every safety-relevant changed axis before recovery steps; init writes and flushes exactly one validated non-newline `prompt:` tail only after a complete prelude, while collision refusal emits no prompt bytes and performs no mutation."
Post-check: "`go test ./internal/commitpolicy ./internal/project ./internal/initspec ./cmd/awf -run 'Test.*(Commit|Outcome|Prompt|Init)'` exits zero and tests prove renderer failure cannot leak partial presentation bytes."

Introduce Steps, `presentation.Diagnostic`, and the governed prompt operation with these first
consumers. Complete the declared child matrix and add the named `TestPresentationTreeContract` unit
with this exact marker, now that every tree node API has a live production consumer:

```go
// invariant: code-design/presentation-ownership:closed-presentation-tree (TestPresentationTreeContract)
```

Create the named `TestCommandOutputBoundary` proof unit without its invariant marker while the
matching tooling claim remains absent. The post-review lifecycle transaction adds that marker
atomically with its claim.

### Task 3.3: Apply outcome modeling and structured command-spec updates
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:closed-presentation-tree", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:interactive-presentation", "structured-agent-oriented-cli-presentation:claim-backing"]
Paths: ["docs/decisions/structured-agent-oriented-cli-presentation.md", ".awf/topics/parts/code-design/presentation-ownership/current-state.md", ".awf/topics/parts/code-design/outcome-modeling/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/code-design/presentation-ownership.md", "docs/topics/code-design/outcome-modeling.md", "docs/topics/tooling/cli.md", "docs/domains/code-design.md", "docs/domains/tooling.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Add `closed-presentation-tree` with this ADR as Origin and the live `TestPresentationTreeContract` proof, and revise `actionable-outcome-protocol` so independently executable remedies render as central `Steps`."
Edge: "Revise `cli-command-spec-single-source` so structured help model data remains single-source; preserve updated claims' existing Origin, backing mode, and proof markers while appending this ADR to provenance."
Post-check: "`./x render && ./x check` is clean and ADR context reports operations one through six Applied, regardless of event order, while operations seven through twelve remain pending."

Append one middle Applied event listing operations three, five, and six in declaration order, mutate
all three claims in the same transaction, and render all generated output.

### Phase close

Stage the complete Phase 3 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(tooling): add typed output boundary (applies presentation)
```

## Phase 4: Convert effort, worktree, and mutation outcomes

**Execution mode: subagent-driven.**

Advances: ["ordinary-output-contract", "explicit-bypass-contract", "standard-presentation-shapes"]
Completes: ["effort-presentation-contract"]

### Task 4.1: Introduce Mutation through project sync
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["internal/presentation/shapes.go", "internal/project/", "cmd/awf/sync.go", "cmd/awf/run_test.go", "cmd/awf/testdata/"]
Representative: "Make project sync the first whole-capability `Mutation` consumer by mapping its typed backups, changes, pruned paths, ownership notes, completion status, and next actions in `internal/project`, then rendering that shape at the command boundary."
Edge: "Preserve initialize-versus-render behavior, backup/index ownership meaning, added/changed/pruned order, operation failure atomicity, and empty successful completion without retaining eager `fmt` writes or `note:` syntax."
Post-check: "`go test ./internal/project ./cmd/awf -run 'Test.*(Sync|Render|Init|Mutation)'` exits zero; exact sync goldens use the common grammar and no ordinary output write remains in `runSyncPrinting`."

Before editing, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/project ./internal/effort ./internal/worktree ./cmd/awf` to exit zero at the
Phase 3 commit. Introduce `presentation.Mutation` only with this project-sync consumer; Task 4.2 may
then reuse it for effort and worktree mappings in the same green phase.

### Task 4.2: Map effort and managed-worktree results into typed presentations
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:explicit-bypasses"]
Paths: ["internal/effort/", "internal/worktree/", "cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/effort_worktree_test.go", "cmd/awf/presentation_boundary_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "README.md", "templates/docs/working-with-awf.md.tmpl", "docs/working-with-awf.md"]
Representative: "Replace `worktree.Result.String`, semicolon-compressed effort mutations, and command-owned effort detail/list formatting with model-owner mappings into `Mutation`, `Detail`, and the core List tree."
Edge: "Remove JSON only from effort new/list/show, retain byte-exact activity JSON protocol, preserve schema-2 records and all rollback/finish changed-axis diagnostics, and render explicit empty collections as labeled scalar `none`."
Post-check: "`go test ./internal/effort ./internal/worktree ./cmd/awf -run 'Test.*(Effort|Worktree|Activity)'` exits zero; `git grep -n -- '--json' internal/clispec cmd/awf/effort.go cmd/awf/effort_test.go` finds only activity-protocol support; no production `Result.String` remains."

Reuse the already-live `presentation.Mutation` from Task 4.1; do not add Report or Collection yet. Keep semantic status, labels, ordering, identity, notes, changes, and next actions
with the package
that owns each typed result. Do not move resident loading, topology decisions, rollback policy, or
activity selection into `internal/presentation`. Exact tests must cover successful and partial
mutations, restartable finish, no-worktree creation, empty list, detail ordering, protocol bytes,
stream choice, and exit mapping. Update the effort new/list/show entries in `README.md` and the
working-with-awf template in this same phase, run `./x render`, and stage the generated working guide;
retain activity `--json` documentation unchanged.

### Task 4.3: Apply the effort command contract update
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:explicit-bypasses", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["docs/decisions/structured-agent-oriented-cli-presentation.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/tooling/cli.md", "docs/domains/tooling.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Revise `effort-command-contract` to make readable text the only ordinary new/list/show form and retain JSON solely for the activity protocol."
Edge: "Preserve the claim's schema, memory, topology, and attachment semantics plus existing proof markers; append provenance without changing unrelated CLI claims."
Post-check: "`./x render && ./x check` is clean and ADR context reports operations one through seven Applied with operations eight through twelve Remaining."

Append one middle Applied event for only operation seven, mutate its claim, and render generated files.

### Phase close

Stage the complete Phase 4 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(tooling): migrate effort output (applies presentation)
```

## Phase 5: Collect checks, migrations, and reports before rendering

**Execution mode: subagent-driven.**

Advances: ["ordinary-output-contract", "standard-presentation-shapes"]
Completes: ["typed-output-boundary", "collected-report-contract"]

### Task 5.1: Collect check and audit outcomes into categorized reports
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["internal/execution/", "internal/project/check.go", "internal/project/check_test.go", "internal/audit/", "internal/prosegate/", "internal/memorycite/", "cmd/awf/check.go", "cmd/awf/checkrepo.go", "cmd/awf/checkstaged.go", "cmd/awf/prosegate.go", "cmd/awf/memorygate.go", "cmd/awf/audit.go", "cmd/awf/check_test.go", "cmd/awf/checkgroup_test.go", "cmd/awf/checkrepo_test.go", "cmd/awf/prosegate_test.go", "cmd/awf/memorygate_test.go", "cmd/awf/audit_test.go"]
Representative: "Have each check/audit owner return semantic findings and summaries, collect every selected execution step under existing continuation policy, then render one `Report` with deterministic `errors` and `warnings` sections and one fixed-schema record per finding."
Edge: "Retain domain rank tokens `error` and `warn`, remove padding and ordinary command `note:` output, preserve accumulated failures and staged-unavailable semantics, and ensure a produced failing report writes once to stdout without a duplicate stderr error."
Post-check: "`go test ./internal/execution ./internal/project ./internal/audit ./internal/prosegate ./internal/memorycite ./cmd/awf -run 'Test.*(Check|Audit|Prose|Memory|Report)'` exits zero; command report goldens contain no `note:` prefix, alignment padding, or semicolon-compressed findings."

Before editing, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/execution ./internal/project ./internal/migrate ./internal/audit ./cmd/awf` to
exit zero at the Phase 4 commit.

Introduce `presentation.Report` with these first production consumers; do not add Collection yet.
Do not change check selection or continuation policy. Bound execution actions collect typed results
rather than writing presentation bytes eagerly. Render deterministic `errors` then `warnings` categories, preserve source order within each category,
and do not preserve source order across categories. Include all failure evidence in the complete
report before exit mapping.

### Task 5.2: Replace migration writers with typed change collection
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary"]
Paths: ["internal/migrate/", "internal/upgrade/", "cmd/awf/upgrade.go", "cmd/awf/upgrade_test.go"]
Representative: "Replace migration-facing `io.Writer` progress prose with a typed ordered change sink returned to the upgrade owner, then render one `Mutation` only after the migration transaction reaches its committed terminal state."
Edge: "On failure, emit one diagnostic that lists proven changed axes and recovery steps without leaking partial success presentation; preserve migration ordering, journal durability, recovery behavior, refusal gates, and rollback semantics."
Post-check: "`go test ./internal/migrate ./internal/upgrade ./cmd/awf -run 'Test.*(Migrate|Upgrade|Recover|Journal)'` exits zero and an AST/search assertion finds no migration operation accepting an `io.Writer` solely for user-visible progress."

Update every migration producer and test in the same batch; do not retain a compatibility writer or
print while mutating. Internal journal persistence is data, not presentation, and remains unchanged.

### Task 5.3: Apply the severity and repository-check ordering clarifications
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["docs/decisions/structured-agent-oriented-cli-presentation.md", ".awf/topics/parts/tooling/audit-commands/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/tooling/audit-commands.md", "docs/topics/tooling/cli.md", "docs/domains/tooling.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "changelog/CHANGELOG.md"]
Representative: "Revise `severity-single-spelling` to retain rank tokens `error` and `warn` while explicitly permitting only the readable presentation section labels `errors` and `warnings`; revise `repo-check-capability-plan` and its changelog description to require deterministic `errors` then `warnings` categories with source order preserved within each category rather than across categories."
Edge: "Retain existing proof markers and their unit names, preserve each claim's Origin and prior provenance, do not rename serialized rank tokens, and do not change repository-check capability selection, continuation, preparation-failure, or error semantics."
Post-check: "`./x render && ./x check` is clean and ADR context reports operations one through nine Applied with only the final three CLI claims Remaining."

Append one middle Applied event for operations eight and nine, mutate both claims and the matching
changelog description in the same transaction, and render generated files.

### Phase close

Stage the complete Phase 5 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(tooling): collect report output (applies presentation)
```

## Phase 6: Convert the remaining ordinary command surfaces

**Execution mode: subagent-driven.**

Advances: ["explicit-bypass-contract"]
Completes: ["deterministic-adoption-gate", "ordinary-output-contract", "repository-consumer-contract", "standard-presentation-shapes"]

### Task 6.1: Convert low-volume mutations, details, and collections
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:standard-result-shapes", "structured-agent-oriented-cli-presentation:semantic-mapping-ownership", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:interactive-presentation", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["internal/presentation/shapes.go", "internal/project/", "internal/config/", "internal/adr/", "cmd/awf/list_add.go", "cmd/awf/init.go", "cmd/awf/new.go", "cmd/awf/adr.go", "cmd/awf/uninstall.go", "cmd/awf/config.go", "internal/project/configreference_print.go", "cmd/awf/changelog.go", "glob:cmd/awf/*_test.go", "cmd/awf/testdata/"]
Representative: "Map enable/disable, init, new, ADR numbering, uninstall, and config-reference results in their model-owning packages into `Mutation`, `Detail`, or `Collection`, with one stable label or record per semantic fact."
Edge: "Preserve init descriptor JSON and selected changelog bytes as isolated protocols/payloads; keep a no-release changelog diagnostic ordinary; forbid Markdown headings, tables, alignment padding, and semicolon-compressed result lines."
Post-check: "`go test ./internal/project ./internal/config ./internal/adr ./cmd/awf` exits zero and exact command goldens cover success, emptiness, refusal, partial mutation, and renderer failure; record every remaining direct-output candidate for the structural detector in Task 6.3 rather than manually classifying it as accepted."

Before editing, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/... ./cmd/awf` to exit zero at the Phase 5 commit.

Introduce `presentation.Collection` with its first config-reference and list consumers. Extract or
retain closed bypass functions named `runReadPlan`, `writeChangelogPayload`,
`writeEffortActivityProtocol`, `writeInitDescriptorProtocol`, and `contextdelivery.Deliver`; the
Phase 6 direct-output search permits only those symbols and fails without manual classification on
any other direct writer.

Treat `internal/render` as the existing project-template engine and leave it independent; do not use it
for CLI presentation. Keep semantic mapping beside typed result owners rather than centralizing
command meanings in `internal/presentation` or a universal command result map.

### Task 6.2: Move repository consumers and active documentation with the labels
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:explicit-bypasses", "structured-agent-oriented-cli-presentation:enforced-adoption"]
Paths: ["README.md", "changelog/CHANGELOG.md", ".awf/docs/", ".awf/parts/", ".awf/skills/", "templates/docs/", "templates/skills/", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go", ".awf/awf.lock", "AGENTS.md", "docs/", ".pi/", ".claude/"]
Representative: "Update the remaining active command/help guidance for Phase 6 mutations, details, collections, payloads, and protocols from tables or Markdown-like examples to the new readable contract."
Edge: "Preserve the exact context spill notice, effort activity JSON, init descriptor JSON, plan projection, and changelog payload descriptions; update authoring sources before `./x render`, never hand-edit generated output, and do not rewrite terminal ADR or historical plan prose."
Post-check: "After `./x render`, `./x check` is clean; `git grep -n -E -- 'effort (new|list|show).*--json|topic.*--json' README.md .awf templates AGENTS.md docs/working-with-awf.md docs/config-reference.md .pi .claude` returns no output, while exact activity/protocol documentation remains."

Add an `[Unreleased]` changelog entry for the readable default, removed convenience JSON modes, and
retained protocols/payloads. Update config reference and working guidance only where behavior or
examples changed. Render generated outputs and inspect representative root, Pi, and Claude surfaces.

### Task 6.3: Add a structural ordinary-output gate
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:enforced-adoption", "structured-agent-oriented-cli-presentation:claim-backing"]
Paths: ["cmd/awf/presentation_boundary_test.go", "cmd/awf/testdata/presentation-boundary/", "internal/presentation/presentation_test.go"]

Build the structural test with `go/packages` and AST inspection, following the repository's existing
context-boundary detector pattern. Scan production `cmd/awf` plus model-owner mapping packages and
reject direct formatted ordinary output, ad hoc builders, padded formatting, Markdown headings, raw
presentation strings, and renderer implementations outside `internal/presentation`. The allowlist is
closed to `runReadPlan`, `writeChangelogPayload`, `writeEffortActivityProtocol`,
`writeInitDescriptorProtocol`, and `contextdelivery.Deliver`; template rendering in
`internal/render` and non-user-facing serialization are not presentation bypasses. Match symbols and
call paths, not merely file names.

Use negative fixtures to prove every detector branch and a positive fixture for each allowlisted
bypass. Keep diagnostics deterministic and path/line attributed. Create the named
`TestOrdinaryCommandOutputUsesPresentation` and `TestExplicitOutputBypasses` proof units without
invariant markers because their claims are still absent. The post-review flip adds all three pending
markers, including the marker for the earlier live `TestCommandOutputBoundary` unit, atomically with
their current-state claims. Run:

```sh
go test ./internal/presentation ./cmd/awf -run 'Test(PresentationTreeContract|OrdinaryCommandOutputUsesPresentation|CommandOutputBoundary|ExplicitOutputBypasses)'
```

The command must succeed, exercise positive and negative branches, and report zero production
findings. This is the Phase 6 executable closure check; no later-phase allowlist is required.

### Phase close

Stage the complete Phase 6 transaction explicitly, including generated output and repository
consumer changes and the structural detector. Run the focused structural proof command from Task 6.3,
then `awf check staged` and `./x gate`; all must pass. Create one commit:

```commit
feat(tooling): migrate ordinary CLI presentation
```

## Phase 7: Pin exhaustive interface and bypass coverage

**Execution mode: subagent-driven.**

Completes: ["exhaustive-contract-coverage", "explicit-bypass-contract"]

### Task 7.1: Pin exact streams, exits, public labels, and protocol bytes
Kind: batch
Latitude: exact
Applying: ["structured-agent-oriented-cli-presentation:readable-text-contract", "structured-agent-oriented-cli-presentation:typed-command-boundary", "structured-agent-oriented-cli-presentation:explicit-bypasses", "structured-agent-oriented-cli-presentation:enforced-adoption", "structured-agent-oriented-cli-presentation:claim-backing"]
Paths: ["glob:cmd/awf/*_test.go", "cmd/awf/testdata/", "internal/presentation/", "internal/contextq/", "internal/commitpolicy/", "internal/project/", "internal/audit/", "internal/effort/", "internal/worktree/", "internal/migrate/", "cmd/contextspilllog/", ".github/workflows/release.yml", "x"]
Representative: "For each ordinary top-level command family, assert a complete representative success or produced-report presentation plus usage/operational failure stream and exit behavior, with stable labels, semantic category order, record schemas, escaping, and exactly one terminal newline."
Edge: "Assert byte-for-byte authored plan/changelog payloads and spill/activity/descriptor protocols, prompt non-newline flush, renderer atomicity, no duplicate stderr after a produced failing report, no convenience JSON option, and contract-aware `x` and release consumers."
Post-check: "`go test ./internal/... ./cmd/awf ./cmd/contextspilllog` exits zero; the four named proof units run and pass; `./x render && ./x check` is a no-op and clean; the structural gate reports zero production findings."

Before editing, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/... ./cmd/awf ./cmd/contextspilllog` to exit zero at the Phase 6 commit.

Use checked-in golden data for long public output and direct literals for short contracts. Do not add
an update-golden command. Assert methods and terminal states rather than freezing corpus counts.

### Phase close

Stage the complete Phase 7 transaction explicitly. Verify no ADR history/status, current-state claim,
or plan status change is staged: the final three operations remain pending until terminal review.
Run `git diff --check`, focused proof tests, `awf check staged`, and `./x gate`; all must pass. Create
one commit:

```commit
test(tooling): enforce CLI presentation adoption
```

## Definition of done

- `dod: presentation-core` The standard-library-only package exposes only the live Document/Field core, validates values and labels, renders atomically, and serves labeled version output plus its repository consumers.
- `dod: closed-presentation-tree` The core grows only with live consumers into the complete bounded Document/Field/Section/List/RecordGroup/Record/Steps tree, exact child matrix, escaping, depth, spacing, and atomic-rendering contract with its named proof.
- `dod: standard-presentation-shapes` Detail, Diagnostic, Mutation, Report, and Collection each land with their first whole-capability consumer, remain presentation-only vocabulary, and lower through the same closed tree.
- `dod: context-presentation-boundary` Context and topic owners map semantics into the common tree, topic convenience JSON is absent, and context spill plus plan projection bytes remain isolated and exact.
- `dod: typed-output-boundary` Produced reports, usage failures, operational failures, partial mutations, stdout/stderr selection, and exit status are typed and never double-rendered.
- `dod: structured-help-and-prompts` Help is structured model data lowered through the common tree, and interactive init uses the sole validated flushed non-newline prompt mode.
- `dod: effort-presentation-contract` Effort and worktree text uses readable typed presentations, ordinary effort JSON modes are removed, activity JSON remains exact, and resident/topology behavior is unchanged.
- `dod: collected-report-contract` Checks, audits, and migrations collect complete typed outcomes before one presentation write while retaining continuation, journal, rollback, and severity semantics.
- `dod: ordinary-output-contract` Every ordinary command result, help surface, advisory, refusal, progress replacement, and partial outcome uses deterministic labeled text with stable grouping, schemas, order, escaping, and newline behavior.
- `dod: repository-consumer-contract` Active docs, help, runners, release automation, generated targets, and adopters consume declared labels or named protocols rather than incidental whitespace or legacy prefixes.
- `dod: explicit-bypass-contract` Only plan/changelog payloads and effort-activity/init-descriptor/context-spill protocols bypass the tree, and each remains byte-for-byte tested without mixed presentation text.
- `dod: deterministic-adoption-gate` AST enforcement with a closed symbol allowlist rejects any ungoverned ordinary output and reaches zero production findings before Phase 6 closes.
- `dod: exhaustive-contract-coverage` Exact-output fixtures, the four named proof units, render/check, and the full gate pin public streams, exits, labels, protocol bytes, and bypass isolation.

## Notes

- Phase 5 recorded a bounded enabling deviation: although Task 5.2's exact Paths omit
  `internal/project`, upgrade's terminal sync already depends on `internal/project.SyncReport`, and
  truthful partial upgrade diagnostics required its actual sync write and mode axes. This does not
  broaden sync business behavior.
- ADR application partition follows complete live implementation rather than declaration position:
  Phase 1 moves the ADR directly from Proposed to Implementing and applies operations 1-2; Phase 2
  applies operation 4; Phase 3 applies operations 3, 5, and 6 in declaration order within its batch;
  Phase 4 applies operation 7; and Phase 5 applies operations 8 and 9, with operation 9 carrying the
  repository-check claim and changelog correction in the same atomic application transaction. Phases
  6-7 complete implementation and proofs without claiming whole-interface authority early.
- After settled terminal implementation review, the deferred flip transaction adds
  `tooling/cli:readable-text-output`, `tooling/cli:typed-command-output-boundary`, and
  `tooling/cli:explicit-output-bypasses` in their `.awf/topics/parts/tooling/cli/current-state.md`
  declaration order and adds the three exact invariant markers to the already-live
  `TestOrdinaryCommandOutputUsesPresentation`, `TestCommandOutputBoundary`, and
  `TestExplicitOutputBypasses` units. It appends one final Applied event for
  operations 10-12, then the Implemented status event, flips this plan to `Implemented`, runs
  `./x render`, stages generated topic/domain/index/lock output, and passes `awf check staged` plus
  `./x gate` before the final lifecycle commit.
- If execution discovers a new ordinary-output bypass, alternate renderer, raw node, unbounded tree,
  domain field in `internal/presentation`, or command-result framework, stop for ADR amendment and
  renewed ADR review. A newly necessary machine protocol is not an implementation detail.
- Phase 1 execution waits for the parallel Sundial-removal effort to integrate and for this branch to
  rebase onto that main. This plan never changes `examples/sundial`, its runner, its current-state
  claims, or its backing tests; the removal effort owns those retirements.
- No change to business continuation policy, effort schema/topology, migration ordering/journal
  semantics, context spill framing, plan/changelog payload content, or audit rank tokens is in scope.
