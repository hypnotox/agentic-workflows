---
format: plan-v2
date: 2026-08-05
adrs: [add-associated-pi-effort-memory-tools]
status: Proposed
---
# Plan: Associated Pi Effort Memory Tools

## Goal

Give an explicitly associated Pi session pathless, dynamically active tools for bounded memory reads,
exact body edits, and metadata updates while preserving binary-owned resident safety, ordinary human
presentation, direct generic-file access, and the advisory one-writer association boundary.

## Architecture summary

The Go effort service owns memory parsing, owner comparison, exact matching, timestamps, atomic
publication, and typed results. `cmd/awf` composes those results into ordinary presentation or the
closed owner-scoped protocol. The generated Pi client owns bounded process transport and strict
protocol decoding; the generated effort index owns association state, active-tool changes,
per-process serialization, Pi's shared file-mutation queue, and active-only guidance. Implementation
lands in three independently green transactions: binary memory semantics with their first CLI
production consumer, generated client transport with its first associated-tool consumer and runtime
floor, then rendered authority and remaining documentation closure.

## Phase 1: Add binary-owned memory operations and their CLI consumer

**Execution mode: subagent-driven.**

Completes: ["binary-memory-contract"]

### Task 1.1: Define typed memory operations and owner-scoped resident inspection
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:binary-owned-memory-operations", "add-associated-pi-effort-memory-tools:advisory-owner-scoped-memory-calls", "add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["internal/effort/memory_operations.go", "internal/effort/memory_metadata.go", "internal/effort/service.go", "internal/effort/activity.go", "internal/effort/store.go", "internal/effort/types.go"]

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/effort ./internal/clispec ./cmd/awf` to exit zero in the managed worktree. The
phase owner retains the ADR, current-state, shared CLI grammar, presentation-boundary fixtures,
render, staging, gate, and single commit transaction; helpers are report-only or commit-disabled.

Add effort-owned input, success, range, diff, and refusal types for read, edit, and update. Keep the
protocol-neutral model closed over the ADR's condition set and facts: memory metadata,
`changedMemory`, recovery actions, optional mechanism cause, edit indexes and occurrence facts,
overlap indexes, and result-size facts. Do not put JSON tags, command grammar, presentation nodes,
or Pi concepts into matching and storage helpers except the advisory owner UUID input.

Add service entrypoints for bounded complete-document read, exact body edit, and structured update.
An owner-free call skips activity inspection. An owner-scoped call validates the lowercase UUIDv4,
loads the proven effort resident, then reads the safe protocol-v2 activity resident directly and
compares its owner without parsing mutable memory metadata. Map absent activity, unsafe activity,
and owner mismatch to distinct typed handled refusals. Do not turn activity into authorization,
hold a cross-process lock, or add a heartbeat precondition.

Reuse `readRegularNoFollowBounded`, `inspectMemory`, `encodeMemory`, and `store.replaceMemory` rather
than adding a second resident parser or publisher. Read pagination is one-indexed across the complete
canonical or legacy document, honors optional positive offset and limit without integer overflow,
then caps selected output at 2,000 complete lines or 50 KiB with exact `startLine`, `endLine`,
`totalLines`, `nextOffset`, and `truncatedBy` facts. Stop before a line that would cross the byte cap;
if the requested first line alone exceeds it, return handled `result-too-large` with its byte size and
51,200-byte maximum. Never split a line or advance continuation past unreturned content. Preserve the
one-MiB resident bound and safe regular-file behavior.

For edit, accept 1 through 128 replacements, bound each decoded string by one MiB, require nonempty
`oldText`, and evaluate every exact match against the original Markdown body after the closed
frontmatter boundary. Reject missing, repeated, nested, or overlapping regions before mutation;
report original request indexes; apply all replacements in original-position order or none. Preserve
frontmatter identity and mutable values, set `updated` from `Service.now()`, encode one complete
memory document, and publish through `replaceMemory`. A body no-op still has a null
`firstChangedLine` and follows the ADR's successful edit/timestamp contract. Compute a deterministic
bounded diff fact without importing command presentation into the effort package.

Refactor `UpdateMemory` only as needed to share the proven load/parse/publish path and return the
updated memory fact. Preserve partial repair of invalid mutable fields, exact legacy migration,
immutable identity checks, and existing owner-free behavior. Model publication-stage uncertainty as
`memory-failure`, setting `changedMemory` true only after the atomic replacement boundary and giving
the caller a read-first recovery action.

### Task 1.2: Prove matching, pagination, safety, and durability
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:binary-owned-memory-operations", "add-associated-pi-effort-memory-tools:advisory-owner-scoped-memory-calls", "add-associated-pi-effort-memory-tools:closed-memory-protocol"]
Paths: ["internal/effort/memory_operations_test.go", "internal/effort/memory_test.go", "internal/effort/memory_metadata_test.go", "internal/effort/activity_test.go", "internal/effort/store_test.go", "internal/effort/durability_test.go", "internal/effort/safety_test.go", "internal/effort/types_test.go"]
Representative: "Read a canonical memory through offset and limit, edit two disjoint body regions against the original body, preserve metadata and unrelated body bytes, advance the injected UTC timestamp once, and return exact range, replacement, and diff facts."
Edge: "With invalid mutable metadata and a matching safe activity owner, allow a complete metadata repair; reject no-match, ambiguous, nested, overlapping, unsafe-resident, missing-activity, wrong-owner, oversized-result, and pre/post-rename durability cases without partial edit publication or a false retry instruction."
Post-check: "`go test ./internal/effort` exits zero; its independently literal test population covers every success and refusal condition in the ADR, all pagination truncation reasons, legacy and canonical input, UTF-8 byte bounds, symlink/non-regular residents, original-content batch evaluation, publication faults before and after replacement, and both expanded effort-management invariant claims through named proof markers."

Use existing test service composition and injected fault stages. Assert typed facts and resident bytes,
not command prose. Cover offsets beyond the available document, `math.MaxInt` limit without overflow,
final lines without a newline, whole-line continuation before the 50-KiB cap, individually unpageable
line refusal, 2,000-line boundaries, repeated replacement text, replacements that create another old
text, no-op replacements, CRLF body preservation where admitted, one-MiB result rejection, and
stable index ordering. Pin independent literal acceptance/rejection pairs for the 50-KiB read and
diff caps, one-MiB bounds, and 128-edit maximum so changing a production constant fails a test. Add
correctly anchored proof markers on named new units that exercise the new clauses of both Applied
effort-management claims. Keep genuine kernel-impossible paths under the existing coverage-ignore
policy rather than weakening the 100% gate.

### Task 1.3: Add exact memory command grammar and dispatch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:presentation-protocol-composition", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary", "add-associated-pi-effort-memory-tools:compatible-runtime-floor"]
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/gate_test.go", "cmd/awf/help_test.go", "cmd/awf/testdata/help/global.txt"]

Keep command composition in this same phase transaction so every new effort export ships with its
first real outside-package production consumer.

Declare exactly the owner-free and owner-scoped `memory read`, `memory edit`, and `memory update`
forms from the ADR. Keep `--owner` and `--json` nonrepeatable and mutually required, accept no
unlisted flag, require one canonical 1-63-byte slug, validate positive read bounds, require an update
field, and reserve stdin solely for the edit request. Grammar failures occur before effort
composition, write empty stdout, use bounded actionable stderr, and exit nonzero.

Decode one closed edit JSON object from stdin with no additional properties, 1 through 128 closed
edit objects, per-string bounds, and a 16-MiB total request cap. Reject trailing JSON values and
malformed or oversized input before managed state is observed. Dispatch all forms through the typed
effort service. Owner-free forms map success and handled refusal models through effort-owned semantic
mappings and `internal/presentation`; owner-scoped forms select the protocol writer. Do not add a
convenience JSON mode, local Markdown renderer, generic map, or new process-exit seam.

Update command spec and help fixtures from the same table. Extend gate probes so every new leaf is
version-gated with valid nonmutating or disposable arguments and stdin where required.

### Task 1.4: Map ordinary presentation and exact protocol envelopes
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:presentation-protocol-composition", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["internal/effort/presentation.go", "internal/effort/presentation_test.go", "cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/presentation_boundary_test.go", "cmd/awf/testdata/presentation-boundary/positive-memory.go"]
Representative: "Render owner-free read, edit, update, and handled refusals through effort-owned semantic mappings and the central presentation tree; encode the corresponding owner-scoped success or refusal as exactly one newline-terminated protocol-1 envelope."
Edge: "A malformed request, pre-observation failure, encoder failure, or output-bound failure exits nonzero with empty stdout; a handled refusal exits zero; only `memory-failure` may carry `outcome.cause`, and `changedMemory` is true only for post-replacement uncertainty."
Post-check: "`go test ./internal/effort ./cmd/awf -run 'Test.*(Memory|Presentation|OutputBypass)'` exits zero; byte-exact tests admit the named memory protocol writer as a required bypass and reject direct successful output from every other new memory command path."

Implement the literal protocol-1 schemas and bounds from the ADR, including condition-specific extra
facts and exact omission rules. Ensure success stdout never exceeds one MiB and stderr never exceeds
50 KiB. Preserve writer failures through the shared typed command boundary. For human reads, make
truncation and continuation visible without altering the selected content bytes; for edits, expose
replacement and diff facts through existing presentation shapes rather than reproducing Pi's TUI.

### Task 1.5: Enter Implementing and apply effort and CLI authority
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:binary-owned-memory-operations", "add-associated-pi-effort-memory-tools:advisory-owner-scoped-memory-calls", "add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:presentation-protocol-composition", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["docs/decisions/add-associated-pi-effort-memory-tools.md", ".awf/topics/parts/tooling/effort-management/current-state.md", "docs/topics/tooling/effort-management.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/tooling/cli.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Move the ADR from Proposed to Implementing and apply `effort-record-authority`, `memory-skeleton-purpose-partition`, `explicit-output-bypasses`, and `effort-command-contract` in the same authored transaction as their live binary and CLI behavior, preserving each existing Origin and Revised-by prefix before appending this ADR."
Edge: "Keep effort activity JSON, authored plan and changelog bytes, init descriptor JSON, and context spill unchanged; do not classify memory JSON as optional convenience output, describe activity as authorization or locking, or apply rendering claims."
Post-check: "After `./x render`, `./x check` exits zero and `./awf context --show pending docs/decisions/add-associated-pi-effort-memory-tools.md cmd/awf/effort.go` reports exactly the two effort-management and two CLI operations Applied while all rendering operations remain pending."

Use `awf-adr-lifecycle` to append the Implementing status event and one Applied event naming all four
qualified claim operations. Update both source current-state parts with the live memory, advisory
owner, ordinary presentation, and required protocol boundaries in the same transaction. Never
hand-edit rendered topics, the decision index, or the lock.

### Phase close

Stage the complete Phase 1 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(tooling): add memory API and protocol (applies memory tools batch)
```

