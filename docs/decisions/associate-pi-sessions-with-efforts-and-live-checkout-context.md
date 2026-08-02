---
format: current-state-v3
slug: associate-pi-sessions-with-efforts-and-live-checkout-context
status: Proposed
date: 2026-08-02
---
# ADR-associate-pi-sessions-with-efforts-and-live-checkout-context: Associate Pi sessions with efforts and live checkout context


## Context

An awf effort identifies one outcome, owns its working memory, and usually owns a
managed worktree. The binary and workflow deliberately do not assign Pi sessions to
efforts. A Pi session can therefore continue from an effort checkpoint while its live
runtime remains in the checkout from which Pi started, and other Remote Pi peers cannot
see which effort or checkout that session is using. Repeated shell `cd` calls change only
individual subprocesses; starting a fresh handoff session preserves the workflow chain
but discards the conversation that needs the new execution context.

The desired association is supportive coordination, not authority. Repository sources and
current-state documentation remain authoritative. Git remains the worktree-topology
authority. Presence can be stale, duplicated, or unavailable, so it must neither lock an
effort nor block an unrelated effort command. A takeover should expose the previous
claim and its age, then proceed. The association must survive the ordinary workflow
through integration, review, retrospective, and the attempt to finish rather than silently
following topology or detaching at a phase boundary.

Effort memory already carries the useful checkpoint fields `Effort:`, `Phase:`, `Next:`,
and `Updated:` in a custom four-line header, but production code intentionally does not
parse its headings beyond handoff's first-line identity check. Remote peer metadata needs
those fields in a bounded canonical form. Since awf will now parse and mutate them, retaining
a bespoke frontmatter-like encoding would add another document grammar without benefit.
Existing untouched memories can also contain the legacy `Updated: Not yet updated.`
sentinel. The new parser therefore needs a deliberately narrow YAML frontmatter contract,
a compatibility path for the legacy header, and no new validation gate for unrelated
memory reads or effort commands.

The runtime behavior crosses three ownership boundaries:

- awf owns effort residents, memory validation and mutation, activity transitions, checkout
  discovery, and the generated `using_effort` extension and support skill;
- Pi owns changing the current directory of a live persisted session without creating a
  new conversation; and
- Remote Pi owns peer identity, capability negotiation, replay, and publication of the
  atomically replaced `awf` metadata namespace.

Pi's accepted but not-yet-released prerequisite design provides command-context-only
`ExtensionCommandContext.changeCwd(targetCwd, options)`. It replaces the same session
runtime, writes the destination into the session header, relocates default session storage
when applicable, gives extensions a fresh `ReplacedSessionContext`, runs destination trust,
and is detected by `typeof ctx.changeCwd === "function"`. Remote Pi's accepted but
not-yet-released prerequisite design provides an awf-only process-local peer-name override,
capability negotiation and replay requests, serialized identity reconciliation, and
continued atomic replacement of the existing `awf` metadata namespace. awf must consume
those owner-provided contracts rather than duplicate them or depend on an unlanded shape.

Activity is stored beside the effort that owns it. This makes the ownership boundary
cohesive but intentionally creates a one-way compatibility boundary: after
`activity.json` exists, an older awf binary is not required to read or finish that effort.
Downgrade compatibility is not a goal.

## Decision

1. Render a Pi extension that exposes one explicit `using_effort` tool and its private
   queued command. A call names an effort and explicitly chooses its managed-worktree or
   receiving-checkout destination; it never discovers an effort from presence, follows a
   checkout automatically, or creates a new conversation. The queued command uses Pi's
   command-only `changeCwd` API to rebind the same persisted session, then commits the new
   checkout fact through awf. Repeated calls switch the associated session between the two
   checkout roles. An explicit detach operation clears the owned activity claim and Remote
   publication without moving the session.

2. Associate one Pi session with at most one effort and let one effort publish at most one
   current session claim. The extension creates an ephemeral session-owner UUID and carries
   it in immutable association snapshots replaced at attach, checkout, heartbeat, ownership
   loss, and detach boundaries; it does not preserve correctness through mutable-field reset
   order. Publication data is derived anew from each binary result. Attaching to another
   effort first detaches the session's prior claim. Attaching over another session reports
   that claim's owner, checkout, attachment time, last heartbeat, and age; a stale claim is
   called out as stale, but stale and fresh takeovers both warn and proceed. Binary owner
   checks, rather than caller-remembered ordering, prevent an old session from accidentally
   updating or deleting its successor's claim. This is not a security boundary or lock.

