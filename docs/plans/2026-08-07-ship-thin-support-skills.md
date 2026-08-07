---
format: plan-v2
date: 2026-08-07
adrs: ["0246"]
status: Implemented
---
# Plan: Ship thin support skills

## Goal

Ship the `using-awf` and `writing-docs` support skills as thin, section-overridable procedural entry points without duplicating the configuration reference, command reference, documentation standard, generated-tree transaction, or guide routing.

## Architecture summary

One rendering transaction starts from accepted ADR-0246 and the unconditionally rendered standard catalog at `internal/catalog/standard.go`. It adds both support-skill catalog profiles and their sectioned templates, proves each owned clause and delegated boundary in catalog-required golden tests, applies both ADR-0246 claims, and renders the templates through the existing target seam to Claude Code and Pi outputs. `writing-docs` delegates file mutation to `using-awf`; both delegate detailed rules to their existing authoritative documents. No target-specific implementation or guide change is introduced.

## Phase 1: Add and prove both support skills

**Execution mode: subagent-driven.**
Completes: ["support-skills-rendered", "delegation-boundaries-proven", "adr-state-applied"]

### Task 1.1: Add clause-level golden contracts
Applying: ["0246:using-awf-skill", "0246:writing-docs-skill", "0246:description-only-selection", "0246:overridable-sections"]
Paths: ["internal/project/spine_test.go"]

Add the catalog-required `TestUsingAwfTemplate` and `TestWritingDocsTemplate` golden tests before the new catalog entries or templates. Prove the exposed description and every owned transaction clause separately. For `using-awf`, assert the generated-source and no-hand-edit boundary, render/check/stage-with-lock/gate transaction, drift-hint handling, upgrade-and-residue route, and pointers to the working-with-awf and configuration-reference authorities. For `writing-docs`, assert single-doc ownership selection, reading the documentation standard, reference-over-restatement, docs-travel, the file-edit handoff to `using-awf`, and the documentation-standard pointer. Add explicit absence assertions showing that `using-awf` contains no configuration-key inventory or general command reference and that `writing-docs` restates neither documentation-standard rules nor the generated-tree transaction. Put one proof marker for each new ADR claim on the test that proves all of that claim's clauses. Do not weaken the catalog-wide template, empty-data, dead-reference, or section-parity sweeps.

### Task 1.2: Add the sectioned catalog skills and templates
Applying: ["0246:mechanism-support-skills", "0246:using-awf-skill", "0246:writing-docs-skill", "0246:description-only-selection", "0246:overridable-sections"]
Paths: ["internal/catalog/standard.go", "templates/skills/using-awf/SKILL.md.tmpl", "templates/skills/writing-docs/SKILL.md.tmpl"]

Add both entries as `WorkflowSupport` profiles with descriptions that independently select generated-tree maintenance and documentation authoring. Declare a nonempty `Sections` list for each entry and give each declared section exactly one matching `awf:section` block. Keep the templates thin and action-oriented: state only the transaction shape committed by ADR-0246, link detailed command and configuration guidance to `docs/working-with-awf.md` and `docs/config-reference.md`, link documentation rules to `docs/doc-standard.md`, and make `writing-docs` invoke `using-awf` when authoring reaches a file edit. Preserve coherent rendering with unset interpolation and use no runtime-specific tool names, target branches, guide roster changes, new configuration, or sidecars.

### Task 1.3: Apply the claims and render the complete transaction
Applying: ["0246:using-awf-skill", "0246:writing-docs-skill"]
Paths: ["docs/decisions/0246-ship-thin-support-skills-for-awf-s-own-mechanisms.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/INDEX.md", "docs/topics/rendering/workflow-skill-templates.md", ".claude/skills/awf-using-awf/SKILL.md", ".claude/skills/awf-writing-docs/SKILL.md", ".pi/skills/awf-using-awf/SKILL.md", ".pi/skills/awf-writing-docs/SKILL.md", ".awf/awf.lock"]

Append the `Implementing` status event and one explicit Applied batch containing both distinct ADR-0246 add operations. In the same transaction, add `using-awf-transaction-home` and `writing-docs-delegation` as test-backed claims with `Origin: ADR-0246`, matching the implemented ownership and delegation boundaries. Run `./x render` so the topic, decision index, both target outputs, and lock reflect the authored sources. Read all four rendered skills and confirm their descriptions select independently, their pointers resolve, `writing-docs` delegates rather than duplicates, and no contradictory or target-specific fragments appear. Then run `./x check`; the terminal result must report no drift.

### Phase close

Land the two coupled skills, tests, state application, rendered outputs, and lock as one independently green transaction.

```commit
feat(rendering): ship thin support skills (applies 0246 batch)
```

## Definition of done

- `dod: support-skills-rendered` The standard catalog renders section-overridable `using-awf` and `writing-docs` skills for both Claude Code and Pi with independently selectable descriptions and valid authority pointers.
- `dod: delegation-boundaries-proven` Named golden tests prove every owned clause and explicitly reject the delegated configuration, command-reference, documentation-standard, and generated-tree content.
- `dod: adr-state-applied` ADR-0246 is Implementing with both additions in one Applied batch, both current-state claims carry matching test proof, and render/check/gate finish cleanly.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- Phase 1 deviation: added `internal/project/docs_sections_test.go` because the strict catalog profile-count oracle correctly failed after the approved two entries; updating its expected count preserves rather than weakens that oracle.
- Phase 1 review: strengthened both claim proofs from clause token checks and short denylists to exact approved rendered-body contracts, so any added canonical-content duplication fails. Added the adopter-facing skills to the Unreleased changelog.