## Phase 2: Add the generated client and associated tools

**Execution mode: subagent-driven.**

Completes: ["associated-tool-contract"]

### Task 2.1: Decode every memory protocol reply strictly
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["templates/pi/awf-effort/client.ts.tmpl", ".pi/extensions/awf-effort/client.ts", "tools/pi-extension-test/tests/using-effort.test.ts", ".awf/awf.lock"]
Representative: "Decode one newline-terminated protocol-1 read, edit, or update success into recursively frozen exact facts, validating slug, UTC-or-sentinel time, integer domains, range consistency, diff bounds, and every closed property set."
Edge: "Reject unknown conditions, extra or missing properties, invalid condition-specific refusal facts, forbidden causes, mismatched effort identities, invalid timestamps, nonintegral numbers, oversized content/diff/stdout/stderr, multiple envelopes, and nonzero child exit."
Post-check: "After `./x render`, `./x check` exits zero and `./x pi-test run` exits zero with the client acceptance matrix covering every success and refusal shape plus one rejection for every closed-schema and bound rule."

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero,
`go test ./...` to exit zero, and `./x pi-test run` to exit zero. The phase owner owns the client
template, rendered output, shared TypeScript test file, render, and the single commit.

Extend the existing activity client without weakening protocol-v2 validation. Keep activity and memory
reply types separate where their mutation axes differ. Export only the typed invocations and facts
needed by the generated index and tests. Preserve the canonical slug, UUIDv4, UTC, text, stdout, and
stderr validators as single homes.

