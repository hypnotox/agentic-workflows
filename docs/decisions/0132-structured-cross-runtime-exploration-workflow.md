---
status: Proposed
date: 2026-07-19
tags: [multi-target, subagent-dispatch, workflow-chain]
related: [38, 122, 123, 125]
domains: [rendering, tooling]
---
# ADR-0132: Structured Cross-Runtime Exploration Workflow

## Context

awf already gives Pi adopters a fresh-context `subagent_explore` tool. Its
child process keeps search commands and intermediate reasoning out of the
parent model context, but its public contract accepts only a free-form `task`
and its fixed prompt asks merely for concise `file:line` findings. Search
breadth, report depth, stopping criteria, and not-found behavior are therefore
reinvented by each caller. The large refactor coupling audit is the only
workflow skill with specialized exploration dispatch; brainstorming searches
for initial context inline, and debugging has no isolation protocol.

That gap matters beyond Pi. awf renders the same workflow standard for Claude
Code, Codex, Copilot, Cursor, Gemini, and Pi. The non-Pi harnesses can use their
native delegation facilities, while Pi permits a deeper awf-owned integration
through its generated extension. The workflow needs one semantic exploration
contract that every target can follow without pretending every runtime exposes
the same tool API.

Two independent choices shape an investigation. Breadth controls how far the
child may search; report detail controls how much of the discovered evidence
returns to the parent. Conflating them prevents useful combinations such as a
repository-wide path map without narrative or a deep explanation of one known
implementation. A breadth request must also bound absence claims: without a
defined search universe, "not found" can be mistaken for proof that something
does not exist.

Grounding established several implementation constraints. A skill named
unconditionally by core brainstorming must itself be core or the default
scaffold becomes invalid. Skill dependencies are directional and the graph
validator does not reject cycles. The existing target template projection
exposes only Pi's closed `targetSubagentTools` capability, so exact syntax for
each non-Pi runtime would enlarge the target seam and conflict with the current
tool-agnostic prose convention. Finally, ADR-0125 Decision item 1 explicitly
keeps existing public schemas unchanged and its exact-contract invariant owns
the current one-field exploration schema, so this change requires a successor
record rather than an in-place edit.

## Decision

1. Add `exploring` to the standard catalog as a core, runtime-neutral task
   skill. It owns the reusable parent-side protocol for deciding when to
   isolate repository investigation, selecting breadth and detail, constructing
   one self-contained task, dispatching one fresh-context child, consuming its
   final report, and optionally issuing a sequential refinement call. Skills
   that name it declare a one-way `requiresSkills` dependency on it; exploring
   declares no reciprocal workflow dependency.

2. Bump the config schema generation and add an ordered migration that reruns
   the existing close-enabled-set operation against the revised catalog. It
   adds exploring to every adopted config whose enabled brainstorming,
   debugging, or refactor coupling audit now requires it before ordinary
   project-open validation runs. The migration reports each addition through
   the existing closure diagnostics and updates the schema/version compatibility
   tables required by the binary-version gate.

3. Invoke exploring when the parent needs repository information whose location
   it does not know and the search transcript would pollute the main session.
   Keep an exact-known-file or genuinely trivial lookup inline. Each dispatch
   carries one information need; unrelated questions are not bundled. The
   child is stateless and cannot retain a search session. After a not-found,
   inconclusive, unverified, or insufficient report, the parent may issue a new
   fresh-context call with a corrected task, different report detail, or wider
   breadth.

4. Require every exploration dispatch to supply a non-empty `task`, a breadth
   maximum, and a report detail. Breadth is the ordered enum
   `targeted | bounded | broad`:
   - `targeted` locates one declaration, implementation, file, or exact fact;
   - `bounded` investigates within a named symbol, package, component, or
     subsystem;
   - `broad` searches across the project search universe, including relevant
     source, tests, documentation, decisions, and workflow artifacts.

   The child starts with the cheapest targeted lookup and widens only when the
   evidence requires it, never beyond the selected maximum. If the boundary is
   exhausted, it reports that explicitly rather than silently widening.

