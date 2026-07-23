---
format: current-state-v2
status: Implemented
date: 2026-07-23
---
# ADR-0155: Implementer-side context grounding in workflow skill templates

## Context

The workflow chain delivers current-state authority to reviewers but not to implementers. The
2026-07-22 instruction-surface audit (the session that produced ADR-0147) found that the skills
whose agents edit files under claims never instruct running `awf context`: `executing-plans`,
`subagent-driven-development`, `writing-plans`, `bugfix`, `debugging`, `tdd`, and
`refactor-coupling-audit` carry no grounding step at all, so claim violations are caught late by
reviewers instead of prevented at edit time. The sharpest gap is `subagent-driven-development`,
whose fresh-context implementer children work blind under topics carrying dozens of claims.
ADR-0147 is the enabler: its topic-grouped `--full` packet dropped roughly 18x in size over a
multi-file diff (310KB to 16.7KB measured), making per-task grounding cheap.

Grounding against the current templates (2026-07-23) established the instruction surface
precisely. Exactly four skills invoke `awf context` today, pinned by
`TestManagedContextCallersChooseProjection` (`internal/project/spine_test.go`): `reviewing-impl`
and `reviewing-plan` paste `awf context --full` output into their reviewer briefs,
`adr-lifecycle` references `--full` output descriptively, and `brainstorming` both instructs a
concise run for its own orientation and pastes that already-held output into its
grounding-check brief. `reviewing-plan-resync` dispatches the same plan-reviewer as
`reviewing-plan`, restricted to the scope-completeness and doc-currency lenses, with no context
bullet at all, although doc-currency is one of the two lenses the sibling's packet exists to
feed; that omission is accidental. `reviewing-adr` dispatches with no packet for a structural
reason: its subject is a decision document, and `awf context <adr-path>` special-cases ADR paths
to lifecycle-progress reporting rather than path-claim grounding.

