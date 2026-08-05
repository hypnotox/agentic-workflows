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
lands in five independently green transactions: effort-domain semantics, CLI composition, generated
client transport, associated tool activation, then rendered authority and documentation closure.

## Phase 1: Add binary-owned memory operations

**Execution mode: subagent-driven.**

Advances: ["binary-memory-contract"]

### Task 1.1: Define typed memory operations and owner-scoped resident inspection
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:binary-owned-memory-operations", "add-associated-pi-effort-memory-tools:advisory-owner-scoped-memory-calls", "add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["internal/effort/memory_operations.go", "internal/effort/memory_metadata.go", "internal/effort/service.go", "internal/effort/activity.go", "internal/effort/store.go", "internal/effort/types.go"]

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/effort` to exit zero in the managed worktree. The phase owner retains the ADR,
current-state, render, staging, gate, and commit transaction; helpers are report-only or
commit-disabled.

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
canonical or legacy document, honors optional positive offset and limit, then caps selected output
at 2,000 lines or 50 KiB with exact `startLine`, `endLine`, `totalLines`, `nextOffset`, and
`truncatedBy` facts. Preserve the one-MiB resident bound and safe regular-file behavior.

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
Post-check: "`go test ./internal/effort` exits zero; its test population covers every success and refusal condition in the ADR, all pagination truncation reasons, legacy and canonical input, UTF-8 byte bounds, symlink/non-regular residents, original-content batch evaluation, and publication faults before and after replacement."

Use existing test service composition and injected fault stages. Assert typed facts and resident bytes,
not command prose. Cover offsets beyond the available document, final lines without a newline,
50-KiB and 2,000-line boundaries, repeated replacement text, replacements that create another old
text, no-op replacements, CRLF body preservation where admitted, one-MiB result rejection, and
stable index ordering. Keep genuine kernel-impossible paths under the existing coverage-ignore
policy rather than weakening the 100% gate.

### Task 1.3: Enter Implementing and apply effort resident authority
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:binary-owned-memory-operations", "add-associated-pi-effort-memory-tools:advisory-owner-scoped-memory-calls", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["docs/decisions/add-associated-pi-effort-memory-tools.md", ".awf/topics/parts/tooling/effort-management/current-state.md", "docs/topics/tooling/effort-management.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Move the ADR from Proposed to Implementing and apply the `effort-record-authority` and `memory-skeleton-purpose-partition` updates in the same authored transaction as their live service behavior, preserving each claim's Origin and Revised-by prefix before appending this ADR."
Edge: "Do not apply CLI or rendering claims, do not describe activity as authorization or locking, and leave every other State-change operation Remaining."
Post-check: "After `./x render`, `./x check` exits zero and `./awf context docs/decisions/add-associated-pi-effort-memory-tools.md` reports exactly the two effort-management operations Applied with the CLI and rendering operations Remaining."

Use `awf-adr-lifecycle` to append the Implementing status event and one Applied event naming the two
qualified claim operations. Update the source current-state part with the live read/edit/update,
repair, owner-check, exact-match, timestamp, and atomic-publication boundaries. Never hand-edit the
rendered topic, decision index, or lock.

### Phase close

Stage the complete Phase 1 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(tooling): add memory operations (applies memory tools batch)
```

## Phase 2: Expose human commands and the closed protocol

**Execution mode: subagent-driven.**

Completes: ["binary-memory-contract"]

### Task 2.1: Add exact memory command grammar and dispatch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:presentation-protocol-composition", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary", "add-associated-pi-effort-memory-tools:compatible-runtime-floor"]
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/gate_test.go", "cmd/awf/help_test.go", "cmd/awf/testdata/help/global.txt"]

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero, and
`go test ./internal/effort ./internal/clispec ./cmd/awf` to exit zero. The phase owner keeps shared
CLI grammar, presentation-boundary fixtures, ADR lifecycle, rendering, and the commit transaction.

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

### Task 2.2: Map ordinary presentation and exact protocol envelopes
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

