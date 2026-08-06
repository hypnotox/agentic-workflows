---
format: plan-v2
date: 2026-08-06
adrs:
  - render-effort-memory-edits-like-pi
status: Proposed
---
# Plan: Pi-Style Effort Memory Mutation Diffs

## Goal

Give associated Pi effort-memory edit and metadata-update calls an asynchronous preview and retained
Pi-style diff without changing normal mutation validation or publication semantics; direct generic
file access and owner-free human command behavior remain out of scope.

## Architecture summary

Implement the linked ADR in two independently green transactions. The first transaction adds the
binary-owned preview and display-diff model, closed CLI protocol, direct line-diff dependency, and
the two tooling claim updates. The second transaction extends the generated Pi client and tool
renderers, proves generated behavior, and applies the rendering claim. Go remains the semantic
owner; command code translates the closed protocol; generated TypeScript orchestrates preview before
mutation and renders binary-authored diff facts through Pi's public TUI API.

## Phase 1: Binary preview and mutation-diff protocol

**Execution mode: subagent-driven.**

Advances: ["binary-preview-contract", "pi-mutation-rendering", "governed-artifacts"]

### Task 1.1: Add binary-owned preview and Pi-compatible display diffs
Latitude: exact
Applying: ["render-effort-memory-edits-like-pi:preview-is-presentation-only", "render-effort-memory-edits-like-pi:pi-compatible-memory-display-diff", "render-effort-memory-edits-like-pi:direct-line-diff-dependency"]
Paths: ["go.mod", "go.sum", "internal/effort/memory_operations.go", "internal/effort/memory_operations_test.go", "internal/effort/presentation.go", "internal/effort/presentation_test.go"]

Before dispatch, require `git status --short` to print nothing, `./x check` to complete with zero
errors and zero warnings, and `go test ./internal/effort ./internal/clispec ./cmd/awf` to exit zero.
The phase owner retains shared ADR, topic, rendered-document, staging, gate, and commit ownership;
helpers are report-only or commit-disabled.

Write failing service tests before production changes. Promote `github.com/pmezard/go-difflib` from
indirect to direct and use its line change model behind one effort-owned display-diff formatter. The
formatter must emit the grammar consumed by Pi's `renderDiff`: `-<old-line> <content>`,
`+<new-line> <content>`, and ` <line> <content>`, with four context lines. Between separated regions,
emit Pi's exact omission row: one leading space, a line-number-width blank field, one space, and
`...`; the omission row carries no line number. Preserve UTF-8, the 50-KiB result bound, and an exact `truncated`
fact. Never cut a row prefix or UTF-8 sequence. If one changed source line cannot fit, retain the
complete numbered row grammar, deterministically elide its content with an explicit ASCII marker,
and set `truncated:true`. If all changed-row prefixes cannot fit, deterministically retain
representative changed rows and replace whole changed rows or ranges with the same explicit omission
row. Assert the stable representative-selection method, omission placement, `truncated:true`, and
renderability; do not require every changed row to survive the fixed bound.

Add the `previewed` success condition and preview selection to the typed memory inputs/results
without adding a second memory parser or public filesystem seam. Extract only cohesive shared
calculation needed by preview and mutation:

- Edit preview performs existing input bounds, safe owner-scoped inspection, metadata validity,
  original-body exact matching, ambiguity detection, overlap detection, and replacement application.
  It returns replacement count plus diff, never calls the clock or store, never encodes a complete
  resident, and never applies the published-result size check. A replacement that would make normal
  edit exceed one MiB can therefore preview successfully; normal edit must retain its existing
  `result-too-large` refusal.
- Update preview retains existing update argument validation and safe-repair eligibility checks. Its
  logical diff contains only requested `phase` and `next` changes using the current canonical or
  exact legacy metadata spelling and line positions. It preserves `updated`, omits publication-only
  canonicalization, returns no memory fact, and never clocks, encodes a prospective complete
  resident, size-checks, or stores. Supplying values equal to current fields yields an empty preview.