### Task 2.2: Add bounded stdin-capable edit invocation
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["templates/pi/awf-effort/client.ts.tmpl", ".pi/extensions/awf-effort/client.ts", "tools/pi-extension-test/tests/using-effort.test.ts", ".awf/awf.lock"]

Add a client-owned child-process dependency that can write the closed edit request to stdin while
capturing bounded stdout and stderr. The production adapter uses a direct Node child process, the
repository-root `./awf`, a 15-second timeout, and the tool abort signal; cancellation and timeout
terminate the process tree, close stdin, and return an uncertainty message that directs a read
instead of blind retry when completion is unknown. Do not use a temporary request file, shell
interpolation, or command-line JSON.

Read and update may reuse the existing `pi.exec` adapter; edit must use the stdin-capable adapter.
All three invocations inject the current associated slug and owner, select `--json`, preserve exact
argument order in tests, and never accept a caller path or slug. Test spawn failure, stdin failure,
abort before and during execution, timeout, signal/exit races, nonzero exit, and output truncation.
Assert that every handle/listener/timer settles on each terminal path.

### Task 2.3: Guard the companion runtime before functional registration
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:compatible-runtime-floor", "add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:publication-safe-memory-tool-templates"]
Paths: ["templates/partials/pi-minimum-runtime.md", "templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-context-usage/index.ts", ".pi/extensions/awf-effort/index.ts", ".pi/extensions/awf-handoff/index.ts", ".pi/extensions/awf-subagents/index.ts", "internal/project/target_test.go", "tools/pi-extension-test/package.json", "tools/pi-extension-test/package-lock.json", "tools/pi-extension-test/tests/context-usage.test.ts", "tools/pi-extension-test/tests/handoff.test.ts", "tools/pi-extension-test/tests/index.test.ts", "tools/pi-extension-test/tests/runtime.test.ts", "tools/pi-extension-test/tests/using-effort.test.ts", ".awf/awf.lock"]

