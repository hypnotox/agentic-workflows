---
format: current-state-v4
slug: render-effort-memory-edits-like-pi
status: Proposed
date: 2026-08-06
---
# ADR-render-effort-memory-edits-like-pi: Render Effort Memory Mutations Like Pi


## Context

ADR-0239 added the pathless `effort_memory_edit` and `effort_memory_update` tools and kept exact
body matching, structured metadata changes, memory validation, clocking, and publication in the awf
binary. Edit currently returns a bounded `diff.text` that concatenates the complete old and new
bodies under literal `before:` and `after:` labels, while update returns no diff. The generated Pi
extension has no custom renderer for either result, so interactive users cannot see the contextual
colored preview and retained mutation diff used by Pi's built-in edit tool.

Pi's edit tool has two distinct presentation moments. Its call renderer computes a preview once the
arguments are complete, and its result renderer retains the authoritative diff after execution. Pi
publicly exports the `renderDiff` presentation helper, but it does not export its whole-file edit
preview computation. Reusing the internal whole-file computation would also be semantically wrong:
effort memory edits may match only the Markdown body, preserve structured metadata, and canonicalize
a valid legacy memory only during publication.

An edit preview therefore needs the binary's body boundary and exact replacement semantics. An
update preview needs the binary's structured-field and safe-repair semantics, but should show only
the requested `phase` and `next` changes; a preview-time `updated` value would never be published.
Neither preview is a prospective publication. In particular, preview must not clock, encode a
prospective complete resident, apply the one-MiB published-result check, or otherwise change whether
the existing normal mutation operation accepts a write. A failed preview must stop the Pi tool
before mutation; a successful preview does not make the later mutation authoritative, because the
memory may change before normal validation runs.

The current diff bound is 50 KiB, while one memory line or replacement string may approach one MiB.
A display diff must retain complete renderable rows and changed content within that bound rather
than cutting an arbitrary byte prefix. Canonical memory bodies begin on line 7, while retained
legacy bodies begin on line 6. A preview must number the document it actually read, and a successful
edit result must number the canonical document it actually published.

Producing the same line-oriented change model as Pi requires a maintained line-diff algorithm.
`github.com/pmezard/go-difflib` is already present transitively, but production use makes it a direct
awf binary dependency and therefore part of the documented dependency surface.

This decision changes terminal ADR-0239 forward through current-state claims. Its history is not
rewritten.

## Decision

1. `decision: preview-is-presentation-only` Add read-only preview modes to the existing effort
   memory edit and update commands. Edit preview performs the safe owner-scoped memory read and only
   the body-boundary, exact-match, ambiguity, overlap, and replacement work required to compute its
   diff. Update preview applies the structured-field and safe-repair checks required to identify the
   requested `phase` and `next` changes, but its diff preserves the current `updated` value and omits
   publication-only canonicalization. Neither preview clocks, encodes a prospective complete
   resident, applies the published-result size check, publishes, or reports a prospective memory
   fact. The normal edit and update paths retain their existing validation and publication semantics
   unchanged. In each generated Pi tool, preview success permits normal execution, while any preview
   refusal or preview execution failure is surfaced as the tool error and prevents normal execution.
   Once normal execution begins, its existing publication and `changedMemory` durability-uncertainty
   semantics remain authoritative, and normal validation may still refuse changed state.

2. `decision: pi-compatible-memory-display-diff` Use one deterministic Pi-compatible display diff
   for mutation previews and authoritative results: removed and added rows use Pi's numbered `-` and
   `+` grammar, unchanged rows provide four lines of surrounding context, and separated regions use
   the same omission convention. Preview line numbers describe the currently stored canonical or
   legacy document; successful results describe the canonical published document. Body no-ops and
   update previews whose requested fields are unchanged retain an empty diff. The 50-KiB protocol
   bound remains. Bounding never cuts the display-row grammar or a UTF-8 sequence; when a changed
   source line cannot fit, its content is deterministically elided inside a complete numbered change
   row and `truncated` is true.

