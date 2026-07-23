---
format: current-state-v2
status: Proposed
date: 2026-07-23
---
# ADR-0152: Checkpoint, check-in, and handoff semantics


## Context

ADR-0145 (Implemented, frozen) made every workflow phase boundary complete a durable working-memory checkpoint, display a visible summary, and treat that summary as "the user's intervention opportunity", with Pi guidance invoking `handoff_session` automatically after each safe checkpoint. In practice Pi agents update working memory and print the checkpoint summary but then fail to invoke `handoff_session`, most often at implementation task boundaries. The root cause is semantic: the current guidance conflates three distinct operations, a durable checkpoint (persistence), a user check-in (a deliberate stop for attention or approval), and a fresh-session handoff (continuation mechanics). Calling every summary an intervention point invites the agent to end its turn and wait, which silently breaks automatic continuation while looking like compliance.

The user's actual control points are the two boundaries where the load-bearing decisions are made: the end of brainstorming and the completion of ADR creation. After those, the chain should run autonomously through planning, implementation, and reviews unless something genuinely needs user attention. Governing constraints, verbatim: "Anything after that is autonomous, unless any issue comes up with a decision that e.g. can't be implemented how it was thought."; "Anything that needs my attention, or that is an unambiguous or bigger decision, should be raised in a check-in."; "A checkpoint in our terminology is more a 'update memory -> anything to raise to the user? -> all good => prepare handoff'".

Two grounding findings shape the scope. First, ADR-0149 (Implementing, concurrent) moves Pi workflow bodies behind the governed `awf_workflow` loader, introduces the `direct` route with a direct-execution skill, and makes chain handoffs single protocol-2 `phase_transitioned` events; this decision must be implemented against that post-0149 architecture. Second, Pi handoff failures that occur after the tool call has queued its continuation command happen after the tool result set `terminate: true`, so no further model turn exists in the old session and prose instructions alone cannot convert such a failure into an agent-raised check-in; only the extension runtime can surface it.

ADR-0145 also left working-memory scope implicit. ADR-0149 settled that small efforts stay free of working-memory ceremony; this decision keeps that boundary: checkpoint protocols attach to memory-backed efforts, and a non-trivial brainstormed effort becomes memory-backed when its first settled decision is persisted.

## Decision

1. A workflow boundary is a three-state protocol executed in order: persist the durable checkpoint in working memory, decide whether user attention is required (the check-in decision), then either stop with a check-in or continue through target-native continuation. A routine checkpoint summary is never, by itself, a user intervention point, and ending the turn after a routine summary without a raised check-in is a protocol violation.

2. The protocol renders as two distinct target-sensitive template partials, one per boundary class: the mandatory approval check-in and the routine autonomous checkpoint. Shared prose stays coherent for every target; only Pi-rendered output names Pi tools.

3. Mandatory approval boundaries are the end of brainstorming (after its single-pass grounding check) and the completion of ADR creation (after ADR review settles). At each, the agent persists working memory, presents the completed design or ADR summary, explicitly requests approval, and ends the turn, even when it has no concern to raise. Continuation begins only after explicit user approval, and the approval plus next action are persisted before target-native continuation. If the user rejects or requests changes, the agent revises, persists and commits as applicable, regenerates the summary, and requests approval again; grounding itself is not repeated.

4. Every other boundary of a memory-backed effort, including intermediate implementation task checkpoints, uses the routine protocol: persist state, then assess for material authority drift, materially different choices than the approved design, significant scope expansion, unresolved correctness or safety concerns, blockers, or failed required verification. If any apply, raise a check-in that names the issue, the options, a recommendation, and the blocked next action, and stop. Otherwise issue a non-interactive continuity notice and continue immediately. Mechanical corrections and implementation details determined by existing authority remain autonomous; during ADR review, meaning-preserving mechanical and reasoned corrections apply autonomously while any material change to approved decision semantics, scope, or trade-offs routes as a user-decision check-in.

5. The implementation skills (plan execution, subagent-driven development, and the direct-route execution skill when the effort is memory-backed) embed the complete routine protocol directly in their per-task checkpoint sections rather than referencing a distant terminal definition, so an intermediate boundary is governed by text at the point of use.