Verify that the exact pinned coding-agent artifact remains
`https://github.com/hypnotox/pi/releases/download/fork-v0.81.1-awf.3/pi-coding-agent-fork-v0.81.1-awf.3.tgz`
with embedded version 0.81.1 and lock integrity
`sha512-Xk34jkheEgNwBPMfT00+jmhY3YHcMkq5xL3C+a1Cr9yR0hsN76J5am6RJkZVQSxwAdHS2GKgzREElp0awve/sQ==`.
Its published declarations already export `getActiveTools`, `setActiveTools`, tool
`promptGuidelines`, and `withFileMutationQueue`; prove those exports through the real pinned package
rather than substituting a private queue, a persistent prompt injection, or a newer unapproved
artifact.

Retain the numeric Pi floor at 0.81.1, the earliest approved pinned artifact with every existing and
newly required API, while extending the closed minimum-runtime feature vocabulary with
`getActiveTools`, `setActiveTools`, tool `promptGuidelines`, and `withFileMutationQueue`. Make the effort entrypoint consume the same one-notice guard before
registering association hooks or tools. Resolve the newly required queue helper in a way that lets an
older or incomplete runtime reach the actionable compatibility notice instead of failing at static
module import. Do not require `changeCwd`, Remote Pi events, or unrelated UI APIs.

