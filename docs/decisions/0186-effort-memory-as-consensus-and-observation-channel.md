---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0186: Effort memory as consensus and observation channel

## Context

The effort working memory (`.awf/efforts/<slug>/memory.md`) is today a pure resume-and-audit
artifact. The skeleton (`internal/effort/memory.go`) scaffolds `## Brief`, `## Decisions`, and
`## Handoff log`, and the checkpoint partials instruct exactly four writes: `Phase:`, `Next:`,
one handoff-log line, and `Updated:`. No instruction anywhere tells a session to record
friction, surprises, or the decisions settled with the user, and only the brainstorming skill
mentions `## Decisions` at all.

Two failure modes follow. First, the retrospective has lost value since efforts replaced the
old flow: it runs in the terminal session only, its recurrence signal is defined as "recurred
within this session", and mid-effort pitfalls observed by earlier sessions are never written
down, so they are invisible at the end. The user's framing: "the last session to notice things
is the one doing that last phase only, and it seems agents tend to not write down pitfalls and
other things that happened during an effort." Second, reviewers cannot check an artifact
against what the user actually agreed to; a plan or implementation can drift from a settled
conversation without any stage raising it.

The root cause identified in brainstorming is that no memory section names a consumer. Agents
reliably write `Phase:`/`Next:` because resume reads them; nothing reads anything else, so
nothing else gets written. The design principle of this decision is therefore: every section
states its contract and its consumer, and every new section has a mechanical consumer wired in
the same change.

The user set the consensus-record bar explicitly: "maybe just a decision log that captures
when a decision with me has been made, basically a consensus log that spans the whole effort";
"Sometimes a decision happens over mutliple replies, also with me affirming specific things
presented to me with small adjustments or specifications, so this must also be applicable in
this design"; "It must be possible to infer my exact meaning including context and wording";
and deviations surface as "we decided X, but during plan writing ... so I recommend doing Y,
approved?". Verbosity was accepted knowingly: "It's potentially a bit much, but it's ephemeral
and necessary for our consensus review."

A grounding check verified the change surface: the skeleton has exactly one producer and no
structural parser beyond the `Effort: <slug>` first line; reviewer lens lists live per agent
template while the shared review spine carries only schema and classification; all four
dispatching review skills already pass the memory path as read-only context; `awf check
memory` and `awf check prose` are content-agnostic to section names. Two traps were found:
this repository overrides `focusItems` wholesale in all three reviewer sidecars, so a catalog
`focusItems` default would be silently dropped here (the documented ADR-0116 pitfall), and
`.awf/skills/parts/retrospective/procedure.md` locally overrides the whole retrospective
procedure section, so it must move in lockstep with the template default.

## Decision

1. The effort memory skeleton is purpose-partitioned with consumer-named sections. The
   scaffold becomes: the unchanged header lines (`Effort:`, `Phase:`, `Next:`, `Updated:`),
   `## Brief` (outcome plus pointers to the effort's durable artifacts: ADR, plan, worktree,
   branch; consumed by resuming sessions), `## Decision log` (replacing `## Decisions`),
   `## Observations` (new), and `## Handoff log` (unchanged boundary audit). Each section's
   placeholder text states what belongs in it and who reads it.
2. The decision log is the effort-spanning consensus record: append-only entries of the form
   `- <date> <phase> (user|autonomous) <decision>. Why: <one line>.` Provenance `user` marks a
   commitment settled with the user and binding on reviewers; `autonomous` marks a choice the
   agent settled under existing authority, challengeable on merit without a check-in.
3. Settlement, not message, is the entry unit. A decision formed over several replies yields
   one entry when it settles. When the user affirms agent-presented content, with or without
   adjustments, the entry restates the agreed substance as adjusted, never a bare "approved".
   A later refinement appends a new entry naming the entry it supersedes; entries are never
   edited.
4. User-provenance entries carry an indented `Record:` evidence block when needed to infer the
   user's exact meaning: verbatim user wording for the load-bearing parts, the referenced
   agent-presented content when required to understand what was affirmed, and the relevant
   sections of each participant when a correction reshaped the decision. Verbosity is
   acceptable because the file is ephemeral and deleted at finish.
