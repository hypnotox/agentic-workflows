---
format: current-state-v3
slug: require-confirmed-outcomes-before-effort-creation
status: Proposed
date: 2026-08-02
---
# ADR-require-confirmed-outcomes-before-effort-creation: Require confirmed outcomes before effort creation

## Context

The unified effort workflow currently tells agents to create or resume an immutable effort once a
concrete non-minimal outcome is identified. The boundary is semantically useful but procedurally
underspecified. A request to analyze options, prioritize candidates, or select the next item can be
mistaken for a concrete outcome before discovery has identified the work. Because an effort title
becomes its immutable slug, managed-worktree path, and branch suffix, premature creation leaves a
generic identity attached to work that only becomes specific in later conversation.

The rendered guidance contains conflicting pressure. The workflow and guide describe the
concrete-outcome threshold, while the brainstorming preamble says to carry an effort slug through
every step. Debugging correctly keeps initial investigation effort-free but creates ownership once
evidence identifies a non-minimal fix. Roadmap graduation, TDD, ADR proposal, and plan writing can
also create a missing effort once their local complexity threshold is met. The checkpoint partials
likewise allow creation when a boundary classifies work as concrete and non-minimal. None of these
surfaces requires the user to confirm the selected outcome or its proposed title before the
irreversible identity is allocated.

ADR-0175 established one mandatory memory-owning effort after a concrete non-minimal outcome is
identified. ADR-0152 established two mandatory approval boundaries and otherwise autonomous
continuation. Both records are terminal history. This decision changes the current-state claims they
established rather than editing either record: discovery must first settle and confirm the outcome
that ADR-0175 uses, and outcome confirmation becomes a third mandatory user boundary under the
checkpoint semantics established by ADR-0152.

The change is guidance-layer policy, not a new CLI state machine. The binary cannot determine
whether a conversation has narrowed sufficiently or whether natural-language acceptance is clear.
Deterministic tests can prove that every rendered instruction presents, stops, waits, and only then
creates; they cannot prove live model compliance by executing a nondeterministic multi-turn agent.
The project does not introduce a golden or interactive agent evaluation for this policy.

## Decision

1. Distinguish effort-free discovery from a confirmed concrete outcome. Analysis, repository
   exploration, prioritization, option comparison, debugging investigation, and selection of what to
   do next remain discovery until the agent can name one concrete non-minimal outcome. Discovery
   creates no effort, owned memory, branch, or managed worktree. Agreement on a preference or
   approach does not itself establish effort ownership.

2. Introduce a mandatory outcome-confirmation boundary before first effort creation. The agent
   presents the proposed transition with clearly identified `Outcome:` and `Effort title:` fields,
   explicitly asks whether to create that effort, and ends the turn without mutation. Only a clear
   user response in a later turn confirms the pair and permits `awf effort new "<confirmed title>"`
   or resumption of the matching resident effort. No exact confirmation phrase is required.

3. Keep the interaction in discovery when the user requests a change to either field or gives an
   ambiguous response. The agent presents a revised pair after a requested change and asks a focused
   clarification after ambiguity. Agreement that occurred before the pair was presented cannot be
   retroactively treated as its confirmation.

4. Apply the boundary to direct concrete non-minimal requests as well as discovery-led requests. A
   direct request establishes strong outcome input but does not authorize allocation of an
   agent-derived immutable title; the agent still presents the faithful outcome/title pair and waits
   for confirmation. A minimal simple fix remains effort-free. Work already inside one confirmed
   effort continues under that effort when it remains within the confirmed outcome, while a newly
   discovered outcome cannot silently reuse, rename, replace, or create beside the active effort.

5. Make first-creation ownership explicit across the workflow. Brainstorming performs the new
   boundary after narrowing and approach selection and before detailed design. Debugging and roadmap
   selection perform it when investigation yields a proposed non-minimal change. Orienting and
   exploration remain report-only and never create an effort. ADR proposal, plan writing, TDD, and
   downstream implementation or review paths require already-confirmed effort ownership for
   non-minimal work and never create a missing effort opportunistically.

