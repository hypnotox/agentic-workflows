---
format: plan-v2
date: 2026-08-06
adrs:
  - bound-agent-guides-as-native-skill-routers
status: Implemented
---
# Plan: Bound agent guides as native-skill routers

## Goal

Make awf's shipped and self-hosted agent guides terse native-skill-aware dispatch layers, keep their
full procedure in existing canonical homes, and warn without failing when an adopter's managed guide
regrows past the fixed advisory bound. The change does not add a fallback skill catalog, configurable
budgets, section attribution, generic document metrics, or a new diagnostics framework.

## Architecture summary

Implementation proceeds through three independently green transactions. The first removes the
catalog copy and relocates reusable model-tier definitions while retaining operative dispatch rules
in native skills. The second compresses default and self-hosted guide prose, strengthens the authoring
standard, and proves the 8 KiB direct-default and 10 KiB self-hosted bounds. The third observes the
already-rendered expected guide only in aggregate `CheckReport.Notes` and adds the 12 KiB advisory.
Existing render, output-plan, check-report, presentation, documentation, skill, and current-state
owners remain in place; no preparatory refactor or new package is needed. Each phase applies exactly
its current-state operation batch, keeps the ADR `Implementing`, regenerates owned outputs, and closes
with one commit. Terminal review owns the later status-only ADR implementation and plan-freeze
transaction.

## Phase 1: Route the guide through native skill discovery

**Execution mode: subagent-driven.**

Completes: ["native-guide-routing"]

