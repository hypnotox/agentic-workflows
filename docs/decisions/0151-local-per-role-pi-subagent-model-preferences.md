---
format: current-state-v2
status: Proposed
date: 2026-07-23
---
# ADR-0151: Local per-role Pi subagent model preferences

## Context

The generated Pi extension exposes four governed subagent tools (`subagent_grounding`,
`subagent_explore`, `subagent_review`, `subagent_implement`), each accepting an optional exact
`provider/model-id`. The current contract (claims `rendering/pi-workflows:pi-subagent-model-routing`
and `rendering/pi-runtime:pi-child-tool-boundaries`, established via ADR-0141 and carried through
the ADR-0148 topic retrofit) inherits the parent model whenever `model` is omitted. In practice
governed calls almost always omit it, so every child runs on the parent model even though the agent
guide asks for deliberate lower-cost child selection. The parent session is the orchestrating
mastermind and typically runs the most capable, most expensive model; grounding sweeps, exploration
fan-outs, and mechanical implementation work do not need it.

Concrete model choices cannot be committed project policy: which providers are registered and
authenticated, and what they cost, are per-developer facts of the local Pi installation. Grounding
against the adopting developer's machine confirmed this concretely: the GPT-5.6 family is exposed
as `openai-codex/gpt-5.6-sol` ($5/$30 per Mtok), `openai-codex/gpt-5.6-terra` ($2.5/$15), and
`openai-codex/gpt-5.6-luna` ($1/$6), each with a request-wide pricing tier above 272k input tokens
(Sol $10/$45, Terra $5/$22.5, Luna $2/$9); a dispatch with the plausible-looking but unregistered
`openai/gpt-5.6-sol` failed only at child startup. Preference data must therefore live outside the
committed config tree, and misconfiguration must surface at session startup rather than mid-work
(an explicit user constraint: errors early, prominently, never silently deferred).

ADR-0149 (Implementing) owns catalog-derived inline/subagent execution routing and the lifecycle
router; this decision is disjoint by construction: it governs which model a queued child uses,
never whether work routes to a child, and touches none of ADR-0149's claims.

## Decision

1. Per-role child model preferences are extension-owned local configuration, never committed
project policy. The extension supports one selection per role (grounding, exploration, review,
implementation) plus a single default child-model fallback shared by all roles.

2. Two JSON preference sources exist, both owned and read by the generated `awf-subagents`
extension: a user-global `<getAgentDir()>/awf-subagents.json` and an optional project-local
`<cwd>/<CONFIG_DIR_NAME>/awf-subagents.local.json`. The project-local file must be gitignored,
and the wizard enforces that: on a project-local save it checks the file's ignore status, offers
to append the exact ignore rule to the repository's `.gitignore` when the file is not ignored
(placed after any `!.pi/` re-inclusion so it wins), and refuses to save if the offer is declined.
Outside a git work tree the ignore check is inapplicable: the wizard saves with a visible notice
instead of refusing, mirroring the extension's existing outside-a-git-checkout degradation. This
repository additionally adds the rule to its own root `.gitignore` in the implementation
commit. Each file holds only the default and the four role keys; unknown keys are invalid.

3. Implicit routing resolves in strict precedence order: explicit per-call `model` argument,
project role, global role, project default, global default, then parent inheritance. An explicit
per-call model is always authoritative above configured preferences; the parent is inherited only
when nothing else resolves.

4. Both preference files are loaded and strictly validated at session startup. Malformed JSON, an
unknown key or role, an invalid canonical `provider/model-id` reference, or a reference to a model
that is not currently registered or not authenticated produces an immediate, prominent error and
blocks all implicit (preference-based) routing until the configuration is repaired. Explicit
per-call models remain usable throughout, because they do not depend on implicit routing. While
either file is invalid, a queued child whose call omits `model` is rejected outright: parent
inheritance is part of the blocked implicit chain, never a silent fallback past broken
configuration, so the only working path is an explicit per-call model. Current registry
availability is revalidated immediately before every queued child, not merely at startup.

5. Routing diagnostics name the source of every child's model: explicit argument, project role,
global role, project default, global default, or parent inheritance.

