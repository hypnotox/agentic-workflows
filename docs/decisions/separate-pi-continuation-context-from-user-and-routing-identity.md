---
format: current-state-v4
slug: separate-pi-continuation-context-from-user-and-routing-identity
status: Implementing
date: 2026-08-04
---
# ADR-separate-pi-continuation-context-from-user-and-routing-identity: Separate Pi continuation context from user and routing identity


## Context

Pi continuation currently crosses two identity boundaries incorrectly.

The generated handoff extension submits its bounded kickoff through
`ReplacedSessionContext.sendUserMessage`. Pi consequently persists the kickoff as a user message,
and the next agent can interpret agent-authored continuation instructions as new user authority.
Pi also offers replacement-bound custom messages with distinct durable and visible identity, but
awf cannot rely on transport-level attribution alone to distinguish handoff context from user input.
A textual ownership envelope is therefore still required for the model-facing content.

The generated effort-association extension publishes the effort slug through the routing-affecting
`remote-pi:name-override:*` event family. That conflicts with the advisory nature of effort
association: metadata already carries the machine-readable effort fact, while temporary effort text
exists only for human presentation. awf must stop publishing presentation through an identity
interface and must never reconstruct or replace a peer's assigned base name.

The external integration exposes a display-suffix capability and event surface that awf can consume
without importing a foreign package. A factory-time capability request alone can miss a response, so
awf needs a deterministic `session_start` retry in addition to its initial request.

This decision governs only awf's generated handoff and effort extensions, their tests, and their
published workflow semantics. The external provider's implementation, architecture, validation,
review, and release policy remain outside this repository's decision record and plan.

## Decision

1. `decision: agent-owned-handoff-context` Represent automatic fresh-session kickoff as a visible
   custom message of type `agent-handoff`, not as a user message. Use Pi's default renderer, which
   supplies the single `[agent-handoff]` label, with no custom renderer or duplicate content label.
   Its model-facing content begins with the exact prefix `Agent-authored handoff context; this is not
   user input:` before carrying the exact accepted kickoff. The replacement-bound custom-message API
   triggers the continuation turn. Editor recovery carries the same ownership envelope, while the
   public bounded `{kickoff}` input and the existing queue, countdown, lineage, cleanup, and
   no-silent-retry boundaries remain intact. This ownership envelope remains load-bearing because
   awf does not rely on transport-level non-user attribution; changing that boundary requires a later
   decision.

2. `decision: presentation-not-routing-identity` Stop emitting the routing-affecting name-override
   event family. awf keeps complete effort metadata as the machine-readable association and treats
   optional temporary text only as presentation data. awf never reads, reconstructs, replaces, or
   composes a peer's assigned name; never turns a presentation value into routing, lifecycle, or lock
   authority; and never falls back to name override when suffix support is unavailable.

3. `decision: display-suffix-capability` Consume the external version-1 display-suffix event surface
   through awf-owned structural types. awf emits payload-free `remote-pi:capabilities:request` and
   consumes complete `remote-pi:capabilities` snapshots. Support is present only when the
   `displaySuffix` member is exactly `{version: 1}`; unrelated members, including independent
   metadata support, remain permitted. awf emits `remote-pi:display-suffix:set` with exactly
   `{value: string}` for its current canonical effort slug or `{value: null}` to clear, and listens
   for payload-free `remote-pi:display-suffix:request`. The display-suffix event family carries no
   namespace, producer identity, base name, or composite name. awf imports no foreign runtime or
   package, and missing, malformed, or failing optional integration degrades silently to metadata
   without restoring name replacement.

4. `decision: authoritative-capability-replay` Treat each received
   `remote-pi:capabilities` event as a complete authoritative snapshot for awf's support state. No
   response leaves the consumer initially unsupported; an actual snapshot with an absent or
   malformed `displaySuffix` member withdraws support and makes awf synchronously emit
   `{value: null}`. At factory load awf installs its capability and replay listeners before emitting
   the capability request. On every awf `session_start`, including a replacement, awf clears its
   association, emits null metadata and an unconditional suffix clear, then requests capabilities
   again. Successful attach and late support publish the current effort slug; successful detach or
   switch-detach, ownership loss, shutdown, and replay without a supported current association
   publish null. A replay request is answered synchronously with exactly awf's current string or
   null, without a promise or timer. Every optional event emission failure leaves awf association,
   context, heartbeat, tool, and metadata behavior intact.

5. `decision: independently-owned-delivery` Test and document only awf's emitted and consumed event
   shapes, replay behavior, metadata independence, name-override removal, and graceful degradation.
   Do not encode another repository's paths, internal symbols, implementation mechanics, tests,
   commands, commits, review policy, or validation obligations as awf authority. Changed generated
   templates preserve coherent generic output under missingkey=zero when variables are empty and
   never emit `<no value>` or an equivalent unresolved-value token.

## State changes

- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-effort-session-association`
- update `rendering/pi-workflows:using-effort-skill`

## Consequences

A replacement transcript distinguishes prior-agent continuation from user authority. The custom
message remains visible and durable, the provider receives an explicit ownership statement, and
manual recovery retains the same semantics. The accepted kickoff is still bounded and exact inside
the envelope, but the complete submitted message is intentionally no longer byte-for-byte equal to
the public input.

Effort attachment continues to publish complete machine-readable metadata while awf no longer emits
name override. When the external capability snapshot declares version-1 suffix support, awf emits
only its canonical effort slug or an explicit null through the display-suffix event. When support is
missing, malformed, withdrawn, or failing, awf continues in metadata-only mode. Routing targets and
identity remain outside awf's presentation publication.

Factory-time and session-start negotiation plus synchronous replay make awf publication deterministic
across extension load order and replacement. Explicit null clears prevent stale awf presentation
without relying on process-local publication flags. These lifecycle mechanics add no authority to
advisory presence and do not affect effort attachment, context injection, heartbeat, or detach.

The event surface is an external input to awf, not an authorization to govern its provider. awf's
fixture and tests prove only this repository's structural use of that surface and do not claim a
package dependency, provider implementation, or cross-repository completion obligation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `sendUserMessage` and improve kickoff wording | The persisted transcript still attributes agent-authored continuation to the user. |
| Add a new upstream Pi provider message role | It would provide stronger transport-level separation but broadens this correction when a visible custom message and explicit envelope meet the required semantics. |
| Keep publishing the effort slug through name override | Optional presentation would continue to use a routing-affecting identity interface. |
| Let awf read a base name and append the slug itself | awf would take ownership of foreign identity and presentation composition that it does not own. |
| Fall back to name override when display suffix is unsupported | Stable routing boundaries are more important than retaining optional effort presentation. |
| Use effort metadata alone with no optional suffix publication | It preserves routing but removes the user-facing glanceable association supported by the external event surface. |

## Status history

- 2026-08-04: Proposed
- 2026-08-04: Implementing; content-sha256: 72b472a2ed203f20eb3aac4e9a1927574f05c3f9fb269b4c8637734ce1c54434
- 2026-08-04: Applied; operations: update `rendering/pi-workflows:pi-session-handoff-lifecycle`, update `rendering/pi-workflows:pi-session-handoff-public-contract`
- 2026-08-04: Amended; content-sha256: 5590e155c7272cf2ebddcdd497e318de145f875dd9edc42c8ff49595df5ab836
