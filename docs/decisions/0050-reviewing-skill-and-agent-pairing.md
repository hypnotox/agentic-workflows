---
status: Proposed
date: 2026-07-02
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [catalog, validation, agents]
related: [0013, 0022, 0046]
domains: [config, rendering]
---
# ADR-0050: Reviewing-skill and agent pairing

## Context

Each reviewing skill in the workflow chain is a thin dispatcher around one reviewer agent:
`reviewing-adr` dispatches `adr-reviewer`, `reviewing-plan` and `reviewing-plan-resync` dispatch
`plan-reviewer`, and `reviewing-impl` dispatches `code-reviewer`. Skills and agents are enabled
independently (`skills:` / `agents:` arrays in `.awf/config.yaml`), and no check ties them
together: the ADR-0046 dead-reference scan matches only `<prefix>-<skill>` tokens
(`internal/project/check.go:358,364`), so the unprefixed agent names in reviewing-skill prose are
outside every existing check. A 2026-07-02 analysis confirmed the blind spot: a config enabling
`reviewing-impl` but disabling `code-reviewer` renders a skill that dispatches a nonexistent
agent, and `awf check` passes.

"Shouldn't all agents always be enabled, since they are load bearing for our workflow?" — the
driving question, resolved as pairing validation rather than always-on: chain-less configs are
deliberately legal (ADR-0046 amended the AGENTS.md chain prose to gate on `brainstorming` exactly
so a docs-only adoption stays valid), and in such configs the reviewer agents would be dead
weight. What must hold is narrower: a dispatcher may never exist without its reviewer.

The existing per-skill gating precedent, `requiresDoc` (ADR-0013), is *suppression*: a doc-gated
skill silently drops out of the effective render set (`internal/project/render.go:78`), and
`awf add skill` prints a note (`cmd/awf/list_add.go`). Suppression is the wrong shape here —
silently dropping a reviewing skill would sever the workflow chain invisibly, turning a
misconfiguration into missing process. The CLI flow also matters: `awf remove` rewrites the
config *before* chaining into `runSync` (`cmd/awf/list_add.go:196-205`), so a validation error
raised only at sync would strand the tree — config already edited, rendered files stale, every
gated command failing until the user re-adds the agent by hand.

## Decision

1. **New catalog field `requiresAgent` on `SkillSpec`.** `templates/catalog.yaml` sets it on all
   four reviewing-skill specs: `reviewing-adr: adr-reviewer`, `reviewing-plan: plan-reviewer`,
   `reviewing-plan-resync: plan-reviewer`, `reviewing-impl: code-reviewer`. The field is empty
   for every other skill.

2. **Hard validation at project open.** `validateAgainstCatalog` (runs in `project.Open`, so it
   guards `sync`, `check`, and every other gated command) fails when an enabled, non-`local`
   skill whose spec carries `requiresAgent` names an agent absent from the `agents:` enable
   array: `skill "reviewing-impl" requires agent "code-reviewer"; enable the agent or disable
   the skill`. This deliberately diverges from `requiresDoc`'s suppression semantics — an
   invalid chain should be loud, not silently thinner. A `local: true` skill sidecar skips the
   check (its content is user-owned, like the rest of catalog validation); the required agent
   itself may be catalog or `local`.

3. **An enabled agent nobody dispatches stays legal.** Local and custom agents exist; agents are
   render artifacts, not dead weight, and flagging them would forbid legitimate configs.

4. **`awf remove agent` refuses upfront.** Before rewriting the config, `runRemove` checks
   whether any enabled skill's spec requires the agent being removed and refuses with the
   pairing message, leaving the config untouched. Without this, the rewrite-then-sync order
   strands a half-broken tree.

5. **`awf add skill` auto-enables the required agent.** Adding a reviewing skill whose agent is
   not enabled appends the agent to the enable array in the same config rewrite, with a note
   (`note: also enabled agent "code-reviewer" (required by skill "reviewing-impl")`). The
   asymmetry with Decision 4 is deliberate: the additive fix is safe to apply silently, the
   destructive one is not.

6. **Fresh init is structurally safe.** `ScaffoldConfig` enables every catalog agent
   unconditionally (`internal/project/scaffold.go`), and the init skill-trim descriptor does not
   touch agents, so no init path can violate the pairing; no init-side change is needed. A test
   pins the catalog side: every reviewing-skill spec carries its `requiresAgent`.

## Invariants

- `inv: reviewing-skill-agent-pairing` — an enabled, non-`local` skill whose catalog spec
  carries `requiresAgent` fails `sync`/`check` (via `project.Open`) when that agent is not in
  the `agents:` enable array.
- `inv: remove-agent-pairing-guard` — `awf remove agent` refuses, with the config file
  unchanged, while an enabled skill requires the agent.
- `inv: add-skill-pairs-agent` — `awf add skill` enables the skill's required agent in the same
  config rewrite when it is missing.
- All four reviewing-skill catalog specs carry `requiresAgent` naming their dispatched agent
  (pinned by a catalog test; textual contract for future reviewing skills).

## Consequences

- The check blind spot closes at the config layer rather than the rendered-prose layer: no new
  scan, no prose parsing, and the error names the fix directly.
- A previously-legal (but broken) config — reviewing skill on, its agent off — becomes a hard
  error on the next gated command. No migration: no schema change, and any adopter in that state
  today has a silently broken chain this ADR makes visible.
- `requiresDoc` and `requiresAgent` now carry different semantics on the same struct
  (suppression vs validation); the field docs in `internal/catalog` must state the contrast.
- Two CLI behaviours change shape: `awf remove agent` can refuse; `awf add skill` can write two
  arrays in one rewrite.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Always-on agents (drop the `agents:` array for catalog agents) | Chain-less configs would render dead-weight reviewers; removing the array is a schema migration disproportionate to the defect. |
| Suppression semantics (mirror `requiresDoc`) | Silently dropping a reviewing skill severs the chain invisibly — the failure mode the workflow exists to prevent. |
| Extend the ADR-0046 dead-reference scan to agent names | Catches the symptom in rendered prose; unprefixed agent names risk false positives, and the config layer states the requirement directly with a better error. |
| Derive agents from enabled skills (agents stop being configurable) | Cleanest mental model but a config-schema migration plus `awf add/remove agent` semantics change; local agents still need the array. |
