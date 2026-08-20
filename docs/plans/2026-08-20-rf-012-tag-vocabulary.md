---
format: plan-v2
date: 2026-08-20
adrs: [pitfalls-as-the-current-tag-vocabulary-carrier]
status: Implemented
---
# Plan: RF-012 Tag Vocabulary

## Goal

Make authored pitfalls the only current tag carrier, retain every and only self-hosted vocabulary
member with a demonstrated current pitfall consumer, preserve legacy ADR tag bytes and parsing as
history compatibility, and correct current documentation and durable checks. Do not rewrite ADR
history, remove parser compatibility, enter RF-002 architecture work, move RF-011 documents, upgrade
adopters, or perform RF-008B/RF-014B cleanup.

## Architecture summary

Keep tag policy in the existing `internal/project` check owner pending the separately authorized
RF-002 extraction. Narrow membership validation and health advisory populations from legacy ADRs
plus pitfalls to pitfalls alone, while leaving ADR parsing and `related:` behavior untouched. Apply
the three linked current-state claim updates with this behavior change. Then add a repository-specific
bidirectional census oracle and prune the self-hosted config vocabulary to the tags carried by current
actionable pitfalls. Generic adopters may still declare an unused vocabulary member as ADR-0103
allows; only this repository's dogfood oracle requires its configured set to equal its current pitfall
tag set.

The authoring census measured thirty-seven pitfall-backed members, sixty legacy-ADR-only members,
and one unused member. Every pitfall-backed member was individually inspected and accurately labels
current actionable knowledge rendered in the pitfall index and leaves; the figures describe the
starting corpus and are not implementation targets. Preserve frozen legacy occurrences for removed
members, report them separately from live metadata, and make no pitfall metadata edit merely to
reach a count.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Narrow generic tag governance to pitfalls

**Execution mode: inline.**

Completes: ["pitfall-only-governance", "current-contract-accurate"]

### Task 1.1: Establish and implement the pitfall-only behavior contract
Applying: ["pitfalls-as-the-current-tag-vocabulary-carrier:pitfalls-are-current-tag-carriers"]
Paths: ["internal/project/check.go", "internal/project/check_test.go", "internal/project/notes_test.go"]
Post-check: "Before production changes, focused tests exit nonzero because an unknown legacy ADR tag still produces drift or changes the health population; after the implementation, `go test ./internal/project -run 'TestCheckTagVocabulary|TestTagHealthNotes'` exits zero and proves pitfall unknown-tag drift, empty meanings, domain collision, coverage, frequency threshold, empty-vocabulary behavior, malformed pitfall propagation, and legacy ADR exclusion."

Change the existing tag vocabulary and health producers so only the validated pitfall corpus supplies
carrier tags. Remove no ADR parser field and do not weaken pitfall structural errors, vocabulary
meaning validation, the tag-versus-domain guard, the strict greater-than frequency threshold,
advisory severity, or empty-vocabulary behavior. Rewrite the focused fixtures so a legacy ADR with an
unknown or absent tag cannot affect drift, coverage, frequency numerators, or denominators, while the
same conditions on pitfalls remain observable. Keep `related:` checking under its existing separate
path and prove it remains green.

### Task 1.2: Apply the three claim updates and correct adopter-facing prose
Kind: batch
Applying: ["pitfalls-as-the-current-tag-vocabulary-carrier:pitfalls-are-current-tag-carriers", "pitfalls-as-the-current-tag-vocabulary-carrier:historical-tags-remain-append-only"]
Paths: ["docs/decisions/pitfalls-as-the-current-tag-vocabulary-carrier.md", ".awf/topics/parts/config/configuration/current-state.md", "internal/configspec/spec.go", "internal/project/configreference_test.go", "docs/known-issues.md", "docs/decisions/INDEX.md", "docs/topics/config/configuration.md", "docs/config-reference.md", "docs/domains/config.md", ".awf/awf.lock"]
Representative: "`tag-vocabulary-governed`, `tag-coverage-note`, and `tag-frequency-note` name authored pitfalls as their carrier population and explicitly exclude parsed legacy ADR tags from current governance."
Edge: "An adopter retains legacy ADR tags absent from its non-empty vocabulary; check remains clean for those tags while still rejecting an unknown pitfall tag and warning for an untagged pitfall."
Post-check: "After the ADR enters Implementing with one Applied event naming all three declared updates and `./x render` completes, `./awf context --show pending docs/decisions/pitfalls-as-the-current-tag-vocabulary-carrier.md` reports no Remaining operation; focused tests exit zero; generated config reference and config topic consistently describe pitfall-only membership and advisories; the known-issues heading `The tag-coverage claim includes ADRs that cannot carry tags` is absent; and `./x check` exits zero."