3. Add optional mutable `.awf/efforts/<slug>/activity.json` beside immutable `state.json`
   and owned `memory.md`. The awf binary alone creates, atomically replaces, reads, and
   removes it. Its versioned bounded record contains the session owner, `attachedAt`,
   `heartbeatAt`, committed absolute CWD, recorded absolute receiving-checkout CWD, and
   checkout role (`managed` or `receiving`), with no redundant `active` flag, workflow
   phase, or copied memory text. The extension-facing grammar is:

   - `awf effort activity resolve <slug> --destination <managed|receiving> [--receiving-checkout <absolute-path>] --json`
   - `awf effort activity attach <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --receiving-checkout <absolute-path> --json`
   - `awf effort activity heartbeat <slug> --owner <uuid> --json`
   - `awf effort activity checkout <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --json`
   - `awf effort activity detach <slug> --owner <uuid> --json`

   Resolve is read-only and returns the validated destination, effort and memory metadata,
   and any prior claim. Attach atomically creates or takes over only after Pi reports the
   live CWD change; heartbeat, checkout, and detach require the current owner. Their protocol-1
   JSON envelope carries `schemaVersion`, a stable `condition`, current activity when present,
   and the operation-specific effort, memory, destination, or prior-claim facts. Handled
   conditions are `ready`, `attached`, `taken-over`, `heartbeat`, `checkout-updated`,
   `detached`, `not-owner`, and `missing`; ownership loss and missing effort are data, not
   error-prose branches.

   Every refusal that observes effort or repository state follows the actionable-outcome
   protocol: it uses the applicable closed category (`operation`, `topology`, or
   `repository-identity`), states the observed present-tense condition, carries a boolean for
   every activity, memory, or CWD axis the composite operation could have changed, gives an
   ordered independently executable remedy, and includes a cause exactly when a mechanism
   call failed. Unsafe resident, invalid header, repository mismatch, missing effort, and
   ownership mismatch are typed identities that the Go CLI and TypeScript extension consume
   without matching message substrings; their stable JSON condition remains available where
   the extension must branch. Malformed invocation and failures that observe no managed state
   exit nonzero with empty stdout and bounded actionable stderr, preserving the existing JSON
   CLI convention. Missing activity means no attachment claim.

4. Treat activity as advisory resident state. It never authorizes mutation, changes Git
   topology, changes effort lifecycle meaning, or blocks show, list, memory update,
   worktree, integrate, remove, finish, render, check, or any unrelated command. Finish may
   consume the optional activity resident through its existing proven-resident deletion
   transaction. A session that later observes the effort or ownership claim missing drops
   its local association and restores its base Remote Pi identity. Presence age is reported
   from `heartbeatAt`; policy and prose may describe an old claim as stale, but age never
   changes what an operation permits.

5. Resolve and validate both checkout roles through awf before Pi changes CWD. The managed
   destination is the native-Git registered worktree at `.awf/worktrees/<slug>/`, and the
   receiving destination is the linked checkout recorded when the session first attaches.
   Both must belong to the same repository primary control root and satisfy the existing
   no-follow, ownership, and confinement posture. A caller may explicitly supply the
   receiving checkout on first attachment. If first attachment begins inside the managed
   worktree and no receiving checkout has been recorded, only that `using_effort` call
   fails with instructions to supply one; awf never guesses the primary checkout. A failed
   runtime rebind does not commit the requested CWD or checkout role.

6. Heartbeat the owned association once per completed Pi turn and on a successful attach or
   checkout switch. Heartbeat failure is surfaced as advisory session status and Remote Pi
   metadata degradation, not as a model-turn or unrelated-tool failure. Detach is attempted
   on orderly extension shutdown. Restart begins detached, restores the configured Remote
   Pi base name, and never infers association from a resident left by an earlier process;
   an explicit later attachment performs the visible takeover.

7. Replace the custom memory header with closed YAML frontmatter containing exactly the
   keys `effort`, `phase`, `next`, and `updated`. `effort` is the immutable slug identity;
   `phase` and `next` are nonblank bounded single-line strings; and `updated` is a UTC
   timestamp. New effort skeletons start with this frontmatter and their real creation
   timestamp. The Markdown body and its canonical sections remain unchanged.

   Add `awf effort memory update <slug> [--phase <text>] [--next <text>]`. `--phase` and
   `--next` are independently optional and at least one is required. The operation preserves
   immutable effort identity, rewrites only the selected values, and always writes the
   current UTC instant to `updated`. On the first update of a legacy memory, it accepts the
   exact four-line `Effort:`, `Phase:`, `Next:`, and `Updated:` header, including the
   `Updated: Not yet updated.` sentinel, and atomically migrates it to canonical frontmatter.
   Serialization quotes values as needed rather than restricting otherwise valid punctuation.