### Task 2.3: Apply the CLI claim updates
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:presentation-protocol-composition"]
Paths: ["docs/decisions/add-associated-pi-effort-memory-tools.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/tooling/cli.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Apply `explicit-output-bypasses` and `effort-command-contract` with exact owner-free presentation and owner-scoped protocol wording, preserving prior provenance and appending this ADR as Revised-by."
Edge: "Keep effort activity JSON, authored plan and changelog bytes, init descriptor JSON, and context spill unchanged; do not classify memory JSON as optional convenience output."
Post-check: "After `./x render`, `./x check` exits zero and `./awf context cmd/awf/effort.go` reports both CLI operations Applied while all rendering operations remain pending."

Append one Applied event for the two distinct CLI operations and mutate both source claims in the same
transaction. Never hand-edit generated topic or index output.

### Phase close

Stage the complete Phase 2 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(tooling): add memory protocol (applies memory tools batch)
```

## Phase 3: Add the generated client transport

**Execution mode: subagent-driven.**

Advances: ["associated-tool-contract"]

### Task 3.1: Decode every memory protocol reply strictly
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["templates/pi/awf-effort/client.ts.tmpl", ".pi/extensions/awf-effort/client.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]
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

### Task 3.2: Add bounded stdin-capable edit invocation
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:closed-memory-protocol", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["templates/pi/awf-effort/client.ts.tmpl", ".pi/extensions/awf-effort/client.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]

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

### Phase close

Stage the complete Phase 3 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(rendering): add Pi memory protocol client
```

## Phase 4: Activate tools only for the associated effort

**Execution mode: subagent-driven.**

Completes: ["associated-tool-contract"]

### Task 4.1: Guard the companion runtime before functional registration
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:compatible-runtime-floor", "add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:publication-safe-memory-tool-templates"]
Paths: ["templates/partials/pi-minimum-runtime.md", "templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "internal/project/target_test.go", "tools/pi-extension-test/tests/runtime.test.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero, and
`./x pi-test run` to exit zero. Verify the pinned Pi package exports `getActiveTools`,
`setActiveTools`, tool `promptGuidelines`, and `withFileMutationQueue`; the phase must fail rather
than substitute a private queue or persistent prompt injection.

Raise the shared documented Pi floor to the earliest pinned package version that exports the
required active-tool metadata and file-mutation queue helper, and add `setActiveTools` to the closed
minimum-runtime API vocabulary. Make the effort entrypoint consume the same one-notice guard before
registering association hooks or tools. Resolve the newly required queue helper in a way that lets an
older or incomplete runtime reach the actionable compatibility notice instead of failing at static
module import. Do not require `changeCwd`, Remote Pi events, or unrelated UI APIs.

Extend the injected `PiLike`/dependency surface only with the exact questions used by the effort
index: `getActiveTools`, `setActiveTools`, package version, and the file-mutation queue function.
Supported runtime operation must emit no compatibility warning. Add renderer and TypeScript tests
for old version, missing methods/helper, exactly one notice across entrypoints, no partial tool/hook
registration, supported registration, and publication-safe empty-variable rendering.

### Task 4.2: Register strict pathless tools and active-only guidance
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:associated-memory-tool-surface", "add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]

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

### Task 4.3: Extend the generated using-effort guidance
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:publication-safe-memory-tool-templates"]
Paths: ["templates/skills/using-effort/SKILL.md.tmpl", ".pi/skills/awf-using-effort/SKILL.md", "internal/project/target_test.go"]

Add concise guidance that an attached Pi session prefers `effort_memory_read`, uses
`effort_memory_edit` only for the Markdown body, uses `effort_memory_update` for phase/next, and
relies on automatic timestamps. Preserve the direct generic-file escape hatch, repository-root
execution, exact owned path, managed-worktree path, explicit association, advisory activity, and
restart-detached rules. Ensure unset prefix and generic adopter rendering remain coherent with no
unresolved token.

### Phase close

Stage the complete Phase 4 transaction explicitly. Run `awf check staged` and `./x gate`; both must
pass. Create one commit:

```commit
feat(rendering): activate associated Pi memory tools
```

## Phase 5: Apply rendering authority and close generated documentation

**Execution mode: subagent-driven.**

Completes: ["authority-and-generated-state", "repository-gate"]

### Task 5.1: Back the rendered Pi memory-tool contract
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:memory-tool-claim-backing", "add-associated-pi-effort-memory-tools:publication-safe-memory-tool-templates"]
Paths: ["internal/project/target_test.go", "internal/project/spine_test.go", "tools/pi-extension-test/container.sh", "tools/pi-extension-test/tests/using-effort.test.ts", "tools/pi-extension-test/tests/runtime.test.ts"]
Representative: "Add `TestPiEffortMemoryToolContract` with the exact invariant proof marker and prove Pi-only derivation, all three strict tool schemas, detached inactivity, active-only guidance, runtime guard, client/index rendering, and using-effort companion text from selected `effort-workflow`."
Edge: "A target without Pi, or Pi without selected `effort-workflow`, renders none of the memory tools or companion artifacts; disable/prune and generic empty-variable render paths leave no stale output or unresolved token."
Post-check: "`go test ./internal/project -run 'TestPi(EffortMemoryToolContract|RuntimeTargetRender|MinimumRuntime)'` and `./x pi-test run` exit zero; `./awf topic rendering/pi-workflows --coverage` finds the proof marker on the named test and no stale or duplicate marker."

Before dispatch, require `git status --short` to print nothing, `./x check` to exit zero,
`go test ./...` to exit zero, and `./x pi-test run` to exit zero. This phase owns every remaining
State-change operation, proof marker, generated output, documentation update, and the final
implementation commit; it does not perform the later terminal-review status flip.

Keep the proof in a current-state test glob and name `TestPiEffortMemoryToolContract` verbatim on a
non-marker line. Extend container coverage inputs only as needed to retain 100% TypeScript statement
coverage; do not exclude reachable branches.

### Task 5.2: Apply all remaining rendering claims
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:associated-memory-tool-surface", "add-associated-pi-effort-memory-tools:contextual-tool-activation", "add-associated-pi-effort-memory-tools:active-memory-guidance", "add-associated-pi-effort-memory-tools:compatible-runtime-floor", "add-associated-pi-effort-memory-tools:memory-tool-claim-backing", "add-associated-pi-effort-memory-tools:cohesive-runtime-boundary"]
Paths: ["docs/decisions/add-associated-pi-effort-memory-tools.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", "docs/topics/rendering/pi-runtime.md", "docs/topics/rendering/pi-workflows.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Apply both `pi-runtime` updates, both existing `pi-workflows` updates, and the new backed `pi-effort-memory-tools` invariant in one final distinct-claim batch, preserving existing provenance before appending this ADR and giving the new invariant this ADR as Origin."
Edge: "Keep the ADR Implementing after every operation is Applied so settled implementation review owns the later status-only Implemented flip; do not add authority for direct-file prohibition, activity locking, automatic checkpointing, or non-Pi rendering."
Post-check: "After `./x render`, `./x check` exits zero; `./awf context docs/decisions/add-associated-pi-effort-memory-tools.md` reports every declared operation Applied and none Remaining; current-state validation resolves the new proof marker to `TestPiEffortMemoryToolContract`."

Append the final Applied event and mutate all five remaining source claims in the same authored
transaction. Update `pi-extension-target-render` with the client/index responsibilities,
`pi-minimum-runtime` with the guarded active-tool/queue floor, `pi-effort-session-association` and
`using-effort-skill` with dynamic activation and guidance, and add the exact backed invariant
`pi-effort-memory-tools`. Never hand-edit generated topics, index, or lock.

### Task 5.3: Update durable architecture, workflow, and testing prose
Kind: batch
Latitude: exact
Applying: ["add-associated-pi-effort-memory-tools:cohesive-runtime-boundary", "add-associated-pi-effort-memory-tools:compatible-runtime-floor", "add-associated-pi-effort-memory-tools:active-memory-guidance"]
Paths: [".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/testing/layout.md", ".awf/parts/working-with-awf/config-and-overrides.md", "docs/architecture.md", "docs/testing.md", "docs/working-with-awf.md"]
Representative: "Document the binary-to-command-to-generated-client-to-associated-tool flow, the shared presentation versus required-protocol split, active-set and file-queue dependencies, and the Go/TypeScript/render proof lanes."
Edge: "Describe memory tools as preferred conveniences, not workflow authority; retain direct file access, one-writer policy, advisory activity, primary memory ownership, managed-worktree execution, and Pi-only derivation."
Post-check: "After `./x render`, `./x check` exits zero; generated durable docs contain the new boundary and no stale claim that activity JSON is the sole effort protocol bypass or that the companion has no package-version floor."

Update authoring sources and render their generated documents. Keep implementation procedure in this
plan rather than copying it into architecture or workflow prose.

### Phase close

Run `./x render`, read back every source and generated target changed by Tasks 5.2 and 5.3, and stage
the complete Phase 5 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass
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

- Pi 0.83.0 documentation names `pi.getActiveTools()`, `pi.setActiveTools()`, tool
  `promptGuidelines`, and exported `withFileMutationQueue()` as the supported dynamic-tool and
  shared-mutation primitives. Implementation must verify the pinned package/type export before
  setting the exact floor and must not replace either primitive with a private analogue.
- The terminal implementation review later owns the plan status flip and the ADR's status-only
  Implemented transition; no implementation phase performs either freeze.
