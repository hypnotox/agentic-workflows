---
format: current-state-v3
slug: associate-pi-sessions-with-efforts-and-live-checkout-context
status: Implementing
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
  discovery, the cross-target `effort-workflow` skill, and the generated Pi-only
  `using_effort` extension and skill;
- Pi owns changing the current directory of a live persisted session without creating a
  new conversation; and
- Remote Pi owns peer identity, capability negotiation, replay, and publication of the
  atomically replaced `awf` metadata namespace.

Effort guidance has two layers. Every enabled target can use target-neutral workflow
instructions to enter the exact existing awf-managed worktree with its native persistent
checkout or context tooling. Only Pi has the runtime API and transient session semantics
needed to associate a session, publish activity, and rebind the same conversation. A
non-Pi target must not receive Pi tooling, learn a `using_effort` tool, or create a parallel
harness-owned worktree beside awf's managed topology.

Pi provides command-context-only
`ExtensionCommandContext.changeCwd(targetCwd, options)`. It replaces the same session
runtime, writes the destination into the session header, relocates default session storage
when applicable, gives extensions a fresh `ReplacedSessionContext`, and runs destination
trust. Remote Pi provides an awf-only process-local peer-name override, capability
negotiation and replay requests, serialized identity reconciliation, and atomic replacement
of the existing `awf` metadata namespace. awf develops against locally owned structural
foreign interfaces for these capabilities and detects their presence at runtime. Package
publication, release versions, and installation topology are not capability authority.

Activity is stored beside the effort that owns it. This makes the ownership boundary
cohesive but intentionally creates a one-way compatibility boundary: after
`activity.json` exists, an older awf binary is not required to read or finish that effort.
Downgrade compatibility is not a goal.

## Decision

1. When the selected effort workflow is rendered for an enabled Pi target, also render a
   Pi extension that exposes one explicit `using_effort` tool and its private queued
   command. A call names an effort and explicitly chooses its managed-worktree or
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
   exact four-line `Effort:`, `Phase:`, `Next:`, and `Updated:` header and atomically migrates
   it to canonical frontmatter. Legacy identity must match the slug, Phase and Next satisfy
   the canonical nonblank bounds, and Updated is either a UTC timestamp or the exact
   `Not yet updated.` sentinel. Attachment publishes that sentinel verbatim until a structured
   update normalizes it. Serialization quotes values as needed rather than restricting
   otherwise valid punctuation.

   The update operation may safely repair a missing or invalid Phase or Next only when the
   corresponding flag supplies its replacement, and it always repairs Updated. Repair still
   requires a bounded recognizable metadata boundary and an unambiguous effort identity that
   matches the command slug; it never guesses or changes identity. Unsafe structure or
   identity requires manual repair rather than destructive normalization.

8. Restrict the new memory-metadata gate to `using_effort`. Attachment reads the effort
   title from immutable state and validates either canonical closed frontmatter or the exact
   legacy four-line header. For repairable metadata it gives the complete `awf effort memory
   update` invocation, including every required replacement flag. For unsafe structure or
   identity it gives bounded manual-repair instructions instead of claiming the command can
   repair it. Subsequent metadata refresh may report memory metadata unavailable if the
   metadata was manually damaged, while preserving the activity association. No show,
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

10. Keep dependency direction explicit. The workflow document and shared checkpoint
    partials remain the single policy home for effort identity, memory, managed-worktree,
    review, integration, removal, retrospective, and finish semantics. The cross-target
    `effort-workflow` skill is the operational entry guide that composes and points to that
    shared policy while adding no competing lifecycle rules or runtime-specific association
    behavior. The generated Pi-only TypeScript extension orchestrates Pi and Remote Pi runtime APIs but
    does not write effort residents directly. It invokes the awf binary's activity and
    memory contracts and translates their structured outcomes at the runtime boundary. The
    effort package owns memory-frontmatter compatibility, mutation,
    and activity policy; the shared frontmatter package owns document splitting; Git
    topology remains behind awf's existing Git boundary; command code owns argument parsing
    and human rendering. Architecture and user documentation describe the optional resident,
    same-session CWD behavior, advisory Remote Pi contract, downgrade boundary, and recovery
    paths. The Implemented transaction updates the authoring sources for `README.md`,
    `AGENTS.md`, `docs/architecture.md`, `docs/working-with-awf.md`, `docs/workflow.md`,
    `docs/testing.md`, every declared claim and its provenance, and any other behavior-stating
    artifact found during implementation, then renders their generated outputs in the same
    commit.