- Normal edit retains every existing validation, clock, encoding, size, atomic publication, and
  durability-uncertainty branch, but its final diff uses the new display formatter and canonical
  published line offset. Body no-op still advances `updated` and has an empty body diff.
- Normal update retains every existing validation, repair, clock, encoding, size, atomic
  publication, and durability-uncertainty branch. Capture the safe raw pre-update document and add an
  authoritative full-document diff against the canonical published document, including the actual
  `updated` line and any legacy-to-canonical migration.

Cover canonical and legacy offsets, one and several disjoint changes, additions, removals, changed
final lines with and without trailing newline, no-op, four-line context boundaries, separated hunks,
large unchanged context, oversized changed-line elision, aggregate truncation, invalid memory,
owner loss, safe repair, absent/ambiguous/overlapping edit refusals, preview nonpublication, preview
clock nonuse, and unchanged normal publication failures. Keep owner-free semantic presentation
compact: preview has no owner-free form, and normal update/edit presentation must not dump raw diff
text.

Run `go test ./internal/effort`; it must exit zero. Run `go mod tidy`, then read back both `go.mod`
and `go.sum` and confirm `go-difflib` is a direct requirement with no unrelated dependency change.

### Task 1.2: Expose the closed owner-scoped preview protocol
Latitude: exact
Applying: ["render-effort-memory-edits-like-pi:binary-owned-preview-protocol"]
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/gate_test.go", "templates/pi/awf-effort/client.ts.tmpl", "tools/pi-extension-test/tests/using-effort.test.ts"]

Add nonrepeatable `--preview` to the edit and update command specifications and help. Accept it only
on these owner-scoped JSON forms, preserving arbitrary valid flag ordering:

```text
awf effort memory edit <slug> --preview --owner <uuid> --json
awf effort memory update <slug> (--phase <text> | --next <text> | --phase <text> --next <text>) --preview --owner <uuid> --json
```

Owner-free edit/update, every read form, and preview with only one of `--owner` or `--json` must fail
grammar validation before service mutation. Update preview requires phase or next exactly as normal
update does. Edit preview retains the exact closed stdin request and all transport bounds.

Pass preview selection into the typed service operation. Extend the protocol writer with exact
operation-specific successes:

```json
{"schemaVersion":1,"condition":"previewed","replacementCount":1,"diff":{"text":"text","firstChangedLine":1,"truncated":false}}
{"schemaVersion":1,"condition":"previewed","diff":{"text":"text","firstChangedLine":1,"truncated":false}}
```

The first is edit preview; the second is update preview. Both forbid a memory fact. `diff.text` is at
most 50 KiB, `firstChangedLine` is positive or null only with an empty diff, and `truncated` is
boolean. Extend normal update success to require `diff` beside its authoritative memory fact. Keep
protocol version 1, exactly one newline-terminated JSON value, stdout/stderr bounds, handled-refusal
exit behavior, and all existing edit/update envelopes and refusal shapes.

Add table-driven command-spec, grammar, dispatch, protocol, short-write, malformed-input, owner-loss,
legacy, no-op, truncation, and normal-update-diff tests. Explicitly assert preview leaves the resident
bytes unchanged, a failed preview never invokes publication, and normal update/edit acceptance and
failure branches remain unchanged. Update the gated command probes only if the command-table change
requires it; do not create a new command.

In the generated-client template and Pi extension test source, make Phase 1 forward-compatible with
the changed normal-update envelope before the binary protocol lands: normal `updated` decoding must
require and freeze the new diff object, while read and normal edit decoding remain byte-for-byte
compatible. Add strict accepted/rejected normal-update diff cases and keep normal update invocation
argv unchanged. Do not add `previewed` decoding, preview argv, preview orchestration, or custom
rendering until Phase 2.

Run `go test ./internal/clispec ./cmd/awf`; it must exit zero. Run the owner-scoped preview commands
against a test repository through the CLI tests, not the active effort resident. The Pi suite runs
only after Task 1.3 renders this template into the generated client.

