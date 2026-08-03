---
format: current-state-v4
slug: add-associated-pi-effort-memory-tools
status: Proposed
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

This changes terminal ADR-0225 forward through current-state claims. Its history is not rewritten.

## Decision

1. `decision: associated-memory-tool-surface` Extend the generated Pi effort companion with three
   pathless model tools while a session is associated: `effort_memory_read` accepts only optional
   one-indexed `offset` and `limit`; `effort_memory_edit` accepts only a nonempty `edits` array of
   `oldText` and `newText` pairs; and `effort_memory_update` accepts only optional `phase` and
   `next`, with at least one required at execution. No tool accepts an effort slug or filesystem
   path. Read includes the complete memory document, frontmatter included. Edit changes only the
   Markdown body. Update changes only mutable frontmatter. Direct use of Pi's generic file tools
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

4. `decision: closed-memory-protocol` Give associated memory calls bounded, closed machine replies
   carrying the facts their tools need. Read returns selected content and continuation facts;
   edit returns replacement count, diff facts, and resulting validated memory metadata; update
   returns resulting validated memory metadata. Expected state refusals remain distinguishable
   from malformed invocation and mechanism failure. Carry an edit request as bounded JSON on the
   binary's stdin, and keep stdout reserved for its single JSON reply. The generated client owns a
   bounded, timeout-aware, cancellation-aware child process for that stdin operation and strictly
   validates every reply; it never reads or writes the effort resident itself.

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
   clocking, and atomic publication. Command code owns grammar, stdin and JSON transport, and human
   presentation. The generated client owns bounded invocation and strict protocol decoding. The
   generated index owns association state, dynamic activation, local serialization, file-queue
   participation, transient context, and Remote Pi publication. The Pi target alone derives these
   tools from selected `effort-workflow`; no non-Pi target names or renders them.

8. `decision: compatibility-and-runtime-floor` Require no on-disk migration and retain the existing
   owner-free structured update command. Extend the Pi runtime floor only by the dynamic-tool and
   file-mutation-queue APIs used by the companion, with an actionable compatibility refusal before
   functional registration when they are unavailable. Verify the binary operations, CLI protocol,
   generated state machine, active-only guidance, target render and prune behavior, safe failure
   boundaries, generated documentation, and repository gate before applying the declared claims.

## State changes

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
