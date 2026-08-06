---
format: current-state-v4
slug: render-effort-memory-edits-like-pi
status: Proposed
date: 2026-08-06
---
# ADR-render-effort-memory-edits-like-pi: Render Effort Memory Edits Like Pi


## Context

ADR-0239 added the pathless `effort_memory_edit` tool and kept exact body matching,
memory validation, clocking, and publication in the awf binary. The tool currently returns a
bounded `diff.text` that concatenates the complete old and new bodies under literal `before:` and
`after:` labels. The generated Pi extension has no custom renderer for that result, so interactive
users see two bodies rather than the contextual colored diff used by Pi's built-in edit tool.

Pi's edit tool has two distinct presentation moments. Its call renderer computes a preview once the
arguments are complete, and its result renderer retains the authoritative diff after execution. Pi
publicly exports the `renderDiff` presentation helper, but it does not export its whole-file edit
preview computation. Reusing the internal whole-file computation would also be semantically wrong:
effort memory edits may match only the Markdown body, preserve structured metadata, and canonicalize
a valid legacy memory only during publication.

A preview therefore needs the binary's body boundary and exact replacement semantics. It is not a
prospective publication. In particular, it must not update a timestamp, encode a canonical
replacement, apply the one-MiB published-result check, or otherwise change whether the existing
normal edit operation accepts a mutation. A failed preview must stop the Pi tool before mutation;
a successful preview does not make the later mutation authoritative, because the memory may change
before normal edit validation runs.

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

1. `decision: preview-is-presentation-only` Add a read-only preview mode to the existing effort
   memory edit command. Preview performs the safe owner-scoped memory read and only the body-boundary,
   exact-match, ambiguity, overlap, and replacement work required to compute the requested diff. It
   never clocks, encodes, size-validates, publishes, or reports a prospective memory fact. The normal
   edit path retains its existing validation and publication semantics unchanged. In the generated
   Pi tool, preview success permits normal edit execution, while any preview refusal or execution
   failure is surfaced as the tool error and prevents mutation. Normal edit validation remains
   authoritative after preview and may still refuse changed state.

2. `decision: pi-compatible-memory-display-diff` Replace the before-and-after body concatenation
   with a deterministic Pi-compatible display diff: removed and added rows use Pi's numbered `-` and
   `+` grammar, unchanged rows provide four lines of surrounding context, and separated regions use
   the same omission convention. Preview line numbers describe the currently stored canonical or
   legacy document; successful edit line numbers describe the canonical published document. Empty
   body changes retain an empty diff. The 50-KiB protocol bound remains, but bounding preserves
   complete display rows and at least changed rows rather than returning a partial row.

3. `decision: binary-owned-preview-protocol` Extend the existing closed owner-scoped memory edit
   protocol with a distinct successful preview condition selected only by nonrepeatable `--preview`
   on `awf effort memory edit`. The preview success carries only replacement count and the bounded
   diff; it cannot be mistaken for publication. The generated client uses the same edit request
   schema and strict decoding for preview and mutation. The binary remains the single owner of body
   parsing, exact replacement, and diff facts; TypeScript neither reads memory directly nor
   reimplements those rules.

4. `decision: stable-pi-edit-rendering` Give `effort_memory_edit` a self-rendered Pi call and result
   surface built from the public `renderDiff` helper. The call renderer starts one association-keyed,
   argument-keyed preview after complete arguments, discards stale completion, and retains the final
   authoritative edit diff in the same stable component. Preview is serialized with association
   lifecycle but never enters the file-mutation queue; mutation retains both association
   serialization and the shared real-path mutation queue. Successful model-visible content is a
   compact replacement summary, while structured details retain the diff and truncation fact for
   interactive rendering. Noninteractive modes remain compact and contain no terminal styling.

5. `decision: direct-line-diff-dependency` Adopt `github.com/pmezard/go-difflib` as a direct binary
   dependency for the line change model. The dependency avoids a project-local diff algorithm while
   leaving awf responsible for Pi-compatible row formatting, document line offsets, context
   selection, and bounded complete-row output.

## State changes

- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/pi-workflows:pi-effort-memory-tools`

## Consequences

Interactive effort memory edits gain the same preview-then-retained-diff shape as Pi's ordinary edit
tool without weakening body-only exact matching or moving memory semantics into the extension.
Preview errors become fail-closed: a transient preview transport failure can prevent a mutation that
would otherwise have succeeded, which is preferable to mutating after the user-visible preview
failed.

Preview and mutation remain two reads separated in time. Their diffs may differ, and normal edit may
refuse after a successful preview. The final result always replaces the preview and remains the only
publication fact.

The protocol gains one success condition and one command flag, the extension gains asynchronous
row-local presentation state, and the binary gains a direct diff dependency. Complete-row bounding
requires deterministic selection when the full contextual diff exceeds 50 KiB. Dependency and Pi
workflow documentation must travel with these changes.

Legacy preview and canonical edit results may use different line offsets by design because each
reports the document represented at that moment. No memory format migration is introduced.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Render the existing `before:` and `after:` payload with colors | Coloring does not provide Pi's contextual diff or call-time preview. |
| Reimplement body parsing and replacement in TypeScript | It creates a second memory semantics implementation that can drift from the binary. |
| Deep-import Pi's internal preview helper | The helper is not a supported public export and applies whole-file rather than body-only semantics. |
| Add a separate `memory preview` subcommand | Preview is a nonpublishing mode of the same edit request, not an independent memory operation. |
| Let execution proceed after preview failure | Mutation after the requested user-visible preview failed is surprising and not fail-closed. |
| Apply publication validation during preview | Preview-only timestamp, encoding, or size checks could change or misrepresent normal edit acceptance. |
| Keep a project-local line-diff implementation | It adds bespoke algorithmic code when the existing transitive library can become an explicit dependency. |

## Status history

- 2026-08-06: Proposed