8. Restrict the new memory-metadata gate to `using_effort`. Attachment reads the effort
   title from immutable state and validates either canonical closed frontmatter or the exact
   legacy four-line header, failing that tool call with the exact `awf effort memory update`
   repair when invalid. Subsequent metadata refresh may report memory metadata unavailable
   if the metadata was manually damaged, while preserving the activity association. No show,
   list, worktree, integrate, remove, finish, render, check, or other effort operation
   acquires this validation precondition. Handoff's bounded identity validation accepts both
   the legacy first-line `Effort: <slug>` and canonical frontmatter `effort: <slug>` during
   migration; it does not validate phase, next, or updated metadata.

9. Publish one advisory Remote Pi `awf` metadata snapshot containing the effort slug and
   title, validated memory Phase/Next/Updated, activity heartbeat, committed live CWD, and
   checkout role through the existing complete-replacement `remote-pi:metadata:set`
   contract. While attached, emit `remote-pi:name-override:set` with namespace `awf` and the
   effort slug; detach clears that namespace with a null value. The persisted `agent_name`
   remains the base identity, and the effective requested name is the awf override, then
   configured base, then CWD default. The title never enters the peer name.

   Detect `nameOverride.version === 1` and its `awf` namespace from
   `remote-pi:capabilities`, request capabilities when needed, and silently retain
   metadata-only behavior when override support is absent. On Remote Pi's metadata or name
   replay request, re-emit the complete current snapshot or still-active override. Consume
   `remote-pi:name-override:assigned` only for diagnostics; assigned names and addresses are
   mutable opaque values, never routing or lock inputs. Remote Pi serializes identity
   reconciliation so extension load order, reconnects, relay state, base-name changes, and
   broker collision suffixes converge. Replay is limited to an association held by the
   currently running extension process, such as after extension load-order races or Remote Pi
   reconnects. A Pi process restart always starts locally detached, restores the configured
   base name, and never infers or replays an association from a resident left by the prior
   process. Metadata and name publication failures never roll back the local awf association
   or CWD, never grant authority, and never become locks. Other peers may use the snapshot to hold a risky
   receiving-checkout mutation and contact the named peer, but that behavior remains
   voluntary.

10. Keep dependency direction explicit. The generated TypeScript extension orchestrates Pi
    and Remote Pi runtime APIs but does not write effort residents directly. It invokes the
    awf binary's activity and memory contracts and translates their structured outcomes at
    the runtime boundary. The effort package owns memory-frontmatter compatibility, mutation,
    and activity policy; the shared frontmatter package owns document splitting; Git
    topology remains behind awf's existing Git boundary; command code owns argument parsing
    and human rendering. Architecture and user documentation describe the optional resident,
    same-session CWD behavior, advisory Remote Pi contract, downgrade boundary, and recovery
    paths. The Implemented transaction updates the authoring sources for `README.md`,
    `AGENTS.md`, `docs/architecture.md`, `docs/working-with-awf.md`, `docs/workflow.md`,
    `docs/testing.md`, every declared claim and its provenance, and any other behavior-stating
    artifact found during implementation, then renders their generated outputs in the same
    commit.

11. Land prerequisites in owner order before awf advertises portable end-to-end runtime
    support. Pi has implemented, tested, documented, and committed SessionManager
    persistence/relocation, runtime and extension types, lifecycle, official TUI/RPC/print
    host bindings, and changelog at commit
    `f9447485497b12c100c2064c295c2c1beead0c29`. Remote Pi has implemented and committed the
    process-local override, capability, replay, status, identity-reconciliation, tests, and
    README contract at commit `3355463ff484bbd4fb80ada9fcd826dcb6ad6a53`. Both prerequisite
    owner contracts are therefore landed for awf development, but neither custom fork has a
    published minimum release yet.

    awf may implement and smoke-test against builds containing those commits while detecting
    their capabilities at runtime. It fills in and raises exact minimum versions only from
    the first published releases that contain them, never guesses release numbers, and does
    not claim portable support before those releases exist. Remote Pi's authoritative signal
    is `nameOverride.version === 1` with the `awf` namespace.
    Guarded awf code may merge earlier only when missing `ctx.changeCwd` leaves committed
    association/location unchanged and makes that `using_effort` call fail with one visible,
    actionable notice naming the required Pi capability and upgrade remedy. Capability
    detection remains mandatory even after awf raises its tested runtime floor. Missing
    Remote Pi override support still degrades to metadata-only through its negotiated
    compatibility contract. awf does not advertise live rebinding before Pi ships it or
    create a private compatibility shim for either owner contract. The two prerequisite
    repositories may land in either order.