### Task 1.3: Apply the tooling claims and direct-dependency documentation
Latitude: exact
Applying: ["render-effort-memory-edits-like-pi:preview-is-presentation-only", "render-effort-memory-edits-like-pi:pi-compatible-memory-display-diff", "render-effort-memory-edits-like-pi:binary-owned-preview-protocol", "render-effort-memory-edits-like-pi:direct-line-diff-dependency"]
Paths: [".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/docs/parts/architecture/dependencies.md", "templates/pi/awf-effort/client.ts.tmpl", ".pi/extensions/awf-effort/client.ts", "tools/pi-extension-test/tests/using-effort.test.ts", "docs/decisions/render-effort-memory-edits-like-pi.md", "docs/decisions/INDEX.md", "docs/topics/tooling/cli.md", "docs/topics/tooling/effort-management.md", "docs/architecture.md", ".awf/awf.lock"]

Update `tooling/cli:effort-command-contract` with the exact owner-scoped-only preview grammar,
operation-specific `previewed` envelopes, update-success diff, and unchanged owner-free and normal
mutation contracts. Preserve its Origin and Backing and append
`ADR-render-effort-memory-edits-like-pi` exactly once to `Revised-by`. Update
`tooling/effort-management:memory-skeleton-purpose-partition` with presentation-only preview,
normal-validation independence, fail-before-execution behavior, Pi-compatible contextual diff,
legacy/canonical offsets, changed-line content elision, deterministic representative changed-row
retention, whole changed-row or range replacement with Pi's exact omission row when prefixes exceed
the bound, `truncated:true` for every omission, and authoritative update timestamp diff. Preserve its
Origin, prior `Revised-by` order, and Backing, then append this ADR exactly once. Add the direct
`go-difflib` role to the authored architecture dependency part.

Move the linked ADR directly from `Proposed` to `Implementing` in this same transaction. Append a
dated `Implementing; content-sha256: <digest>` event followed by one `Applied` event listing exactly
these declaration members:

```text
update `tooling/cli:effort-command-contract`, update `tooling/effort-management:memory-skeleton-purpose-partition`
```

Do not apply the rendering operation and do not append `Implemented`. Obtain the digest
mechanically: place a 64-zero lowercase placeholder, run `./x check`, copy only the computed digest
reported for this ADR, replace the placeholder, and rerun. Run `./x render`, then read back every
listed authored and generated document. Confirm the rendered CLI, effort-management, and
architecture prose says preview cannot alter normal validation, update preview omits `updated`,
normal update final diff includes the actual timestamp, and every bounded diff omission retains
deterministic representative changed rows, uses Pi's exact omission row for whole changed rows or
ranges, and sets `truncated:true`. Confirm the generated client requires the
new normal-update diff without yet exposing preview or custom rendering, and no literal `<no value>`
appears. Run `./x pi-test run`; it must exit zero against the Phase 1 binary envelope and rendered
forward-compatible client.

### Phase close

Stage the complete Phase 1 transaction explicitly. `git diff --cached --check`, `./awf check staged`,
and `./x gate` must all exit zero; staged current-state output must classify exactly the two tooling
operations Applied and the rendering operation Remaining. Create the single closing commit:

```commit
feat(awf): add previews (applies render-effort-memory-edits-like-pi)
```

## Phase 2: Pi preview orchestration and retained rendering

**Execution mode: subagent-driven.**

Completes: ["binary-preview-contract", "pi-mutation-rendering", "governed-artifacts"]

### Task 2.1: Strictly invoke and decode edit and update previews
Latitude: exact
Applying: ["render-effort-memory-edits-like-pi:binary-owned-preview-protocol", "render-effort-memory-edits-like-pi:stable-pi-mutation-rendering"]
Paths: ["templates/pi/awf-effort/client.ts.tmpl", "tools/pi-extension-test/tests/using-effort.test.ts"]

Before dispatch, require `git status --short` to print nothing, `./x check` to complete with zero
errors and zero warnings, `go test ./internal/project -run
'TestPiEffortMemoryToolContract|TestPiRealRuntimeSmoke'` to exit zero, and `./x pi-test run` to exit
zero. The phase owner retains shared topic, ADR, generated outputs, staging, gate, and commit
ownership; helpers are report-only or commit-disabled.

