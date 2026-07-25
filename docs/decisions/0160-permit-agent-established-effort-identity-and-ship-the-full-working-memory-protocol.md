---
format: current-state-v2
status: Proposed
date: 2026-07-26
---
# ADR-0160: Permit agent-established effort identity and ship the full working-memory protocol

## Context

The rendered working-memory instructions contain a contradiction that has been observed to
misfire. Both shared checkpoint partials tell the agent to create `.awf/memory/<effort-slug>.md`
with "the exact active `Effort: <active-effort-id>` line" and, in the same sentence, "never
invent or infer an effort ID". Both clauses presuppose a runtime-assigned active effort ID.
Only the Pi target has one: the `awf_workflow` router settles it (ADR-0149). In a session on
any other target no active effort ID exists and no rendered text says who may establish one,
so the two clauses cannot both be satisfied and the instruction is unsatisfiable.

The observed failure is the inverted outcome: during a prior effort, an agent in a non-Pi
session declined to create a working-memory file at all, reasoning that there was no active
effort ID and that the rules forbade inventing one. Working memory exists so that the chain's
state survives session loss; an instruction that suppresses the file on every non-Pi target
defeats its purpose.

Mechanical facts established by a grounding pass against source:

- The phrase "never invent or infer an effort ID" appears in no ADR. It exists only in
  `templates/partials/checkpoint-approval.md` and `templates/partials/checkpoint-routine.md`,
  introduced by rendering commits `8dabc021` and `a375c3ab`. A third, differently worded
  instance ("never infer the ID") sits in `templates/skills/brainstorming/SKILL.md.tmpl`,
  which renders at line 20 of the brainstorming skill body while the approval partial's copy
  renders at line 63. During a brainstorm it is therefore the first identity instruction the
  agent reads, and the likely proximate source of the observed refusal.
- The nearest actual authority is ADR-0149 Decision 5, whose final sentence is "The runtime
  does not mine prompts, filenames, arbitrary prose, session ancestry, or repository state for
  identity." Its grammatical subject is the runtime, not the agent, and the sentence sits in
  the paragraph governing `/resume` and `/awf-resume-effort`, that is, continuation rather than
  creation. Its matching rejected alternative is "Infer continuation from prompts or memory
  filenames", also scoped to continuation. The partials generalised a runtime-scoped
  resume-safety rule into an agent-scoped creation prohibition on every target.
- ADR-0149 Decision 6 constrains the write direction the other way already: "When a workflow
  later creates or updates `.awf/memory/<slug>.md`, the file carries `Effort: <id>`" presupposes
  the identity exists before the file.
- The shipped workflow doc has instructed the agent to inspect memory files and match one to
  in-flight work since before ADR-0149, and still does. That agent-side, filename-mediated
  recognition was never treated as violating Decision 5, which is direct evidence that
  Decision 5 was always read as runtime-scoped.
