---
format: plan-v2
date: 2026-08-05
adrs:
  - derive-render-completeness-from-output-authority
  - layer-catalog-list-defaults-and-project-entries
  - separate-structural-markdown-headings-from-section-bodies
status: Proposed
---
# Plan: Streamline rendering authority and section composition

## Goal

Make render completeness derive from existing output authority, preserve catalog list defaults while
layering project entries, and separate awf-owned Markdown headings from replaceable section bodies.
The change does not introduce a universal render graph, semantic prose inference, generic list
identity, or configurable structural headings.

## Architecture summary

Implementation proceeds through four independently green transactions. The project package first
consolidates declaration facts and exhaustive projections, then strengthens ordinary fresh-render
drift and the human semantic boundary. Two schema generations subsequently introduce list layering
and structural headings with fixed-snapshot, preflighted migrations. Existing catalog, output-plan,
render-policy, configspec, migration, and topic owners remain in place; no parallel authority is
created. Every phase applies its current-state operations with its code and proof markers, leaves its
ADR `Implementing`, regenerates owned outputs, and closes with one commit. Terminal review owns the
later status-only `Implemented` and plan-freeze transaction.

## Phase 1: Derive completeness from existing output authority

**Execution mode: subagent-driven.**

Completes: ["render-authority-complete"]