Starting from Phase 1's required normal-update diff decoder, extend the generated client operation
model so strict decoding additionally distinguishes edit preview and update preview. Keep one
validation home for shared memory facts, diff facts, and refusal facts while enforcing the two
distinct `previewed` envelopes:
edit preview requires `replacementCount`; update preview forbids it; both forbid memory. Retain Phase 1's normal update requirement for memory plus diff. Reject extra, missing, wrong-type,
over-bound, mismatched slug,
invalid nullability, nonempty diff with null line, empty diff with a line, and wrong-operation
envelopes.

Extend `memoryEdit` and `memoryUpdate` with a closed typed preview option rather than parallel public
functions. Preview invocation inserts `--preview` in deterministic argv while retaining exact owner
and JSON flags. Edit preview continues through the bounded stdin child adapter; update preview uses
the bounded Pi executor because it has no stdin. Preserve the request-size cap, timeouts,
cancellation, process-group termination, stdout/stderr caps, single-envelope parsing, and existing
normal invocation argv byte-for-byte.

Add client test source before implementation for exact argv and stdin, every accepted preview shape,
cross-operation rejection, refusal parity, transport bounds, and preview nonmutation assumptions.
Do not run the Pi suite against stale generated files here; Task 2.3 renders the template and runs the
suite before the phase closes.

