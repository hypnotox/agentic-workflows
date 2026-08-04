---
format: current-state-v4
slug: separate-pi-continuation-context-from-user-and-routing-identity
status: Proposed
date: 2026-08-04
---
# ADR-separate-pi-continuation-context-from-user-and-routing-identity: Separate Pi continuation context from user and routing identity


## Context

Pi continuation currently crosses two identity boundaries incorrectly.

The generated handoff extension submits its bounded kickoff through
`ReplacedSessionContext.sendUserMessage`. Pi consequently persists the kickoff as a user message,
and the next agent can interpret agent-authored continuation instructions as new user authority.
Pi also offers replacement-bound custom messages. They persist with a distinct custom role and
visible type, but the current provider adapter converts their content to a user-role provider
message. A textual ownership envelope is therefore still required if the model is to distinguish
handoff context from user input without a new upstream Pi message role.

The generated effort-association extension publishes the effort slug through Remote Pi's
`name-override` interface. Remote Pi uses that effective name for both presentation and operational
identity, including the broker address and relay room key. Attaching an effort can therefore replace
a collision-assigned name such as `awf#5` with the effort slug and change the address used by
independent peers. That conflicts with the advisory nature of effort metadata: metadata already
identifies the associated effort, while the temporary effort text exists only to help a person see
what the peer is doing.

Capability discovery also has a lifecycle race. The effort extension requests Remote Pi
capabilities during factory evaluation. If that request precedes Remote Pi listener registration,
the response is lost. A fresh handoff session can then attach to an effort while retaining no
presentation capability until another event happens to repair the state. Pi emits `session_start`
after extension factories have completed and before replacement-bound continuation work, providing
a deterministic second negotiation point.

The desired boundary spans independently owned systems. awf owns the generated handoff and effort
consumers, their workflow claims, and its structural foreign-interface fixture. Remote Pi owns the
normative presentation capability, broker identity, relay room identity, display composition, and
application projection. Neither package publication nor an imported runtime dependency should
become capability authority.

## Decision

1. `decision: agent-owned-handoff-context` Represent automatic fresh-session kickoff as a visible
   custom message of type `agent-handoff`, not as a user message. Use Pi's default renderer, which
   supplies the single `[agent-handoff]` label, with no custom renderer or duplicate content label.
   Its model-facing content begins with the exact prefix `Agent-authored handoff context; this is not
   user input:` before carrying the exact accepted kickoff. The replacement-bound custom-message API
   triggers the continuation turn. Editor recovery carries the same ownership envelope, while the
   public bounded `{kickoff}` input and the existing queue, countdown, lineage, cleanup, and
   no-silent-retry boundaries remain intact. This ownership envelope remains load-bearing while Pi
   custom messages serialize to a provider user role; an upstream non-user role may replace it only
   through a later decision.

2. `decision: presentation-not-routing-identity` Keep Remote Pi effort presentation separate from
   routable identity. Effort attachment never changes the configured or broker-assigned peer name,
   opaque mesh address, lock identity, or relay room identity. Remote Pi alone composes the stable
   assigned base and advisory effort suffix for human-facing surfaces. Effort metadata remains the
   machine-readable association fact, and neither the suffix nor its composed label becomes routing
   authority, lifecycle authority, or a lock.

3. `decision: display-suffix-capability` Replace awf's use of the routing-affecting name override
   with a distinct capability-gated display-suffix interface. Remote Pi is the normative owner of
   version 1, including the supported `awf` namespace, set and explicit-null clear payloads, replay
   request, bounded validation, composition, and presentation projection. awf publishes only its
   canonical effort slug and pins the exact structural interface with Remote Pi owner-commit
   provenance; it does not import or version-gate a Remote Pi package. Unsupported, malformed, or
   failing integration degrades silently to metadata without falling back to name replacement.

4. `decision: authoritative-capability-replay` Treat each received Remote Pi capabilities event as
   a complete authoritative singleton-provider snapshot. No response leaves the consumer in its
   initial unsupported state; an actual snapshot with absent or malformed display-suffix support
   withdraws that support and clears active presentation. Request capabilities during factory load
   and again at `session_start`, publish current state when support arrives late, accept idempotent
   explicit-null clears, clear unconditionally at session lifecycle boundaries, and request
   synchronous producer replay after Remote Pi starts a replacement session. These mechanics are
   load-bearing because either side may be reused or loaded first and process-local publication
   flags cannot prove that foreign presentation state is clear.

5. `decision: independently-owned-delivery` Keep dependency direction explicit across the two
   repositories. awf specifies only the consumed event shapes, replay and degradation behavior, and
   generated workflow semantics. Remote Pi specifies stable identity and room behavior plus the
   user-facing projection. Each repository proves its side and pins matching contract fixtures;
   neither treats the other repository's internal identity, reconnect, collision, notification, or
   rendering mechanisms as local authority. Changed generated templates preserve coherent generic
   output under missingkey=zero when variables are empty and never emit `<no value>` or an equivalent
   unresolved-value token.

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

Effort attachment no longer changes peer addresses or relay room identity. A person can see a label
such as `awf#5 - pi-handoff-identity` while agents continue routing to the stable opaque address
ending in `@awf#5`. Suffix changes can update presentation in place. Older Remote Pi installations
lose the former temporary rename rather than retaining a behavior that destabilizes routing;
metadata continues to expose the effort association.

The capability protocol gains snapshot semantics, a new event family, replay behavior, and
cross-repository provenance. Both extension lifecycles must clear and replay advisory state, and
Remote Pi must separate presentation helpers from identity helpers. These additions are more
explicit than the old override but remove hidden coupling between a human label and operational
identity.

Delivery requires coordinated but independently governed changes. Either consumer or provider can
land first because absence degrades to metadata-only behavior, but complete presentation appears
only when both sides implement the shared structural contract. awf documentation states the foreign
boundary without claiming authority over Remote Pi internals.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `sendUserMessage` and improve kickoff wording | The persisted transcript still attributes agent-authored continuation to the user. |
| Add a new upstream Pi provider message role | It would provide stronger transport-level separation but broadens this correction into a Pi protocol change when a visible custom message and explicit envelope meet the required semantics. |
| Send the effort slug as a complete name override | It replaces the assigned base and changes mesh and relay identity, making optional presentation affect routing. |
| Let awf read the current assigned name and append the slug | The assigned name is mutable Remote Pi state; reconstructing it in awf races startup, collision assignment, rename, reconnect, and failover and can double-apply decoration. |
| Extend name override with a suffix mode | It overloads an identity contract with presentation-only semantics and preserves coupling that this decision removes. |
| Fall back to the old name override when display suffix is unsupported | Stable routing is more important than retaining optional effort presentation on an older integration. |
| Use effort metadata alone with no visible suffix | It preserves routing but removes the user-facing glanceable association that motivated temporary presentation. |

## Status history

- 2026-08-04: Proposed
