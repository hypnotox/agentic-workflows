---
format: current-state-v4
slug: add-associated-pi-effort-memory-tools
status: Implementing
date: 2026-08-04
---
# ADR-add-associated-pi-effort-memory-tools: Add associated Pi effort memory tools


## Context

ADR-0218 introduced explicit Pi effort association, and ADR-0225 reduced it to a direct
process-local attachment that supplies fixed repository-relative memory and managed-worktree
paths. The associated agent still reaches memory through Pi's generic file tools or through an
`awf effort memory update` shell command. It must repeat the fixed path, keep body edits separate
from checkpoint metadata, and arrange a second command when a body change should advance the
memory timestamp. This is both unnecessary model work and an avoidable opportunity to select the
wrong effort path.

The awf binary already owns the stronger boundary needed here. It validates the immutable effort
identity and closed memory frontmatter, bounds memory to one MiB, injects UTC time, and publishes a
complete replacement through a synced sibling, atomic rename, and directory sync. Structured
metadata update deliberately repairs invalid mutable `phase` or `next` values when every invalid
field is supplied, so a preliminary heartbeat that parses all memory metadata would incorrectly
block that repair. Protocol-v2 `activity.json` independently records the current advisory Pi owner
and can establish whether an associated operation still belongs to the calling process without
turning activity into authorization or a lock.

Pi can register tools while leaving them inactive, change the active set at runtime, attach prompt
guidance only to active tools, and place custom mutations on the same per-file queue as its generic
file tools. Its extension execution helper does not accept stdin, however, while exact replacement
requests can contain multiline text that should not be placed in command-line arguments. A small
bounded child-process transport is therefore necessary for the edit request, but memory parsing,
matching, timestamping, and publication remain binary-owned.

Pi's built-in edit implementation exposes an exact-replacement contract but also contains a fuzzy
whitespace and punctuation fallback. That fallback has little value for structured effort memory,
would create a second normalization policy in Go, and makes a purportedly exact request less
predictable. The useful conventions are instead batch evaluation against original content, unique
matches, non-overlap, familiar diagnostics, and diff facts.

Concurrent work is standardizing ordinary command presentation around one closed syntax renderer,
model-owner semantic mappings, and an explicit set of byte-exact protocol bypasses. Effort memory
operations intersect that boundary: owner-free output is ordinary presentation, while owner-scoped
JSON exists because the generated Pi tools require a machine protocol. This decision must build on
the settled shared presentation boundary rather than introduce another local output grammar or let
required protocol JSON be mistaken for the optional convenience modes that work removes.

This changes terminal ADR-0225 forward through current-state claims. Its history is not rewritten.

## Decision

1. `decision: associated-memory-tool-surface` Extend the generated Pi effort companion with three
   pathless model tools while a session is associated: `effort_memory_read` accepts only optional
   one-indexed `offset` and `limit`; `effort_memory_edit` accepts only a nonempty `edits` array of
   `oldText` and `newText` pairs; and `effort_memory_update` accepts only optional `phase` and
   `next`, with at least one required at execution. No tool accepts an effort slug or filesystem
   path. Read pagination selects from the complete memory document rather than the body alone, so
   frontmatter appears when the selected range includes it. Edit changes only the Markdown body. Update changes only mutable frontmatter. Direct use of Pi's generic file tools
   and ordinary awf commands remains permitted.

2. `decision: binary-owned-memory-operations` Keep safe memory semantics in the awf binary. Add a
   bounded paginated read and a body-only exact edit beside structured metadata update. Body edit
   parses and preserves the closed frontmatter, evaluates every requested match against the
   original body, requires each match to be present exactly once, rejects nested or overlapping
   regions as one failed batch, applies all replacements or none, and sets `updated` from the
   injected UTC clock in the same atomic complete-file publication. It does not implement Pi's
   fuzzy fallback. Every operation retains the one-MiB resident bound, immutable effort identity,
   safe regular-file handling, canonical encoding, and durability diagnostics.