11. Define the foreign runtime interfaces awf develops against without importing, pinning,
    or publishing either owner package. The Pi interface is an optional command-context
    `changeCwd` method with its replacement-session callback and structured result. The
    Remote Pi interface is a closed set of awf-owned structural event payloads for metadata
    replacement, capability discovery, replay, transient name override, and assigned-name
    diagnostics. Owner commits `f9447485497b12c100c2064c295c2c1beead0c29` in Pi and
    `3355463ff484bbd4fb80ada9fcd826dcb6ad6a53` in Remote Pi record the implementation
    provenance for those interface shapes; they do not establish a package, release, version,
    artifact, checksum, or installation prerequisite.

    Detect `typeof ctx.changeCwd === "function"` immediately before a queued switch. When it
    is absent, leave CWD, activity, and memory unchanged and emit one visible actionable
    refusal naming the missing capability and reload remedy. Detect Remote Pi through the
    optional event bus and require `nameOverride.version === 1` with the `awf` namespace
    before requesting a transient name. If the event bus is absent, local effort association,
    switching, heartbeat, and detach continue without peer publication. If metadata exists
    without name override, publication remains metadata-only. Replay, metadata, and naming
    failures are advisory. Capability presence, not a foreign package version or publication
    channel, is the compatibility boundary.

12. Catalog a core cross-target `effort-workflow` support skill as the single user-facing
    selection knob. New project scaffolds enable it by default; existing adopters retain
    their current selection and opt in once with `awf enable skill effort-workflow`. Every
    enabled target renders that general skill. It directs non-Pi runtimes to use native
    persistent checkout or context tooling only to enter the exact awf-managed worktree,
    never to create a parallel harness-owned worktree, claim activity, or invoke a Pi effort
    tool.

    When and only when `effort-workflow` is selected and Pi is enabled, derive two Pi
    target-owned outputs from the same selection: the `using-effort` skill and the
    `awf-effort` extension. `using-effort` is not a second catalog selection and never renders
    for a non-Pi target. The target output declaration, render, prune, and drift paths use the
    same predicate so partial output cannot persist. Add the extension to the containerized
    TypeScript strict-check and 100 percent line/function/branch coverage lane. awf-owned
    contract fixtures prove the structural Pi and Remote Pi interfaces, same-session CWD
    transfer, legacy-memory compatibility and migration, frontmatter metadata publication,
    capability replay, transient-name restoration, complete Remote Pi absence, and advisory
    failure behavior. The existing pinned real-runtime smoke remains scoped to its retained
    subagent, handoff, skill-discovery, and routing contracts; `using_effort` does not add a
    foreign release prerequisite to that lane. Render coverage also exercises every new
    template with empty optional values, proving coherent missingkey-zero output with no
    `<no value>` token.

13. Add `rendering/pi-workflows:pi-effort-session-association`,
    `rendering/workflow-skill-templates:effort-workflow`, and
    `rendering/pi-workflows:using-effort-skill` as `Backing: test`. The implementation
    transaction places their exact proof markers in Go catalog/render tests. The proofs
    verify the public Pi tool and skill, binary orchestration boundary, advisory semantics,
    the single selection knob, scaffold-only default adoption, cross-target native use of
    the existing awf-managed worktree, Pi-only derived output, and non-Pi absence.

## State changes

- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- add `rendering/pi-workflows:pi-effort-session-association`
- update `rendering/project-output-plan:multi-target-render`
- update `rendering/catalog-and-targets:target-dialect-render`
- update `rendering/pi-workflows:pi-native-workflow-skills`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- add `rendering/workflow-skill-templates:effort-workflow`
- add `rendering/pi-workflows:using-effort-skill`

## Consequences

A user can deliberately keep one conversation while moving its live Pi runtime into the
checkout appropriate to the current effort step. Peers gain enough current context to
coordinate receiving-checkout mutations with the right session, and the memory update
command makes the published checkpoint fields reliable without making working memory a
global machine-owned document. Memory metadata becomes ordinary closed frontmatter rather
than an awf-specific pseudo-header, while dual-format reads and first-update migration keep
existing efforts usable.

The design adds a mutable effort resident, binary mutation protocol, generated extension,
two-layer skill model, target-owned output predicate, external capability negotiation, and
heartbeat lifecycle. New project scaffolds receive the core `effort-workflow` skill by
default, while existing adopters opt in without upgrade changing their selections. One
selection renders target-neutral existing-worktree guidance everywhere and derives the Pi
skill and extension only for Pi; non-Pi targets remain native-tool-oriented and never gain
Pi association semantics or parallel harness-owned topology. Crash leftovers and
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
enable `effort-workflow` once. Foreign runtime capabilities may be absent from any particular
installation; structural detection and bounded degradation keep that absence local to the
optional integration instead of coupling awf support to a publication channel.

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
| Implement private Pi or Remote Pi behavior shims in awf | awf owns only the structural consumption interfaces and degradation policy; reproducing foreign runtime behavior would create competing implementations. |
| Require published Pi or Remote Pi packages and exact release pins | Capability presence is the runtime compatibility boundary; publication topology is unrelated and custom local extensions are valid providers. |
| Catalog or render `using-effort` for every target | It leaks Pi-only runtime semantics and an unavailable tool into non-Pi guidance. |
| Let each harness create a parallel worktree for the effort | The existing awf-managed worktree is the one workflow topology; competing topology would obscure authority and cleanup. |
| Expose separate selection knobs for `effort-workflow`, `using-effort`, and the Pi extension | Independent toggles permit incoherent partial output; one workflow selection can derive its Pi-owned companion outputs. |
| Auto-enable the new core skill while upgrading existing adopters | Core defines the default for new scaffolds, not permission to mutate committed adopter selections. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Accepted; content-sha256: 7ce4df8ed04b71c447ecd1ed0bf0d7b50620b5107b71b358d2fd374d715d7d51
- 2026-08-02: Amended; content-sha256: 13c681481627dbe1c084409ad708ea8c0b803374cf143dea285b1ce47b50f4b0
- 2026-08-02: Implementing; content-sha256: 13c681481627dbe1c084409ad708ea8c0b803374cf143dea285b1ce47b50f4b0
- 2026-08-02: Applied; operations: update `tooling/cli:effort-command-contract`, update `tooling/effort-management:effort-record-authority`, update `tooling/effort-management:memory-skeleton-purpose-partition`, update `rendering/pi-workflows:pi-session-handoff-lifecycle`, update `rendering/pi-workflows:pi-session-handoff-public-contract`, update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- 2026-08-02: Amended; content-sha256: ea7654729e8ad4a3ff4a825a8634a65bfaa956fd43ab7577dc0d30a7ffcd4ac2
- 2026-08-02: Applied; operations: update `rendering/pi-runtime:pi-extension-target-render`, add `rendering/pi-workflows:pi-effort-session-association`, update `rendering/project-output-plan:multi-target-render`, update `rendering/catalog-and-targets:target-dialect-render`, update `rendering/pi-workflows:pi-native-workflow-skills`, update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`, add `rendering/workflow-skill-templates:effort-workflow`, add `rendering/pi-workflows:using-effort-skill`
- 2026-08-02: Amended; content-sha256: 6b904e7e26609ecf60c7b2ff95fe76a4ad24cfc58fceb23e061083d1dca70971
