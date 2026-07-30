---
format: current-state-v2
status: Implemented
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
   `- D<n> <date> <phase> (user|autonomous) <decision>. Why: <one line>.` where `D<n>` is a
   monotonic ordinal unique within the effort and never reused. Provenance `user` marks a
   commitment settled with the user and binding on reviewers; `autonomous` marks a choice the
   agent settled under existing authority, challengeable on merit without a check-in.
3. Settlement, not message, is the entry unit. A decision formed over several replies yields
   one entry when it settles. When the user affirms agent-presented content, with or without
   adjustments, the entry restates the agreed substance as adjusted, never a bare "approved".
   A later refinement appends a new entry declaring `supersedes D<n>`; entries are never
   edited.
4. Every user-provenance entry carries an indented `Record:` evidence block: always the
   verbatim user wording for the load-bearing parts, plus the referenced agent-presented
   content when required to understand what was affirmed, and the relevant sections of each
   participant when a correction reshaped the decision. It must be possible to infer the
   user's exact meaning, context and wording included, from the entry alone. Verbosity is
   acceptable because the file is ephemeral and deleted at finish.
5. `## Observations` is the append-only at-occurrence log of friction, surprises, near-misses,
   and recurrences (`- <date> <phase> <observation>.`), written the moment something bites.
   Both checkpoint partials add one backstop clause to the writer-owned batch: append any
   decision settled and any observation hit since the last boundary that is not yet recorded.
6. The decision log's user entries gain reviewers as consumers. The dispatching review skills
   reviewing-adr, reviewing-plan, and reviewing-impl paste the user-provenance entries
   verbatim into the reviewer brief, `Record:` blocks included; reviewing-plan-resync stays
   out because its pass is narrowed by design to scope-completeness and doc-currency, and
   that contract is not reopened here. The consensus-adherence check lands in the shared
   review spine (`templates/partials/review-spine-head.md`), spliced into all three reviewer
   templates by `awf:include` outside any overridable section: one home that no adopter
   override can drop. (A per-agent universal-lenses item is an overridable `awf:section`,
   and a catalog `focusItems` default is dropped wholesale by local overrides, the ADR-0116
   pitfall; both were rejected for that reason.) The spine rule is conditional: when the
   brief carries pasted consensus entries, check the artifact against them. A deviation from
   a user entry is always a `user-decision` finding whose `location` cites the deviating
   artifact passage, whose `issue` names the deviation, and whose `suggested_fix` carries the
   escalation phrasing "we decided X; during <phase> we found Z; recommend Y, approve?";
   never silently absorbed. A brief without consensus entries leaves the check idle.
7. The observation and decision logs gain the retrospective as consumer. Retrospective step 2
   (reflect and record) reads both logs as primary input alongside the terminal session's own
   context and confirms every user decision either landed in a durable artifact or was
   explicitly re-decided; step 3's recurrence signal (verify recurrence before promoting)
   extends to recurrence across the effort's sessions as visible in the observation log. The
   sentence "never treat ephemeral memory as the next retrospective's authority" is rephrased
   to its intent: recording observations in memory is the expected path; deferring lessons
   there past finish is the forbidden one. The local override
   `.awf/skills/parts/retrospective/procedure.md` is edited in lockstep with the template
   default.
8. Pre-existing resident memory files are migrated on first write: a session appending an
   entry whose section heading is absent first appends the missing `## Decision log` or
   `## Observations` heading; an existing `## Decisions` section and its content are left in
   place. Nothing parses section headings, so no binary-side migration exists or is needed.
9. The workflow doc stays the single detailed home of the skeleton and entry contract. The
   working-memory section of `templates/docs/workflow.md.tmpl` is updated in the same change
   to the new section list, the entry format, and the settlement and evidence rules, while
   each skeleton placeholder stays a short contract-and-consumer statement, so the two homes
   cannot drift into contradiction. No operation is owed on
   `rendering/guide-and-doc-templates:working-memory-single-home`: its prose does not
   enumerate the skeleton sections, and the guide-level non-duplication rule it scopes is
   preserved; the backing test literals that pin the old section names
   (`internal/project/spine_test.go`, `internal/effort/store_test.go`, and any other test
   asserting the literal old skeleton section names) move with the rename in the same
   commit.
10. Both added claims are invariants with `Backing: test`, their prose authored at Apply
    time: `memory-skeleton-purpose-partition` is proved on the skeleton test in
    `internal/effort/memory_test.go`; `memory-log-consumer-coverage` is proved on the
    catalog-derived template-content tests that back the sibling workflow-skill claims
    (`internal/project/spine_test.go`, `internal/evals/chain_test.go`). The updated
    `memory-checkpoint-chain-coverage` keeps its existing test backing.

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
  the changelog under Others; existing resident memory files keep working because nothing
  parses section headings, and sessions append missing headings on first write (Decision 8).
- The shared review spine gains its first content beyond schema, classification, and
  procedure; the conditionality on pasted entries keeps resync's narrowed contract and
  non-effort briefs unaffected.
- Binding provenance can be laundered: an autonomous choice mislabeled `(user)` converts a
  reviewer's merit objection into a deviation question, and no mechanical check reads the
  untracked file. The mandatory `Record:` block is the mitigation: a user entry without
  verbatim user wording behind it is visibly unanchored. Transcription risk remains: a parent
  pasting entries into a brief can omit or paraphrase one and the reviewer cannot tell; this
  is accepted as the cost of keeping the brief the reviewer's single self-contained input.
- Agents must judge settlement boundaries and Record-block completeness; a sloppy log
  degrades review quality. Mitigation: the entry contract has one detailed home in the
  workflow doc (Decision 9), and the consensus-adherence check itself surfaces gaps when an
  artifact cannot be checked against the log.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Record-at-occurrence only (each session lands lessons durably, no memory log) | Interrupts implementation flow and loses weak signals not yet worth a durable entry; retrospective loses its input entirely |
| Tiered dual channel (durable landing plus memory log) | Two rules to apply correctly mid-flow; the single log with a retrospective promotion pass is simpler and sufficient |
| Observations folded into the handoff log | Conflates audit with signal, caps observations at one boundary line, and misses mid-phase events |
| Separate Consensus log and Decisions sections | Two sections competing for the same writes; chronological interleaving of user and autonomous entries is lost |
| Full verbatim phase log of the conversation | Heavy, redundant with the handoff log, mostly noise for reviewers; the user withdrew it in favour of the consensus log |
| Consensus-adherence lens as a catalog `focusItems` default | This repository (and any adopter overriding `focusItems`) would silently drop it; the documented ADR-0116 pitfall |
| Consensus-adherence lens as a per-agent universal-lenses item | `universal-lenses` is an overridable `awf:section`, so an adopter override drops the check; also three duplicated homes for one rule |
| Reviewer reads the decision log from the memory path it already receives | The brief stays the reviewer's single self-contained input: findings remain reproducible from the brief alone, and report-only children keep a path-as-context, not read-obligation, relationship to shared memory; the transcription-omission cost is accepted and named in Consequences |

## Status history

- 2026-07-30: Proposed
- 2026-07-30: Implemented; content-sha256: dea7fd3ebce4d78f983b5c47c503b1829c43163a6418b5757ec1d476e68a259a; state-sequence: 100
