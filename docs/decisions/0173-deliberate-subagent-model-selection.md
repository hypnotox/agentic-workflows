---
format: current-state-v2
status: Implemented
date: 2026-07-29
---
# ADR-0173: Deliberate subagent model selection

## Context

Governed subagent dispatch guidance normally omits model selection. Pi can resolve an omitted model
through its configured role preference and parent fallback, but the parent agent cannot currently
see the effective role defaults or a concise set of alternatives before it chooses a dispatch.
Other rendered targets do not consistently require an explicit, deliberate choice. A plausible
dispatch can therefore use more time and tokens than its work warrants, or choose too little
capability and fail, without making the trade-off visible.

ADR-0151 introduced strict user-global and project-local Pi model preferences for a shared default
and four roles, an atomic `/awf-subagent-models` wizard, and an embedded recommended preset. It did
not define complexity tiers or expose effective routing state in the model-visible prompt. Its
preference files are extension-owned local state: no older active project shares this extension,
and adopter upgrades keep the generated extension and expected schema aligned. The schema can
therefore evolve directly without a compatibility sidecar.

ADR-0166 moved implementation ownership and dispatch from checkbox tasks to independently green
phases. Model-selection guidance must be applied to those final dispatch boundaries, including
phase implementers, optional batch helpers, primary reviews, and verify reviews, rather than to the
superseded task-level shape.

Model registries, availability, and authentication are dynamic. A routing summary captured only at
session start can become stale after preference edits or registry changes. Pi also permits an
optional `model` tool argument, so pseudo-values such as `inherit parent`, `default`, or `auto` can
look meaningful while actually being invalid model references. Omission must remain the only syntax
for configured or inherited routing.

## Decision

1. Every governed subagent dispatch deliberately selects the smallest model expected to complete
   the work reliably. The semantic tiers are `small`, `standard`, and `large`: small is for narrow,
   mechanical, low-ambiguity work; standard is for substantive but bounded work; and large is for
   broad, intricate, cross-cutting, or high-consequence work. Uncertainty, failed reasoning, or
   widened scope requires reconsideration and possible escalation.

2. Pi normally uses its configured role default by omitting the `model` field and explicitly
   overrides that default with the exact model reference for the selected tier when complexity
   warrants it. Other targets explicitly select a target-native model per dispatch. When a target
   cannot select models, its guidance uses the harness default and visibly states that explicit
   selection is unavailable.

3. The cross-runtime guidance remains semantic and provider-neutral. Generic rendered output names
   tiers and selection behavior but contains no Pi tool name, provider-specific model reference,
   price, context limit, or registry catalog.

4. The Pi user-global and project-local `awf-subagents` preference files directly add optional
   `small`, `standard`, and `large` exact model-reference fields alongside `default` and the four
   role fields. Project values override global values per field. No compatibility sidecar, schema
   version, migration file, or legacy parser is introduced.

5. Completeness is evaluated after project-over-global merging. A complete effective configuration
   has an explicit effective entry for `default`, each of the four roles, and each of the three
   tiers. The shared default fallback does not count as an explicitly configured role. Missing
   entries remain valid, while malformed, unauthenticated, unregistered, or unavailable configured
   entries retain the strict block on all implicit routing. A valid explicit per-call model remains
   usable while implicit routing is blocked.

6. Every configured or explicit model reference is at most 256 characters and has the exact
   `provider/model-id` form accepted by the live registry. Omission of the `model` field is the only
   request for configured default routing or parent inheritance. Literal sentinel values, including
   `inherit parent`, `default`, and `auto`, are invalid and receive an actionable rejection that
   tells the caller to omit the field; they are never normalized silently.

7. The generated Pi extension reloads both preference files and revalidates their merged state
   against the current registry before each `before_agent_start` routing-card decision. For a child
   call, any validation before enqueueing is preflight only: after queue acquisition and immediately
   before child startup, the extension reloads the files and registry and revalidates both configured
   and explicit references. This preserves next-run preference updates and current availability
   rather than relying on session-start or pre-queue state.

