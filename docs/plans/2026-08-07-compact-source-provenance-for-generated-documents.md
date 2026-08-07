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

Add one formatter beside banner injection, while each document producer supplies its own reader-facing source list. Land the marker contract, topic consumers, and their operational guidance first; then extend behavior and documentation together across the remaining opaque families. Current-state claim mutations travel with the production behavior they describe; the linked ADR remains Implementing and the plan remains Proposed until post-implementation assurance settles.

## Phase 1: Establish the compact marker contract and topic provenance

**Execution mode: subagent-driven.**

Advances: ["opaque-output-guidance", "canonical-source-instructions", "repository-green"]
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

### Task 1.3: Apply the render-engine claims and publish topic editing guidance
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:source-marker-contract", "compact-source-provenance-for-generated-documents:reader-facing-source-policy", "compact-source-provenance-for-generated-documents:instruction-authority"]
Paths: ["docs/decisions/compact-source-provenance-for-generated-documents.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/render-engine/current-state.md", "internal/render/render_test.go", "internal/project/banner.go", "internal/project/banner_test.go", "templates/docs/working-with-awf.md.tmpl", ".awf/parts/working-with-awf/config-and-overrides.md", "README.md", "docs/working-with-awf.md", "docs/topics/rendering/render-engine.md", "glob:docs/topics/**", ".awf/awf.lock"]
Post-check: Run `./x render`, then verify `./x check` reaches zero findings. Read back the ADR, generated ADR index, source topic part, rendered render-engine topic, working guide, README, representative topic page, topic index, and `.awf/awf.lock`; confirm only lifecycle-authorized source and generated changes remain. Run `rg -n 'awf:source|no-section-marker-leak|source-marker-informational' internal .awf/topics/parts/rendering/render-engine/current-state.md README.md docs/working-with-awf.md docs/topics/rendering` and require every hit to be an intended implementation, proof/touches marker, active claim, operational instruction, or generated output.

Use `awf-adr-lifecycle` for the first incremental application. Change `status:` to `Implementing`; append the dated `Implementing; content-sha256:` event before the Applied event; obtain its content digest mechanically by inserting 64 zeros, running `./x check`, copying the reported digest exactly, replacing the zeros, and rerunning rather than precomputing it. Then append one Applied event containing exactly:

- update `rendering/render-engine:no-section-marker-leak`
- add `rendering/render-engine:source-marker-informational`

In the same transaction, update `no-section-marker-leak` so `awf:section` and `awf:end` remain forbidden while the banner, `awf:edit` family, and informational `awf:source` are the only allowed rendered awf markers. Add the test-backed `source-marker-informational` invariant with Origin naming the pending ADR: one renderer-owned HTML comment follows the banner, carries a nonempty compact source list, is never parsed as an edit/read-back boundary, and is absent when no family policy supplies sources. Keep the existing no-leak proof and add the new proof/touches markers at the units established in Tasks 1.1 and 1.2.

In the same behavior transaction, update both the default working-with-awf template and this repository's replacement section with the universal banner versus `awf:edit` versus informational `awf:source` distinction, the exact topic metadata/part pair, the topic-index globs, and the output-plan-versus-lock authority distinction. Add a concise README sentence that routes detailed source editing to working-with-awf. Do not describe Phase 2 families as already implemented. Run `./x render` so the ADR index, working guide, topic outputs, and lock travel with their sources.

Perform a focused semantic review of `docs/topics/rendering/render-engine.md`, `docs/topics/rendering/index.md`, `docs/working-with-awf.md`, and README: the banner/source ordering and exact path/glob readings must agree, `awf:source` must not be described as `awf:edit` or an exhaustive dependency list, surrounding prose must not contradict the new marker, and any literal placeholder syntax must be an intentional example.

### Phase close

Stage the complete Phase 1 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass, with the gate reporting 100% statement coverage. Create the one closing commit:

```commit
feat(rendering): add compact topic source provenance
```

## Phase 2: Cover every remaining opaque document producer

**Execution mode: subagent-driven.**

Completes: ["opaque-output-guidance", "canonical-source-instructions", "repository-green"]

### Task 2.1: Thread family-owned source policy through declarative and bridge rendering
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:reader-facing-source-policy", "compact-source-provenance-for-generated-documents:opaque-output-scope", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["internal/project/render.go", "internal/project/output_plan_test.go", "internal/project/glossary_test.go", "internal/project/pitfalls_test.go"]