6. A new `/awf-subagent-models` command runs an atomic, TUI-only setup wizard: choose scope
(user-global or project-local), see current effective preferences and any validation errors,
optionally apply the recommended preset, otherwise configure the default and all four roles
sequentially, review an effective-routing summary, confirm, then write once. Model selectors
present provider/id, display name, base pricing with request-wide pricing tiers clearly
distinguished, context and output limits, reasoning and image support, current-parent/existing/
recommended markers, and role-specific guidance, so the choice is informed. Persistence is atomic:
create the parent directory if missing, write a sibling temporary file with owner-only
permissions, detect stale concurrent writers, clean up the temporary file on failure, and rename
into place; in-memory preference state refreshes immediately after a successful save. Cancellation
or failure at any point leaves the existing file unchanged. The command doubles as the repair path
for invalid configuration and is expected to be infrequent setup, not repeated single-field
editing.

7. The extension embeds one recommended preset, offered only when every referenced model is
currently registered and authenticated, never applied silently: child default and implementation
`openai-codex/gpt-5.6-terra`, grounding and review `openai-codex/gpt-5.6-sol`, exploration
`openai-codex/gpt-5.6-luna`. The wizard explains that it configures child routing only: the parent
session remains the mastermind responsible for brainstorming and plan authoring.

8. Rendered guidance (the dispatch and implementation skill templates, the agent-guide template,
and the working-with-awf doc template) is updated to state that long implementations should favor
sequential implementation subagents so the orchestrating parent stays lean: length and
parent-context pressure are explicit reasons to prefer subagent implementation, coupling may still
justify inline execution, and the orchestrator decides case by case. The shared guidance prose
stays target-generic: the Pi wizard command and Pi subagent tool names appear only in
Pi-conditional template branches, preserving the skill-prose-tool-agnostic contract and the
no-Pi-tool-name-leak boundary.

9. The preference logic and the wizard live in the existing `awf-subagents` index template; the
governed five-file Pi extension surface is unchanged.

## State changes

- update `rendering/pi-workflows:pi-subagent-model-routing`
- update `rendering/pi-runtime:pi-child-tool-boundaries`
- add `rendering/pi-workflows:pi-subagent-model-preferences`
- add `rendering/pi-workflows:pi-subagent-model-wizard`

## Consequences

Cost-appropriate child routing becomes the default rather than per-call ceremony: a configured
machine sends exploration to a cheap model and implementation to a mid-tier one while the parent
stays on the strongest model, with no change to any call site. Per-developer freedom is preserved:
no per-developer preference data is ever committed. The embedded recommended preset is the one
provider-specific piece of committed template content: it is an inert offer gated on live registry
eligibility, and it will need maintenance as the provider's model lineup changes.

The strict startup gate is deliberately unforgiving: one invalid entry in either file blocks all
implicit routing until repaired, even for roles whose entries are valid. That is the accepted
trade-off for the errors-early constraint: a half-working silent fallback would hide
misconfiguration until mid-work. Explicit per-call models keep working during an outage, and the
wizard is the sanctioned repair path.

The embedded recommended preset can go stale against a developer's registry. Mitigation: it is
offered only when every referenced model is currently registered and authenticated, and manual
selection is always available; eligibility uses live registry state, never mere configured
authentication. The wizard is TUI-only; noninteractive configuration is explicitly out of scope
until a later design adds it. Preference files live outside the committed tree, so `awf check`
does not govern them; their validation authority is the extension's startup gate.

Downstream work: render tests and Pi extension tests for resolution precedence, startup
validation, revalidation before queueing, atomic persistence, and wizard behavior. All four
touched claims, the two updated and the two added, are test-backed invariants (`Backing: test`)
whose proof markers land with the implementation, matching the destination topics' existing
convention. `docs/decisions/INDEX.md` is regenerated via `./x sync` at this ADR's status changes.
A plan follows this ADR; a plan-resync must reconcile with Implementing ADR-0149 before
implementation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| One model for all child roles | Insufficient control; grounding/review and exploration have opposite cost profiles. |
| Per-role selections without a common fallback | Unnecessarily repetitive for developers who want one child model everywhere. |
| Committed project policy (config-tree vars) | Registry contents, authentication, and pricing are per-developer facts; committing them breaks portability and publication safety. |
| Repeated single-field edit command | This is infrequent setup; an atomic wizard is simpler and leaves no partially-edited file behind. |
| Silent fall-through past an invalid configured model | Hides misconfiguration until mid-work; violates the errors-early constraint. |
| No embedded recommended preset (manual selection only) | First-run friction and uninformed choices outweigh the preset's staleness-maintenance cost, given the live-registry eligibility gate and the always-available manual path. |

## Status history

- 2026-07-23: Proposed
