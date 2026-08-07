---
format: plan-v2
date: 2026-08-07
adrs: [compact-source-provenance-for-generated-documents]
status: Proposed
---
# Plan: Compact Source Provenance for Generated Documents

## Goal

Give every otherwise-opaque generated documentation family a compact, actionable `awf:source` pointer and publish one accurate source-editing guide, without changing `awf:edit`, machine dependency semantics, or authored-artifact ownership.

## Architecture summary

Add one formatter beside banner injection, while each document producer supplies its own reader-facing source list. Land the marker contract with topic consumers first, extend it across the remaining opaque producer families second, and finish with canonical documentation plus a semantic review of representative generated outputs. Current-state claim mutations travel with the production behavior they describe; the linked ADR remains Implementing until post-implementation assurance settles.

## Phase 1: Establish the compact marker contract and topic provenance

**Execution mode: subagent-driven.**

Advances: ["opaque-output-guidance", "repository-green"]
Completes: ["compact-marker-contract"]

### Task 1.1: Add provenance formatting and isolation regressions
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:source-marker-contract", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["internal/project/banner.go", "internal/project/banner_test.go"]

Start only from the committed and reviewed plan in the managed worktree: `git status --short` prints nothing, `git log -1 --format=%s` names the plan or its review settlement, `./x check` completes with zero findings, and `./x gate` passes at 100% statement coverage.

Add a single formatter at the banner boundary for the literal HTML form `<!-- awf:source <source> [<source>...] -->`. It emits only for a nonempty renderer-owned source slice, joins sources with one ASCII space, and inserts exactly one comment immediately after the already-injected generated-by banner; an empty slice returns the bannered content byte-identically. Preserve frontmatter and shebang placement behavior. Source payloads are already-normalized reader-facing paths, globs, or `derived:<authority>` tokens; do not infer them from `ConsumedInputs`, parse them as edit boundaries, or add them to the lock schema.

Cover plain Markdown placement, frontmatter placement, a headingless body, multiple compact sources, and the empty-source identity case in `internal/project/banner_test.go`. Put the proof marker for `rendering/render-engine:source-marker-informational` on the named test that proves one informational marker follows the banner without affecting body content. Do not broaden ADR or plan scaffold stripping: the approved scope keeps `awf:source` out of their rendered support templates, and Phase 2's negative matrix test pins that boundary; if that scope changes later, its `adr-new-strips-markers` authority must change deliberately with it.

### Task 1.2: Emit exact source pairs on topic pages and glob families on topic indexes
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:source-marker-contract", "compact-source-provenance-for-generated-documents:reader-facing-source-policy", "compact-source-provenance-for-generated-documents:opaque-output-scope"]
Paths: ["internal/project/topics.go", "internal/project/topics_test.go"]

In `generateTopicDocs`, annotate each individual topic after banner injection with its exact normalized pair:

- `.awf/topics/metadata/<domain>/<topic>.yaml`
- `.awf/topics/parts/<domain>/<topic>/current-state.md`

Annotate each domain topic index with these compact globs, using the actual domain identity:

- `.awf/topics/metadata/<domain>/*.yaml`
- `.awf/topics/parts/<domain>/*/current-state.md`

Keep the existing exact `DependsOn` and `ConsumedInputs` sets unchanged; the marker is presentation, not dependency authority. Extend `TestTopicRenderLifecycle` or add focused neighboring tests that assert banner-then-source ordering, exact page paths, domain-specific index globs, one marker per output, no source comment inside the rendered topic body, and unchanged drift behavior after metadata and part edits. Use empty optional metadata/data fixtures to prove no `<no value>` or unresolved source token appears.