Start only after Phase 1 is committed in the managed worktree: `git status --short` prints nothing, `git log -1 --format=%s` names the Phase 1 closing commit, `./x check` completes with zero findings, and `./x gate` passes at 100% statement coverage.

Add an optional producer-supplied source list to the existing render options/spec seam without adding it to `RenderedFile`, `OutputRecipe`, `ConsumedInputs`, or manifest structures. Ensure options are allocated safely for neutral docs that previously needed none. Apply these exact policies:

- glossary: `.awf/docs/glossary.yaml derived:awf-standard-vocabulary`
- pitfalls: `.awf/docs/pitfalls.yaml`
- every descriptor-owned target bridge: `AGENTS.md`

Do not annotate other standard docs, `AGENTS.md` itself, skills, agents, target outputs, hooks, resident files, ADR/plan support templates, synthesized local docs with adequate `awf:edit`, or any `local: true` artifact. Keep bridge machine inputs unchanged; `AGENTS.md` is the reader routing policy, not a claim that bridge bytes are hashed from the guide. A discovered need to alter machine dependency semantics stops execution for a separately authorized design deviation.

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

### Task 2.3: Complete the operational source map and publication record
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:instruction-authority", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["templates/docs/working-with-awf.md.tmpl", ".awf/parts/working-with-awf/config-and-overrides.md", "templates/docs/config-reference.md.tmpl", "README.md", ".awf/docs/glossary.yaml", "changelog/CHANGELOG.md"]

Extend the Phase 1 working-with-awf source map to every implemented family: ordinary docs and `AGENTS.md`, domains, topic pages and indexes, glossary, pitfalls, ADR index, config reference, target bridges, generated local docs, and banner-free authored ADRs/plans/`local: true` docs. State the edit-render-check transaction once. Re-derive this repository's replacement section and correct its stale plan-v1 authoring statement to plan-v2.

In the config-reference template introduction, identify configspec/catalog descriptions, `.awf/config.yaml`, relevant sidecars, and effective output state as authority layers; state that only its declared intro section is convention-part editable. Keep field semantics in the reference rather than duplicating template mechanics. Keep README concise: distinguish rendered outputs, section overrides, source markers, regenerated outputs, and banner-free authored/local artifacts, preserve the exact topic pair, and route details to working-with-awf.

In `.awf/docs/glossary.yaml`, make these exact vocabulary edits:

- Correct `current-state topic` to say strict metadata and the authored current-state part are paired authorities.
- Add `source marker`: "An informational `awf:source` comment on generated documentation that points to concise reader-facing authorities without acting as an edit boundary or exhaustive dependency list."
- Add `regeneration-derived document`: "A managed document recomputed from repository or catalog state rather than only its template and sidecar; drift is checked by regeneration."

Add one concise entry under the Unreleased section of `changelog/CHANGELOG.md` announcing compact `awf:source` markers for opaque generated docs and the canonical source-editing map. Preserve plain ASCII punctuation and the glossary terseness threshold.

### Task 2.4: Apply opaque-document authority, regenerate, and audit the complete feature
Latitude: exact
Applying: ["compact-source-provenance-for-generated-documents:opaque-output-scope", "compact-source-provenance-for-generated-documents:instruction-authority", "compact-source-provenance-for-generated-documents:publication-safe-source-markers"]
Paths: ["docs/decisions/compact-source-provenance-for-generated-documents.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", "internal/project/source_marker_test.go", "AGENTS.md", "CLAUDE.md", "docs/working-with-awf.md", "docs/topics/rendering/doc-outputs.md", "glob:docs/topics/**", "glob:docs/domains/**", "docs/glossary.md", "docs/pitfalls.md", "docs/config-reference.md", ".awf/awf.lock"]
Post-check: Run `./x render`, then verify `./x check` reaches zero findings. Read back every generated mutation target reported by render, the ADR and generated ADR index, all listed representative outputs, and `.awf/awf.lock`. Run `git diff --check`; run `rg -n '<no value>|awf:source[^>]*(<domain>|<topic>|TBD)' README.md AGENTS.md CLAUDE.md docs` and require the probe to run with no matches. Run `rg -n '^<!-- awf:source ' docs/topics docs/domains docs/glossary.md docs/pitfalls.md docs/decisions/INDEX.md docs/config-reference.md CLAUDE.md` and inspect the terminal set against the family policy rather than asserting a frozen count. Probe exclusions with `rg -n '^<!-- awf:source ' AGENTS.md docs/decisions/README.md docs/decisions/template.md docs/plans/README.md docs/plans/template.md`; success is the command running and returning no matches.