Extend the injected `PiLike`/dependency surface only with the exact questions used by the effort
index: `getActiveTools`, `setActiveTools`, package version, and the file-mutation queue function.
Supported runtime operation must emit no compatibility warning. Add renderer and TypeScript tests
for old version, missing methods/helper, exactly one notice across entrypoints, no partial tool/hook
registration, supported registration, and publication-safe empty-variable rendering.

### Task 2.4: Register strict pathless tools and active-only guidance
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:associated-memory-tool-surface", "add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "tools/pi-extension-test/tests/using-effort.test.ts", ".awf/awf.lock"]

Register `effort_memory_read`, `effort_memory_edit`, and `effort_memory_update` with closed TypeBox
schemas exactly matching the ADR. Give each an explicit `promptSnippet` and nonduplicative
`promptGuidelines` that name the tool, prefer pathless associated-memory use, separate body edits
from metadata updates, and state that timestamps are automatic. Do not forbid or intercept built-in
file tools and do not restate checkpoint eligibility.

At factory/session start, keep all three registered but inactive by filtering only their names from
the current active set. A successful attach adds all three to the then-current active set with set
semantics and preserves unrelated tools. Explicit detach, restart, shutdown, missing/unsafe
activity, or owner loss removes only these names and clears local association/publication as the ADR
requires. Invalid memory and transport degradation do not falsely transfer ownership. Dynamic
changes must rebuild Pi's native prompt metadata rather than injecting a persistent guidance
message.

Every tool operation runs through the existing association `serial` chain and snapshots the current
slug/owner inside that chain. A read invokes the client directly. Edit and update wrap the entire
binary read-modify-write window in Pi's exported `withFileMutationQueue` using the absolute primary
memory path derived from `ctx.cwd` and the associated slug. On `not-owner`, `missing`, or
`unsafe-activity`, clear association and deactivate tools before returning the explanatory result.
Strictly validate the decoded reply and throw tool errors for malformed transport rather than
inventing a local fallback.

Return concise model-visible content plus typed details for reads, diff results, updates, and
refusals. Test detached schemas and inactive state, attach/detach cycles, unrelated-tool
preservation, repeated attach, switching efforts, restart/shutdown, heartbeat ownership loss,
operation-triggered ownership loss, same-turn parallel calls, association-chain ordering, queue-key
path and queue ordering against a simulated generic mutation, cancellation, no-op edit, and direct
file-tool availability.

### Task 2.5: Extend the generated using-effort guidance
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:publication-safe-memory-tool-templates"]
Paths: ["templates/skills/using-effort/SKILL.md.tmpl", ".pi/skills/awf-using-effort/SKILL.md", "internal/project/target_test.go", ".awf/awf.lock"]

Add concise guidance that an attached Pi session prefers `effort_memory_read`, uses
`effort_memory_edit` only for the Markdown body, uses `effort_memory_update` for phase/next, and
relies on automatic timestamps. Preserve the direct generic-file escape hatch, repository-root
execution, exact owned path, managed-worktree path, explicit association, advisory activity, and
restart-detached rules. Ensure unset prefix and generic adopter rendering remain coherent with no
unresolved token.