8. When any awf subagent tool is active, `before_agent_start` appends exactly one deterministic
   routing card of at most 4096 UTF-8 bytes to the system prompt for that agent run. Its normal form
   lists the exact effective role defaults, exact tier mappings, missing or invalid state, and one
   line instructing the agent to use the role default by omission or override deliberately by tier.
   It never includes raw registry errors: invalid state uses deterministic field names, bounded
   reason codes, and one repair line, and no model reference is truncated. The maximum-length normal
   form must fit the budget. As a defensive guard, an unexpected over-budget construction injects a
   bounded failure card instead of partial mappings and emits an actionable TUI warning to the user;
   it does not weaken strict routing semantics. The card includes no prices, limits, or full registry
   catalog and is not appended as a user, assistant, custom, or other persisted session message.

9. Active awf subagent tools are determined from `selectedTools` when present and from the effective
   active-tool set as a deterministic fallback when it is absent. No routing card is injected when
   none of the four tools is active.

10. An incomplete but otherwise valid merged configuration emits one concise missing-state line in
    the routing card and one non-blocking TUI notice per session identity. Session-scoped notice
    tracking replaces process-global notice suppression. Invalid configured state is shown in the
    card and retains the existing strict implicit-routing error semantics.

11. `/awf-subagent-models` configures the shared default, all four roles, and all three tiers as one
    atomic preference transaction. It preserves scope selection, current-state and validation-error
    display, cancellation without writes, owner-only permissions, sibling-temporary-file rename,
    stale-writer detection, live registry validation, in-memory refresh, and the project-local
    gitignore rule.

12. The embedded recommended preset remains registry-gated and fills every role and tier. Its tier
    palette maps small to Luna, standard to Terra, and large to Sol, while its role defaults retain
    the established Luna, Terra, and Sol assignments. Provider-specific references remain confined
    to the generated Pi preset and the dynamically generated local routing card, not generic target
    output.

13. Every final governed dispatch occurrence after ADR-0166, including primary dispatches,
    implementation-phase dispatches, optional helper dispatches, and verify or review dispatches,
    carries the deliberate-selection rule. Dispatches that use configured Pi routing omit the
    `model` field entirely; examples never pass a sentinel value.

14. The four Pi subagent tool schemas, runtime validation, extension guidance, and regression tests
    enforce omission or a bounded exact reference consistently. Direct schema parsing accepts
    omission and exact references at the 256-character boundary and rejects sentinel values and
    overlong references with the omit-the-field repair message.

15. Tests cover complete, partial, absent, malformed, unauthenticated, and unavailable preference
    configurations; project-over-global completeness and tier precedence; one invalid tier blocking
    otherwise valid implicit role defaults; wizard roles, tiers, preset, cancellation, atomicity,
    stale writers, live validation, and ignore behavior; and all exact-reference boundaries.

16. Routing-card tests cover deterministic ordering and length, exactly-once injection only with an
    active awf subagent tool, `selectedTools` fallback, preference and registry refresh, invalid and
    missing state, and one notice per session. A real Pi runtime smoke assertion proves that the card
    reaches the model request without becoming a persisted session message.

17. Cross-target tests enumerate every final governed dispatch occurrence and prove Pi
    default-or-override behavior, non-Pi explicit selection and unsupported-selection fallback,
    provider and Pi-tool isolation, and complete proof coverage for the new cross-runtime invariant.
    Affected templates are also rendered with empty variables under `missingkey=zero` and must produce
    coherent generic prose without unresolved-value or no-value tokens.

18. The generated Pi extension extracts preference parsing and merging, validation-state
    representation, and routing-card construction into a bounded model-routing module with pure,
    directly testable seams. The entrypoint retains tool registration, queueing, process lifecycle,
    and runtime integration. This enabling refactor is part of the Pi implementation batch rather
    than a separate architectural layer.

