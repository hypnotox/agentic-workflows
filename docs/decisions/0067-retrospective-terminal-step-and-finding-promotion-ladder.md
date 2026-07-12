---
status: Implemented
date: 2026-07-06
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [workflow, skills, feedback-loop, testing]
related: [8, 17, 22, 46, 50, 52, 53, 54]
domains: [rendering, tooling]
---
# ADR-0067: Retrospective terminal step and finding-promotion ladder

## Context

awf wraps a probabilistic agent in a deterministic harness: the gate, 100% coverage
(ADR-0012), rendered-file drift, invariant-backing, and fresh-context reviewer agents that
grade work the generator cannot grade for itself. What the harness does **not** have is a way
to *close the loop* — to turn a recurring review finding or a repeatedly-hit implementation
pitfall into a durable, deterministic check. The field discipline this project benchmarks
against names exactly this move: "error analysis first, then codify" — harvest the failures
you actually observe into code-based graders so the probabilistic reviewer no longer has to
catch them by hand and cannot miss them
(`docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md`, Pillar 4; "Where awf
trails," item 3).

Today that loop is open, and every existing signal is either ephemeral or lands one rung too
low:

- **Reviewer findings are ephemeral.** The `code-reviewer` emits structured findings
  (`{focus, severity, location, issue, suggested_fix, classification}`,
  `.claude/agents/code-reviewer.md`), routes them, applies fixes, and they vanish with the
  session. Nothing records that a class of finding keeps recurring.
- **The one informal promotion path lands in another *probabilistic* check.** The
  `code-reviewer`'s "Project-specific focus items" (`error returns must wrap with %w`, `nil
  map/pointer dereferences`) are hand-promoted recurring findings — but promoted into the
  reviewer's own prose checklist, not a deterministic gate.
- **`pitfalls.md` is prose, not a gate.** It captures tricky knowledge for the next agent to
  read; it never fails a build.

The destinations for a *deterministic* rung already exist, so no new check-running machinery
is needed — only a trigger and a routing discipline:

- **Invariants** (`inv:` slug + a `<marker> invariant: <slug>` backing comment/test) are
  config-driven and language-agnostic (`invariants.sources` sets the globs and marker,
  ADR-0008/ADR-0064), so an adopter can add a real deterministic check *in their own
  language with zero awf Go changes*.
- **The adopter's own gate** (`gateCmd`/`checkCmd` vars) is the destination for an ordinary
  mechanically-checkable rule that does not rise to a load-bearing invariant.
- **`code-reviewer` project-focus items** and **`pitfalls.md`** remain the destinations for
  rules that need per-case judgment or are not mechanically checkable at all.

The decisive design question is therefore *where recurrence is recognized and the promotion
is controlled*. The workflow's reviewers run in **fresh context** by construction (the whole
point of an independent verifier, ADR-0017): a fresh-context reviewer literally cannot know a
finding *recurred* — it can only observe that a rule already written down is still being
violated, and it never sees the friction the implementer hit along the way. The **main
thread** — the agent that drove the implementation — has the session context to judge real
recurrence, saw the pitfalls no reviewer sees, and is where the human is present to control
whether a promotion (which for a load-bearing rule spins an entire ADR chain) is worth the
effort now. Recognition and control belong there, not in a subagent.

A heavier alternative — persist every finding to a ledger and fire a deterministic
recurrence-count signal — was weighed and set aside (see Alternatives). It buys *measurement*
of recurrence but not the *decision* of what to codify, which still requires human/agent
judgment; the ledger's schema, drift surface, and noisy signature-clustering are real cost for
a project with few findings, and the judgment it cannot remove is the actual work.

This decision relates to ADR-0053 (the deterministic golden-task eval suite that verifies the
rendered harness) and ADR-0017 (`awf audit`, process-conformance) as the third leg of
"verify the agent": ADR-0053 checks the harness still hangs together, ADR-0017 checks the
agent's process from git history, and this ADR feeds *observed* failures back into the
deterministic harness over time.

## Decision

1. **Add a new Core skill, `retrospective`,** as a `SkillSpec` in `catalog.Standard`
   (`internal/catalog/standard.go`) with `Core: true` and no `RequiresAgent`/`RequiresDoc`.
   It renders as `<prefix>-retrospective` and is enabled in awf's own `.awf/config.yaml`. Core
   is required for coherence: it becomes a canonical chain node, and every chain node is Core
   so a default `awf init` (ADR-0022) never renders a chain whose terminal step points at a
   disabled skill.

2. **`retrospective` is the new terminal node of the canonical chain**:
   `… → implementation → reviewing-impl → retrospective`. `reviewing-impl` gains a terminal
   handoff naming `<prefix>-retrospective`, rendered **unconditionally** like its existing
   Core sibling references to `executing-plans`/`subagent-driven-development` (safe precisely
   because all are Core). The canonical-chain description in `docs/workflow.md` and the rendered
   `AGENTS.md` is updated to end at the retrospective, and the `internal/evals` chain graph
   gains the tenth node in `chainNodes`, the `reviewing-impl → retrospective` edge in the pinned
   handoff-edge list, and — decisively — the `chainTerminal` constant flips from
   `reviewing-impl` to `retrospective`. That last move is not optional: `TestChainConnectivity`
   exempts only `chainTerminal` from the outgoing-edge requirement, so a `retrospective` added to
   `chainNodes` without moving the constant would be flagged orphaned for having no successor
   (`chainEdges` is derived from `chainNodes`, so it needs no separate edit).

3. **The retrospective runs in the main thread, not a dispatched subagent.** It is a skill the
   driving agent follows (like `executing-plans`), because it depends on full session context:
   the `reviewing-impl` findings, the friction and pitfalls hit during implementation, and
   which issues recurred. It never dispatches a fresh-context reviewer.

4. **It routes recurring, codifiable observations up a four-rung promotion ladder** to the
   strongest rung each can support:
   1. **Invariant** — a load-bearing rule the project must remember: an `inv:` slug in an ADR
      plus a backing comment/test. Adopter-portable via the config-driven invariant mechanism.
   2. **Gate test / lint rule** — an ordinary mechanically-checkable rule added to the
      project's gate (`gateCmd`); no ADR.
   3. **`code-reviewer` project-focus item** — needs per-case judgment; persistent but still
      probabilistic.
   4. **`pitfalls.md` entry** — tricky knowledge that is not mechanically checkable.
   First-occurrence observations are *noted* (rung 3/4) rather than promoted; the ladder climbs
   only for issues that recur.

5. **Recurrence is recognized from main-thread session context plus "documented-yet-still-
   present."** An issue is a promotion candidate when the main thread observed it recur this
   session, or when it matches an existing `pitfalls.md` / project-focus capture and *still
   occurred* — i.e. prose memory already recorded it and did not prevent it, which is the
   signal to climb to a deterministic rung.

6. **The main thread controls the promotion effort: codify-now versus deferred follow-up, and
   never an unverified auto-promotion.** A rung-1 (invariant) promotion is almost always a
   *deferred* follow-up, because it spins the full ADR chain (propose → review → resync →
   implement → flip). The retrospective verifies a candidate before spending effort and the
   human decides; promotion is never delegated to a subagent.

7. **The retrospective skips trivial sessions** (nothing worth noting or promoting) so it is
   not ceremony on a one-line change, and it **runs even on docs-only sessions** — closing a
   gap the `reviewing-impl` docs-only skip leaves, where a doc or process pitfall would
   otherwise never surface.

8. **The `code-reviewer` and the shared review-spine partial (ADR-0052) are untouched.** No
   change to the six-field finding schema, the digest, or the `P>0` control flow. The loop is
   additive: a new terminal skill plus docs, with no edit to how findings are produced,
   classified, or displayed.

9. **No new source-level `inv:` slug is introduced for the mechanism itself.** The retrospective
   is a skill + templates + docs; its chain-node integrity is already machine-enforced by the
   *existing* eval machinery extended to cover it — the ADR-0053 full-catalog fixture is
   catalog-derived (`inv: evals-full-catalog-coverage`) and so picks up the new skill
   automatically, and the ADR-0054 handoff/connectivity assertions cover the new node once it
   is added to `chainNodes`, the pinned handoff-edge list, and the `chainTerminal` constant is
   moved to it (see Decision 2 — the connectivity test only exempts the constant's node from the
   outgoing-edge requirement). The invariants the ladder *produces* at rung 1 are
   adopter-authored, not awf-authored.

10. **Docs travel with the change.** `docs/workflow.md` gains the retrospective and the
    promotion loop folded into an existing (order-locked) section rather than a new section;
    the rendered `AGENTS.md` chain/skill enumeration includes it; `pitfalls.md` is framed as
    rung 4 of the ladder. All via part/template edits re-rendered with `./x sync`, never by
    hand-editing rendered output.

## Invariants

- `retrospective` is a Core catalog skill and the terminal node of the canonical workflow
  chain; `reviewing-impl` hands off to it and no chain node hands off past it. Mechanically
  enforced by the ADR-0053 catalog-derived eval fixture and the ADR-0054 chain-graph
  assertions once `retrospective` is added to `chainNodes`, the pinned handoff-edge list, and
  the `chainTerminal` constant is moved to it — this ADR adds no new slug of its own. (textual)
- The retrospective runs in the main thread and dispatches no fresh-context subagent. (textual)
- Promotion routes a recurring, codifiable observation to the strongest applicable rung, and a
  rung-1 promotion uses the config-driven, language-agnostic invariant mechanism so it is
  adopter-portable with no awf Go change. (textual)
- The `code-reviewer` agent, the shared review-spine partial, and the six-field finding schema
  are unchanged by this decision. (textual)

## Consequences

Easier:
- The feedback loop closes **by default**: every non-trivial implementation ends by reflecting
  and, where warranted, hardening a real check — the field's "error analysis, then codify"
  discipline becomes a standing step instead of an informal habit.
- Recurrence is judged where the context actually lives (the main thread), turning a weak,
  fresh-context, documented-only signal into a real one.
- Implementer-side pitfalls and friction are captured, not just reviewer findings; and
  docs-only sessions — invisible to `reviewing-impl` — now have a place to record process
  pitfalls.
- A codified rung-1 promotion is adopter-portable for free: adopters codify in their own
  language via `invariants.sources`, no awf change required.

Harder / accepted trade-offs:
- A retrospective step now sits at the end of every non-trivial implementation. Mitigated by
  the trivial-skip (Decision 7); the step is reflection plus optional promotion, not a second
  review pass.
- Recurrence recognition remains a judgment call. Accepted deliberately — the ledger
  alternative that would make it a code-based signal (Approach B) does not remove the judgment
  of *what to codify*, only measures frequency, at real cost.
- A rung-1 promotion incurs the full ADR chain. Mitigated by the main-thread control that
  makes such promotions deferred follow-ups rather than inline work (Decision 6).
- The canonical chain grows from nine nodes to ten; the eval graph, `workflow.md`, and
  `AGENTS.md` must all reflect the new terminal node in the same change (docs travel).
- `retrospective` is Core (always rendered), but two of its four rungs name **toggleable**
  destinations: rung 4 is the `pitfalls` doc (`Mandatory: false`) and rung 3 is the
  `code-reviewer` project-focus. Rung 3 is safe by construction — `retrospective` only runs
  after `reviewing-impl`, which `RequiresAgent: code-reviewer` (ADR-0050), so the reviewer is
  always present when the retrospective is reachable. Rung 4 is not: an adopter can enable the
  chain while leaving `pitfalls` disabled. The rung-4 reference must therefore degrade to
  generic prose (and carry no inline link to `pitfalls.md`) when the doc is absent, per the
  publication-safe-templates invariant (ADR-0001/ADR-0045) and the no-dead-internal-links
  invariant (ADR-0020) — a template obligation for the downstream plan, not a blocker here.

Ruled out:
- A findings ledger with a deterministic recurrence-count signal (Approach B).
- Wiring promotion detection into the `code-reviewer` subagent / shared review-spine.
- An adopter check-authoring plugin (Approach C) — the invariant mechanism already provides a
  config-driven, language-agnostic deterministic destination.
- The retrospective as an opt-in task skill rather than a Core terminal chain node.

Downstream work unblocked: an implementation plan covering — the `retrospective` catalog entry
and its template with `awf:section` markers matching the declared `Sections`
(`inv: skill-section-parity`); the `reviewing-impl` terminal-handoff edit; the
`internal/evals` chain-graph node/edge additions **and the `chainTerminal` constant flip**; the
`docs/workflow.md` and `AGENTS.md`
canonical-chain updates; the `pitfalls.md` reframing; and enabling the skill in awf's own
`.awf/config.yaml`. When this ADR flips to `Implemented`, the same commit regenerates
`docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Wire promotion detection into the `code-reviewer` subagent (report "promotion candidates" in the digest) | The reviewer runs in fresh context and cannot detect real recurrence, only "documented-yet-still-violated"; the digest lives in the shared review-spine (ADR-0052) spliced into all three reviewers, so the change would leak into the ADR/plan reviewers; and the spine's `P=0` auto-proceed would silently swallow candidates. The main thread is the correct locus for recognition and control. |
| Findings ledger + deterministic recurrence-count signal (Approach B) | Buys measurement of recurrence but not the decision of what to codify, which still needs judgment; adds a persisted artifact, a finding schema, a drift/currency surface, and noisy signature-clustering — real cost for a project with few findings. |
| Adopter check-authoring plugin so a codified rule becomes a first-class `awf check` (Approach C) | Unnecessary: the invariant mechanism is already config-driven and language-agnostic (ADR-0008/ADR-0064), giving adopters a deterministic destination with no awf Go change. A plugin is a large subsystem for marginal value. |
| `retrospective` as an opt-in task skill (peer of `tdd`/`bugfix`), suggested conditionally by `reviewing-impl` | The loop would close only when someone remembers to invoke it, undercutting the "reliably close the loop" goal. Every canonical chain node is Core; a genuine terminal step belongs in the chain, not the opt-in pool. |
| Nothing — keep the informal `pitfalls.md` / project-focus habit | Leaves the loop open: the informal path promotes into another probabilistic checklist, never a deterministic gate, and fires only when an author happens to remember. The observed failure mode — recurring findings never hardening into checks — stays uncovered. |