### Task 2.2: Preview first and retain the authoritative diff in both Pi tools
Latitude: exact
Applying: ["render-effort-memory-edits-like-pi:stable-pi-mutation-rendering", "render-effort-memory-edits-like-pi:publication-safe-mutation-templates"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", "tools/pi-extension-test/tests/using-effort.test.ts"]

Import only public Pi exports for `renderDiff` and public Pi TUI components. Build one cohesive
mutation-rendering helper used by `effort_memory_edit` and `effort_memory_update`; do not duplicate
renderer state machines and do not deep-import Pi internals. Use `renderShell: "self"` and a mutable
call component following Pi's built-in edit shape: title, success/error/pending background, spacer,
and rendered diff. Empty preview/final diff keeps the stable header without a blank raw body. A
`truncated:true` preview or result adds a visible themed warning outside the diff.

Key preview work by tool-call id plus immutable association slug/owner plus exact arguments. Once
arguments are complete, `renderCall` starts the binary preview asynchronously and calls
`context.invalidate()` only for the still-current key. Keep a tool-call preview-promise map outside
row-local TUI state so `execute` can await the same preview. `execute` must also create and await the
preview when no renderer ran, preserving behavior in JSON and print modes. Await preview outside the
normal mutation's association-chain callback to avoid self-deadlock. Preview itself enters the
existing association serialization chain but never the real-path file queue. After successful
preview, invoke the existing normal mutation through its association serialization and shared
real-path mutation queue.

Convert every handled non-`previewed` preview reply and every preview transport failure into a
thrown tool error and never invoke normal mutation. Clear association on the same preview ownership
loss conditions as other memory calls. Once normal mutation starts, retain its existing handled
refusal and durability-uncertainty behavior. Always replace preview state with the authoritative
normal result or error; discard late stale completion; clean preview-map entries after settlement.
A race after preview may therefore yield a normal refusal without displaying the stale preview as
final.

Return compact model-visible successes: edit names its replacement count; update says metadata was
updated. Keep the structured protocol reply in details. Custom `renderResult` renders the final diff
for `edited` and `updated`, the existing outcome text for handled refusals, and the thrown error for
preview failures. Update preview displays only phase/next rows because the binary omits `updated`;
normal update result displays the actual timestamp row from its authoritative diff.

Add deterministic renderer/harness tests for partial arguments, pending state, one preview per key,
asynchronous invalidation, edit and update preview argv, non-TUI preview creation, preview refusal,
transport rejection, no mutation after preview failure, association clearing, stale-key suppression,
normal refusal after preview, authoritative result replacement, no-op, legacy offsets, truncation
warning, expanded/collapsed stability, map cleanup, and preservation of unrelated active tools and
queue calls. Use fake theme/components and controlled promises; no wall-clock sleeps.

### Task 2.3: Render, prove the generated contract, and apply the final claim
Latitude: exact
Applying: ["render-effort-memory-edits-like-pi:stable-pi-mutation-rendering", "render-effort-memory-edits-like-pi:publication-safe-mutation-templates"]
Paths: [".awf/topics/parts/rendering/pi-workflows/current-state.md", "internal/project/target_test.go", "templates/pi/awf-effort/client.ts.tmpl", "templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/client.ts", ".pi/extensions/awf-effort/index.ts", "docs/decisions/render-effort-memory-edits-like-pi.md", "docs/decisions/INDEX.md", "docs/topics/rendering/pi-workflows.md", ".awf/awf.lock"]

Update `rendering/pi-workflows:pi-effort-memory-tools` to cover owner-scoped binary preview for edit
and update, fail-before-execution gating, unchanged normal validation, shared stable `renderDiff`
presentation, compact model content, stale suppression, association serialization, and mutation-only
file queuing. Preserve Origin and Backing and append
`ADR-render-effort-memory-edits-like-pi` exactly once to `Revised-by`.

Extend `TestPiEffortMemoryToolContract` and its existing proof marker rather than creating a second
proof home. Assert template source and rendered extension both contain the strict preview protocol,
public renderer imports, preview gating for both tools, update timestamp split, shared mutation
renderer, compact content, and truncation warning; assert non-Pi targets contain neither preview
protocol nor Pi TUI rendering references. Preserve the template missingkey-zero checks and add
empty-variable rendering assertions that reject `<no value>` and unresolved tokens.

Append one final `Applied` event to the linked ADR listing exactly:

```text
update `rendering/pi-workflows:pi-effort-memory-tools`
```

Leave ADR status `Implementing`, even though Remaining is empty; terminal implementation review owns
the later status-only `Implemented` transition. Run `./x render`, then read back both generated
extension files, the rendered Pi workflow topic, INDEX, and lock.

Perform the focused semantic rendering review at the generated `.pi/extensions/awf-effort/index.ts`
boundary: an edit preview and result must read as one stable edit row with contextual `-`/`+` lines,
not `before:`/`after:` bodies; an update preview must show phase/next without `updated`; its final
result must include the actual `updated` change; no refusal may look successful; truncation must be
visible; and empty-variable output must remain coherent. Record any reasoned deviation under Notes.

Run `go test ./internal/project -run 'TestPiEffortMemoryToolContract|TestPiRealRuntimeSmoke'`, then
`./x pi-test run`; both must exit zero. Run `rg -n 'before:|after:|<no value>'
.pi/extensions/awf-effort templates/pi/awf-effort`; it must report no obsolete diff labels or
unresolved token, while any intentional test fixture match must be explained in Notes rather than
silently deleted.

### Phase close

Stage the complete Phase 2 transaction explicitly. `git diff --cached --check`, `./awf check staged`,
and `./x gate` must all exit zero; staged current-state output must classify all three declared
operations Applied with no Remaining operations, while the ADR remains Implementing. Create the
single closing commit:

```commit
feat(rendering): add diffs (applies render-effort-memory-edits-like-pi)
```

## Definition of done

- `dod: binary-preview-contract` Owner-scoped edit and update preview return strict bounded
  `previewed` envelopes without publishing, clocking, or changing normal mutation validation; normal
  edit/update retain publication semantics and return authoritative Pi-compatible diffs.
- `dod: pi-mutation-rendering` Both associated Pi mutation tools preview before execution, stop on
  preview failure, retain the authoritative colored diff, keep model-visible content compact, and
  preserve association and file-queue boundaries in TUI and non-TUI modes.
- `dod: governed-artifacts` All three ADR claim operations are Applied with test backing, direct
  dependency and generated documentation are current, templates remain publication-safe, the ADR is
  Implementing pending terminal review, and `./x render`, `./x check`, `./x pi-test run`, and
  `./x gate` are green.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.
Record deviations, spike answers, follow-ups, and findings surfaced during implementation.