5. Define report detail as the ordered enum `paths | summary | analysis`,
   independent of breadth:
   - `paths` returns only relevant `file:line` or `file:start-end` locations,
     with minimal labels needed to distinguish them and no search narrative;
   - `summary` returns grounded locations plus concise explanations of what
     each contains and why it matters;
   - `analysis` directly answers the task and grounds its synthesis of
     relationships, call flow, usage patterns, assumptions, and uncertainty.

   Reports distinguish not-found, inconclusive, and unverified outcomes. A
   not-found result is successful tool execution and begins `Not found within
   <breadth> boundary: <what was searched>`. It may suggest one concise next
   refinement. An inconclusive or unverified result never becomes an absence
   claim. These are semantic text contracts, not a JSON result schema.

6. Define the broad project search universe as tracked files plus non-ignored
   untracked working-tree files under the current repository root. It includes
   tracked generated and vendored files. It excludes ignored files, `.git`,
   nested-repository contents, and external dependencies unless the task
   explicitly brings one of those surfaces into scope. A broad absence report
   names this project universe and its searched surfaces rather than claiming
   unrestricted filesystem absence.

7. Change Pi's existing `subagent_explore` public schema to require exactly
   `{task, breadth, detail}` with the enums above and reject additional
   properties. Its fixed child prompt receives the selected values and enforces
   the adaptive maximum, report contract, explicit outcome distinctions,
   project search universe, evidence grounding, and one-information-need
   policy. The extension still exposes exactly grounding, exploration,
   governed review, and serialized implementation; grounding keeps required
   `{task}`, review keeps required `{kind, task}`, and implementation keeps
   required `{task, allowCommits}`. The exploration allowlist, inherited model
   and thinking level, no-session process, fixed output bounds,
   context-isolated progress, renderers, cancellation, and failure behavior
   remain unchanged. This replaces ADR-0125 item 1 because its
   unchanged-existing-schemas clause is no longer true while preserving that
   item's four-role boundary and every non-exploration schema.
   `supersedes: ADR-0125#1`
   `cites: ADR-0125#2`
   `supersedes-invariant: ADR-0125#pi-subagent-four-tool-contract`

8. Render the exploring skill for every enabled target. Pi instructions call
   the awf-owned structured tool explicitly. Every other target instructs the
   parent to use an available target-native fresh-context exploration subagent
   and embeds the same task, breadth, detail, boundary, outcome, and report
   semantics in its dispatch brief. Non-Pi prose stays generic rather than
   naming runtime APIs; exact per-runtime invocation syntax and a wider target
   capability projection are out of scope. Pi remains awf's primary harness
   for deep custom orchestration because its extension model permits that
   integration; other targets receive semantic workflow parity, not a promise
   of identical extension machinery. This deliberately adapts the
   tool-agnostic prose rule: generic action language remains the standard, but
   an awf-owned Pi tool is a project-specific identifier used only in the
   capability-selected Pi rendering.
   `refines: ADR-0038#1`

9. Integrate the shared protocol into brainstorming's initial context search,
   debugging's evidence-location loop, and the refactor coupling audit. Keep
   brainstorming's later dedicated grounding operation unchanged. Add
   exploring to the agent guide's task-skill guidance. In the behavior commits,
   update `README.md`, `docs/working-with-awf.md`, `docs/architecture.md`,
   `docs/releasing.md`, `docs/testing.md`, `changelog/CHANGELOG.md`, the
   rendering and tooling current-state parts under `.awf/domains/parts/`,
   `templates/agents-doc/AGENTS.md.tmpl`, the Sundial config and README, and all
   generated AGENTS.md, domain-doc, target-skill, and example-adopter outputs.

10. Preserve the report-only boundary as policy. Exploration receives `read`,
   `grep`, `find`, `ls`, and `bash`; `bash` means neither no-mutation nor
   no-recursion is an OS-enforced sandbox. The skill and prompt prohibit edits,
   commits, and recursive delegation, while tests prove the closed direct tool
   allowlist and instruction contract rather than arbitrary model compliance.