6. On the Pi target, the clear branch of the routine protocol prepares and invokes `handoff_session`, alone in its tool batch, after persistence and the continuity notice. Mandatory approval boundaries never hand off before approval; after approval, continuation follows the target-native successor. Non-Pi targets render the same protocols with coherent target-native continuation and no Pi tool name.

7. A failed Pi handoff leaves the durable checkpoint valid and is never portrayed as successful. A failure surfaced to the model before the continuation command is queued becomes an agent-raised check-in in the old session, not a silent retry or silent continued work. For failures after queueing, the handoff extension provides a runtime recovery path: when a post-queue failure leaves the old session active, the extension presents a visible failure notice, consumes the pending request, and places recovery content in the editor so the user can retry the handoff or resume the old session; it never auto-retries and never initiates a model turn itself. The user's explicit five-second countdown cancellation remains an intentional stop with notification, not a failure.

8. Working memory remains optional for small, checkpoint-less efforts. A non-trivial brainstormed effort becomes memory-backed when its first settled decision is persisted; from that point its boundaries carry the protocols above.

9. Implementation sequences against the post-ADR-0149 architecture: the governed Pi workflow loader, the `direct` route, and protocol-2 phase transitions are the surfaces this decision's Pi guidance lands on, and files under concurrent ADR-0149 implementation are not modified until the relevant 0149 batches are on the base branch.

10. Tests classify skills by boundary class. Mandatory-boundary tests prove the explicit approval request, the stop, post-approval memory persistence, and only then target-native continuation. Routine tests prove the attention branch stops with a check-in while the clear branch continues without ending the turn. Implementation-skill tests scope their assertions to the per-task checkpoint sections so a distant terminal handoff cannot satisfy them. Rendered Pi and non-Pi outputs, current-state backing, publication-safe empty-variable rendering, sync and drift checks, staged checks, and the gate are all required.

## State changes

- add `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-workflows:pi-session-handoff-workflow`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`

## Consequences

The chain gains exactly two hard stops, at the boundaries where design authority is created, and loses the ambient implicit pause at every other boundary. Users get a predictable contract: approve the design, approve the ADR, then expect autonomous progress punctuated only by check-ins that carry an issue, options, and a recommendation. The recurring failure mode of a printed checkpoint with no handoff is converted from a prose-compliance hope into a testable protocol: the clear branch that stops is a violation, and the attention branch that continues is one too.

The check-in decision places classification judgment on the agent. A misclassified clear branch continues past something the user wanted to see; the explicit trigger list (authority drift, changed choices, scope expansion, correctness and safety concerns, blockers, failed verification) narrows but cannot eliminate that risk. The mandatory boundaries cap the blast radius, since autonomous work always descends from an explicitly approved design.

The Pi handoff extension widens: the post-queue recovery path adds visible failure UI, editor recovery content, and matching deterministic tests. Non-Pi targets change only in prose. ADR-0145 stays frozen history; this decision updates the claims it established rather than editing it. Implementation is serialized behind the concurrent ADR-0149 work on the shared Pi workflow surfaces, which delays landing but avoids cross-effort file conflicts.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep ADR-0145 semantics and strengthen the prose to "always invoke the handoff" | Leaves checkpoint, check-in, and handoff conflated; agents already fail this exact instruction, and every summary would still read as an invitation to stop. |
| Make every checkpoint an approval gate | Maximizes control but destroys the autonomy the user explicitly wants after the two design boundaries. |
| Runtime-only inference of whether a checkpoint is safe to hand off | The runtime cannot see design drift or blocked decisions; the agent must classify attention needs explicitly. Rejected during brainstorming. |
| One merged protocol partial with conditional wording | Blurs the mandatory stop into the routine flow and prevents tests from binding the two behaviors separately. |
| Silent retry or automatic model turn after a failed post-queue handoff | Hides a failure the user must arbitrate and risks duplicate continuation work in two sessions. |
| Amend ADR-0145 in place | ADR-0145 is Implemented; frozen history is corrected forward through claim operations, never by editing the record. |

## Status history

- 2026-07-23: Proposed
