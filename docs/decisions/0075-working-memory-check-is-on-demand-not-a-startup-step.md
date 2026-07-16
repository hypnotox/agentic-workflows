---
status: Implemented
date: 2026-07-08
supersedes: []
superseded_by: ""
tags: [working-memory]
related: [69]
domains: [rendering, tooling]
---
# ADR-0075: Working-memory check is on-demand, not a startup step

## Context

ADR-0069 established the working-memory convention: one file per in-flight effort at
`.awf/memory/<effort-slug>.md`, discoverable through a section in the always-loaded
agent guide. Its Decision item 5 (the prose contract) instructs the agent, verbatim,
that "at session start check `.awf/memory/` for in-flight effort files and resume from
the recorded phase." The rendered agent guide carries this as an unconditional
imperative: **"On starting work, check `.awf/memory/`."**

In practice the unconditional framing over-fires. It directs the agent to scan the
memory directory for *every* request regardless of whether the request has anything to
resume: a self-contained one-off ("rename this variable", "what does X do?") triggers
the same startup ritual as a genuine mid-effort continuation. The check is noise for the
common case, and an instruction that fires on every request trains the agent to treat it
as boilerplate rather than a meaningful signal.

The countervailing force is the exact scenario ADR-0069 was built for: **session death
and context compaction**. After a compaction or in a freshly spawned session, nothing in
the live context announces that an effort is in flight, and the user frequently
*restates* the task as a self-contained request rather than saying "continue." The
unconditional check was chosen precisely so that silent-resume-after-compaction case is
caught. Any softening must not sacrifice that guarantee, which is ADR-0069's core
rationale (its Context and Consequences both turn on it).

The session-start-check is pure Decision-item-5 prose. It is not a machine-backed
invariant: ADR-0069's three `inv:` slugs cover the self-ignoring `.gitignore`
(`memory-gitignore-always-on`), the agents-doc section parity
(`agents-doc-section-parity`), and the chain-node checkpoint coverage
(`memory-checkpoint-chain-coverage`); none constrains *when* the agent reads memory.
So the trigger can be refined by prose alone, with no backed invariant to retire.

## Decision

1. **The working-memory check is on-demand, not an unconditional startup step.** The
   agent reads `.awf/memory/` when either condition holds:
   - the request implies earlier work to continue: an explicit "continue"/"resume", or
     a reference to an in-flight effort; **or**
   - it is a fresh or context-compacted session, `.awf/memory/` is non-empty, and the
     agent cannot otherwise account for that state (the compaction-resume safety net).

   For a self-contained request the agent can fully serve without prior context (with
   an empty or already-accounted memory directory), the check is skipped.

2. **The resume-discipline is unchanged.** Whenever the check does run: if one effort
   file matches, resume from its recorded `Phase:`/`Next:` lines; if several match, or a
   file matches no in-flight work the agent can verify, ask the user which (if any) to
   resume: never silently resume a stale effort.

3. **The change is a prose edit to the agent-guide template default**
   (`templates/agents-doc/AGENTS.md.tmpl`, section `working-memory`), so every adopter
   inherits the softened trigger. It is a section-body edit only: no `awf:section`
   marker is renamed, so the `agents-doc-section-parity` contract is untouched and no
   catalog change is required. `awf sync` re-renders `AGENTS.md`/`CLAUDE.md`.

4. **Supersedence scope.** This ADR refines, and does not replace, **ADR-0069 Decision
   item 5**: it overrides only that item's "at session start check" clause, leaving the
   rest of the working-memory convention (file location, skeleton, ground rules,
   just-in-time retrieval, ephemerality) fully in force. This is partial-item
   supersedence recorded via `related`, not a `supersedes` flip; ADR-0069 keeps its
   `Implemented` status and both ADRs remain live in `ACTIVE.md`.

## Invariants

- The agent-guide working-memory section frames the memory check as on-demand (gated on
  a continuation signal or the fresh-session-with-non-empty-memory safety net), never as
  an unconditional every-request startup step.
- The section preserves the resume-discipline: match → resume from `Phase:`/`Next:`;
  several-or-unverifiable → ask the user; never silently resume a stale effort.

## Consequences

- **Easier:** the common self-contained request no longer pays for an irrelevant memory
  scan, and the check reads as a meaningful signal rather than boilerplate, so the agent
  is likelier to honour it when it does fire.
- **Preserved:** the compaction-resume guarantee that motivated ADR-0069 survives via the
  fresh-session safety net: a non-empty `.awf/memory/` in a session that can't account
  for it still triggers the check even when the user restated the task self-contained.
- **Harder / risk:** the trigger now depends on agent judgment ("does this request imply
  continuation? can I account for the memory state?") rather than an unconditional rule.
  The safety net bounds the downside: the failure mode requires *both* a mis-judged
  continuation signal *and* a memory directory the agent wrongly believes it can account
  for. Accepted as the trade-off for removing per-request noise.
- No source or test change beyond the template edit and its re-render: the softening
  touches guidance prose only, backed by no machine-enforced invariant, so the two
  Invariants above are textual contracts verified by reading the rendered section.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the unconditional "always check on starting work" | The over-firing that prompted this ADR: noise on every self-contained request, and boilerplate-training that erodes the signal. |
| Pure on-demand (check only on an explicit continuation signal) | Sacrifices ADR-0069's core rationale: a compacted session whose user restated the task self-contained would silently skip an in-flight effort. |
| A this-repo part override instead of a template edit | The softening is a standard-wide guidance improvement; scoping it to awf alone would leave every other adopter with the over-firing rule. |
| A new machine-backed invariant asserting the on-demand wording | Would reduce to brittle prose-matching over guidance text; the trigger is judgment-shaped, not mechanically checkable, so it stays a textual contract. |