3. `decision: advisory-owner-scoped-memory-calls` Let the generated client pass its ephemeral owner
   UUID on each associated memory operation. When an owner is supplied, the binary checks only
   that the safe protocol-v2 `activity.json` exists and names that owner before it reads or mutates
   memory; it does not require valid mutable memory metadata as a precondition. Missing activity,
   an unsafe activity resident, or a different owner returns a stable explanatory refusal. The Pi
   extension then clears its local association, publication, and memory-tool activation so the
   agent may explicitly reattach. Ordinary owner-free memory commands remain independent of
   activity. The owner check is advisory process consistency, not permission, lifecycle state, a
   cross-process lock, or protection against a takeover after the check.

4. `decision: closed-memory-protocol` Add these exact owner-free human forms:

   - `awf effort memory read <slug> [--offset <positive-line>] [--limit <positive-lines>]`
   - `awf effort memory edit <slug>`
   - `awf effort memory update <slug> [--phase <text>] [--next <text>]`

   Add these exact owner-scoped protocol forms:

   - `awf effort memory read <slug> [--offset <positive-line>] [--limit <positive-lines>] --owner <uuid> --json`
   - `awf effort memory edit <slug> --owner <uuid> --json`
   - `awf effort memory update <slug> [--phase <text>] [--next <text>] --owner <uuid> --json`

   `--owner` and `--json` are nonrepeatable and mutually required: supplying either without the
   other is invalid. Their presence selects the advisory owner check and machine protocol;
   owner-free forms retain ordinary direct-command presentation. Edit in either form reads one
   closed JSON object from stdin with this literal shape:

   ```json
   {"edits":[{"oldText":"nonempty string","newText":"string"}]}
   ```

   The object contains no additional properties and contains 1 through 128 edit objects with no
   additional properties. Each decoded string is individually bounded by the one-MiB memory limit,
   and the complete stdin request is bounded to 16 MiB. Read selection is capped at 2,000 complete
   lines or 50 KiB after honoring its optional line limit. Byte truncation stops before the first
   line that would exceed the cap and sets `nextOffset` to that line; it never returns a partial
   line. If the line at the requested offset cannot fit by itself, read returns handled
   `result-too-large` with that line's byte size and the 50-KiB maximum so line-offset continuation
   never skips content.

   A syntactically valid read offset beyond the document's total line count returns the handled
   `offset-out-of-range` refusal rather than an empty success or malformed-bound failure.

   JSON mode writes exactly one newline-terminated protocol-1 envelope, with stdout bounded to one
   MiB and stderr bounded to 50 KiB. Every memory fact uses a canonical slug through 63 bytes,
   phase and next strings through 500 bytes, and an updated value that is either RFC3339Nano UTC or
   the exact legacy `Not yet updated.` sentinel. A successful read has exactly this shape, where
   every line fact is a positive integer, `nextOffset` is a positive integer or null, and
   `truncatedBy` is exactly `none`, `limit`, `lines`, or `bytes`:

   ```json
   {"schemaVersion":1,"condition":"read","memory":{"effort":"slug","phase":"text","next":"text","updated":"RFC3339Nano UTC"},"content":"text","range":{"startLine":1,"endLine":1,"totalLines":1,"nextOffset":null,"truncatedBy":"none"}}
   ```

   A successful edit has exactly the following shape. `replacementCount` is an integer from 1
   through 128, `diff.text` is a string bounded to 50 KiB, `firstChangedLine` is a positive integer
   or null for a body no-op, and `truncated` is boolean:

   ```json
   {"schemaVersion":1,"condition":"edited","memory":{"effort":"slug","phase":"text","next":"text","updated":"RFC3339Nano UTC"},"replacementCount":1,"diff":{"text":"text","firstChangedLine":1,"truncated":false}}
   ```

   A successful update has exactly this shape:

   ```json
   {"schemaVersion":1,"condition":"updated","memory":{"effort":"slug","phase":"text","next":"text","updated":"RFC3339Nano UTC"}}
   ```

   Handled refusal conditions are `not-owner`, `missing`, `unsafe-activity`, `invalid-memory`,
   `unsafe-memory`, `offset-out-of-range`, `no-match`, `ambiguous-match`, `overlapping-edits`,
   `result-too-large`, and `memory-failure`. Every refusal has exactly `schemaVersion`, `condition`,
   and `outcome`, except
   for the condition-specific extra fact stated below. `outcome` has exactly string
   `category:"operation"`, nonempty present-tense string `condition` through 4 KiB, boolean
   `changedMemory`, and `nextActions` containing 1 through 8 nonempty strings through 4 KiB each.
   Only `memory-failure` adds a nonempty string `cause` through 50 KiB to `outcome`; every other
   condition forbids it. The base refusal shape is:

   ```json
   {"schemaVersion":1,"condition":"not-owner","outcome":{"category":"operation","condition":"observed state","changedMemory":false,"nextActions":["independently executable action"]}}
   ```

   `offset-out-of-range` adds exactly `"range":{"offset":2,"totalLines":1}`, where `offset` is a
   positive integer greater than the positive integer `totalLines`. `no-match` adds exactly
   `"edit":{"index":0}`. `ambiguous-match` adds exactly `"edit":{"index":0,"occurrences":2}`.
   `overlapping-edits` adds exactly `"edits":{"firstIndex":0,"secondIndex":1}`.
   `result-too-large` adds exactly `"size":{"bytes":1048577,"maxBytes":1048576}`; for an
   individually unpageable read line, `bytes` is that line's byte size and `maxBytes` is 51,200.
   Indexes and byte counts are nonnegative integers, and an occurrence count is an integer greater
   than one. No other
   refusal carries an extra fact. `changedMemory` is false for every refusal except `memory-failure` after atomic
   replacement, where it is true. Handled refusals exit zero. Malformed grammar or stdin, invalid
   bounds, and failures before managed state is observed use nonzero exit, empty stdout, and bounded
   actionable stderr. Human mode uses the same typed results and refusals through effort-package
   rendering. The generated client owns a bounded, timeout-aware, cancellation-aware child process
   for edit stdin and strictly validates every reply; it never reads or writes the effort resident
   itself.

