---
format: plan-v2
date: 2026-08-13
adrs: []
status: Proposed
---
# Plan: Terse current documentation sweep

## Goal

Audit every tracked Markdown surface and make current internal and adopter-facing guidance terse, pointed, and easy to route from. Preserve frozen history and the semantics of active authority.

## Architecture summary

Build a complete path census from the pre-edit commit, classify every Markdown file by audience and lifecycle, and use that census as the scope ledger. Edit hand-authored public prose directly and `.awf/` or template sources for generated docs. Historical ADRs, Implemented plans, changelog entries, and dated research receive a read-only disposition. Current-state claims receive a semantic review but no wording change unless separately authorized through the ADR lifecycle. Each generated edit closes with its source, output, and lock in one render transaction.

## Phase 1: Census and public entry points

**Execution mode: inline.**

Advances: ["complete-doc-census", "terse-current-docs", "generated-tree-clean"]

### Task 1.1: Build the Markdown scope ledger
Kind: batch
Latitude: exact
Paths: ["glob:**/*.md"]
Representative: Classify `README.md` as a current adopter-facing entry point and an Implemented ADR as `historical-frozen`.
Edge: Classify each generated output and its source separately, linking the output disposition to its source path.
Post-check: From the pre-edit phase commit, run `git ls-files -z -- '*.md'` and classify every returned path exactly once in effort scratch as `edit`, `already-terse`, `recently-cleaned`, `historical-frozen`, `dated-evidence`, `generated-counterpart`, or `separately-governed`; reject duplicate, missing, and untracked classifications by comparing sorted NUL-delimited path sets.

Classify root and community docs, catalog guides, generated agent guides, native skills and agents for both supported runtimes, current-state domains and topics, pitfalls, roadmap, historical decisions and plans, research, and changelog. Record a concise reason and owning source for every `edit` or `separately-governed` disposition. Do not commit the scratch ledger.

### Task 1.2: Tighten the public README
Paths: ["README.md"]

Remove repeated render/check and workflow narration while retaining identity, installation, quickstart, compatibility, generated command inventory, primary documentation links, status, and contribution entry points. Keep the first-use path readable without requiring a second document.

### Task 1.3: Shorten project glossary entries
Kind: batch
Latitude: exact
Paths: [".awf/docs/glossary.yaml", ".awf/docs/parts/glossary/prepend.md", "docs/glossary.md", ".awf/awf.lock"]
Representative: Reduce a definition that narrates implementation history to the term's present meaning.
Edge: Preserve a second sentence only for a load-bearing contrast, retired label, exact syntax, or exact path.
Post-check: Before editing, capture the rendered glossary's term-column set from the pre-edit commit. Run `./awf render`, then `./awf check repo`; require no glossary terseness advisory in output. Parse the post-edit rendered term column and require exact set equality with the baseline and no duplicates.

Replace implementation history and mechanism inventories with one sentence defining the term and a second only for a load-bearing contrast. Retain retired labels and exact paths or syntax only when they distinguish the term.

### Phase close

```commit
docs(awf): tighten public entry points and terminology
```

## Phase 2: Current guides and generated workflow prose

**Execution mode: inline.**

Advances: ["complete-doc-census", "terse-current-docs", "generated-tree-clean"]

### Task 2.1: Settle catalog and contributor guides
Kind: batch
Latitude: exact
Paths: ["AGENTS.md", "docs/agents-md-standard.md", "docs/architecture.md", "docs/config-reference.md", "docs/development.md", "docs/doc-standard.md", "docs/maintainable-code-design.md", "docs/releasing.md", "docs/testing.md", "docs/workflow.md", "docs/working-with-awf.md", "templates/docs", ".awf/docs/parts", ".awf/parts", ".awf/agents-doc.yaml", ".awf/awf.lock"]
Representative: Replace repeated procedure with one action and a link to its owning guide.
Edge: Retain complete reference tables and exact protocols in the document that owns them.
Post-check: Run `./awf render` and `./awf check repo`; inspect every changed guide from its H1 through its final section and require that repeated facts have one owner and every retained implementation detail is needed by that guide's audience.

Use the scope ledger to revisit all current catalog and contributor guides, including the two recently cleaned families. Edit only remaining concrete violations: repeated ownership, mechanism narration where a link suffices, buried routine actions, or audience leakage. Preserve complete reference tables and exact protocols whose owning document must remain authoritative.

### Task 2.2: Settle native skills and agents
Kind: batch
Latitude: exact
Paths: ["templates/skills", "templates/agents", "templates/partials", ".awf/skills/parts", ".pi/skills", ".pi/agents", ".claude/skills", ".claude/agents", "changelog/CHANGELOG.md", ".awf/awf.lock"]
Representative: Replace duplicated cross-skill protocol with the existing owning partial or canonical pointer.
Edge: Preserve runtime-specific native tool instructions where the target capability requires them.
Post-check: Run `./awf render` and `./awf check repo`; compare every changed Pi and Claude output against its source and require semantic parity across runtimes, no duplicated shared protocol outside its owning partial, and no loss of a trigger, stop condition, authority boundary, or verification obligation.

Compress only workflow prose classified `edit` by the ledger. Keep skills action-first and self-contained, agents report- or implementation-role focused, and shared protocol in its existing partial. Do not broaden or redesign workflow behavior during editorial compression. Add one concise `[Unreleased]` changelog entry for adopter-facing template changes; preserve released entries.

### Phase close

```commit
docs(rendering): tighten current workflow guidance
```

## Phase 3: Maintainer guidance and authority disposition

**Execution mode: inline.**

Completes: ["complete-doc-census", "terse-current-docs", "generated-tree-clean"]