Add test-backed invariant `rendering/doc-outputs:opaque-doc-source-guidance` with Origin naming the pending ADR. Its prose must enumerate the marked families and the non-duplication/banner-free boundary from ADR Decision `opaque-output-scope`, while making clear that marker payloads are reader guidance rather than exhaustive machine dependencies. Put its proof marker on the comprehensive matrix test. Append one Applied event to the still-Implementing ADR for exactly `add rendering/doc-outputs:opaque-doc-source-guidance` in the same transaction. Run `./x render` and retain all generated outputs with their source changes.

Perform a focused semantic review of README, `docs/working-with-awf.md`, `docs/config-reference.md`, `docs/glossary.md`, the topic page/index pair for `rendering/doc-outputs`, `docs/domains/rendering.md`, `docs/pitfalls.md`, `docs/decisions/INDEX.md`, and `CLAUDE.md`. Confirm the same concepts survive rendering, exact paths and globs agree with marker payloads, no family is simultaneously included and excluded, computed sections are distinguished from ordinary section overrides, `derived:` tokens read as authorities rather than file paths, and literal placeholders are intentional examples rather than unresolved tokens. Record any reasoned deviation in Notes before closing the phase.

Leave the ADR Implementing and the plan Proposed after this phase. After independent implementation review settles, `effort-workflow` owns one status-only terminal transaction that first reconciles final findings and deviations into Notes, then flips both the ADR and this plan to Implemented.

### Phase close

Stage the complete Phase 2 transaction explicitly. Run `awf check staged` and `./x gate`; both must pass, with the gate reporting 100% statement coverage. Create the one closing commit:

```commit
feat(rendering): mark opaque document sources
```

## Definition of done

- `dod: compact-marker-contract` One renderer-owned `awf:source` grammar has deterministic banner-adjacent placement, compact exact/glob/derived payloads, publication-safe empty behavior, and no edit/read-back semantics.
- `dod: opaque-output-guidance` Every approved opaque document family carries its exact reader-facing marker policy, existing machine dependencies remain authoritative and unchanged, and already-actionable or authored families receive no duplicate marker.
- `dod: canonical-source-instructions` Working-with-awf is the complete operational source map; README, config reference, glossary, and in-file markers agree on where edits belong.
- `dod: repository-green` The successor ADR's three State changes are Applied while it remains Implementing and the plan remains Proposed, source and rendered outputs are synchronized, focused semantic review is complete, `awf check staged` is clean, and `./x gate` passes at 100% statement coverage for every phase; after assurance, the effort-owned terminal transaction reconciles Notes and freezes both artifacts as Implemented.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation. After implementation review settles, `effort-workflow` reconciles the final findings here and owns the status-only transaction that flips the still-Implementing ADR and this still-Proposed plan to Implemented.

- Phase 1 added `.awf/topics/metadata/rendering/render-engine.yaml` paths for `internal/project/banner.go`, `internal/project/banner_test.go`, and `internal/project/topics_test.go`. The reviewed marker proofs otherwise sat outside the render-engine topic's effective selector scope, so staged current-state validation required this bounded metadata expansion; it changes no marker behavior or approved family scope.
- Phase 2's required broad unresolved-token probe also reports intentional historical prose in authored plans and research documents under `docs/`; the new outputs contain no unresolved values or placeholder markers. The probe's scope is preserved rather than rewriting unrelated historical records, and the focused matrix plus rendered-output review verifies the approved family boundary.
- Phase 2 review removed hash/slash-banner source insertion and replaced the shell-backed bridge fixture with descriptor-owned Markdown. The ADR fixes the marker grammar as an HTML comment on generated documentation, so emitting it inside an otherwise executable script was invalid; bridge inputs and dependency semantics remain unchanged. The same settlement narrows the source-guidance claim to `local: true` artifacts, expands the negative family matrix, and documents both metadata and body authorities for synthesized local docs.
