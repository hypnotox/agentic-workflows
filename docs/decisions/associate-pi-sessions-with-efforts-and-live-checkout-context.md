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

Effort memory already carries the useful checkpoint fields `Phase:`, `Next:`, and
`Updated:`, but production code intentionally does not parse its headings. Remote peer
metadata needs those fields in a bounded canonical form. Existing untouched memories can
also contain the legacy `Updated: Not yet updated.` sentinel. The new parser therefore
needs a deliberately narrow header contract without turning all memory reads or all effort
commands into validation gates.

The runtime behavior crosses three ownership boundaries:

- awf owns effort residents, memory validation and mutation, activity transitions, checkout
  discovery, and the generated `using_effort` extension and support skill;
- Pi owns changing the current directory of a live persisted session without creating a
  new conversation; and
- Remote Pi owns peer identity, capability negotiation, replay, and publication of the
  atomically replaced `awf` metadata namespace.

Pi's prerequisite design provides a command-context-only `changeCwd` operation. It replaces
the same session runtime, writes the destination into the session header, relocates the
configured default directory when applicable, gives extensions a fresh replacement context,
trusts the caller-selected destination, and is detected by API presence. Remote Pi's
prerequisite design provides an awf-only transient peer-name override, capability
negotiation and replay requests, serialized identity reconciliation, and continued atomic
replacement of the existing `awf` metadata namespace. awf must consume those owner-provided
contracts rather than duplicate them or depend on an unlanded shape.

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
   current session claim. The extension creates an ephemeral session-owner UUID and keeps
   it for owner-checked heartbeat, checkout-update, and detach calls. Attaching to another
   effort first detaches the session's prior claim. Attaching over another session reports
   that claim's owner, checkout, attachment time, last heartbeat, and age; a stale claim is
   called out as stale, but stale and fresh takeovers both warn and proceed. This protocol
   prevents an old session from accidentally updating or deleting its successor's claim;
   it is not a security boundary or lock.

3. Add optional mutable `.awf/efforts/<slug>/activity.json` beside immutable `state.json`
   and owned `memory.md`. The awf binary alone creates, atomically replaces, reads, and
   removes it. Its versioned bounded record contains the session owner, `attachedAt`,
   `heartbeatAt`, committed absolute CWD, and checkout role (`managed` or `receiving`),
   with no redundant `active` flag, workflow phase, or copied memory text. Attach/takeover,
   heartbeat, checkout-update, and detach are explicit binary-owned operations; the latter
   three require the current owner. Missing activity means no attachment claim. Every
   transition reports a structured outcome suitable for extension action and human
   diagnostics.

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

7. Add `awf effort memory update <slug> [--phase <text>] [--next <text>]`. `--phase` and
   `--next` are independently optional and at least one is required. The operation preserves
   immutable `Effort: <slug>`, rewrites only the selected canonical single-line header
   fields, and always writes the current UTC instant to `Updated:`. It accepts the legacy
   `Updated: Not yet updated.` sentinel and normalizes it on update. New effort skeletons
   start with their real UTC creation timestamp. Values are nonblank, bounded, valid UTF-8
   single lines, and the header contains exactly one canonical occurrence of each required
   field before `## Brief`.

8. Restrict the new memory-header gate to `using_effort`. Attachment reads the effort title
   from immutable state and validates the canonical `Effort:`, `Phase:`, `Next:`, and
   `Updated:` header, failing that tool call with the exact `awf effort memory update`
   repair. A valid legacy Updated sentinel is accepted. Subsequent metadata refresh may
   report memory metadata unavailable if the header was manually damaged, while preserving
   the activity association. No show, list, worktree, integrate, remove, finish, render,
   check, handoff, or other effort operation acquires this validation precondition.

9. Publish one advisory Remote Pi `awf` metadata snapshot containing the effort slug and
   title, validated memory Phase/Next/Updated, activity heartbeat, committed live CWD, and
   checkout role. While attached, request a transient peer-name override equal to the
   effort slug; on detach, missing ownership, shutdown, or restart, remove the override and
   reconcile to the configured base name. Use Remote Pi capability negotiation, replay
   requests, and serialized identity reconciliation so extension load order and reconnects
   converge. Metadata and name publication failures never roll back the local awf
   association or CWD, never grant authority, and never become locks. Other peers may use
   the snapshot to hold a risky receiving-checkout mutation and contact the named peer, but
   that behavior remains voluntary.

10. Keep dependency direction explicit. The generated TypeScript extension orchestrates Pi
    and Remote Pi runtime APIs but does not write effort residents directly. It invokes the
    awf binary's activity and memory contracts and translates their structured outcomes at
    the runtime boundary. The effort package owns memory-header and activity policy; Git
    topology remains behind awf's existing Git boundary; command code owns argument parsing
    and human rendering. Architecture and user documentation describe the optional resident,
    same-session CWD behavior, advisory Remote Pi contract, downgrade boundary, and recovery
    paths.

11. Land prerequisites in owner order before awf's end-to-end runtime integration: first
    Pi accepts, implements, tests, and releases the live `changeCwd` contract; independently,
    Remote Pi accepts, implements, tests, and releases transient awf peer-name override plus
    capability/replay support; only after both published contracts are available does awf
    update its pinned Pi test/runtime floor and Remote Pi integration expectation, render the
    extension, and add real-runtime smoke coverage. The two prerequisite repositories may
    land in either order, but awf does not ship a private compatibility shim or guess their
    final APIs.

12. Catalog a `using-effort` support skill for new adopters and explicitly enable it in this
    repository. Existing adopters receive the capability through normal catalog availability
    and documented `awf enable skill using-effort`; upgrade does not silently mutate their
    enabled-skill selection. Add the new generated extension to the containerized TypeScript
    strict-check and 100 percent line/function/branch coverage lane, and extend pinned
    real-runtime smoke to prove same-session CWD persistence, memory metadata publication,
    capability replay, transient-name restoration, and advisory failure behavior.

## State changes

- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`
- add `rendering/pi-workflows:pi-effort-session-association`
- update `rendering/pi-workflows:pi-native-workflow-skills`

## Consequences

A user can deliberately keep one conversation while moving its live Pi runtime into the
checkout appropriate to the current effort step. Peers gain enough current context to
coordinate receiving-checkout mutations with the right session, and the memory update
command makes the published checkpoint fields reliable without making working memory a
global machine-owned document.

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
| Persist workflow Phase/Next in `activity.json` | It creates competing mutable copies of checkpoint truth; memory remains their single source. |
| Guess the primary checkout when attaching from a managed worktree | The primary control root is not necessarily the intended receiving checkout. |
| Add an `active` boolean | File existence already expresses the attachment claim, and the boolean would have no useful false state. |
| Make all effort commands validate the memory header | A narrow metadata feature would become a repository-wide operational gate contrary to the supportive design. |
| Persist the effort slug as the configured Remote Pi name | The override is session-scoped coordination context, not user configuration, and must disappear on detach or restart. |
| Implement private Pi or Remote Pi compatibility shims in awf | The owning projects have settled prerequisite shapes; duplicating provisional runtime behavior would create competing contracts. |

## Status history

- 2026-08-02: Proposed