### Task 2.6: Update the runtime-floor and testing-tier documentation
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:compatible-runtime-floor", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: [".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/testing/layout.md", ".awf/docs/parts/testing/tiers.md", ".awf/docs/parts/development/dependencies.md", ".awf/docs/parts/releasing/content.md", "templates/docs/working-with-awf.md.tmpl", "README.md", "docs/architecture.md", "docs/testing.md", "docs/development.md", "docs/releasing.md", "docs/working-with-awf.md", ".awf/awf.lock"]
Representative: "Document the companion's active-tool and shared file-queue dependencies, retained exact fork-v0.81.1-awf.3 artifact and numeric 0.81.1 floor, expanded required-API feature floor, and the Go, TypeScript, render, coverage, and real-runtime test tiers that prove the generated client and first index consumer together."
Edge: "Keep Pi packages adopter-supplied rather than binary dependencies, preserve the exact fork URL and integrity, retain the one-notice compatibility refusal, remove the stale no-version-floor claim, and do not claim that unit stubs replace the pinned real-runtime smoke."
Post-check: "After `./x render`, `./x check` exits zero; README and every generated architecture, testing, development, releasing, and working-with-awf document consistently name the retained 0.81.1 fork floor, expanded required APIs, and test tiers exercised by the same runtime phase."

Update every listed authoring source and render its generated document in the same transaction that
extends the runtime feature floor and introduces its first functional consumer. Read back every
source and output; never hand-edit generated documents or the lock.