### Task 1.1: Retire the rendered skill catalog and its obsolete proofs
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:native-skill-discovery-owns-catalog", "bound-agent-guides-as-native-skill-routers:guide-is-dispatch-layer"]
Paths: ["internal/project/render.go", "templates/agents-doc/AGENTS.md.tmpl", "internal/project/guide_scopes_test.go", "internal/project/spine_test.go", "internal/project/drift_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

Delete `Project.skillRows`, the `skillRows` template-data key, and the guide's enabled-skills block.
Retain `.skills` in render data because other templates and config-hash behavior still consume the
effective skill set. Replace `TestGuideCatalogRowsAreCompleteSafeAndAdvisory`,
`TestAgentsDocGuide`, and `TestAgentsDocTaskSkillsGating` assertions with proofs that the Workflow
section tells the agent to use enabled native skills whose exposed descriptions fit; contains no
standard or local skill name, purpose, trigger, kind, relationship, or fallback row; and remains
coherent with empty prefix, vars, data, and skills. Preserve missingkey=zero behavior, no unresolved
no-value token, generic Conventional Commits fallback, commit-scope derivation, and the other
non-roster guide contracts. Update the config-hash test so changing enabled skills no longer marks
neutral `AGENTS.md` stale merely to mirror native frontmatter, while artifacts that still consume
`.skills` remain covered by `skills-set-in-confighash`.

### Task 1.2: Move full model-tier definitions to their canonical document
Kind: batch
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:guide-definition-ownership"]
Paths: ["templates/partials/model-selection.md", "templates/agents-doc/AGENTS.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/parts/working-with-awf/config-and-overrides.md", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/exploring/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "internal/project/subagent_model_selection_test.go"]
Representative: "The brainstorming and review skill templates retain their operative smallest-reliable-tier, escalation, and target-branch rules, but point to docs/working-with-awf.md rather than the agent guide for the reusable tier definitions."
Edge: "Generic empty-data renders remain provider-neutral and leak-free; AGENTS.md contains no full tier definition, while working-with-awf contains the shared full definition exactly once even when project-specific configuration prose is overridden."
Post-check: "Render the guide, working-with-awf document, and all eight affected skills for Claude and Pi; targeted tests prove every governed dispatch keeps its operative rule, both target branches remain coherent, the full definition occurs exactly once in working-with-awf, and no stale guide pointer remains."

Remove the shared model-selection include from the guide and retain the include already present at
the governed-dispatch section of `templates/docs/working-with-awf.md.tmpl`; do not add a second
include. Reconcile the self-hosted working-with-awf part so it does not duplicate the include and
points to its own rendered document section. Change every affected
skill pointer from the agent guide's Workflow section to `docs/working-with-awf.md` without changing
model selection semantics, tool-neutrality, or target-specific branches. Rewrite the existing
model-selection proof around this ownership; do not fork or copy the shared definitions.

### Task 1.3: Apply native-routing claims and regenerate target outputs
Kind: batch
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:native-skill-discovery-owns-catalog", "bound-agent-guides-as-native-skill-routers:guide-is-dispatch-layer", "bound-agent-guides-as-native-skill-routers:guide-definition-ownership"]
Paths: ["docs/decisions/bound-agent-guides-as-native-skill-routers.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/INDEX.md", "AGENTS.md", "docs/working-with-awf.md", "docs/topics/rendering/guide-and-doc-templates.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/domains/rendering.md", ".claude/skills/awf-brainstorming/SKILL.md", ".claude/skills/awf-exploring/SKILL.md", ".claude/skills/awf-executing-plans/SKILL.md", ".claude/skills/awf-subagent-driven-development/SKILL.md", ".claude/skills/awf-reviewing-adr/SKILL.md", ".claude/skills/awf-reviewing-plan/SKILL.md", ".claude/skills/awf-reviewing-plan-resync/SKILL.md", ".claude/skills/awf-reviewing-impl/SKILL.md", ".pi/skills/awf-brainstorming/SKILL.md", ".pi/skills/awf-exploring/SKILL.md", ".pi/skills/awf-executing-plans/SKILL.md", ".pi/skills/awf-subagent-driven-development/SKILL.md", ".pi/skills/awf-reviewing-adr/SKILL.md", ".pi/skills/awf-reviewing-plan/SKILL.md", ".pi/skills/awf-reviewing-plan-resync/SKILL.md", ".pi/skills/awf-reviewing-impl/SKILL.md", ".awf/awf.lock"]
Representative: "AGENTS.md routes selection to harness-exposed native descriptions without enumerating awf-brainstorming or its neighbors, while the Claude and Pi brainstorming skills retain the same operative dispatch rule and point to working-with-awf for tier definitions."
Edge: "An empty/minimal project renders a coherent guide with no fallback roster, and a project with local skills does not leak those local names into the neutral guide; generated target skills still carry their native frontmatter descriptions."
Post-check: "After `./x render`, read every reported changed output and run a focused semantic review over AGENTS.md, docs/working-with-awf.md, one representative Claude skill, one representative Pi skill, and empty-data goldens: no contradictory fragments or stale guide pointer remain, paraphrases preserve model-routing meaning, and every literal placeholder is intentional. `rg -n 'Enabled skills:|the agent guide.s workflow section|skillRows' AGENTS.md templates internal/project .awf/parts/working-with-awf .claude/skills/awf-* .pi/skills/awf-*` returns no live-surface residue after separately confirming the probe paths exist."

Transition the ADR directly from `Proposed` to `Implementing` in this transaction. Compute its
canonical content stamp after all permitted body edits are complete, set frontmatter status to
`Implementing`, append the stamped Implementing event, and append one Applied event naming exactly:

- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/workflow-skill-templates:deliberate-subagent-model-selection`

Rewrite the first claim around native-skill routing and absence of a duplicated catalog. Rewrite the
second around operative skill-local rules and one full definition in working-with-awf. Preserve each
claim's Origin and prior Revised-by sequence, append this ADR once, retain test backing, and place
proof markers on the renamed/replacement tests. Run `./x render`, read back every mutation target,
and retain every generated guide, doc, target skill, topic, domain, decision-index, and lock output
reported for the transaction. Confirm the other three ADR operations remain Remaining.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
refactor(rendering): use native skill discovery (applies ADR batch)
```

## Phase 2: Enforce progressive disclosure and guide bounds

**Execution mode: subagent-driven.**

Completes: ["terse-guide-bounds"]

### Task 2.1: Compress the shipped guide and strengthen its authoring contract
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:guide-is-dispatch-layer", "bound-agent-guides-as-native-skill-routers:guide-authoring-cost-test", "bound-agent-guides-as-native-skill-routers:guide-definition-ownership", "bound-agent-guides-as-native-skill-routers:fixed-guide-budgets"]
Paths: ["templates/agents-doc/AGENTS.md.tmpl", "templates/docs/agents-md-standard.md.tmpl", "internal/project/spine_test.go", "internal/project/docs_sections_test.go", "internal/project/render_tree_test.go", "internal/project/agents_doc_budget_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

Add focused tests before the prose change that render the direct default guide and require no more
than `8*1024` bytes, and that read this repository's committed `AGENTS.md` and require no more than
`10*1024` bytes. Use a direct default render for the shipped bound, not the authored adopter fixture;
use the existing project-surface test helper or an equally direct repository-root seam for the
self-hosted bound rather than a working-directory assumption. Make failure output report observed and
allowed bytes plus the largest section contributions for diagnosis without creating production
section attribution.

Then reduce the default Working with awf section to generated ownership, the render/check loop, and
its canonical pointer; reduce Workflow to native-skill selection, approved-design preservation,
authority-lifetime routing, and commit discipline; reduce Working memory to the minimum effort/resume
trigger, one-writer boundary, and canonical workflow pointer; and keep Commands and Document map as
concise executable navigation. Compress default invariant prose to imperative statements with
canonical references instead of mechanisms. Update the authoring standard's Layout, Content, and
Rules so native skill inventories, procedure, rationale, and mechanism narration are prohibited; each
sentence must be necessary before native skill selection or point to canonical authority; and byte
budgets are regression signals rather than fill targets. Preserve section order, part replacement,
mandatory-document mapping, missingkey=zero, empty-string fallbacks, and no unresolved-value token.

### Task 2.2: Make awf's self-hosted guide model the shipped standard
Kind: batch
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:guide-is-dispatch-layer", "bound-agent-guides-as-native-skill-routers:guide-authoring-cost-test", "bound-agent-guides-as-native-skill-routers:guide-definition-ownership", "bound-agent-guides-as-native-skill-routers:fixed-guide-budgets"]
Paths: [".awf/agents-doc.yaml", ".awf/parts/agents-doc/awf-setup.md", ".awf/parts/agents-doc/you-and-this-project.md", ".awf/parts/agents-doc/identity.md", ".awf/parts/agents-doc/working-memory.md", ".awf/parts/agents-doc/commands.md", ".awf/parts/working-with-awf/commands.md", ".awf/parts/working-with-awf/config-and-overrides.md"]
Representative: "The append-only ADR, staged gate, backed-invariant, plain-punctuation, memory-citation, coverage, dead-code, and version-gate guide bullets become terse cross-cutting imperatives with an ADR or canonical-doc pointer; their enforcement grammar remains in current-state topics, workflow, working-with-awf, or the decision guide."
Edge: "Merge authorization, checkpoint/handoff ordering, hook resolution, command detail, and generated-runtime descriptions appear in their canonical docs but not as copied procedure in AGENTS.md; genuinely cross-cutting obligations and the exact commands an agent runs remain discoverable before mutation."
Post-check: "Build a section byte census from the rendered AGENTS.md before and after the edits, require the final file to satisfy the 10 KiB test without weakening the bound, and inspect each removed paragraph against docs/decisions/README.md, docs/doc-standard.md, docs/workflow.md, docs/working-with-awf.md, or its owning current-state/ADR source. `./x render` followed by a focused semantic reading must show no contradictory fragments, lost cross-cutting rule, accidental placeholder interpretation, or duplicated procedure."

Trim the self-hosted parts and invariant data rather than hand-editing `AGENTS.md`. The completed
census has no guide-unique operational fact to relocate: merge authorization, hook resolution,
command flags, context-spill behavior, and version-gate detail are already canonical in the authored
working-with-awf parts; effort creation, working-memory ownership, checkpoint/handoff ordering,
integration, and retrospective are already canonical in the workflow template and native skills;
ADR lifecycle detail is already canonical in the decision guide and lifecycle skill; and invariant
enforcement grammar is already canonical in current-state topics and owning ADRs. Delete those guide
copies without changing the workflow document or adding another prose home. Keep Identity and
ownership stance dense and present-tense. Keep the command list to commands routinely executed and
one-line outcomes. Preserve the cross-cutting nature of every retained invariant, its owning ADR
reference where available, the publication-safe unset behavior, and this repository's
generated-source workflow.

### Task 2.3: Apply progressive-disclosure claims and regenerate the bounded guides
Kind: batch
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:guide-is-dispatch-layer", "bound-agent-guides-as-native-skill-routers:guide-authoring-cost-test", "bound-agent-guides-as-native-skill-routers:guide-definition-ownership", "bound-agent-guides-as-native-skill-routers:fixed-guide-budgets"]
Paths: ["docs/decisions/bound-agent-guides-as-native-skill-routers.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", "docs/agents-md-standard.md", "AGENTS.md", "docs/working-with-awf.md", "docs/topics/rendering/guide-and-doc-templates.md", "docs/domains/rendering.md", ".awf/awf.lock"]
Representative: "The guide carries only pre-skill routing and canonical pointers, while the workflow document remains the complete working-memory authority and the authoring standard rejects catalog and procedure duplication."
Edge: "The direct default render remains coherent below 8 KiB with empty values, and the self-hosted render remains below 10 KiB while retaining every cross-cutting invariant and necessary command entry point."
Post-check: "After `./x render`, run the direct-default and self-hosted byte-bound tests, `./x check`, and focused semantic review of the default golden, AGENTS.md, docs/agents-md-standard.md, docs/workflow.md, and docs/working-with-awf.md. Success requires both fixed bounds, no unresolved-value token, no working-memory procedure copied into the guide, and no missing canonical destination for removed prose."

Append one Applied event to the already-Implementing ADR naming exactly:

- update `rendering/guide-and-doc-templates:working-memory-single-home`
- add `rendering/guide-and-doc-templates:agent-guide-size-budgets`

Narrow the existing working-memory claim so the workflow document owns the protocol, the native
skills own operative creation/checkpoint behavior, and the guide owns only minimum pre-selection
resume/effort routing. Preserve its Origin and Revised-by history and append this ADR once. Add the
size-budgets claim with this ADR as Origin, test backing, and proof markers naming the direct-default
and self-hosted bound test. Run `./x render`, read back every reported output, retain all generated
guide, standard, workflow, working-with-awf, topic, domain, index, and lock changes, and confirm only
the aggregate-advisory operation remains Remaining.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
docs(rendering): enforce terse agent guide bounds (applies ADR batch)
```

## Phase 3: Warn on oversized managed guides

**Execution mode: subagent-driven.**

Completes: ["adopter-guide-advisory", "authority-applied"]

### Task 3.1: Prove the aggregate-only advisory before implementing it
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:fixed-guide-budgets", "bound-agent-guides-as-native-skill-routers:simple-budget-observation"]
Paths: ["internal/project/check.go", "internal/project/check_test.go", "internal/project/render_tree_test.go", "cmd/awf/checkrepo.go", "cmd/awf/checkrepo_test.go", "cmd/awf/check_presentation_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

Add `TestAgentGuideSizeAdvisoryBoundary` for an expected managed `AGENTS.md` below, exactly at, and
one byte above `12*1024`; only the overage produces one note containing observed bytes, allowed bytes,
and the `docs/agents-md-standard.md` pointer. Add
`TestCheckReportAgentGuideSizeAdvisoryManagedOnly` with `agents-doc.local: true`, proving that no
expected guide node and no size note exist. Add `TestAggregateCheckAgentGuideSizeWarning`, proving
aggregate `awf check` renders a structured warning and returns nil when it is the only finding, while
`./awf check repo drift`, initialization-compatible `AdvisoryNotes`, and other non-aggregate paths do
not receive it. Before production changes run
`go test ./internal/project ./cmd/awf -run 'Test(AgentGuideSizeAdvisoryBoundary|CheckReportAgentGuideSizeAdvisoryManagedOnly|AggregateCheckAgentGuideSizeWarning)$'`;
expect a nonzero result in which the overage note is absent and aggregate output remains completed
rather than warnings, while the below/equal/local exclusions do not produce false positives. Record
that red result, then proceed without committing the red state.

Implement one narrowly named helper over `OutputPlan.writeFiles()` or its expected `RenderedFile`
projection, matching only path `AGENTS.md` and using `len(Content)` bytes. Invoke it only inside
`Project.CheckReport` after common advisory collection, appending to `CheckReport.Notes`; do not put it
in `advisoryNotesWithState`, `AdvisoryNotes`, drift classification, resident-file observation, or
presentation code. Reuse existing aggregate note presentation. Cover absence, boundary, overage,
local ownership, stale or missing resident independence, warning ordering, and warning-only zero exit
without adding configurable thresholds, provenance accounting, or generic metrics.

### Task 3.2: Apply the advisory claim and publish the adopter-facing change
Kind: batch
Latitude: exact
Applying: ["bound-agent-guides-as-native-skill-routers:fixed-guide-budgets", "bound-agent-guides-as-native-skill-routers:simple-budget-observation"]
Paths: ["docs/decisions/bound-agent-guides-as-native-skill-routers.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", "changelog/CHANGELOG.md", "docs/decisions/INDEX.md", "docs/topics/rendering/sync-and-drift.md", "docs/domains/rendering.md", ".awf/awf.lock"]
Representative: "A managed expected AGENTS.md of 12 KiB plus one byte yields one aggregate warning naming observed and allowed bytes and the authoring guide, while all ordinary drift and current-state findings retain their existing categories and order."
Edge: "Exactly 12 KiB, local ownership, direct `./awf check repo drift`, initialization advisories, missing resident output, and an otherwise clean project yield no size warning outside aggregate CheckReport.Notes."
Post-check: "Run the boundary/local project tests and aggregate/direct command tests, then `./x render` and inspect every reported topic, domain, index, lock, and changelog change. `./x check` on this repository is clean because its self-hosted guide is below 10 KiB; a temporary oversized managed fixture reports one structured warning and exits zero."

Append the final Applied event naming exactly:

- add `rendering/sync-and-drift:agent-guide-size-advisory`

Author the claim with this ADR as Origin, test backing, and proof markers on the project boundary/local
case and aggregate command case. State that only deterministic expected managed guide bytes feed
aggregate `CheckReport.Notes`, the threshold is fixed at 12 KiB, warning-only remains zero exit, and
local guides plus non-aggregate consumers are excluded. Add an Unreleased changelog entry explaining
that native harness discovery replaces the generated roster, canonical docs now carry full
procedure, fixed default/self-hosted proofs prevent base regrowth, and aggregate check warns on an
oversized managed guide. Run `./x render`, read back every mutation target, and confirm all ADR
operations are Applied while the ADR stays `Implementing`; terminal review owns the later
`Implemented` event and plan status flip.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
feat(rendering): warn on oversized agent guides (applies ADR batch)
```

## Definition of done

- `dod: native-guide-routing` The neutral guide contains no enabled-skill catalog or fallback, every retained target still exposes native skill frontmatter, and governed skills retain their operative routing rules with full reusable tier definitions canonical in working-with-awf.
- `dod: terse-guide-bounds` The direct default guide is no more than 8 KiB, awf's self-hosted guide is no more than 10 KiB, the authoring standard forbids duplicated catalogs and procedure, and every removed fact is either duplicate or present in an existing canonical home.
- `dod: adopter-guide-advisory` Aggregate `awf check` alone warns and exits zero when deterministic expected managed AGENTS.md exceeds 12 KiB, with exact boundary and local-guide exclusions proved and no new configuration or metrics abstraction.
- `dod: authority-applied` All five declared current-state operations are Applied with matching claim mutations and proofs, the ADR remains Implementing pending terminal review, generated outputs are current, and the full gate passes.

## Notes

- Phase 1 review strengthened the native-routing proof to reject heterogeneous standard and local
  names, purposes, triggers, kind formatting, and relationships, and restored a cross-target scan
  proving workflow relationships remain advisory while selected skills retain operative controls.
- Phase 1 review's changelog finding is deferred to Task 3.2, which already owns the single
  adopter-facing Unreleased entry for the complete native-discovery, canonical-home, bounds, and
  advisory change; Phase 1 does not add a partial duplicate.
- Phase 2 measurements: direct default `AGENTS.md` is 3,287 bytes (limit 8,192; largest sections:
  Invariants 637, Workflow 521, Working with awf 442); self-hosted `AGENTS.md` changed from
  16,637 to 6,686 bytes (limit 10,240; final largest sections: Document map 2,127, Invariants
  1,751, Commands 632). Before census largest sections were Invariants 5,501, Working with
  awf 2,698, and Document map 2,274 bytes.
- Phase 2 prose disposition: guide copies of merge authorization, hook resolution, command flags,
  context spills, effort lifecycle, checkpoint and handoff protocol, ADR lifecycle, and invariant
  mechanisms were deleted rather than relocated; their canonical homes already remain in the
  workflow, working-with-awf, current-state, ADR, and native-skill sources. No guide-unique fact
  required relocation; deviations: none.
- Phase 2 review preserved the plain-punctuation invariant across all tracked text, added exact
  over-budget diagnostic and boundary proofs, and made both the direct-default and self-hosted guide
  tests enforce every minimum working-memory routing clause with mutation checks.
- Phase 3 red test result: `go test ./internal/project ./cmd/awf -run
  'Test(AgentGuideSizeAdvisoryBoundary|CheckReportAgentGuideSizeAdvisoryManagedOnly|AggregateCheckAgentGuideSizeWarning)$'`
  failed only at `TestAgentGuideSizeAdvisoryBoundary/over`: the expected overage note was absent;
  the boundary and local exclusions passed. After implementation, the same focused command passed.
  The deterministic expected guide is measured at 12 KiB plus one byte in the fixture; missing and
  stale resident files retain its one aggregate note. Deviations: none.
- Phase 3 review added an advisory-only aggregate presentation case, exact multi-note presentation
  ordering, and a `Project.CheckReport` assertion that ordinary advisories precede the generated
  guide-size advisory.
- Terminal review found that awf's full `config-and-overrides` convention part suppressed the shared
  model-tier include despite the approved canonical-home requirement. Repository rendering authority
  made the compliant correction deterministic: move the one shared include to a dedicated,
  non-overridden Working with awf section, update the self-hosted pointer, and prove the committed
  document contains the definition exactly once. This reasoned section-placement deviation preserves
  the approved ownership and adds no second definition. The same review added the required coherent
  empty-prefix guide fallback and regression proof. The single verify pass found two remaining
  direct prefix interpolations in default guide prose; the mechanical residual fix applied the same
  fallback to both and made the empty-render proof reject any empty inline-code token.

Record deviations, rendered-size measurements, prose-disposition findings, review findings, and any
implementation fact that invalidates an approved bound or canonical-home assumption. Do not relax a
budget to make a phase pass; return an infeasible bound or guide-unique requirement to the user as an
approval-requiring design fact.