5. `decision: contextual-tool-activation` Register the three memory tools with the generated
   effort extension but leave them inactive while detached. Successful attachment adds them to
   Pi's current active set without removing unrelated tools. Explicit detach, restart, missing
   activity, or ownership loss removes only these three names. Keep each complete associated
   operation on the extension's existing process-local association chain, and put mutations on
   Pi's real-path file-mutation queue so the companion does not race a generic edit in the same
   process. This serialization is local consistency only and creates no new authority or lock.

6. `decision: active-memory-guidance` Attach guidance to the active tools and the generated
   `using-effort` skill that tells an associated agent to prefer the pathless memory tools, use
   body edit separately from metadata update, and rely on automatic timestamps. The guidance does
   not forbid or intercept generic `read`, `edit`, or `write`, does not duplicate the workflow's
   checkpoint policy, and does not make report-only children memory writers.

7. `decision: cohesive-runtime-boundary` Preserve the existing ownership split. The Go effort
   package owns resident safety, activity-owner comparison, memory parsing, exact body matching,
   clocking, atomic publication, typed outcomes, and the semantic mapping for human output. The
   repository's shared presentation authority owns ordinary text syntax and rendering. Command code
   owns grammar, stdin and JSON transport, presentation-versus-protocol selection, stream choice,
   and exit mapping. The generated client owns bounded invocation and strict protocol decoding.
   The generated index owns association state, dynamic activation, local serialization, file-queue
   participation, transient context, and Remote Pi publication. The Pi target alone derives these
   tools from selected `effort-workflow`; no non-Pi target names or renders them.

8. `decision: compatible-runtime-floor` Require no on-disk migration and retain the existing
   owner-free structured update command. Extend the Pi runtime floor only by the dynamic-tool and
   file-mutation-queue APIs used by the companion, with an actionable compatibility refusal before
   functional registration when they are unavailable.