Delivery mechanics were verified against both runtimes: every dispatched child can execute
commands. Pi reviewer children hold `bash` in `REVIEW_TOOLS` and the report-only wrapper forbids
only editing and committing (`.pi/extensions/awf-subagents/index.ts`); the Pi implement role
holds `bash` in `IMPLEMENT_TOOLS`; rendered Claude reviewer agents carry no tool restriction.
Pasting packet output into a dispatch brief is therefore never a capability requirement, and it
costs the orchestrating parent real context: for `reviewing-impl` the parent runs a command it
does not need, ingests roughly 17KB, and re-emits it, against the explicit project goal of
keeping the orchestrating parent lean. The one site where pasting is free is `brainstorming`,
whose parent already holds the concise output for its own use. The reconciliation rule for
projection choice ("managed complete-authority callers use `--full`, orientation stays
concise") lives only in the tooling domain narrative, on no surface an agent reads at decision
time. `skill-prose-tool-agnostic` permits CLI commands in skill prose (the shell `grep` is
explicitly allowed, and `brainstorming` already ships instruct-style `awf context` prose), so
instruct-style grounding steps fit existing convention.

## Decision

1. **Delivery principle.** The dispatching parent always passes the resolved argument set and
the exact command; it pastes command output into a child brief only when it already holds that
output for its own purposes. Grounding is otherwise instruct-style: the agent that needs the
packet runs the command itself.

2. **Two caller camps, unchanged in spirit.** Implementer and orientation callers run concise
`awf context` and drill down with `awf topic` on demand; complete-authority callers (reviewer
dispatches and `adr-lifecycle`) use `awf context --full`. No third mode is introduced. The
`tooling/context-and-topic:context-full-authority-packet` claim text changes accordingly: its
complete-authority caller set grows to include the resync dispatch, and its delivery clause
becomes instruct-style, the dispatching parent passing resolved arguments rather than pasted
output.

3. **Seven implementer skills gain a concise instruct-style grounding step**, each placed at
the natural moment and carrying a one-clause in-place rationale (orient, then drill down with
`awf topic` as needed): `executing-plans` before implementing each task, over the task's named
paths, falling back to the plan's file-structure header paths when a task names none;
`subagent-driven-development` in each per-task brief, as the resolved command the implementer
child runs first; `writing-plans` while drafting the file-structure header and tasks, over the
plan's touched paths; `bugfix` and `tdd` over the implementation and test paths before writing
the failing test; `debugging` once suspect files are located, before proposing a fix;
`refactor-coupling-audit` over the refactor scope, feeding the ADR Context section.

4. **`reviewing-impl` and `reviewing-plan` convert from paste to instruct.** The dispatch brief
passes the resolved arguments and instructs the reviewer to run the command itself:
`reviewing-impl` passes the concrete SHAs and instructs
`awf context --full $(git diff --name-only <base>..<head>)` with those SHAs substituted;
`reviewing-plan` passes the plan's file-structure paths and instructs `--full` over them. The
context bullet stays in the shared bullet list outside the `{{ if .targetSubagentTools }}` fork
so every target's render carries it.

5. **`reviewing-plan-resync` closes its gap** by adding the same instruct-style bullet as its
sibling: `--full` over the plan's file-structure paths.

6. **Deliberate omissions are documented, not closed.** `brainstorming` keeps its held-output
paste and concise orientation run unchanged. `reviewing-adr` stays packet-free (ADR paths
report lifecycle progress, not path-claim grounding) and instead gains a hint that its reviewer
may run `awf topic <domain>/<topic>` on the destination topics named in the ADR's State
changes. `exploring` stays grounding-free as a generic dispatcher whose target paths are
unknown up front. The tooling domain narrative (authored parts) records the updated
reconciliation rule and both deliberate omissions.

7. **Enforcement extends in the same commit as the template edits.**
`TestManagedContextCallersChooseProjection` adds the seven implementer skills to its concise
map and `reviewing-plan-resync` to its complete map; the test's existing presence requirement
then covers all twelve mapped skills, the enlarged maps being the only new enforcement. The
added claim `rendering/workflow-skill-templates:implementer-context-grounding` is an invariant
with `Backing: test`, proven by an
`invariant: rendering/workflow-skill-templates:implementer-context-grounding` marker on that
same test alongside its existing marker.

8. **Publication safety.** The new and converted grounding prose interpolates no new template
vars; where a grounding step sits inside an existing conditional branch it degrades to
coherent generic prose when the var is unset, preserving the no-unresolved-token invariant.

9. **Adopter visibility.** The template changes are adopter-visible skill drift resolved by
next sync; `changelog/CHANGELOG.md` gains an `[Unreleased]` entry in the same change.

10. **Index currency.** Every status transition of this ADR regenerates
`docs/decisions/INDEX.md` via `./x sync` in the same commit.

## State changes

- add `rendering/workflow-skill-templates:implementer-context-grounding`
- update `tooling/context-and-topic:context-full-authority-packet`

## Consequences

- Claim violations move from late detection (reviewer findings) to early prevention: every
agent that edits files under claims orients on the applicable topics before touching them.
- The orchestrating parent stays lean: reviewer dispatches no longer route a roughly 17KB
packet through parent context, and `subagent-driven-development` avoids multiplying that cost
per task.
- Guaranteed packet presence is traded for instructed compliance: a child that skips its
grounding step works ungrounded. Accepted because the same briefs already rely on instruction
compliance for every other step, and the spine test guarantees the instruction itself is
present in every rendered skill.
- The spine test now pins twelve skills instead of four; any future skill that adds a context
invocation must classify itself in the same commit, which is the intended guardrail.
- Adopters see skill drift on next sync across eleven templates; the changelog entry makes the
change discoverable.
- More template prose to maintain: seven new grounding steps plus two converted and one added
reviewer bullet, each carrying a one-clause rationale.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Paste `--full` packets into every dispatch brief (original pattern extended) | Guarantees presence but routes every packet through parent context, multiplied per task in subagent-driven development; contradicts the lean-parent goal for no capability gain since all children can run commands. |
| Concise everywhere, reviewers included | Loses the complete-authority packet reviewers need; reviewers should not spend rounds drilling down claim by claim. |
| `--full` everywhere, implementers included | Bloats implementer context with detail they may never need; concise plus on-demand drilldown fits the orientation posture. |
| Core-four scope (executing-plans, subagent-driven-development, writing-plans, bugfix) first | Leaves the chain half-grounded for little savings; each remaining edit is a small template addition following the same pattern. |
| Promote the reconciliation rule to AGENTS.md | Guide bloat for a rule that becomes self-executing once every skill carries a correctly-projected command; the narrative and spine test are the durable homes. |
| One shared grounding include instead of seven per-skill steps | The placement moment and the path source differ per skill (task paths, plan header, suspect files, refactor scope); a shared partial cannot parameterize the step without the deferred template-partials work. |
| Close the `reviewing-adr` gap too | Its reviewer's authority inputs are the ADR text and destination topics; `awf context` on an ADR path answers a different question (lifecycle progress). A topic-query hint serves the actual need. |

## Status history

- 2026-07-23: Proposed
- 2026-07-23: Implemented; content-sha256: 9a4d246018e4eafdb0a003df919cd851a35274d32f0843dea8d810c7881f1be2; state-sequence: 36