5. `## Observations` is the append-only at-occurrence log of friction, surprises, near-misses,
   and recurrences (`- <date> <phase> <observation>.`), written the moment something bites.
   Both checkpoint partials add one backstop clause to the writer-owned batch: append any
   decision settled and any observation hit since the last boundary that is not yet recorded.
6. The decision log's user entries gain reviewers as consumers: the dispatching review skills
   (reviewing-adr, reviewing-plan, reviewing-plan-resync, reviewing-impl) paste the
   user-provenance entries verbatim into the reviewer brief, and each reviewer agent template
   (adr-reviewer, plan-reviewer, code-reviewer) gains a consensus-adherence lens in its
   hardcoded universal-lenses section (not a catalog `focusItems` default, which local
   overrides would drop). A deviation from a user entry is always a `user-decision` finding of
   the form "we decided X; during <phase> we found Z; recommend Y, approve?", never silently
   absorbed.
7. The observation and decision logs gain the retrospective as consumer: retrospective step 2
   reads both as primary input alongside the terminal session's own context, the recurrence
   signal extends to recurrence across the effort's sessions as visible in the observation
   log, and the retrospective confirms every user decision either landed in a durable artifact
   or was explicitly re-decided. The sentence "never treat ephemeral memory as the next
   retrospective's authority" is rephrased to its intent: recording observations in memory is
   the expected path; deferring lessons there past finish is the forbidden one. The local
   override `.awf/skills/parts/retrospective/procedure.md` is edited in lockstep with the
   template default.

## State changes

- add `tooling/effort-management:memory-skeleton-purpose-partition`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- add `rendering/workflow-skill-templates:memory-log-consumer-coverage`

## Consequences

- The retrospective regains cross-session sight: pitfalls recorded at occurrence by any
  session survive to the terminal session, and recurrence within one effort becomes visible
  instead of structurally invisible.
- Terminal and stage reviews can answer "has this accomplished what the user wanted" against
  the recorded consensus, and drift from a settled agreement surfaces as an explicit
  user-decision finding instead of passing silently.
- Checkpoint writes grow by one backstop clause and review briefs grow by the pasted user
  entries; memory files become substantially more verbose. This is accepted: the file is
  ephemeral, deleted at finish, and consensus content lands durably in ADRs, plans, and
  implementation before then.
- The skeleton change is adopter-visible (new scaffold content from the binary) and lands in
  the changelog under Others; existing resident memory files are unaffected because nothing
  parses section headings.
- The consensus-adherence lens prose is duplicated across the three reviewer templates;
  folding it into the shared spine joins the already-deferred reviewer-spine dedup follow-up
  rather than this change.
- Agents must judge settlement boundaries and Record-block relevance; a sloppy log degrades
  review quality. Mitigation: the entry contract in the skeleton placeholder and workflow doc
  is explicit, and the consensus-adherence lens itself surfaces gaps when an artifact cannot
  be checked against the log.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Record-at-occurrence only (each session lands lessons durably, no memory log) | Interrupts implementation flow and loses weak signals not yet worth a durable entry; retrospective loses its input entirely |
| Tiered dual channel (durable landing plus memory log) | Two rules to apply correctly mid-flow; the single log with a retrospective promotion pass is simpler and sufficient |
| Observations folded into the handoff log | Conflates audit with signal, caps observations at one boundary line, and misses mid-phase events |
| Separate Consensus log and Decisions sections | Two sections competing for the same writes; chronological interleaving of user and autonomous entries is lost |
| Full verbatim phase log of the conversation | Heavy, redundant with the handoff log, mostly noise for reviewers; the user withdrew it in favour of the consensus log |
| Consensus-adherence lens as a catalog `focusItems` default | This repository (and any adopter overriding `focusItems`) would silently drop it; the documented ADR-0116 pitfall |

## Status history

- 2026-07-30: Proposed