9. `decision: publication-safe-memory-tool-templates` Require every changed template to retain
   missingkey-zero behavior and render coherent generic output for empty variables without
   `<no value>` or another unresolved-value token.

10. `decision: memory-tool-claim-backing` Add the
    `rendering/pi-workflows:pi-effort-memory-tools` invariant with `Backing: test`, proved by the
    `TestPiEffortMemoryToolContract` unit and its matching proof marker.

11. `decision: presentation-protocol-composition` Classify owner-scoped memory JSON as a required,
    byte-exact machine protocol and explicit ordinary-presentation bypass, never as an optional
    convenience renderer. Owner-free memory results use the repository's accepted ordinary text
    contract through effort-owned semantic mappings. No memory command creates a feature-local
    presentation tree, syntax renderer, or second human-output contract.

## State changes

- update `tooling/cli:explicit-output-bypasses`
- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-workflows:pi-effort-session-association`
- update `rendering/pi-workflows:using-effort-skill`
- add `rendering/pi-workflows:pi-effort-memory-tools`

## Consequences

An associated agent can read or update its effort memory without reconstructing a path, supplying a
slug, or consulting the system clock. Body and checkpoint metadata remain deliberately separate,
and the binary makes body plus timestamp one durable transaction. Dynamic activation limits tool
and guidance noise to sessions where the operations can succeed, while direct file access remains
an explicit escape hatch.

The binary and generated client gain a new closed protocol and the effort extension gains three
public tools. Exact matching in Go intentionally differs from Pi's hidden fuzzy fallback, so text
that is merely similar does not match. Large memory remains readable through pagination rather
than entering model context at once. A cancellation near process completion can make the caller
uncertain whether publication completed; the tool must report that uncertainty and direct a read
instead of retrying blindly.

Owner comparison prevents a stale local association from knowingly operating after another Pi
session has taken over. It does not authorize memory, stop direct commands, or close the unavoidable
race between a successful advisory check and later takeover. The one-user-managed-writer workflow
rule remains the authority boundary.

The generated edit client needs a small child-process path because Pi's extension executor cannot
supply stdin. That mechanism is bounded and isolated in the client rather than leaking filesystem
or process details into the model tool. Reusing the existing association chain and Pi file queue
avoids a second locking system.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep using generic file tools and shell commands | Agents must repeat a known path and perform timestamp bookkeeping that the association and binary can own safely. |
| Override or intercept Pi's generic file tools | Hidden path substitution and blocking would make direct access surprising and turn preference guidance into enforcement. |
| Keep memory tools active while detached | Calls would usually fail and their schemas and guidance would pollute unrelated sessions. |
| Put the slug or path in each tool input | Caller-supplied identity defeats the safety and convenience provided by explicit association. |
| Reproduce Pi's fuzzy edit fallback | It adds a second normalization policy for little benefit and weakens the exact-match contract. |
| Send edit JSON through command-line arguments | Multiline and potentially large replacements do not fit a robust argument transport. |
| Pass edit JSON through a temporary file | It creates cleanup, ownership, link, and replacement-race policy solely to work around missing stdin support. |
| Treat activity as a lock or authorization gate | Activity is advisory and cannot replace the workflow's one-writer rule or prevent direct filesystem access. |
| Combine body edits with `phase` and `next` updates | It obscures checkpoint semantics and lets a general text replacement mutate structured workflow metadata. |

## Status history

- 2026-08-04: Proposed
- 2026-08-05: Implementing; content-sha256: a4263a1323591448c62c8bf4b64c8b469822218f123bd3d8c4a5ac6bfc33fbe9
- 2026-08-05: Applied; operations: update `tooling/effort-management:effort-record-authority`, update `tooling/effort-management:memory-skeleton-purpose-partition`, update `tooling/cli:explicit-output-bypasses`, update `tooling/cli:effort-command-contract`