6. Keep the working-memory contract in its existing canonical workflow-doc home and render concise
   summaries in the agent guide and workflow chain. Add a distinct outcome-confirmation block rather
   than reusing the final-approval partial, because final brainstorming and ADR-review approval
   already operate inside an effort. Routine and approval checkpoint partials validate confirmed
   ownership when an effort exists and never make reaching a boundary or recognizing complexity an
   independent creation trigger. Project-owned guide and workflow overrides carry the same rule.

7. Update the mandatory-boundary contract from two user stops to three: pre-creation outcome/title
   confirmation, final grounded brainstorming approval, and settled ADR approval. The new first stop
   authorizes only effort allocation and continued design; it does not replace final design approval.
   All later routine autonomy and issue-triggered check-in semantics remain unchanged.

8. Keep `awf effort new` and the resident effort model unchanged. If creation fails after a confirmed
   pair, report the concrete failure and recovery action without treating the outcome as unconfirmed.
   No confirmation flag, conversational state, title reservation, mutable slug, or automatic
   inference is added to the binary.

9. Pin the policy with deterministic rendering and projection tests. Tests classify every applicable
   discovery and downstream skill, require the discovery prohibition and minimal/existing-effort
   cases, prove that brainstorming orders pair presentation, a stop, and a later response before
   creation and detailed design, reject opportunistic downstream creation, and ensure checkpoint
   prose cannot create ownership. Tests render every enabled target and the example adopter under
   publication-safe empty values. They verify the instructional contract rather than executing an
   interactive agent evaluation.

10. Update workflow documentation, the agent guide, project convention parts, rendered skills,
    current-state claims, and generated outputs in the same implementation. Run focused tests,
    render and drift checks, staged checks, and the full project gate before each implementation
    transaction commits.

## State changes

- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`

## Consequences

Effort identity follows the selected work rather than the generic request that initiated discovery.
The user gets a predictable opportunity to correct both scope and immutable title before awf creates
resident files or Git topology. Discovery can span several turns without coordination ceremony, and
downstream skills no longer hide a missing confirmation by creating an effort locally.

Every concrete non-minimal request gains one short round trip before creation, including a request
whose outcome already appears precise. Brainstorming gains a third hard stop overall: effort
confirmation permits detailed design, final brainstorming approval still settles that design, and
ADR approval still settles durable authority. This is deliberate ceremony at the irreversible
identity boundary.

Natural-language confirmation remains a judgment exercised by the agent. The labeled pair,
required later response, ambiguity rule, and deterministic rendered-order tests narrow the risk but
cannot eliminate model noncompliance. Adding binary conversational state would create false
certainty because the CLI cannot observe the conversation that gives confirmation meaning.

Existing efforts are not renamed or migrated. The change governs first creation after the rendered
contract ships. CLI behavior, effort schema, managed-worktree mechanics, and cleanup remain
unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add only a warning not to create during initial discovery | It states a prohibition without defining the positive transition, leaving agents to infer the same ambiguous concrete-outcome threshold. |
| Restrict the rule to brainstorming or option-selection prompts | Debugging, roadmap graduation, TDD, ADR proposal, and planning contain equivalent local creation paths and could reproduce the same premature identity allocation. |
| Create autonomously once the agent believes the outcome is specific | It improves naming only when the agent classifies specificity correctly and gives the user no chance to correct an immutable title. |
| Require one exact confirmation phrase | It is mechanically clear but rejects ordinary unambiguous consent and adds unnecessary command-like ceremony to conversation. |
| Delay creation until final brainstorming approval | Detailed design and grounding would lack the effort ownership and checkpoint continuity intended for a now-confirmed non-minimal outcome. |
| Add a CLI confirmation token or interactive agent evaluation | The CLI cannot verify conversational meaning, and a nondeterministic live-agent test would not provide the deterministic contract proof used by this repository. |

## Status history

- 2026-08-02: Proposed