11. Test the revised exact Pi schema and prompt in the TypeScript extension
    harness, including every enum value, rejected shapes, representative report
    combinations, not-found/refinement wording, and all unchanged process and
    context-isolation boundaries. Go catalog/render tests prove the core and
    dependency graph, six-target skill rendering, Pi structured dispatch,
    generic non-Pi native dispatch, generated-output fanout, and replacement
    exact-contract invariant. Every new or modified template preserves
    `missingkey=zero`, renders coherent generic prose when variables are empty,
    and emits no unresolved or no-value token. Each backed invariant below
    receives a matching `// invariant: <slug>` annotation on an executed Go
    test under `**/*_test.go`: Pi contract, cross-runtime dispatch, and bounded
    reporting proofs live in `internal/project/target_test.go`, while the skill
    closure proof lives in the catalog/scaffold test group. `internal/evals`
    covers only a composed cross-artifact skill-to-runtime seam, not
    single-prompt behavior.
    The existing credential-free gate remains deterministic; release smoke
    guidance adds one successful exploration and one not-found/refinement
    sequence on real Pi.

12. In the final lifecycle commit, record implementation deviations, flip this
    ADR to Implemented, and run `./x sync`. Commit the regenerated
    `docs/decisions/ACTIVE.md` and rendering/tooling domain indexes with the
    status change. No `docs/decisions/README.md` row is owed because ADR-0005
    defines that file as a how-to guide rather than the decision index.

## Invariants

- `invariant: pi-structured-exploration-contract`: the generated Pi extension
  exposes exactly grounding, exploration, governed review, and serialized
  implementation; exploration requires the closed task, breadth, and detail
  schema and receives the selected structured policy without changing the
  other role schemas or process boundaries.
- `invariant: cross-runtime-exploration-dispatch`: the core exploring skill
  renders for every target with one semantic breadth/detail protocol, Pi uses
  its awf-owned tool, and non-Pi targets require generic target-native
  fresh-context delegation without Pi tool-name leakage.
- `invariant: exploration-skill-closure`: every standard skill that names
  exploring declares the one-way requirement, the default core scaffold
  includes exploring, and the dependency graph introduces no reciprocal edge.
- `invariant: bounded-exploration-reporting`: rendered exploration guidance and
  Pi's fixed prompt define adaptive-maximum breadth, the project search
  universe, grounded detail contracts, explicit outcome distinctions, and
  parent-driven sequential refinement.

## Consequences

- Agents gain a consistent way to move repository discovery out of the main
  context while requesting only the information the parent needs.
- Independent breadth and detail prevent over-searching and over-reporting, but
  add two required choices to every Pi invocation and every native dispatch
  brief.
- Explicit search-universe language makes absence reports more honest. It also
  means ignored build artifacts, nested repositories, and external dependencies
  require deliberate task scope rather than accidental discovery.
- The standard gains a twelfth core skill and additional requirement edges.
  Existing adopters receive exploring through the schema migration before the
  revised requirement closure is validated; no otherwise-valid adopted tree is
  stranded behind a project-open error it cannot repair.
- Pi receives the strongest integration and deterministic schema checks. Other
  targets depend on their native delegation capabilities and generic semantic
  instructions, so invocation ergonomics may differ without weakening the
  workflow contract.
- Prompt policy cannot guarantee that an arbitrary model never mutates through
  `bash`, recursively launches a process, or exceeds a semantic boundary.
  Closed direct tools, explicit instructions, bounded output, and honest docs
  expose that limitation rather than claiming sandboxing.
- A public schema change may break a hand-authored Pi prompt that calls
  `subagent_explore` with only `task`; the required arguments make the failure
  immediate and actionable instead of silently selecting an unintended search
  shape.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `{task}` and encode every choice in prose | Preserves the public schema but leaves callers to reinvent vocabulary, boundaries, and output shape. |
| Add optional breadth and detail with defaults | Preserves old calls, but silently retains an ambiguous search shape and defeats the requirement that every caller deliberately select both dimensions. |
| Add structured Pi arguments without a shared skill | Improves Pi while leaving the standard workflow and native-subagent targets inconsistent. |
| Put exploration rules directly in every consuming skill | Avoids one artifact but duplicates a policy expected to gain more call sites. |
| Add exact dispatch syntax for every runtime | Requires a larger target projection, couples prose to changing runtime APIs, and conflicts with the existing tool-agnostic convention. |
| Let the child widen beyond the requested breadth | May find more evidence, but removes caller control and makes cost and absence semantics unpredictable. |
| Return a rigid JSON result | Easier to parse mechanically, but less useful to a parent consuming paths, summaries, and evidence-grounded analysis of different shapes. |
| Treat ignored files and external dependencies as part of every broad search | Expands cost and environment sensitivity while weakening the meaning of the project-owned search boundary. |
