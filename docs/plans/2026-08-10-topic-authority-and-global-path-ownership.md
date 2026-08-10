---
format: plan-v2
date: 2026-08-10
adrs:
  - retire-domain-staleness-audit-heuristics
  - global-topic-path-ownership
status: Proposed
---
# Plan: Topic Authority and Global Path Ownership

## Goal

Retire the obsolete domain-staleness audit heuristics and let globally applicable topics own explicit domain-bounded paths, with exact operation replay, structural validation, coverage, fan-out, schema activation, and orientation surfaces remaining coherent. Semantic document-staleness inference and ownership outside configured domains are non-goals.

## Architecture summary

Execute three independently green transactions. First relocate domain-selector validation and remove the audit heuristics. Second advance the schema and introduce the complete core applicability-versus-ownership model, including every compile-time and machine-output consumer of the replaced witness field. Third project the separated model through context and rendered documentation, then dogfood it by adding `internal/presentation/**` ownership to the natural global presentation-ownership topic while preserving the complementary scoped package-boundary topic. Applicability remains the dependency used by context and markers; bounded ownership is a separate dependency used by coverage and fan-out. Execution leaves both ADRs Implementing with every operation Applied and leaves this plan Proposed; post-implementation assurance owns the later status-only ADR completion, Notes settlement, and plan freeze.

## Phase 1: Retire domain-staleness auditing safely

**Execution mode: subagent-driven.**

Completes: ["audit-heuristics-retired"]

### Task 1.1: Relocate domain-selector validation
Applying: ["retire-domain-staleness-audit-heuristics:validate-domain-selectors-structurally"]
Paths: ["internal/config/config.go", "internal/config/config_test.go", "internal/topic/tree.go", "internal/topic/tree_test.go", "internal/pathglob/"]
Post-check: `go test ./internal/config ./internal/topic ./internal/pathglob` passes with current and staged malformed-domain-selector cases reaching their expected rejection paths.

Move anchored-glob validation for domain sidecar `paths` into the shared working-tree and staged sidecar-loading boundaries before removing audit input assembly. Current and staged malformed selectors must fail explicitly; historical sparse audit projection must continue omitting domain sidecars. Reuse the shared `internal/pathglob` validator rather than introducing another dialect or audit dependency.