### Task 1.1: Unify conditional output declaration and render facts
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:existing-output-authority", "derive-render-completeness-from-output-authority:live-template-completeness"]
Paths: ["internal/project/output_plan.go", "internal/project/render.go", "internal/project/singleton.go", "internal/project/output_declarations_test.go", "internal/project/output_plan_test.go", "internal/project/templateid_test.go", "internal/project/descriptor_parity_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

Extend the existing declaration structures rather than adding another registry. Give each
conditional config-tree unit currently spelled independently in `BuildOutputDeclarations` and
`renderAllBase` one bounded descriptor carrying only its shared enable predicate, output path,
template identity, render kind, and fixed section/input facts; keep unit-specific data construction,
encoding, output policy, and lifecycle branches explicit. Make declaration building and render
dispatch consume that descriptor population. Derive live template identities from the catalog, kind,
target, singleton, conditional-unit, resident, and topic declarations, and classify the retained
co-owned runner identity as recognition-only. Add negative parity tests in which a descriptor is
visible to only one projection, plus embedded-filesystem resolution for every derived live identity.
Do not add a second hand-maintained expected template list.

### Task 1.2: Make config-reference live-state classification exhaustive
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:exhaustive-live-state-classification"]
Paths: ["internal/configspec/spec.go", "internal/configspec/spec_test.go", "internal/project/configreference.go", "internal/project/configreference_print.go", "internal/project/configreference_test.go", "templates/docs/config-reference.md.tmpl"]

Replace the hand-maintained live-current switch with one exhaustive typed classification over every
config-reference key: either a live-state projection with its resolver, or an explicitly static
not-applicable row. Ensure reflection/configspec parity fails when a field is omitted or assigned to
the wrong class. Feed both CLI printing and generated reference data from that classification while
preserving existing absent/default/current presentation. Add a temporary test mutation that removes
one field classification and prove the exhaustive test fails before restoring it.

### Task 1.3: Prove singleton conditional context from declarations
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:live-singleton-conditionals"]
Paths: ["internal/project/singleton.go", "internal/project/render.go", "internal/project/catalog_sweep_test.go", "internal/project/spine_test.go", "internal/project/target_test.go", "internal/catalog/standard.go"]

Derive the singleton template population and its render-context path from the owning catalog and
singleton declarations. For every live conditional in that population, prove the referenced key is
supplied on the artifact's real render path and exercise both outcomes. Preserve `missingkey=zero`
and coherent empty-value fallback behavior. The check must discover a newly declared conditional
without adding its template identity to another closed list; historical recognition-only templates
remain excluded.

### Task 1.4: Apply the authority claim batch and regenerate shared outputs
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:existing-output-authority", "derive-render-completeness-from-output-authority:live-template-completeness", "derive-render-completeness-from-output-authority:exhaustive-live-state-classification", "derive-render-completeness-from-output-authority:live-singleton-conditionals"]
Paths: ["docs/decisions/derive-render-completeness-from-output-authority.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/config/configspec-and-reference/current-state.md", ".awf/topics/parts/rendering/templates/current-state.md", "docs/decisions/INDEX.md", "docs/config-reference.md", "docs/topics", "docs/domains", ".awf/awf.lock"]

Transition the ADR from `Proposed` to `Accepted`, then to `Implementing`, and append one Applied event
covering exactly these operations with their claim mutations and proof markers:

- update `rendering/project-output-plan:output-plan-complete`
- add `rendering/project-output-plan:conditional-unit-single-source`
- update `rendering/project-output-plan:template-id-single-derivation`
- add `config/configspec-and-reference:live-state-projection-explicit`
- add `rendering/templates:singleton-conditional-key-live`

Preserve prior Origin and Revised-by history on updates; new claims use this ADR as Origin. Run
`./x render`, read back every path it reports changed, and retain all generated topic, domain,
config-reference, decision-index, lock, and runtime outputs belonging to this transaction. Confirm
`./x check` reports clean and the declaration/render, live-template, exhaustive-classification, and
singleton-conditional test sets pass without frozen population counts.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
refactor(rendering): derive output completeness (applies ADR batch)
```

## Phase 2: Distinguish binary drift and reinforce semantic review

**Execution mode: subagent-driven.**

Completes: ["fresh-render-and-semantic-boundary"]

### Task 2.1: Compare ordinary outputs with a fresh render before attribution
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:current-render-freshness"]
Paths: ["internal/project/check.go", "internal/project/staged_drift.go", "internal/project/render.go", "internal/project/output_plan.go", "internal/project/drift_test.go", "internal/project/staged_drift_test.go", "internal/project/inplace_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

In the ordinary frozen-output branch, after template and config hashes match, render the current
planned bytes and compare their hash with the locked output hash before comparing the worktree or
index bytes. Report binary-derived stale output when the fresh hash differs from the lock; attribute a
hand edit only when the fresh hash still matches the lock and observed bytes do not. Reuse the same
classification in staged drift. Leave regenerated and in-place nodes on their declared regeneration
policy. Cover ordinary clean, binary-derived stale, hand-edited, missing, regenerated, in-place, and
staged equivalents, including a controlled renderer change with unchanged template/config hashes.
Temporarily falsify the fresh-render comparison and prove the binary-derived test fails, then restore
only that mutation.

### Task 2.2: Place semantic rendering checks at planning and review boundaries
Kind: batch
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:semantic-boundary"]
Paths: ["templates/skills/writing-plans/SKILL.md.tmpl", "templates/agents/plan-reviewer.md.tmpl", "templates/agents/code-reviewer.md.tmpl", "internal/evals/chain_test.go", "internal/project/target_test.go", "internal/project/golden_test.go"]
Representative: "The writing-plan contract requires a focused check for contradictory generated prose, concept-preserving paraphrase, and literal placeholder intent at each affected output boundary, while the plan reviewer checks that the plan schedules it."
Edge: "The code reviewer verifies produced outputs and tests without claiming synonym detection, contradiction inference, placeholder-intent inference, or a universal output-language validator; missingkey=zero and generic empty fallbacks remain unchanged."
Post-check: "Authority check over include-expanded live planning and reviewer templates: prove each enabled target receives the focused semantic-boundary instructions, then run exact golden/eval cases for empty data and literal placeholder examples; success is a clean targeted test run with no unresolved no-value token, not a source-line count."

Add concise, adopter-neutral instructions only at planning and plan/code review moments that can judge
meaning. Do not add a generic scanner or widen deterministic checks beyond exact known tokens. Keep
the agent digest and workflow dispatch contracts unchanged.

### Task 2.3: Apply the drift and semantic claim batch and regenerate outputs
Latitude: exact
Applying: ["derive-render-completeness-from-output-authority:current-render-freshness", "derive-render-completeness-from-output-authority:semantic-boundary"]
Paths: ["docs/decisions/derive-render-completeness-from-output-authority.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/INDEX.md", "AGENTS.md", ".pi", ".claude", "docs/topics", "docs/domains", ".awf/awf.lock"]

Append one Applied event covering exactly:

- add `rendering/sync-and-drift:ordinary-render-freshness`
- add `rendering/workflow-skill-templates:semantic-rendering-review`

Author both claims with test backing and exact proof markers. Run `./x render`, read back every reported
mutation, and retain generated Pi, Claude, guide, topic, domain, index, and lock outputs. Confirm the
ADR has no Remaining operations but stays `Implementing`; terminal review, not this phase, owns its
status-only `Implemented` transition.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
fix(rendering): detect binary-derived drift (applies ADR batch)
```

## Phase 3: Layer catalog list defaults and migrate replacements

**Execution mode: subagent-driven.**

Completes: ["catalog-list-layering"]

### Task 3.1: Model, validate, merge, and hash list layers
Latitude: exact
Applying: ["layer-catalog-list-defaults-and-project-entries:list-data-layers", "layer-catalog-list-defaults-and-project-entries:explicit-default-suppression", "layer-catalog-list-defaults-and-project-entries:null-list-refusal", "layer-catalog-list-defaults-and-project-entries:specialized-list-transforms"]
Paths: ["internal/config/config.go", "internal/config/config_test.go", "internal/project/datamerge.go", "internal/project/datamerge_test.go", "internal/project/validate.go", "internal/project/validate_test.go", "internal/project/confighash.go", "internal/project/confighash_test.go", "internal/project/glossary.go", "internal/project/glossary_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

Add `DataDefaults map[string]bool` to `config.Sidecar`. At project open, validate every entry against
the artifact's declared catalog data: both boolean values require a same-key list default; reject an
unknown, non-list, local-only, or specialized differently keyed value with sidecar and key. For a
catalog-backed list key, reject a present null or non-list project value, accept an empty list, and
compute `defaults + authored` unless the map value is explicitly false, in which case compute only
the authored list or an empty list. Preserve shallow replacement for non-list defaults and preserve
the glossary's `standardTerms`/`terms` identity-aware transform outside this generic path. Keep the
suppression map and effective data in the existing config-hash boundary. Cover every absence, empty,
true, false, invalid-key, invalid-type, ordering, alias-safety, and glossary-exclusion branch.

### Task 3.2: Add the fixed-snapshot list-replacement migration
Latitude: exact
Applying: ["layer-catalog-list-defaults-and-project-entries:fixed-snapshot-migration"]
Paths: ["internal/migrate/migrate.go", "internal/migrate/layercataloglists.go", "internal/migrate/layercataloglists_test.go", "internal/migrate/configedit.go", "internal/migrate/migrate_test.go"]

Register the next schema generation after the execution snapshot's current registry tip. In
`layercataloglists.go`, freeze an explicit kind/artifact/key population for catalog-backed list
replacements at this cutover; never derive that population from future `catalog.Standard` during
upgrade. Preflight every matching sidecar before writing: a non-null non-list replacement produces
the ADR's operation-category actionable refusal, reports every changed axis false, and names the
single edit followed by `awf upgrade`. For a valid present list, add `dataDefaults.<key>: false` and
retain the list; for null, add suppression and remove the null custom key. Preserve comments,
ordering, unrelated mappings, modes, and absent files. Write each changed sidecar through atomic
replacement, announce each mutation, and make rerun a no-op. Tests must prove refusal leaves the
entire fixture byte-identical, an injected later-file write failure is safely retryable, and a
post-cutover catalog key is not suppressed by the frozen snapshot.

### Task 3.3: Project list-layer state into the configuration reference
Latitude: exact
Applying: ["layer-catalog-list-defaults-and-project-entries:list-data-layers", "layer-catalog-list-defaults-and-project-entries:explicit-default-suppression", "layer-catalog-list-defaults-and-project-entries:specialized-list-transforms"]
Paths: ["internal/configspec/spec.go", "internal/configspec/spec_test.go", "internal/project/configreference.go", "internal/project/configreference_print.go", "internal/project/configreference_test.go", "templates/docs/config-reference.md.tmpl"]

Add the sidecar field to configspec key parity and replace whole-value override wording with four
observable states: catalog default, catalog default plus project entries, explicitly suppressed
default plus project entries, and project-only/specialized data. Keep CLI and generated-doc models on
the same typed rows. The reference must distinguish `dataDefaults: true` from absence only as
configuration presence, not different effective content, and state that null is invalid for a
catalog-backed list. Preserve specialized glossary documentation.

### Task 3.4: Upgrade this adopter, apply claims, and regenerate
Latitude: exact
Applying: ["layer-catalog-list-defaults-and-project-entries:list-data-layers", "layer-catalog-list-defaults-and-project-entries:explicit-default-suppression", "layer-catalog-list-defaults-and-project-entries:null-list-refusal", "layer-catalog-list-defaults-and-project-entries:fixed-snapshot-migration", "layer-catalog-list-defaults-and-project-entries:specialized-list-transforms"]
Paths: ["docs/decisions/layer-catalog-list-defaults-and-project-entries.md", ".awf/skills/tdd.yaml", ".awf/skills/proposing-adr.yaml", ".awf/agents/adr-reviewer.yaml", ".awf/agents/plan-reviewer.yaml", ".awf/agents/code-reviewer.yaml", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", "docs/config-reference.md", "docs/decisions/INDEX.md", "docs/topics", "docs/domains", ".awf/awf.lock"]

Transition the ADR through `Accepted` to `Implementing` and append one Applied event for exactly:

- update `rendering/project-output-plan:sidecar-key-overrides-default`
- add `rendering/project-output-plan:catalog-list-data-layering`
- add `config/configuration:sidecar-data-defaults-control`
- add `config/migrations-and-locks:list-replacement-fixed-snapshot`

Run `./awf upgrade` with the newly built binary, then read back every listed adopter sidecar and the
lock before trusting the compound mutation. Verify each former replacement carries only its matching
fixed-snapshot suppression and retains its project list. Apply claim prose and proof markers with
correct provenance. Run `./x render`; read back all reported changes and retain generated outputs.
Confirm a second upgrade is a no-op, `./x check` is clean, the migration registry is strictly ordered
with unique names, and the ADR has no Remaining operations while staying `Implementing`.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
feat(config): layer catalog lists (applies ADR batch)
```

## Phase 4: Separate structural Markdown headings from bodies

**Execution mode: subagent-driven.**

Completes: ["structural-section-headings"]

### Task 4.1: Parse and assemble policy-aware structural headings
Latitude: exact
Applying: ["separate-structural-markdown-headings-from-section-bodies:optional-structural-heading", "separate-structural-markdown-headings-from-section-bodies:heading-body-assembly", "separate-structural-markdown-headings-from-section-bodies:no-heading-configuration"]
Paths: ["internal/render/section.go", "internal/render/section_test.go", "internal/render/render.go", "internal/render/render_test.go", "internal/project/render.go", "internal/project/output_plan.go", "internal/project/section_default_render_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` produces no output,
`./x check` reports clean and exits zero, and `./x gate` exits zero.

Extend `Segment` with explicit optional structural-heading source separate from `Text`, and make
section parsing accept a heading mode selected from the output node's declared Markdown policy.
Recognize only a whole ATX heading line in the first structural position after the marker; a
non-Markdown hash-prefixed line remains body. Assemble pointer, heading, then default/part/in-place
body; `drop` emits none. Make `writePartBody` and `SectionDefaultSentinel` splice only `Text`, never
the heading. Keep the heading in the same `text/template` execution and publication-safety check as
the skeleton. Update in-place pointer wording so a headed section says the heading is awf-owned and
only the following body is preserved. Test headed, headingless, stub, part, repeated section-default,
non-Markdown, drop, empty fallback, and unresolved-value branches.

### Task 4.2: Exclude rendered structural headings from in-place read-back
Latitude: exact
Applying: ["separate-structural-markdown-headings-from-section-bodies:structural-heading-drift"]
Paths: ["internal/project/render.go", "internal/project/inplace_test.go", "internal/render/render.go", "internal/render/render_test.go"]

Before read-back, render each raw structural heading through the artifact's existing data and
missingkey-zero execution path so the expected output line is known without reading it from disk.
Pass that exact expectation to `planSections`/`readBackInPlaceBody`: consume the expected heading
slot after the in-place pointer, preserve only the following body interior verbatim, and regenerate
the expected heading. A modified structural heading must produce fresh expected heading plus the
preserved body so regeneration comparison reports awf-owned tamper; it must not become body or hide
the drift. Preserve registered-pointer bounding, missing-pointer fallback, empty-body semantics,
internal blank lines, and comment-style behavior. Add tests for exact heading, changed heading,
missing heading, body beginning with a subordinate heading, and first-render fallback. Temporarily
make read-back retain the structural heading and prove the tamper/duplication test fails, then restore
only that mutation.

### Task 4.3: Normalize every live Markdown section site
Kind: batch
Latitude: exact
Applying: ["separate-structural-markdown-headings-from-section-bodies:exact-heading-migration"]
Paths: ["glob:templates/**/*.tmpl", "internal/project/section_heading_census_test.go", "internal/project/docs_sections_test.go", "internal/project/skill_sections_test.go", "internal/project/spine_test.go"]
Representative: "For a marker followed immediately by an ATX heading, retain marker then heading in the structural slot and leave only prose/list content in the replaceable body model."
Edge: "Move an immediately preceding heading below its section marker; retain genuinely headingless fragments and all non-Markdown hash comments unchanged; refuse multiple, intervening, or otherwise ambiguous candidates in the census classifier."
Post-check: "Authority check over the include-expanded live template population, restricted to section-bearing outputs whose declared policy is Markdown and excluding recognition-only identities: every site must resolve to exactly one of headed-inside, headed-before-normalized, or headingless, with no unclassified or multiply associated terminal entries; the check must prove its input population was visited before accepting an empty finding set."

Perform the exhaustive mechanical template migration. Keep headings byte-identical apart from their
marker-relative position and do not edit section body prose. Update parity and golden tests so a
newly added live Markdown section automatically enters the classifier without maintaining another
template list.

### Task 4.4: Add the fixed-snapshot convention-part heading migration
Latitude: exact
Applying: ["separate-structural-markdown-headings-from-section-bodies:adopter-part-migration"]
Paths: ["internal/migrate/migrate.go", "internal/migrate/structuralheadings.go", "internal/migrate/structuralheadings_test.go", "internal/migrate/configedit.go", "internal/migrate/migrate_test.go"]

Register the next schema generation after Phase 3. Freeze the cutover's exact artifact/section/heading
mapping in `structuralheadings.go`; later template headings must not widen it. Preflight every matching
convention part. Remove an exact heading in the leading structural position while preserving all
remaining bytes, comments, modes, and framing; leave no-heading parts unchanged. A different,
multiple, or ambiguous leading heading returns the ADR's operation-category outcome before mutation,
with all axes false and the ordered edit-then-`awf upgrade` remedy. Replace changed files atomically,
announce them, and make rerun a no-op. Test exact, no-heading, custom, multiple-candidate, authoring-
comment, injected write-failure/retry, and future-heading cases; preflight refusal must leave the
whole fixture byte-identical.

### Task 4.5: Upgrade this adopter, apply claims, and regenerate
Latitude: exact
Applying: ["separate-structural-markdown-headings-from-section-bodies:optional-structural-heading", "separate-structural-markdown-headings-from-section-bodies:heading-body-assembly", "separate-structural-markdown-headings-from-section-bodies:structural-heading-drift", "separate-structural-markdown-headings-from-section-bodies:exact-heading-migration", "separate-structural-markdown-headings-from-section-bodies:adopter-part-migration", "separate-structural-markdown-headings-from-section-bodies:no-heading-configuration"]
Paths: ["docs/decisions/separate-structural-markdown-headings-from-section-bodies.md", ".awf/parts", ".awf/skills/parts", ".awf/agents/parts", ".awf/docs/parts", ".awf/domains/parts", ".awf/topics/parts/rendering/render-engine/current-state.md", ".awf/topics/parts/rendering/inplace-and-placeholders/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", "docs/decisions/INDEX.md", "AGENTS.md", ".pi", ".claude", "docs/topics", "docs/domains", ".awf/awf.lock"]

Transition the ADR through `Accepted` to `Implementing` and append one Applied event for exactly:

- update `rendering/render-engine:section-edit-pointer`
- add `rendering/render-engine:structural-heading-owned`
- update `rendering/render-engine:section-default-splice`
- update `rendering/inplace-and-placeholders:in-place-readback`
- update `rendering/inplace-and-placeholders:in-place-spacing-owned`
- update `rendering/inplace-and-placeholders:in-place-tamper-drift`
- add `config/migrations-and-locks:structural-heading-part-migration`

Run `./awf upgrade`, then read back every part reported changed and the lock before trusting the
compound mutation. Confirm exact copied headings were removed, other body bytes stayed stable, and a
second upgrade is a no-op. Apply claim/proof changes with correct provenance, then run `./x render`
and read back every reported generated mutation. Verify the post-render working diff contains no
unexplained prose change, `./x check` is clean, every live section is classified, and the ADR has no
Remaining operations while staying `Implementing`.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
feat(rendering): separate section headings (applies ADR batch)
```

## Definition of done

- `dod: render-authority-complete` Declaration and render projections share bounded existing authority; every live template, config-reference field, and singleton conditional is exhaustively classified and test-backed.
- `dod: fresh-render-and-semantic-boundary` Working and staged drift distinguish binary-derived ordinary output from hand edits, while planning and review carry focused meaning-dependent checks without semantic inference machinery.
- `dod: catalog-list-layering` Catalog-backed lists compose before project entries, explicit suppression and null/type validation are hash-visible, specialized transforms remain intact, and the fixed-snapshot migration is clean and retryable.
- `dod: structural-section-headings` Markdown structural headings are awf-owned, body sources remain independently replaceable or preserved, drop removes the complete section, and exhaustive template/part migration settles without ambiguity.

## Notes

- All three ADRs remain `Implementing` after their last explicit Applied batch. Terminal implementation review owns the later status-only `Implemented` events, plan `status: Implemented`, decision-index regeneration, managed-worktree integration/removal, and retrospective ordering.
- Any migration preflight refusal is a blocker, not permission to weaken the snapshot or auto-resolve custom content. Record the affected path and operator resolution here before resuming the same phase.