Use `awf-adr-lifecycle` to move the reviewed ADR through Accepted to Implementing and apply the three
claim updates in the same governed transaction as their source prose. Narrow the claims without
renaming them, append the pending ADR slug to `Revised-by`, and retain their existing backing. Update
the configspec description to say that a non-empty vocabulary governs pitfall tags, that meanings
must be nonempty, and that generic unused declarations remain allowed. Update its focused prose
oracle without pinning the self-hosted vocabulary size.

Remove the completed known-issue section through its in-place editable body. Render the current-state
topic, config domain projection, config reference, decision index, and lock. Semantically inspect the
generated text for contradictory ADR-carrier claims and confirm the reference derives its live
vocabulary summary rather than embedding an authored count.

### Phase close

Land the behavior, tests, three applied current-state updates, corrected known issue, configspec prose,
and generated projections as one independently green transaction.

```commit
feat(config): govern current tags through pitfalls (applies ADR batch)
```

## Phase 2: Cull the self-hosted vocabulary to demonstrated consumers

**Execution mode: inline.**

Completes: ["consumer-backed-vocabulary", "append-only-history-preserved"]

### Task 2.1: Add the repository census oracle and prune unsupported members
Kind: batch
Applying: ["pitfalls-as-the-current-tag-vocabulary-carrier:vocabulary-requires-current-consumer", "pitfalls-as-the-current-tag-vocabulary-carrier:historical-tags-remain-append-only"]
Paths: ["internal/project/tag_vocabulary_dogfood_test.go", ".awf/config.yaml"]
Representative: "A configured member carried by a current pitfall appears in both sets and remains; a member found only in legacy ADR frontmatter is absent from current config but its historical bytes remain untouched."
Edge: "An unused configured member or an undeclared pitfall tag makes the bidirectional census test fail with the exact set difference, while legacy ADR-only tags do not enter either current set."
Post-check: "Observe the new dogfood test fail against the pre-cull config for configured members absent from current pitfall metadata, then prune the config and run the same focused test green. A deterministic YAML census over `.awf/config.yaml` and `.awf/docs/pitfalls/*.md` prints a success sentinel only when both tag sets are equal; `git diff 941d2c160c263ccee158545114918a8d -- .awf/docs/pitfalls docs/decisions` shows no historical ADR or pitfall metadata rewrite except the pending successor ADR and its lifecycle history."

Add a repository-level test that loads the real self-hosted config and authored pitfall corpus, then
compares configured vocabulary keys and the union of pitfall metadata tags in both directions. It
must diagnose missing and surplus members by name and must not scan legacy ADR tags. Observe it red
before editing config and green after.

Remove `topic-navigation` and every member whose only carrier is legacy ADR frontmatter. Retain all
individually reviewed pitfall-backed members because each labels current actionable pitfall knowledge
and is validated and displayed in generated pitfall outputs. Do not rename, merge, or retag pitfalls
without a newly exposed accuracy defect; no arbitrary target count authorizes a metadata edit.