### Task 1.2: Remove the three audit heuristics and apply their claims
Applying: ["retire-domain-staleness-audit-heuristics:retire-domain-staleness-heuristics", "retire-domain-staleness-audit-heuristics:preserve-exact-authority-checks"]
Paths: ["internal/audit/audit.go", "internal/audit/audit_test.go", "internal/project/project.go", "internal/project/audit_inputs_test.go", "internal/configspec/spec.go", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/docs/glossary.yaml", ".awf/docs/pitfalls.yaml", "templates/docs/working-with-awf.md.tmpl", "docs/working-with-awf.md", "docs/topics/tooling/audit-and-snapshots.md", "docs/topics/config/validation.md", "docs/config-reference.md", "docs/glossary.md", "docs/pitfalls.md", "docs/decisions/retire-domain-staleness-audit-heuristics.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Delete `domain-doc-staleness`, `domain-code-staleness`, and `undocumented-domain` inputs, accumulators, findings, and tests without changing operation-transition replay or surviving audit rules. Update the fixed advisory inventory, remove the three retired claims, add the test-backed `config/validation:domain-path-globs-valid` claim with current/staged evidence, and revise authored config-reference, glossary, pitfall, and working-with-awf sources so domain paths are described through ownership, context, and coverage. Append the ADR's first `Implementing` event and one Applied event containing exactly all five declared operations; mutate exactly those claims in this transaction and regenerate every listed output.

### Phase close

`go test ./internal/audit ./internal/config ./internal/topic ./internal/project` passes; `./awf context --show pending docs/decisions/retire-domain-staleness-audit-heuristics.md` reports all five operations Applied and none Remaining; the range audit exposes none of the retired warning identities; current and staged malformed selectors still fail structurally; historical projection remains reduced; rendered drift is clean.

```commit
fix(tooling): retire domain staleness heuristics (applies ADR batch)
```

## Phase 2: Establish global topic ownership semantics

**Execution mode: subagent-driven.**

Completes: ["global-ownership-core"]

### Task 2.1: Advance the schema and accept combined global metadata
Applying: ["global-topic-path-ownership:separate-global-applicability-and-ownership", "global-topic-path-ownership:activate-combined-metadata-safely"]
Paths: ["internal/topic/topic.go", "internal/topic/topic_test.go", "internal/project/project.go", "internal/project/version_test.go", "internal/migrate/", "internal/manifest/", "internal/upgrade/", "internal/configspec/spec.go", "cmd/awf/upgrade_test.go", ".awf/topics/parts/config/migrations-and-locks/current-state.md", "templates/docs/working-with-awf.md.tmpl", "docs/working-with-awf.md", "docs/topics/config/migrations-and-locks.md", "docs/config-reference.md", ".awf/awf.lock"]
Post-check: `go test ./internal/topic ./internal/project ./internal/migrate ./internal/manifest ./internal/upgrade ./cmd/awf` passes, and upgrade fixtures prove the new generation without rewriting valid path-only or global-only topic metadata.

Permit path-only, global-only, and combined `applies: global` plus nonempty anchored `paths` metadata while continuing to reject every other empty or contradictory form. Advance the schema generation and minimum binary mapping; make manifest and lock output stamp it; add the no-rewrite migration that advances older valid trees while preserving path-only and global-only metadata. Update the public metadata grammar in the authored working-with-awf template and generated document. Keep parser, migration, upgrade, manifest, and binary-gate tests green before project metadata adopts the combined form.

### Task 2.2: Split applicability from ownership and apply core invariants
Latitude: exact
Applying: ["global-topic-path-ownership:bound-global-ownership-by-domain", "global-topic-path-ownership:global-ownership-satisfies-coverage", "global-topic-path-ownership:global-ownership-counts-in-fanout", "global-topic-path-ownership:distinguish-applicability-from-ownership-evidence", "global-topic-path-ownership:selectors-are-the-only-ownership-declaration", "global-topic-path-ownership:back-global-ownership-invariant"]
Paths: ["internal/topic/markers.go", "internal/topic/coverage.go", "internal/topic/coverage_test.go", "internal/topic/query.go", "internal/topic/query_test.go", "internal/topic/presentation.go", "internal/topic/presentation_test.go", "internal/topic/corpus_test.go", "internal/topic/tree_test.go", "internal/project/currentstate.go", "internal/project/currentstate_test.go", "cmd/awf/topic_test.go", ".awf/topics/parts/invariants/topics-and-markers/current-state.md", ".awf/docs/glossary.yaml", ".awf/docs/pitfalls.yaml", "docs/topics/invariants/topics-and-markers.md", "docs/glossary.md", "docs/pitfalls.md", "docs/decisions/global-topic-path-ownership.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Introduce separate applicability and ownership matching paths. Global applicability must remain repository-wide for `TopicsForPath` and marker validation. Ownership must require both a topic selector and its owning domain selector; claim-bearing matches satisfy only that domain's coverage, and every matching owner counts once in fan-out even when claimless. Replace machine and human `matchedPaths` with selected-universe `applicablePaths` and `ownedPaths` without an alias, updating every compile-time consumer in this phase. Make current-state fan-out output ownership-neutral rather than limiting its wording to path-scoped topics, and pin that public text in `TestCurrentStateReportRouting`. Update the glossary and the recorded global-topic coverage pitfall when the new semantics become active. Add `TestGlobalTopicPathOwnership` with the exact invariant annotation declared by the ADR, covering combined parsing, domain bounding, claim-bearing coverage, claimless fan-out, and applicability outside ownership. Apply the four core `invariants/topics-and-markers` operations in the ADR's first Implementing/Applied transaction, preserving the two projection operations as Remaining.

### Phase close

`go test ./internal/topic ./internal/config ./internal/migrate ./internal/manifest ./internal/upgrade ./internal/project ./cmd/awf` passes; `./awf context --show pending docs/decisions/global-topic-path-ownership.md` reports the four core invariant operations Applied and exactly the rendered-applicability and context-navigation operations Remaining; no production or test reference to `MatchedPaths`, JSON `matchedPaths`, or human `matched-paths` remains; rendered drift is clean.

```commit
feat(invariants): add global topic path ownership (applies ADR batch)
```

## Phase 3: Project and dogfood separated ownership

**Execution mode: subagent-driven.**

Completes: ["ownership-surfaces-coherent"]

### Task 3.1: Update context and rendered topic projections
Latitude: exact
Applying: ["global-topic-path-ownership:distinguish-applicability-from-ownership-evidence", "global-topic-path-ownership:separate-global-applicability-and-ownership"]
Paths: ["internal/topic/render.go", "internal/topic/render_test.go", "internal/project/topics.go", "internal/contextq/context_projection.go", "internal/contextq/context_projection_test.go", "internal/contextq/render.go", "internal/contextq/render_test.go", "internal/contextq/context_test.go"]

Rendered topic applicability prose must state repository-wide global authority and, when present, domain-bounded ownership selectors. Context selector projections must distinguish the global declaration from ownership selectors while remaining declaration-only and witness-free. Keep global topics visible to context for paths outside their ownership selectors.

### Task 3.2: Adopt the natural global owner and apply projection claims
Applying: ["global-topic-path-ownership:separate-global-applicability-and-ownership", "global-topic-path-ownership:distinguish-applicability-from-ownership-evidence", "global-topic-path-ownership:global-ownership-satisfies-coverage", "global-topic-path-ownership:global-ownership-counts-in-fanout"]
Paths: [".awf/topics/metadata/code-design/presentation-ownership.yaml", ".awf/topics/metadata/code-design/presentation-package.yaml", ".awf/topics/parts/code-design/presentation-ownership/current-state.md", ".awf/topics/parts/code-design/presentation-package/current-state.md", ".awf/domains/parts/code-design/current-state.md", ".awf/topics/parts/invariants/topics-and-markers/current-state.md", ".awf/topics/parts/tooling/context-and-topic/current-state.md", "docs/decisions/global-topic-path-ownership.md", "docs/decisions/INDEX.md", "docs/topics/code-design/dependency-composition.md", "docs/topics/code-design/outcome-modeling.md", "docs/topics/code-design/package-composition.md", "docs/topics/code-design/presentation-ownership.md", "docs/topics/code-design/single-home.md", "docs/topics/code-design/state-ownership.md", "docs/topics/code-design/test-design.md", "docs/topics/code-design/presentation-package.md", "docs/topics/invariants/topics-and-markers.md", "docs/topics/tooling/context-and-topic.md", "docs/domains/code-design.md", ".awf/awf.lock"]

Add `internal/presentation/**` to the global `code-design/presentation-ownership` metadata while preserving the scoped `presentation-package` topic and its distinct ADR-0234 package-boundary claim. Preserve the code-design domain selector as the sole domain owner. Update the domain narrative and remaining topic claims to distinguish applicability, ownership, and coverage. Apply the remaining `rendered-applicability-selectors-only` and `context-applicability-navigation` operations in one Applied event, preserving prior Applied history, and regenerate all listed managed outputs.

Perform focused semantic review over the full generated global code-design topic set named in Paths: `presentation-ownership` is the representative combined global-plus-ownership case and `dependency-composition` is the global-only edge case. Also inspect the scoped `presentation-package` topic, code-design domain document, context selector output, and topic coverage output. The outputs must not imply that ownership narrows global applicability, creates domain ownership, or eliminates the complementary package-local contract.

### Phase close

`go test ./internal/topic ./internal/contextq ./internal/project ./cmd/awf` passes; `./awf context --show pending docs/decisions/global-topic-path-ownership.md` reports every operation Applied and none Remaining; `./awf topic code-design/presentation-ownership --coverage` reports repository-wide applicability plus bounded `internal/presentation/**` ownership; context outside that selector still includes the global topic; the scoped presentation-package claim remains active; all named generated prose passes focused semantic review; rendered drift is clean. Both ADRs remain Implementing and this plan remains Proposed for terminal assurance and status-only closure.

```commit
feat(tooling): project global topic ownership (applies ADR batch)
```

## Definition of done

- `dod: audit-heuristics-retired` Audit exposes none of the three retired rule identities while current and staged domain selectors retain explicit anchored-glob validation and historical audit keeps its reduced projection.
- `dod: global-ownership-core` Combined global metadata is schema-safe, applicability remains repository-wide, and domain-bounded ownership deterministically drives coverage and fan-out with the named invariant proof.
- `dod: ownership-surfaces-coherent` Topic, context, rendered documentation, and the real `internal/presentation/**` adoption consistently separate global applicability from bounded ownership, every declared ADR operation is Applied, the complementary scoped package boundary remains active, and the repository gate is green.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- 2026-08-10 plan review: Moved all `MatchedPaths` consumers into Phase 2 so its public machine-field replacement compiles and gates independently.
- 2026-08-10 plan review: Preserved `code-design/presentation-package` and its package-boundary claim because ADR-0234 gives it independent authority; global path ownership complements rather than removes that scoped contract.
- 2026-08-10 plan review: Expanded semantic review to every generated global code-design topic, with combined and global-only representative cases.
- 2026-08-10 plan review: Kept generic clean-baseline, staging, and gate choreography with the execution workflow as required by the plan standard; phase closes name only change-specific focused evidence and terminal state.
- 2026-08-10 Phase 1 deviation: Added `.awf/parts/working-with-awf/config-and-overrides.md` because it is the active replacement source needed to make the planned working-with-awf output change survive rendering.
- 2026-08-10 Phase 1 deviation: The planned pitfall update had no applicable audit-staleness entry in `.awf/docs/pitfalls.yaml`; no pitfall was changed rather than inventing unrelated guidance. The later global-topic coverage pitfall remains assigned to Phase 2.
- 2026-08-10 Phase 1 review: Expanded the domain-path invariant proof across empty, duplicate, and malformed current/staged selectors plus historical sidecar omission, and added the adopter-facing warning removals to the Unreleased changelog.
- 2026-08-10 Phase 2 deviation: Updated `.awf/parts/working-with-awf/config-and-overrides.md` instead of relying on the plan-listed template because the part is the active full-replacement source for the affected public grammar; render, staged check, and gate verified the generated result.
- 2026-08-10 Phase 2 review: Strengthened the ownership and fan-out proofs across both selector boundaries, TopicsForPath, and marker applicability; pinned the new and retired CLI labels; corrected ownership terminology in code documentation; and recorded the schema-41 compatibility break in the Unreleased changelog.
- 2026-08-10 Phase 3 deviation: The shared applicability renderer changed 47 topic documents in total, including 37 outside the enumerated topic-output paths. The additional generated outputs are required by render drift authority and were included in the phase transaction.
- 2026-08-10 Phase 3 review: Added context-navigation proof markers to outside-ownership visibility and complete selector-record rendering, restored the displaced full-authority marker, and documented the fixed-schema context selector compatibility change.