19. Implementation updates the existing Pi model preference, routing, wizard, extension-target, and
    real-runtime smoke claims and adds a backed cross-runtime deliberate-selection claim. The
    operations apply in declaration order in three batches: first the workflow-skill-template add;
    second the three Pi-workflows updates together; and third the two Pi-runtime updates together.
    Each Applied event pairs atomically with exactly its batch's current claim truth, authored
    sources, generated outputs, provenance, proof markers, tests, and next global state sequence.
    Unapplied operations remain Remaining; abandonment cancels only unapplied operations and
    preserves every Applied effect. The first matching batch updates the authored AGENTS source and
    regenerates `AGENTS.md` with the dispatch convention. Every status transition runs `./x render`
    and commits the regenerated `docs/decisions/INDEX.md`. Documentation travels with each matching
    implementation batch, and no generated output is hand-edited.

## State changes

- add `rendering/workflow-skill-templates:deliberate-subagent-model-selection`
- update `rendering/pi-workflows:pi-subagent-model-preferences`
- update `rendering/pi-workflows:pi-subagent-model-routing`
- update `rendering/pi-workflows:pi-subagent-model-wizard`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-real-runtime-smoke`

## Consequences

Dispatch choices become visible, portable, and proportional to expected complexity. Pi agents see
both their role default and a concise escalation palette at the moment of selection, while other
targets receive equivalent semantic guidance without Pi-specific leakage. Strict rejection of
sentinels removes an ambiguous failure mode, and per-run refresh prevents preference and registry
state from silently going stale.

The preference schema and wizard become larger, and the system prompt gains a small dynamic card
when governed tools are active. Incomplete configuration now produces a notice, which adds some UI
noise, and direct schema evolution means an old extension cannot interpret a file written by the
new wizard. That compatibility cost is accepted because extension ownership and adopter upgrades
keep the parser and file schema together.

The tiers are qualitative rather than a permanent price or capability table. They cannot guarantee
a successful choice, and provider catalogs can change. Choosing the smallest reliable tier,
revalidating against the live registry, and escalating after uncertainty or failure mitigate that
risk without embedding volatile prices or catalogs in durable guidance.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Always inherit the parent or use the role default | It hides cost and capability trade-offs and offers no deliberate escalation path. |
| Always require an explicit exact model in Pi | It bypasses useful per-role preferences and makes every dispatch repeat local configuration. |
| Publish prices, limits, and the full registry catalog in guidance | Those facts are volatile, provider-specific, and too large for a concise routing aid. |
| Add a compatibility sidecar or schema version | No active older consumer shares the extension, so parallel formats add complexity without protecting a real compatibility boundary. |
| Treat missing role entries as satisfied by the shared default | It masks incomplete deliberate configuration and prevents the card and wizard from identifying the missing choice. |
| Silently normalize `default`, `auto`, or `inherit parent` | Sentinel strings are not exact model references; normalization would conceal caller mistakes and weaken strict routing. |
| Inject the routing card as a persisted session message | The card is dynamic run-local configuration and would pollute durable conversation history with stale state. |

## Status history

- 2026-07-29: Proposed
- 2026-07-29: Implementing; content-sha256: 8de600f958f3833ee1cc5733d9dd5d34c46099b12aeddd9c33f3abfaa7baffec
- 2026-07-29: Applied; state-sequence: 80; operations: add `rendering/workflow-skill-templates:deliberate-subagent-model-selection`
- 2026-07-29: Applied; state-sequence: 81; operations: update `rendering/pi-workflows:pi-subagent-model-preferences`, update `rendering/pi-workflows:pi-subagent-model-routing`, update `rendering/pi-workflows:pi-subagent-model-wizard`
- 2026-07-29: Applied; state-sequence: 82; operations: update `rendering/pi-runtime:pi-extension-target-render`, update `rendering/pi-runtime:pi-real-runtime-smoke`
- 2026-07-29: Implemented; content-sha256: 8de600f958f3833ee1cc5733d9dd5d34c46099b12aeddd9c33f3abfaa7baffec