### Task 1.3: Apply the render-engine source-marker claims with the first consumer
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:source-marker-contract", "compact-source-provenance-for-generated-documents:reader-facing-source-policy"]
Paths: ["docs/decisions/compact-source-provenance-for-generated-documents.md", ".awf/topics/parts/rendering/render-engine/current-state.md", "internal/render/render_test.go", "internal/project/banner.go", "internal/project/banner_test.go", "docs/topics/rendering/render-engine.md", "glob:docs/topics/**", ".awf/awf.lock"]
Post-check: Run `./x render`, then verify `./x check` reaches zero findings. Read back the ADR, source topic part, rendered render-engine topic, representative topic page, topic index, and `.awf/awf.lock`; confirm only lifecycle-authorized generated changes remain. Run `rg -n 'awf:source|no-section-marker-leak|source-marker-informational' internal .awf/topics/parts/rendering/render-engine/current-state.md docs/topics/rendering` and require every hit to be an intended implementation, proof/touches marker, active claim, or generated output.

Use `awf-adr-lifecycle` to move the linked Proposed ADR to Implementing and append one Applied event containing exactly:

- update `rendering/render-engine:no-section-marker-leak`
- add `rendering/render-engine:source-marker-informational`

In the same transaction, update `no-section-marker-leak` so `awf:section` and `awf:end` remain forbidden while the banner, `awf:edit` family, and informational `awf:source` are the only allowed rendered awf markers. Add the test-backed `source-marker-informational` invariant with Origin naming the pending ADR: one renderer-owned HTML comment follows the banner, carries a nonempty compact source list, is never parsed as an edit/read-back boundary, and is absent when no family policy supplies sources. Keep the existing no-leak proof and add the new proof/touches markers at the units established in Tasks 1.1 and 1.2. Run `./x render` so all topic outputs and lock entries travel with their `.awf/` sources.

### Phase close

Stage the complete Phase 1 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass, with the gate reporting 100% statement coverage. Create the one closing commit:

```commit
feat(rendering): add compact topic source provenance
```

## Phase 2: Cover every remaining opaque document producer

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["opaque-output-guidance"]

### Task 2.1: Thread family-owned source policy through declarative and bridge rendering
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:reader-facing-source-policy", "compact-source-provenance-for-generated-documents:opaque-output-scope", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["internal/project/render.go", "internal/project/output_plan_test.go", "internal/project/glossary_test.go", "internal/project/pitfalls_test.go"]

Start only after Phase 1 is committed in the managed worktree: `git status --short` prints nothing, `git log -1 --format=%s` names the Phase 1 closing commit, `./x check` completes with zero findings, and `./x gate` passes at 100% statement coverage.

Add an optional producer-supplied source list to the existing render options/spec seam without adding it to `RenderedFile`, `OutputRecipe`, `ConsumedInputs`, or manifest structures. Ensure options are allocated safely for neutral docs that previously needed none. Apply these exact policies:

- glossary: `.awf/docs/glossary.yaml derived:awf-standard-vocabulary`
- pitfalls: `.awf/docs/pitfalls.yaml`
- every descriptor-owned target bridge: `AGENTS.md`

Do not annotate other standard docs, `AGENTS.md` itself, skills, agents, target outputs, hooks, resident files, ADR/plan support templates, synthesized local docs with adequate `awf:edit`, or any `local: true` artifact. Keep bridge machine inputs unchanged unless an independently justified dependency contract requires otherwise; `AGENTS.md` is the reader routing policy, not a claim that bridge bytes are hashed from the guide.

Add assertions to glossary and pitfalls rendering tests for exact marker payload and banner ordering, including empty/null computed data branches. Extend `TestBridgeRenderIdentity` to cover the headingless bridge marker, empty template variables, and no fictitious sidecar/dependency inputs. Add negative assertions on representative ordinary docs and non-document targets so scope cannot silently expand.

### Task 2.2: Annotate regenerated indexes, mixed domain pages, and config reference
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:reader-facing-source-policy", "compact-source-provenance-for-generated-documents:opaque-output-scope", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["internal/project/render.go", "internal/project/configreference.go", "internal/project/banner_test.go", "internal/project/topics_test.go", "internal/project/configreference_test.go", "internal/project/source_marker_test.go"]

Supply these exact remaining policies at their family producers:

- ADR `INDEX.md`: `derived:authored-adr-corpus`
- domain page `<domain>` topic navigation: `.awf/topics/metadata/<domain>/*.yaml .awf/topics/parts/<domain>/*/current-state.md`
- config reference: `derived:configspec derived:project-configuration`

Retain each domain page's existing `awf:edit current-state` pointer without duplicating its convention-part path in `awf:source`. Keep config-reference regeneration dependencies, ADR record inputs, domain topic inputs, and output-plan relationships unchanged.

Create `internal/project/source_marker_test.go` as the comprehensive family-boundary regression. Build one fixture that renders enabled glossary and pitfalls docs, one topic and its domain, the ADR index, config reference, `AGENTS.md`, and a bridge. Assert the exact positive marker matrix and the negative set for outputs already explained by `awf:edit` or intentionally authored. Assert every marked output has one source comment immediately after its banner and contains no `<no value>`. Keep focused existing tests for the producer-specific edge cases.

### Task 2.3: Apply the opaque-document coverage claim and regenerate affected outputs
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:opaque-output-scope"]
Paths: ["docs/decisions/compact-source-provenance-for-generated-documents.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", "internal/project/source_marker_test.go", "docs/topics/rendering/doc-outputs.md", "glob:docs/domains/**", "docs/glossary.md", "docs/pitfalls.md", "docs/decisions/INDEX.md", "docs/config-reference.md", "CLAUDE.md", ".awf/awf.lock"]
Post-check: Run `./x render`, then verify `./x check` reaches zero findings. Read back every listed representative output and `.awf/awf.lock`. Run `rg -n '^<!-- awf:source ' docs/topics docs/domains docs/glossary.md docs/pitfalls.md docs/decisions/INDEX.md docs/config-reference.md CLAUDE.md` and require the terminal hit set to match the family policy, with no marker in an excluded or authored output. Probe exclusions separately with `rg -n '^<!-- awf:source ' AGENTS.md docs/decisions/README.md docs/decisions/template.md docs/plans/README.md docs/plans/template.md`; success is the command running and returning no matches.

Add test-backed invariant `rendering/doc-outputs:opaque-doc-source-guidance` with Origin naming the pending ADR. Its prose must enumerate the marked families and the non-duplication/banners-free boundary from ADR Decision `opaque-output-scope`, while making clear that marker payloads are reader guidance rather than exhaustive machine dependencies. Put its proof marker on the comprehensive matrix test. Append one Applied event to the still-Implementing ADR for exactly `add rendering/doc-outputs:opaque-doc-source-guidance` in the same transaction. Run `./x render` and retain all generated outputs with their source changes.

Perform a focused semantic rendering review of `docs/topics/rendering/doc-outputs.md`, `docs/topics/rendering/index.md`, `docs/domains/rendering.md`, `docs/glossary.md`, `docs/pitfalls.md`, `docs/decisions/INDEX.md`, `docs/config-reference.md`, and `CLAUDE.md`: the marker must point to the intended authority, the surrounding banner/`awf:edit` prose must not contradict it, globs must preserve the domain identity, and `derived:` tokens must read as authorities rather than file paths.

### Phase close

Stage the complete Phase 2 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass, with the gate reporting 100% statement coverage. Create the one closing commit:

```commit
feat(rendering): mark opaque document sources
```

## Phase 3: Publish canonical source-editing instructions

**Execution mode: inline.**

Completes: ["canonical-source-instructions", "repository-green"]

### Task 3.1: Make working-with-awf the complete operational source map
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:instruction-authority"]
Paths: ["templates/docs/working-with-awf.md.tmpl", ".awf/parts/working-with-awf/config-and-overrides.md", "templates/docs/config-reference.md.tmpl", "docs/working-with-awf.md", "docs/config-reference.md"]