### Task 2.7: Apply the runtime-floor authority with its live behavior and docs
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:compatible-runtime-floor"]
Paths: ["docs/decisions/add-associated-pi-effort-memory-tools.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", "docs/topics/rendering/pi-runtime.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Apply `pi-minimum-runtime` in the same authored transaction as the guarded active-tool/file-queue feature floor and its complete durable documentation set, preserving the claim's Origin and Revised-by prefix before appending this ADR."
Edge: "Apply no other rendering claim; leave `pi-extension-target-render`, both `pi-workflows` updates, and the new backed invariant Remaining for Phase 3."
Post-check: "After `./x render`, `./x check` exits zero and `./awf context --show pending docs/decisions/add-associated-pi-effort-memory-tools.md templates/partials/pi-minimum-runtime.md` reports `pi-minimum-runtime` Applied while the other rendering operations remain pending."

Append one Applied event for the runtime-floor operation and mutate its source claim in the same
transaction. Never hand-edit the rendered topic, decision index, or lock.

### Phase close

Stage the complete Phase 2 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(rendering): activate memory tools (applies memory tools batch)
```

## Phase 3: Apply rendering authority and close generated documentation

**Execution mode: subagent-driven.**

Completes: ["authority-and-generated-state", "repository-gate"]

### Task 3.1: Back the rendered Pi memory-tool contract
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:memory-tool-claim-backing", "add-associated-pi-effort-memory-tools:publication-safe-memory-tool-templates"]
Paths: ["internal/project/target_test.go", "internal/project/spine_test.go", "tools/pi-extension-test/container.sh", "tools/pi-extension-test/tests/using-effort.test.ts", "tools/pi-extension-test/tests/runtime.test.ts"]
Representative: "Add `TestPiEffortMemoryToolContract` with the exact invariant proof marker and prove Pi-only derivation, all three strict tool schemas, detached inactivity, active-only guidance, runtime guard, client/index rendering, and using-effort companion text from selected `effort-workflow`."
Edge: "A target without Pi, or Pi without selected `effort-workflow`, renders none of the memory tools or companion artifacts; disable/prune and generic empty-variable render paths leave no stale output or unresolved token."
Post-check: "`go test ./internal/project -run 'TestPi(EffortMemoryToolContract|RuntimeTargetRender|MinimumRuntime)'` and `./x pi-test run` exit zero before the claim-creation task runs proof coverage."

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero,
`go test ./...` to exit zero, and `./x pi-test run` to exit zero. This phase owns every remaining
State-change operation, proof marker, generated output, remaining documentation update, and the final
implementation commit; it does not perform the later terminal-review status flip.

Keep the proof in a current-state test glob and name `TestPiEffortMemoryToolContract` verbatim on a
non-marker line. Extend container coverage inputs only as needed to retain 100% TypeScript statement
coverage; do not exclude reachable branches.

### Task 3.2: Apply all remaining rendering claims
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:associated-memory-tool-surface", "add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:memory-tool-claim-backing", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["docs/decisions/add-associated-pi-effort-memory-tools.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", "docs/topics/rendering/pi-runtime.md", "docs/topics/rendering/pi-workflows.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Apply `pi-extension-target-render`, both existing `pi-workflows` updates, and the new backed `pi-effort-memory-tools` invariant in one final distinct-claim batch, preserving existing provenance before appending this ADR and giving the new invariant this ADR as Origin."
Edge: "Keep the ADR Implementing after every operation is Applied so settled implementation review owns the later status-only Implemented flip; do not add authority for direct-file prohibition, activity locking, automatic checkpointing, or non-Pi rendering."
Post-check: "After `./x render`, `./x check` exits zero; `./awf context --show pending docs/decisions/add-associated-pi-effort-memory-tools.md` reports every declared operation Applied and none Remaining; `./awf topic rendering/pi-workflows --coverage` then finds the proof marker on `TestPiEffortMemoryToolContract` with no stale or duplicate marker."

Append the final Applied event and mutate all four remaining source claims in the same authored
transaction. Update `pi-extension-target-render` with the client/index responsibilities,
`pi-effort-session-association` and `using-effort-skill` with dynamic activation and guidance, and
add the exact backed invariant
`pi-effort-memory-tools`. Never hand-edit generated topics, index, or lock.

### Task 3.3: Update durable architecture and workflow prose
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:cohesive-runtime-boundary", "add-associated-pi-effort-memory-tools:active-memory-guidance"]
Paths: [".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/parts/working-with-awf/config-and-overrides.md", "docs/architecture.md", "docs/working-with-awf.md", ".awf/awf.lock"]
Representative: "Document the binary-to-command-to-generated-client-to-associated-tool flow, the shared presentation versus required-protocol split, and the active associated-memory workflow boundary."
Edge: "Describe memory tools as preferred conveniences, not workflow authority; retain direct file access, one-writer policy, advisory activity, primary memory ownership, managed-worktree execution, and Pi-only derivation."
Post-check: "After `./x render`, `./x check` exits zero; generated durable docs contain the new boundary and no stale claim that activity JSON is the sole effort protocol bypass."

Update authoring sources and render their generated documents. Keep implementation procedure in this
plan rather than copying it into architecture or workflow prose.

### Phase close

Run `./x render`, read back every source and generated target changed by Tasks 3.2 and 3.3, and stage
the complete Phase 3 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass
with clean drift, current-state validation, 100% Go and TypeScript coverage, dead-code checks, and the
real pinned Pi runtime smoke. Create one commit:

```commit
feat(rendering): back memory tools (applies memory tools batch)
```

## Definition of done

- `dod: binary-memory-contract` Owner-free human and owner-scoped protocol forms implement every read, edit, update, safety, refusal, bound, timestamp, durability, presentation, and exit contract in the ADR through one effort-owned semantic model.
- `dod: associated-tool-contract` The three strict pathless tools are registered but inactive while detached, activate and carry guidance only while associated, serialize mutations through Pi's real-path file queue, and clear only their own association/tool state on every declared loss boundary.
- `dod: authority-and-generated-state` Every State-change operation is Applied with valid provenance, the new invariant resolves to `TestPiEffortMemoryToolContract`, rendered Pi outputs and durable docs match their authoring sources, and the ADR remains Implementing for terminal review.
- `dod: repository-gate` `./x render`, `./x check`, `./x pi-test run`, and `./x gate` all exit zero from a clean managed worktree after the final implementation transaction.

## Notes

- The exact pinned `fork-v0.81.1-awf.3` coding-agent artifact, embedded version 0.81.1, exports
  `pi.getActiveTools()`, `pi.setActiveTools()`, tool `promptGuidelines`, and
  `withFileMutationQueue()`. Phase 2 retains that numeric/artifact floor, extends its guarded feature
  vocabulary, proves the lock-pinned declarations and real runtime, and never replaces either
  primitive with a private analogue.
- The terminal implementation review later owns the plan status flip and the ADR's status-only
  Implemented transition; no implementation phase performs either freeze.