### Task 2.2: Render, document, and verify the complete live/frozen disposition
Kind: batch
Applying: ["pitfalls-as-the-current-tag-vocabulary-carrier:vocabulary-requires-current-consumer", "pitfalls-as-the-current-tag-vocabulary-carrier:historical-tags-remain-append-only"]
Paths: ["changelog/CHANGELOG.md", "docs/config-reference.md", ".awf/awf.lock"]
Post-check: "After adding one concise Unreleased bug-fix entry and running `./x render`, the bidirectional dogfood test, `go test ./internal/project`, `./x check`, and the full gate exit zero. A deterministic report compares the base config vocabulary with the final live config and current pitfall metadata, lists every removed member with all preserved legacy ADR frontmatter paths, identifies the unused member separately, and reports no removed member in live config or pitfall metadata. Focused semantic review confirms the config reference, current-state topic, pitfall index, and representative pitfall leaves describe and display the retained live tags without frozen-history claims."

Add an Unreleased entry explaining that current tag governance follows authored pitfalls while legacy
ADR tag bytes remain accepted history. Render generated config reference and lock changes. Produce
the exhaustive completion evidence from parsed metadata, not raw prose grep: retained tags and their
pitfall consumers, removed ADR-only tags and frozen carrier paths, the unused member, and an explicit
empty set for removed live config/current-metadata references. Frozen ADR body prose and frontmatter
are evidence, not cleanup targets.

Run focused and full verification. Keep the ADR Implementing and this plan Proposed after assurance;
do not number, terminally close, integrate, remove topology, finish the effort, edit the audit
program, or begin another issue.

### Phase close

Land the dogfood oracle, consumer-backed self-hosted vocabulary, generated reference and lock, and
adopter-facing changelog as one independently green transaction.

```commit
fix(config): cull historical-only tag vocabulary
```

## Definition of done

- `dod: pitfall-only-governance` Non-empty vocabulary membership and tag-health behavior evaluate authored pitfalls only, preserve all established pitfall validation and advisory semantics, ignore legacy ADR tags as current governance inputs, and leave ADR parsing and `related:` behavior intact.
- `dod: current-contract-accurate` All three current-state claims, configspec and generated config reference prose, tests, and known-issue inventory agree on the pitfall-only carrier boundary with no stale governed-ADR or legacy-ADR coverage claim.
- `dod: consumer-backed-vocabulary` The self-hosted configured vocabulary equals the current authored pitfall tag union in both directions, every retained tag has an individually demonstrated current validation and display consumer, and no count is a target or oracle.
- `dod: append-only-history-preserved` Removed live vocabulary members have no live config or current pitfall-metadata reference, all frozen legacy ADR occurrences remain byte-preserved and are exhaustively reported, parser and `related:` compatibility remain intact, and RF-002, RF-008B, RF-014B, adopters, and the audit program remain untouched.

## Notes

Implementation evidence:

- The repository census test failed before the config cull with 61 configured members lacking a
  pitfall consumer and no pitfall tag missing from config, then passed after the cull.
- The final parsed census reports 37 configured members equal to the 37-tag current pitfall union,
  sixty removed members with byte-preserved legacy ADR frontmatter carriers, and the separately
  unused `topic-navigation` member. Removed live config references, removed current pitfall-metadata
  references, and changed frozen carriers are all empty sets.
- Render inspection confirms that the generated config reference derives the 37-member live summary,
  the current-state topic excludes legacy ADRs, and the pitfall index and representative leaves
  continue to display retained tags without frozen-history claims.
- The adopter-facing changelog entry landed early in the Phase 1 mechanical settlement because that
  review required release-note currency. Phase 2 reuses that exact entry rather than duplicating it.
- Report-only Phase 2 review covered `19b980aeb..1ff5135ec` at the clean phase tip and returned no
  findings across correctness, authority, documentation, test strength, generated ownership,
  frozen-history preservation, and maintainability.

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material
cross-owner revisions rather than editing the plan; the parent supplies the report to phase review
and reconciles required plan changes with findings in one focused post-review settlement commit
before checkpointing or later execution. Record the red/green oracle observations, exact live/frozen
disposition evidence, semantic render inspection, review findings, and material route deviations
here before terminal assurance.

After implementation assurance settles, return integration-ready with the ADR still Implementing,
this plan still Proposed, and managed topology intact. The orchestrator owns numbering, terminal
closure, integration, topology removal, effort finish, and audit-program completion evidence.