Add one compact generated-source subsection and document-family matrix to the default working-with-awf template and re-derive this repository's full replacement section so the rendered guide retains the same contract. Define the distinction among the universal banner, section-bound `awf:edit`, informational non-exhaustive `awf:source`, the output plan's machine source/dependency authority, and the lock's drift authority. For each family, name the output shape and the exact editable path, glob, or derived authority: ordinary docs and `AGENTS.md`, domains, topic pages and indexes, glossary, pitfalls, ADR index, config reference, target bridges, generated local docs, and banner-free authored ADRs/plans/`local: true` docs. State the edit-render-check transaction once.

Correct this repository's stale plan authoring statement from plan-v1 to plan-v2 while editing the replacement section. In the config-reference template introduction, identify configspec/catalog descriptions, `.awf/config.yaml` and relevant sidecars as the live authority layers, and state that only its declared intro section is convention-part editable; keep field semantics in the reference rather than duplicating template mechanics.

### Task 3.2: Align onboarding and glossary vocabulary
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:instruction-authority"]
Paths: ["README.md", ".awf/docs/glossary.yaml", "docs/glossary.md"]

Keep README guidance concise: distinguish rendered outputs, section overrides, source markers, regenerated outputs, and banner-free authored/local artifacts; preserve the exact topic metadata/part pair already present and point detailed operation to working-with-awf. Correct the `current-state topic` glossary meaning so both metadata and the authored part are authoritative. Add terse entries for `source marker` and `regeneration-derived document` only if they improve lookup without duplicating the working guide; meanings must remain within the enforced glossary terseness threshold and use plain ASCII punctuation.

### Task 3.3: Regenerate and semantically audit the published guidance
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:instruction-authority", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["AGENTS.md", "CLAUDE.md", "docs/working-with-awf.md", "docs/config-reference.md", "docs/glossary.md", "glob:docs/topics/**", "glob:docs/domains/**", "docs/decisions/INDEX.md", "docs/pitfalls.md", ".awf/awf.lock"]
Post-check: Run `./x render` and require `./x check` to reach zero findings. Read back every generated mutation target reported by render. Run `git diff --check`; run `rg -n '<no value>|awf:source[^>]*(<domain>|<topic>|TBD)' README.md AGENTS.md CLAUDE.md docs` and require the probe to run with no matches. Run `rg -n '^<!-- awf:source ' AGENTS.md docs/decisions/README.md docs/decisions/template.md docs/plans/README.md docs/plans/template.md` and require no matches. Run the positive marker probe from Phase 2 and inspect its terminal set rather than asserting a frozen count.

Perform a focused human semantic review of README, `docs/working-with-awf.md`, `docs/config-reference.md`, `docs/glossary.md`, the topic page/index pair for `rendering/doc-outputs`, `docs/domains/rendering.md`, `docs/pitfalls.md`, `docs/decisions/INDEX.md`, and `CLAUDE.md`. Confirm the same concepts survive rendering, exact paths and globs agree with marker payloads, no family is simultaneously included and excluded, computed sections are distinguished from ordinary section overrides, and literal placeholders are intentional examples rather than unresolved tokens. Record any reasoned deviation in Notes before closing the phase.

### Phase close

Stage the complete Phase 3 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass, with the gate reporting 100% statement coverage. Create the one closing commit:

```commit
docs(rendering): document generated source guidance
```

## Definition of done

- `dod: compact-marker-contract` One renderer-owned `awf:source` grammar has deterministic banner-adjacent placement, compact exact/glob/derived payloads, publication-safe empty behavior, and no edit/read-back semantics.
- `dod: opaque-output-guidance` Every approved opaque document family carries its exact reader-facing marker policy, existing machine dependencies remain authoritative and unchanged, and already-actionable or authored families receive no duplicate marker.
- `dod: canonical-source-instructions` Working-with-awf is the complete operational source map; README, config reference, glossary, and in-file markers agree on where edits belong.
- `dod: repository-green` The successor ADR's three State changes are Applied while it remains Implementing, source and rendered outputs are synchronized, focused semantic review is complete, `awf check staged` is clean, and `./x gate` passes at 100% statement coverage for every phase.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.