### Task 3.1: Compress the roadmap without changing its decisions
Kind: batch
Latitude: exact
Paths: [".awf/docs/parts/roadmap/ideas.md", ".awf/docs/parts/roadmap/deferred.md", "docs/roadmap.md", ".awf/awf.lock"]
Representative: Reduce an item to its open problem, retaining evidence, governing constraint, and next decision.
Edge: Preserve measured evidence and sequencing constraints verbatim unless reverified or explicitly marked historical.
Post-check: Capture the pre-edit source headings and top-level list-item starts from the Phase 2 commit, then run `./awf render` and compare the rendered roadmap against that baseline; require every retained item to remain identifiable under the same section and explain every removed or moved item in plan Notes.

State each open problem, evidence that makes it worth retaining, governing constraint, and next decision in the shortest complete form. Preserve measured evidence, sequencing constraints, live defect descriptions, and explicit decision requirements. Do not silently graduate or drop an item, rewrite frozen records, or turn tentative work into current authority.

### Task 3.2: Replace the debugging stub with repository-specific routes
Paths: [".awf/docs/parts/debugging/surfaces.md", ".awf/docs/parts/debugging/recipes.md", "docs/debugging.md", ".awf/awf.lock"]

Name the smallest useful inspection set: `git status --short` and `git diff` for transaction state, `./awf check repo drift` for generated drift, `./awf check repo state` plus `./awf context <affected-path>` and `./awf topic <domain>/<topic>` for authority failures, and `./x test` or `./x gate` for code failures. Tell the reader to select the path from the failing output and the qualified topic from the context report. Cover four symptom families: rendered drift, current-state refusal, binary-version refusal, and gate failure. Link `working-with-awf.md`, `testing.md`, and `development.md` for owned procedure. Run `./awf render` and `./awf check repo`; require no unauthored-content advisory for debugging and require all named commands and Markdown links to validate.

### Task 3.3: Close the census and freeze the plan
Latitude: exact
Paths: ["docs/decisions", "docs/plans", "docs/research", "docs/topics", "docs/domains", "docs/pitfalls.md", "docs/pitfalls", ".awf/topics/parts", ".awf/docs/pitfalls", "changelog/CHANGELOG.md", ".awf/awf.lock", "docs/plans/2026-08-13-terse-current-documentation-sweep.md"]

Resolve every remaining ledger entry. Confirm historical and dated files have no cosmetic diff. Review current-state topics and domains for terse lookup shape, but record any semantic compression candidate in Notes as separately governed rather than editing claims. Verify pitfalls remain focused on one hazard and recovery boundary; edit an authored pitfall source only when meaning is unchanged, then render it. Record all material implementation findings and dispositions in Notes. Keep the plan Proposed through phase review and implementation assurance; `effort-workflow` owns the deferred `Implemented` freeze. Run `./awf render`, inspect `git diff --check` and the complete semantic diff, then run `./awf check repo`, stage the complete transaction explicitly, run `./awf check staged`, and run `./x gate`; every command must finish clean.

### Phase close

```commit
docs(awf): sharpen maintainer guidance
```

## Definition of done

- `dod: complete-doc-census` Every tracked Markdown file has one lifecycle and audience disposition, and every current surface is either changed, verified terse, recently cleaned and rechecked, or explicitly routed to separate authority.
- `dod: terse-current-docs` Current internal and adopter-facing docs state actions, ownership, and boundaries without repeated rationale or unnecessary implementation narration.
- `dod: generated-tree-clean` Every generated documentation change originates in its source, renders with its lock transaction, and passes repository, staged, and gate checks without touching frozen historical content.

## Notes

Record deviations, findings, and follow-ups before the final status freeze.

- Plan review: widened the plan from four selected surfaces to a complete tracked-Markdown census after review found that the initial scope did not satisfy the repository-wide request. The user approved the census and disposition model.
- Verify review: corrected the glossary baseline comparison, made debugging query examples executable, and added changelog ownership for adopter-facing template edits.
- Phase 1: classified every baseline tracked Markdown path exactly once. Tightened README and all project glossary definitions; phase review found four lost semantic boundaries, which a settlement restored before the verify pass returned clean.
- Phase 2: rechecked current catalog and contributor guides, compressed repeated architecture, workflow, gate, and overview prose, and tightened three native skill families with byte-identical Pi and Claude outputs. Existing tests required command names, trigger text, and fallback phrases that initially appeared redundant, so these contract-bearing literals remained. Phase review returned clean.
- Phase 3 roadmap: retained all 19 top-level Ideas entries and all 17 source headings under their original sections. Compressed narration while preserving recurrence evidence, measured values, authorities, recovery commands, and the pending-numbering integration sequence. Grounding found and prompted restoration of the post-Accepted Amended-event rule, the current-workflow scope on parallel workers, and one grammar correction.
- Phase 3 debugging: replaced both stubs with executable routes for transaction state, generated drift, current-state refusals, binary-version refusals, and gate failures. Repository check now reports no unauthored content.
- Phase 3 authority disposition: frozen ADRs, prior plans, research, current-state sources and outputs, and pitfalls have no implementation diff. All 38 pitfall sources and publications remain focused and terse. Current-state review routed future lifecycle work for the broad workflow-skill, Pi-workflow, ADR-lifecycle, CLI, and migration topics, plus broad rendering/tooling/ADR domain summaries; no active claim changed.
- Reasoned deviation: the original Task 3.3 requested freezing this plan before phase review, but current workflow authority keeps plans Proposed through implementation assurance. The deferred terminal transaction remains owned by `effort-workflow`.
- Terminal assurance restored the glossary's operation-specific freeze boundary: each State changes operation freezes when an Applied event references it, while unapplied operations remain amendable.
