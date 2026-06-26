---
name: awf-brainstorming
description: Use before any non-trivial awf work — new feature, refactor, bug investigation that changes behaviour, workflow rule change. Explores intent, requirements, and design before any code or doc is written. Hands off to awf-writing-plans or awf-proposing-adr at the end.
---

# awf-brainstorming

The project's brainstorming skill. The design lands in the ADR (if load-bearing) or the plan (if not) — never in a separate spec document.

## When to invoke

Per `AGENTS.md`: hard prerequisite for any non-trivial change. Narrow exceptions: typo/formatting fix, one-line bugfix with an already-failing test, mechanical follow-up to a just-merged plan.

## Procedure

1. **Explore project context.** Read `AGENTS.md`, relevant docs (architecture, workflow, testing), recent commits in the affected area (`git log --oneline -20 <path>`). Check domain docs under `docs/domains`. Identify which packages and which existing ADRs the work touches.

2. **Ask clarifying questions, one at a time.** Prefer multiple choice (`AskUserQuestion` tool when available). Each question narrows scope. Avoid asking for everything in one mega-question.

3. **Propose 2-3 approaches** with trade-offs and your recommended choice. Each approach gets a name, a one-line summary of how it works, the main strength, the main weakness. The recommendation goes first with "I'd lean X" framing.

4. **Present the design in sections**, getting approval after each section. Sections cover: architecture (what changes structurally), components (what new files / what existing files change), data flow (if non-obvious), error handling (boundaries: and any others relevant), testing (unit test, integration/e2e, regression test placement). Scale each section to the change's complexity.

5. **Do NOT write a spec document.** The design is captured in either the ADR (if load-bearing) or directly in the plan (if not). See `docs/decisions/README.md` for when an ADR is warranted.

6. **Run a single grounding-check subagent.** Once the user has agreed the design, dispatch ONE subagent via the `Agent` tool (`subagent_type: Explore` by default; `general-purpose` only when the grounding-check needs to run a command rather than read files). The subagent does NOT see this conversation — it works from a self-contained brief and returns findings. Do NOT write the brief to a file.

   Synthesise, in the `Agent` prompt: the problem, the agreed approach, the concrete design decisions, the files/packages/ADRs touched, the assumptions made (flag anything asserted from memory rather than verified against code), and the chosen testing approach. Quote key user constraints verbatim.

   Ask the subagent specifically to:
   - Verify the brainstorm's factual premises against the codebase: do the named types/functions/packages exist? does the approach fit the project's architecture as described?
   - Surface unstated assumptions and edge cases the brainstorm glossed over.
   - Flag altitude/scope concerns: load-bearing enough for an ADR? complex enough for a plan? too large for one effort?
   - Check convention fit: does it contradict an Accepted/Implemented ADR or an invariant in `AGENTS.md`?
   - Return findings as a list of `{kind: open-question | possible-issue, topic, detail, grounding (file:line), confidence: verified | interpreted | unverified}`. `confidence` is load-bearing: `verified` = factual claim mechanically confirmed against source; `interpreted` = reading requires judgment; `unverified` = claim could not be confirmed.

   Surface findings to the user: `interpreted`-confidence findings go back into the brainstorm as open questions, not settled facts; `verified` findings can be folded in; `unverified` findings are flagged for user triage.

   Advisory and single-pass — never gates, rewrites, or commits. No automated re-review loop. Skip only for the narrow exceptions at the top.



7. **Decide the terminal step** based on the (reviewed) brainstorm result:
   - **Load-bearing + complex** → invoke `awf-proposing-adr` first (which chains through `awf-reviewing-adr`); once the ADR is settled, invoke `awf-writing-plans`.
   - **Load-bearing + simple** → invoke `awf-proposing-adr` only; implement directly after the ADR is committed.
   - **Complex but not load-bearing** → invoke `awf-writing-plans` only.
   - **Neither** → implement directly without a plan or ADR.

## Definitions

- **"Load-bearing"** means the project must remember this decision: new package boundary, auth model change, non-trivial new dependency, workflow rule change, new top-level directory. Examples specific to this project: see `docs/decisions/README.md` "When to write an ADR".
- **"Complex"** means multi-commit implementation, interdependent steps, or any change where a future reader (or you on session resume) would benefit from knowing the per-step sequence. See `AGENTS.md`.

## Anti-patterns to avoid

- Diving into code before the brainstorm completes.
- Presenting only one approach (forces accept/reject without alternatives).
- Asking compound questions — split into one at a time.
- Writing a separate spec document for the design.
- Going straight from brainstorm to commit, skipping warranted plan/ADR steps.
