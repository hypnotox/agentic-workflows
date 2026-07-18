---
status: Implemented
date: 2026-06-29
tags: [multi-target, doc-standard]
related: [18, 37]
domains: [rendering]
---
# ADR-0038: Tool-Agnostic Skill and Agent Prose

## Context

ADR-0037 makes awf render its skill and agent standard to more than one runtime (Claude Code and
Cursor, with the same `SKILL.md`/`AGENTS.md` body written to each target's paths). That exposes a
content problem the single-runtime era hid: the shipped skill prose names **Claude-specific tools**.
The chain skills say "Invoke the `Agent` tool with `subagent_type: Explore`", "via the `Skill`
tool", and "Prefer multiple choice (`AskUserQuestion` tool)". A grep of the shipped templates finds
this vocabulary in ~10 skill templates (`brainstorming`, the four `reviewing-*`,
`refactor-coupling-audit`, `executing-plans`, `proposing-adr`, `writing-plans`, and
`subagent-driven-development`) in both capitalised (`Agent` tool) and lowercase ("the agent tool
with subagent type") forms. Rendered verbatim into `.cursor/skills/`, these instruct a Cursor agent
to call tools by names that are Claude's, not Cursor's.

ADR-0018 already established a documentation authoring standard (`doc-standard.md`) with rules like
"keep linter-rules out of prose" and "no editorializing or dating". The portability rule belongs in
that same standard: awf's rendered prose is a multi-runtime standard, so it must describe **actions**
the agent takes, not the **tool names** one runtime happens to expose. Cursor 2.4 ships subagents and
skills, so action-language ("dispatch a fresh-context `<name>` subagent", "ask a multiple-choice
question") reads correctly in both runtimes without per-adapter substitution, keeping ADR-0037's
"same body to every target, no transform" property intact.

This was carved out of ADR-0037 (its original Decision 6) so each ADR records one load-bearing
decision: ADR-0037 owns the render mechanism; this ADR owns the authoring rule. They land together in
one implementation plan.

## Decision

1. **Add a tool-agnostic-prose rule to `doc-standard.md`.** Rendered skill and agent prose names the
   action an agent performs, not the tool a specific runtime exposes to perform it: "dispatch a
   fresh-context `<name>` subagent" rather than "invoke the `Agent` tool with `subagent_type:
   <name>`"; "ask a multiple-choice question" rather than naming the `AskUserQuestion` tool; "invoke
   the `<name>` skill" phrased without binding to a named `Skill` tool. Project-specific references
   (skill names like `awf-reviewing-adr`, commands like `./x gate`) are **not** runtime tool names
   and stay.

2. **Neutralise the affected skill templates.** Rewrite the Claude-tool vocabulary in the ~10
   templates above to action-language, preserving each skill's procedure and meaning. Agent
   templates and `agents-doc` carry no such vocabulary today and are unchanged except as the rule
   incidentally applies.

3. **Back the rule with a case-insensitive regression guard.** A golden test asserts no rendered
   skill or agent body contains a runtime tool-name token. The denylist is the complete current
   rendered vocabulary, matched case-insensitively and word-anchored (so it does not fire on the
   neutral `subagent` replacement language): `subagent_type` / "subagent type", "Agent tool" / "the
   agent tool", the "`Agent` prompt" phrasing (`brainstorming`, `subagent-driven-development`),
   "Skill tool", and "AskUserQuestion". This catches both the capitalised and lowercase forms the
   `reviewing-*` skills use, not only the templates carrying the capitalised tokens.

## Invariants

Tagged slugs are backed by tests landing with implementation (enforced by `awf check` once this ADR
is `Implemented`); untagged bullets are textual contracts.

- `invariant: skill-prose-tool-agnostic`: every rendered skill and agent body is free of runtime
  tool-name tokens; the backing check matches the complete neutralised vocabulary
  **case-insensitively** and word-anchored (`subagent_type` / "subagent type", "Agent tool" / "the
  agent tool", the "`Agent` prompt" phrasing, "Skill tool", "AskUserQuestion"), so the lowercase "the
  agent tool with subagent type" forms in the `reviewing-*` skills and the bare "`Agent` prompt"
  references in `brainstorming`/`subagent-driven-development` are caught, not only the capitalised
  tokens.
- The rule constrains rendered prose only; project-specific identifiers (skill names, `./x`
  commands) are not runtime tool names and are exempt. (Textual.)

## Consequences

- awf's rendered standard reads correctly under any runtime that follows the SKILL.md/AGENTS.md
  conventions, not just Claude Code, completing the portability ADR-0037 begins at the mechanism
  level.
- The neutralisation is a one-time prose sweep of ~10 templates plus one `doc-standard.md` rule; the
  golden guard prevents regression. No engine, config, or schema change.
- Because the same body still renders to every target unchanged, this preserves ADR-0037's
  no-per-adapter-transform property: the prose is neutral, so no substitution is needed.
- `doc-standard.md` is a managed doc (ADR-0011/0018) and the rule lands in its existing `rules`
  section: no new section, so `docs_sections_test`'s section-parity is unaffected and stays green.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `doc-standard.md` rule (Decision 1) and the ~10 template neutralisations (Decision 2)
  re-render via `./x sync`, kept green by `./x check`, in the commit(s) that make them true.
- In the commit that flips this ADR to `Implemented`, `docs/decisions/ACTIVE.md` is regenerated via
  `./x sync` (the same `./x sync` covers ADR-0037's flip in the shared plan). No
  `docs/decisions/README.md` index row is owed: the README is a how-to guide; `ACTIVE.md` is the
  generated index (ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Per-adapter tool-vocabulary substitution (render-time vars) | Reintroduces a per-adapter transform ADR-0037 deliberately avoids; action-language reads in both runtimes without it. |
| Leave the prose as-is | Cursor agents would be told to call tools by Claude's names; the standard would not actually be portable, only co-located. |
| A new `awf check` drift kind forbidding tool tokens | Heavier than warranted; a golden test backing `skill-prose-tool-agnostic` guards regressions without a new drift kind or gate surface. |
| Keep the rule bundled in ADR-0037 | Two unrelated rationales (render mechanism vs authoring standard) in one record; the project records one load-bearing decision per ADR. |