- The two partials are backed by different invariants with disjoint proof surfaces.
  `TestMemoryCheckpointCoverage` proves `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
  and never reads a body that renders the approval partial; the approval partial is proven by
  `TestMandatoryApprovalBoundaries` under
  `rendering/workflow-skill-templates:mandatory-approval-boundaries`. No test anywhere pins the
  invent-or-infer clause, so removing it breaks nothing, but a clause added to both partials
  needs an assertion in both proofs.
- `templates/pi/awf-handoff/index.ts.tmpl` validates the memory file's `Effort:` value with
  `boundedIdentifier` and then requires equality against an independently copied active
  association. A kebab-case slug passes the shape check but runtime IDs are `randomUUID()`
  values, so an agent-established identity can never satisfy the equality check.

A second, independent defect surfaced while scoping the fix. This repository's convention part
`.awf/parts/workflow/working-memory.md` carries the entire effort-identity, resume, and
one-way-ledger prose, appended after the shipped section default. The shipped template
`templates/docs/workflow.md.tmpl` carries none of it, while both partials ship the pointer
sentence "The file skeleton and ground rules live in the workflow doc's working-memory
section" to every adopter. A Pi-enabled adopter therefore receives the `handoff_session`
guidance from the shipped conditional but not the identity rules that govern it, and the
`working-memory-single-home` pointer resolves to absent content.

That split is residue, not design. The part was created by `095e1cb2` on 2026-07-23, the same
day as ADR-0157, whose Context records that at the time "the guide and the singleton docs
render neutrally, once, through render data that never sets `targetSessionHandoff`", making
"every `{{if .targetSessionHandoff}}` branch in the guide and workflow-doc templates ... dead
code". Pi-conditional prose in a singleton doc template could not render at all, so parking
the Pi sentences in a local part was the only way to get them into this project's own rendered
doc. ADR-0157 Decision 6 made the signal live project-wide the same day
(`internal/project/render.go`, `anyTargetHasCapability`), but the prose was never unparked.

## Decision

1. Effort identity has two sources, and the rendered protocol names both. When a runtime
   assigns an active effort ID, working memory uses that ID exactly, unchanged from today.
   When no runtime assigns one, the agent establishes the new effort's own identity: a short
   kebab-case slug naming the effort, recorded in the `Effort:` line, used as the file's own
   `<effort-slug>` name, and surfaced to the user when the file is created.

2. The prohibition is narrowed to the scope it originally had, and restated in terms an agent
   can act on without a ledger. The agent must not adopt or fabricate a match to another
   effort's identity: not an ID another file in `.awf/memory/` already carries, not one found
   in prose, a plan, an ADR, or a commit message, and not one belonging to completed,
   abandoned, or pruned work. Establishing a brand-new effort's own identity is not covered by
   this prohibition and never was.

3. Identity flows one way, from ID to filename. The agent names the file after the identity it
   established; nothing derives identity by reading a filename. ADR-0149 Decision 5's
   runtime-scoped guarantee is therefore preserved verbatim, and Decision 6's ordering, that
   the identity exists before the file, is preserved as stated.

4. An agent-established identity yields to a runtime-assigned one. When work carrying an
   agent-established `Effort:` value later runs under a runtime that assigns an active ID, the
   next checkpoint rewrites the `Effort:` line to the runtime ID before any handoff is
   attempted. This is Decision 1's first clause applied over time, and it closes the otherwise
   silent dead end where `handoff_session` refuses a slug that can never equal a UUID.

5. The working-memory protocol ships complete. Every sentence currently in
   `.awf/parts/workflow/working-memory.md` moves into the `working-memory` section of
   `templates/docs/workflow.md.tmpl`, with the Pi-specific sentences behind
   `{{if .targetSessionHandoff}}` and their existing runtime-scoped phrasing retained, per
   ADR-0157's constraint that singletons render once and their Pi-gated prose must not
   misdirect a non-Pi session. The convention part is deleted: nothing in it is specific to
   this project. The identity protocol from Decisions 1 through 4 is authored in that same
   shipped section, so adopters and this repository render identical text.

6. The point-of-use clause is short, harmonised, and citation-free. All three authoring sites
   (`templates/partials/checkpoint-routine.md`, `templates/partials/checkpoint-approval.md`,
   and `templates/skills/brainstorming/SKILL.md.tmpl`) carry one identical phrasing of the
   two-source rule and the narrowed prohibition, replacing the three divergent phrasings that
   exist today. The full semantics stay in the shipped workflow-doc section. No template
   prose cites an ADR by number, since `TestTemplateSourceResidue` scans the whole embedded
   template source and rejects any concrete `ADR-NNNN` citation.

7. Both partials' clause is proven by its own invariant's test. `TestMemoryCheckpointCoverage`
   gains the clause in its ordered-phrase list and `TestMandatoryApprovalBoundaries` gains a
   matching assertion, so neither partial carries rendered-but-unproven text and each revised
   claim is backed by a proof that actually reads the surface it describes.

## State changes

- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/guide-and-doc-templates:working-memory-single-home`

## Consequences

Working memory becomes creatable on every target. The instruction that suppressed the file
outside Pi is gone, and the failure mode that prompted this decision cannot recur through the
same reasoning.