12. Catalog a `using-effort` support skill for new adopters and explicitly enable it in this
    repository. Existing adopters receive the capability through normal catalog availability
    and documented `awf enable skill using-effort`; upgrade does not silently mutate their
    enabled-skill selection. Add the new generated extension to the containerized TypeScript
    strict-check and 100 percent line/function/branch coverage lane, and extend pinned
    real-runtime smoke to prove same-session CWD persistence, legacy-memory compatibility and
    migration, frontmatter metadata publication, capability replay, transient-name
    restoration, and advisory failure behavior. Render
    coverage also exercises every new template with empty optional values, proving coherent
    missingkey-zero output with no `<no value>` token.

13. Add `rendering/pi-workflows:pi-effort-session-association` as `Backing: test`. The
    implementation transaction places its exact proof marker in a Go catalog/render spine
    test named `TestPiEffortSessionAssociationContract`, which verifies the public tool,
    support skill, binary orchestration boundary, advisory semantics, and generated output.

## State changes

- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- add `rendering/pi-workflows:pi-effort-session-association`

## Consequences

A user can deliberately keep one conversation while moving its live Pi runtime into the
checkout appropriate to the current effort step. Peers gain enough current context to
coordinate receiving-checkout mutations with the right session, and the memory update
command makes the published checkpoint fields reliable without making working memory a
global machine-owned document. Memory metadata becomes ordinary closed frontmatter rather
than an awf-specific pseudo-header, while dual-format reads and first-update migration keep
existing efforts usable.

The design adds a mutable effort resident, binary mutation protocol, generated extension,
skill, external capability negotiation, and heartbeat lifecycle. Crash leftovers and
concurrent takeovers are unavoidable; visible prior-claim diagnostics, owner-checked writes,
atomic replacement, restart-detached behavior, and nonblocking semantics bound their harm.
The peer name can collide with another peer and Remote Pi may disambiguate its displayed
name; the effort slug and metadata, not an assumed unique display name, carry identity.

An attached session can remain pointed at a checkout whose topology changes underneath it.
That is intentional: neither integration nor removal silently moves a conversation. The
next explicit switch is revalidated, heartbeat exposes the committed location, and normal
filesystem or Git failures remain visible to the agent.

Older awf binaries may reject an effort containing `activity.json`. Users must upgrade
rather than downgrade or manually relocate the resident. Existing adopters must explicitly
enable the support skill. awf implementation and release are sequenced behind two external
owner releases, which delays the feature but avoids coupling shipped behavior to provisional
APIs.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Automatically select the current effort or follow worktree lifecycle transitions | Hidden CWD changes and inferred ownership would be surprising and could target the wrong checkout. |
| Use fresh-session handoff to enter every checkout | It loses the same-conversation property and makes checkout movement depend on a checkpoint boundary. |
| Treat activity as a lock or refuse fresh takeovers | Presence is fallible telemetry; making it authority would strand work after crashes and create an unrelated gate. |
| Store activity outside the effort directory for downgrade compatibility | It splits one concern across resident roots and makes cleanup and ownership less cohesive; downgrade support is not promised. |
| Let the TypeScript extension write `activity.json` and parse memory directly | It duplicates awf policy, bypasses resident safety, and lets runtime representation own effort state. |
| Keep the custom four-line memory header as the permanent structured format | Once awf parses and mutates the fields, the bespoke frontmatter-like grammar has no advantage over the repository's existing closed YAML frontmatter machinery. |
| Require immediate repository-wide migration to memory frontmatter | It would break live efforts and turn a supportive metadata feature into an unrelated compatibility gate; dual reads and first-update migration are sufficient. |
| Persist workflow Phase/Next in `activity.json` | It creates competing mutable copies of checkpoint truth; memory remains their single source. |
| Guess the primary checkout when attaching from a managed worktree | The primary control root is not necessarily the intended receiving checkout. |
| Add an `active` boolean | File existence already expresses the attachment claim, and the boolean would have no useful false state. |
| Make all effort commands validate the memory header | A narrow metadata feature would become a repository-wide operational gate contrary to the supportive design. |
| Persist the effort slug as the configured Remote Pi name | The override is session-scoped coordination context, not user configuration, and must disappear on detach or restart. |
| Implement private Pi or Remote Pi compatibility shims in awf | The owning projects have settled prerequisite shapes; duplicating provisional runtime behavior would create competing contracts. |

## Status history

- 2026-08-02: Proposed