3. `decision: binary-owned-preview-protocol` Permit nonrepeatable `--preview` only on these exact
   owner-scoped protocol forms; owner-free human forms reject it:

   - `awf effort memory edit <slug> --preview --owner <uuid> --json`
   - `awf effort memory update <slug> (--phase <text> | --next <text> | --phase <text> --next <text>) --preview --owner <uuid> --json`

   Edit retains its closed stdin request. A successful edit preview has exactly this protocol-1
   envelope, where `replacementCount` is 1 through 128:

   ```json
   {"schemaVersion":1,"condition":"previewed","replacementCount":1,"diff":{"text":"text","firstChangedLine":1,"truncated":false}}
   ```

   A successful update preview has exactly this protocol-1 envelope:

   ```json
   {"schemaVersion":1,"condition":"previewed","diff":{"text":"text","firstChangedLine":1,"truncated":false}}
   ```

   In both forms `diff.text` is bounded to 50 KiB, `firstChangedLine` is a positive integer or null
   for an empty diff, and `truncated` is boolean. Preview carries no memory fact and cannot be
   mistaken for publication. A successful normal update adds the same required `diff` object beside
   its authoritative memory fact; normal edit retains its existing success shape with the new diff
   representation. The generated client selects and strictly decodes the operation-specific closed
   envelope. The binary remains the single owner of memory parsing, safe repair, exact replacement,
   and diff facts; TypeScript neither reads memory directly nor reimplements those rules.

4. `decision: stable-pi-mutation-rendering` Give `effort_memory_edit` and `effort_memory_update`
   self-rendered Pi call and result surfaces built from the public `renderDiff` helper. Each call
   renderer starts one association-keyed, argument-keyed preview after complete arguments, discards
   stale completion, and retains the final authoritative mutation diff in the same stable component.
   Preview is serialized with association lifecycle but never enters the file-mutation queue;
   mutation retains both association serialization and the shared real-path mutation queue.
   Successful model-visible content is a compact mutation summary, while structured details retain
   the diff and truncation fact for interactive rendering. Noninteractive modes remain compact and
   contain no terminal styling.

5. `decision: direct-line-diff-dependency` Adopt `github.com/pmezard/go-difflib` as a direct binary
   dependency for the line change model. The dependency avoids a project-local diff algorithm while
   leaving awf responsible for Pi-compatible row formatting, document line offsets, context
   selection, and bounded complete-row output.

6. `decision: publication-safe-mutation-templates` Require every changed Pi extension template to
   retain missingkey-zero behavior and render coherent generic output for empty variables without
   `<no value>` or another unresolved-value token.

## State changes

- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/pi-workflows:pi-effort-memory-tools`

## Consequences

Interactive effort memory edits and metadata updates gain the same preview-then-retained-diff shape
as Pi's ordinary edit tool without weakening exact replacement, structured update, or safe-repair
semantics and without moving memory rules into the extension. Update preview intentionally omits the
automatic timestamp, while its final result shows the actual published timestamp change. Preview
errors become fail-closed: a transient preview transport failure can prevent a mutation that would
otherwise have succeeded, which is preferable to mutating after the user-visible preview failed.

Preview and mutation remain two reads separated in time. Their diffs may differ, and normal edit may
refuse after a successful preview. The final result always replaces the preview and remains the only
publication fact.

The protocol gains one success condition across two operation-specific envelopes and one command
flag on each mutation, the extension gains asynchronous row-local presentation state, and the binary
gains a direct diff dependency. Changed-row content elision means an extreme line is represented
faithfully as a changed row but not reproduced in full. Dependency and Pi workflow documentation
must travel with these changes. Every updated invariant claim remains test-backed; the
implementation plan owns the concrete layered proof transactions.

Legacy preview and canonical edit results may use different line offsets by design because each
reports the document represented at that moment. No memory format migration is introduced.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Render the existing `before:` and `after:` payload with colors | Coloring does not provide Pi's contextual diff or call-time preview, and update has no payload to render. |
| Reimplement memory parsing and mutation in TypeScript | It creates a second memory semantics implementation that can drift from the binary. |
| Deep-import Pi's internal preview helper | The helper is not a supported public export and applies whole-file rather than body-only semantics. |
| Add a separate `memory preview` subcommand | Preview is a nonpublishing mode of the same edit request, not an independent memory operation. |
| Let execution proceed after preview failure | Mutation after the requested user-visible preview failed is surprising and not fail-closed. |
| Apply publication validation during preview | Preview-only timestamp, encoding, or size checks could change or misrepresent normal mutation acceptance. |
| Preview an update timestamp | The preview-time value would never be the timestamp published by normal execution. |
| Keep a project-local line-diff implementation | It adds bespoke algorithmic code when the existing transitive library can become an explicit dependency. |

## Status history

- 2026-08-06: Proposed