Adopters gain the identity protocol they never had. Because Decision 5 moves the prose into the
shipped template, a Pi-enabled adopter's rendered workflow doc gains the effort-identity,
resume, and one-way-ledger sentences on next sync, and the `working-memory-single-home` pointer
resolves to real content for the first time. Non-Pi adopters gain the two-source rule and the
narrowed prohibition. This repository stops carrying a local copy of standard prose and
therefore renders exactly what it publishes.

The `working-memory-single-home` claim needs its boundary stated rather than assumed. The
partials now carry a short operational clause rather than a pure pointer, which is consistent
with ADR-0152 Decision 5's rule that protocol text is embedded where it fires; the claim is
updated to distinguish canonical prose that the partials point at (skeleton, ground rules,
just-in-time retrieval, and now the full identity semantics) from operational protocol steps
that they embed.

Cross-runtime continuation acquires a documented transition where it previously had a silent
failure. Decision 4 does not make an agent-established identity acceptable to
`handoff_session`; it requires the line to be rewritten before handoff is attempted. An effort
that skips that rewrite still fails the equality check, which ADR-0152 Decision 7 already
degrades to a check-in rather than a corrupting outcome.

The prohibition in Decision 2 remains advisory on targets without a ledger. An agent on a
non-Pi target can check `.awf/memory/` and the prose it has read, but nothing mechanically
prevents it from reusing a retired identity. This is accepted: the risk is a confusing filename,
not a corrupted ledger, since an agent-established identity creates no ledger entry. That
distinction is also why this decision does not inherit ADR-0149's rejection of creating an
effort at every fresh session start: no durable effort is created here, only a file.

The rendered surface grows slightly in every checkpoint-carrying skill, and `awf sync`
regenerates roughly a hundred rendered files across every target and the bundled example. The
transaction is larger than its authored diff suggests, and the claim mutations must travel in
the same checked transaction as the render.

Two adjacent overrides are deliberately left in place. `.awf/parts/agents-doc/working-memory.md`
stays, because ADR-0157 Decision 6 caps the guide's Pi branch at the routing minimum and keeps
lifecycle detail in the surfaces that carry it, making that part a genuine local choice rather
than residue. `.awf/parts/working-with-awf/config-and-overrides.md` stays for now, because
`templates/docs/working-with-awf.md.tmpl` carries no `targetSessionHandoff` conditional at all;
giving it one is new shipped-template capability and belongs to its own decision. Both, and the
nine convention parts that fully replace their shipped default rather than appending to it, are
left to a follow-on audit of where this project over-overrides the standard it publishes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Treat the fix as doc clarity and write no ADR | The change narrows a rule that a reader will reasonably attribute to ADR-0149; without a recorded reconciliation, "the agent establishes the id" reads as contradicting "never mine for identity". Naming who assigns the identity also revises claim text, which ADR-0134 routes through an ADR. |
| Pointer-only in the partials, all semantics in the workflow doc | Contradicts ADR-0152 Decision 5, which legislated against referencing a distant definition instead of embedding protocol at the point of use. The failure this decision fixes happened at the point of use. |
| Render different identity text per target behind a conditional | No signal expresses "this target has a workflow router"; `targetSessionHandoff` denotes handoff capability, and reusing it would need a new signal. Unified text degrades correctly without one, since a target with a router always has an active ID and the second clause never fires. ADR-0157 also records dead target conditionals as a live hazard in these templates. |
| Also strengthen the "working memory is optional" framing | ADR-0152 Decision 8 already establishes that a non-trivial brainstormed effort becomes memory-backed at its first settled decision, and the routine partial's wording correctly governs mid-effort boundaries rather than optionality in general. Changing it would rewrite canonical text pinned in two proofs, a claim, and the glossary for no defect. |
| Fix the identity rule in this project's convention part only | Leaves every adopter with a partial that points at content their workflow doc never receives, and keeps awf diverging from the standard it publishes. |

## Status history

- 2026-07-26: Proposed
