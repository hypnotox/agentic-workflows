---
format: plan-v2
date: 2026-08-10
adrs: [split-pitfall-corpus-and-generated-index]
status: Proposed
---
# Plan: Split Pitfall Corpus and Generated Index

## Goal

Replace the monolithic pitfall sidecar and publication with a strict per-entry authored corpus, a compact generated index and generated leaves, a safe forward migration, and supported `awf new pitfall` creation. Do not restore context surfacing, artifact selection, lifecycle states, a tag index, or a generic indexed-document framework.

## Architecture summary

`internal/pitfall` owns source identity, strict frontmatter parsing, corpus validation, title equivalence, deterministic serialization, relative-link migration preflight, and slug allocation. `internal/project` loads that model and owns repository resolution, index and leaf recipes, output declarations, rendering, hashing, drift, lock, backup, pruning, and closed-tree orchestration. `internal/migrate` consumes the shared model to move generation-42 sidecars into the new corpus before retiring the old authority. The schema activation, complete producer family, migration, dogfood conversion, and six non-CLI claim operations land atomically; the independently useful CLI scaffold and its two claim operations follow in a second green transaction.

## Phase 1: Activate the pitfall corpus and generated family

**Execution mode: subagent-driven.**

Completes: ["pitfall-family", "pitfall-migration"]

### Task 1.1: Build the shared pitfall source model
Applying: ["split-pitfall-corpus-and-generated-index:per-entry-authored-corpus", "split-pitfall-corpus-and-generated-index:path-derived-scaffolding", "split-pitfall-corpus-and-generated-index:cohesive-pitfall-model", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["internal/pitfall", "internal/frontmatter/frontmatter.go", "internal/frontmatter/frontmatter_test.go"]

Create `internal/pitfall` as the single semantic home for `Entry`, path-derived identity, corpus loading from an injected tree/file set, strict V1 frontmatter decoding, canonical serialization, title comparison, Markdown-safe title projection, migration link preflight, and slug allocation. Reuse the repository's frontmatter boundary rather than adding another delimiter parser; extend that boundary only when a pitfall requirement cannot be expressed through its current strict decode API.

The source contract is exact: direct regular `.md` leaves only; lowercase kebab slug matching `[a-z0-9]+(?:-[a-z0-9]+)*`; reserved `index`; required trimmed nonempty title with CR and LF rejected; optional nonempty duplicate-free `domains`, `tags`, and positive `related`; no unknown or duplicate keys; nonblank body; no duplicate active title after `strings.Join(strings.Fields(title), " ")` and `strings.EqualFold`. Preserve display title and body bytes after frontmatter separation. The allocator lowercases the trimmed title, collapses maximal non-ASCII-alphanumeric runs to one hyphen, trims edge hyphens, refuses empty or `index`, and selects the first absent candidate from the base, `-2`, `-3`, onward while honoring reservations made earlier in the same migration.

Treat titles as plain text. Supply deterministic escaping for heading and link-label contexts and table cells so CommonMark punctuation, literal backslashes, brackets, and pipes display literally without changing structure; never transform the body. The relative-link preflight must ignore external URLs, absolute targets, and fragments, ignore fenced and inline-code examples, and report each actual path-relative inline link, image, autolink, or reference-definition target with source identity and destination. It detects and refuses; it never rewrites Markdown.

Tests in `internal/pitfall` must refute every grammar, path, title, duplicate, allocation, escaping, serialization, and link-preflight clause. Include byte-stable round trips, punctuation-heavy titles, collision gaps and in-run reservations, reference-style links, images, fenced examples, and non-ASCII titles whose ASCII slug becomes empty. Keep filesystem and repository orchestration out of this package.

### Task 1.2: Replace sidecar parsing with one project corpus loader and validation projection
Applying: ["split-pitfall-corpus-and-generated-index:per-entry-authored-corpus", "split-pitfall-corpus-and-generated-index:cohesive-pitfall-model", "split-pitfall-corpus-and-generated-index:preserve-validation-and-guidance", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["internal/project/pitfalls.go", "internal/project/pitfalls_test.go", "internal/project/glossary.go", "internal/project/check.go", "internal/project/check_test.go", "internal/project/notes_test.go", "internal/project/surface_coverage_test.go", "internal/project/currentstate.go", "internal/project/currentstate_test.go"]

Replace `pitfallEntry`, `pitfallEntries`, `pitfallsTransform`, and all repeated `data.pitfalls` reads with one operation-owned parsed corpus consumed by render, check, advisory, and staged projections. Project validation retains existing configured-domain, tag-vocabulary, tag-health/frequency, and related-ADR resolution semantics, but findings identify the exact authored source path and slug. Remove disabled-document branches and local-sidecar assumptions that conflict with unconditional catalog rendering; preserve sections-only sidecar behavior through the ordinary doc-section loader.

Do not cache the corpus on `Project`. Derive it once per aggregate operation and thread it to consumers, following the current operation-owned state rule. Direct compatibility projections may derive their own corpus when called independently. Tests prove malformed sources are hard errors in both render and check, domain/tag/ADR findings retain their rank and meaning, domainless entries remain valid, and no consumer reads `data.pitfalls`. Convert `surface_coverage_test.go` pitfall advisory and check cases from sidecar data to malformed and valid corpus-source fixtures rather than deleting their coverage.

### Task 1.3: Produce the compact index and per-entry leaves through the output plan
Applying: ["split-pitfall-corpus-and-generated-index:indexed-leaf-publication", "split-pitfall-corpus-and-generated-index:index-only-convention-framing", "split-pitfall-corpus-and-generated-index:complete-generated-family", "split-pitfall-corpus-and-generated-index:preserve-validation-and-guidance", "split-pitfall-corpus-and-generated-index:bounded-navigation-scope", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["templates/docs/pitfalls.md.tmpl", "templates/pitfalls/entry.md.tmpl", "templates/embed.go", "internal/project/render.go", "internal/project/output_plan.go", "internal/project/output_declarations_test.go", "internal/project/output_plan_test.go", "internal/project/render_test.go", "internal/project/render_tree_test.go", "internal/project/drift_test.go", "internal/project/staged_plan.go", "internal/project/staged_test.go", "internal/project/source_marker_test.go", "internal/project/install.go", "internal/project/install_test.go"]

Keep the catalog `pitfalls` document as `docs/pitfalls.md`, but replace its data transform with an index model assembled from corpus metadata plus existing `prepend` and `append` sections. Render an alphabetical table with linked plain-text titles, domains, tags, and related ADRs; render alphabetical by-domain link groups and an `Unassigned` group; render a coherent explicit empty state. Add one leaf recipe at `docs/pitfalls/<slug>.md` using a dedicated embedded template with the generated banner, exact source marker, one escaped H1, visible metadata, and verbatim body. Preserve `missingkey=zero` and prove absent overrides, empty vars, and an empty corpus emit no `<no value>` token.

Enumerate the index and leaves in both `BuildOutputDeclarations` and executable `OutputPlan` from the same corpus facts. Each leaf consumes its exact source and full source projection. The index declares the corpus sources for provenance but hashes only stable metadata plus framing/template inputs, so a body-only edit changes one leaf recipe and not the index recipe. Assign explicit Markdown policy, declarers, template identity, source guidance, and dependencies to every node. Reuse ordinary foreign-file backup, hand-edit detection, missing-output drift, and policy scans rather than creating pitfall-specific persistence.

Focused tests prove empty, singleton, multiple, punctuation-heavy, domainless, and cross-domain indexes; deterministic title/domain ordering; table escaping; exact leaf body preservation; declaration/plan parity; full consumed-input reporting; index hash isolation from bodies; metadata propagation; working/staged parity; collision handling; lock-ready recipes; and semantic rendering of the first, a middle, and the final dogfood leaf plus the index boundaries.

### Task 1.4: Claim the source tree and prove complete lifecycle behavior
Applying: ["split-pitfall-corpus-and-generated-index:per-entry-authored-corpus", "split-pitfall-corpus-and-generated-index:complete-generated-family", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["internal/project/sweep.go", "internal/project/sweep_test.go", "internal/project/check.go", "internal/project/check_test.go", "internal/project/output_plan.go", "internal/project/output_plan_test.go", "internal/project/drift_test.go", "internal/project/staged_test.go", "internal/project/install_test.go", "internal/project/project_test.go"]

Extend the claimed config-tree model with exactly `.awf/docs/pitfalls/*.md`. Make the closed-tree walker and pitfall corpus agree: reject nested directories, unsupported extensions, invalid and reserved stems, symlinks, and other non-regular entries without following them. Do not claim generated `docs/pitfalls/` as authored input.

Drive the existing render/check/lock/install/prune seams with the dynamic output nodes. Integration tests create, edit, stage, delete, and replace source leaves and prove corresponding index/leaf changes, lock membership, stale-output pruning, hand-edit drift, staged universe isolation, and foreign-output backup. A source deletion removes its leaf and index row without bespoke deletion code. Mutation-check the declaration parity and body-only hash-isolation assertions.

### Task 1.5: Add generation-43 forward migration and activate the schema atomically
Applying: ["split-pitfall-corpus-and-generated-index:path-derived-scaffolding", "split-pitfall-corpus-and-generated-index:safe-forward-migration", "split-pitfall-corpus-and-generated-index:index-only-convention-framing", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["internal/migrate/pitfallcorpus.go", "internal/migrate/pitfallcorpus_test.go", "internal/migrate/migrate.go", "internal/migrate/migrate_test.go", "internal/migrate/pitfalls.go", "internal/migrate/pitfalls_test.go", "internal/migrate/forwardport_test.go", "internal/migrate/retireplanresync_test.go", "internal/migrate/globaltopicownership_test.go", "internal/project/project.go", "internal/project/version_test.go", "internal/project/hooks_test.go", "changelog/CHANGELOG.md"]

Leave the frozen generation-9 part-to-sidecar migration byte-for-byte unchanged. Register the new migration after the execution baseline's current generation (generation 43 on the authored baseline; if integration has consumed that generation, merge and assign the next absolute generation before implementation rather than using relative arithmetic). Update `minVersionBySchema`, current-generation pins, hook fixtures, and forward-port coverage in the same transaction.

Decode the old sidecar through its legacy canonical semantics: preserve its trimmed title, ordered domains/tags/related values, and body. Reserve deterministic slugs in authored order. Before mutation, validate every entry, destination, relative-link refusal, duplicate title, sidecar remainder, and serialized output. On execution, accept an existing byte-identical leaf, refuse conflicting bytes, exclusively create missing leaves, and remove `data.pitfalls` only after all leaves exist and validate. Retain a sections-only sidecar; delete it only when empty. A failure or interruption before authority removal leaves the old sidecar retryable. Do not add a journal or edit relative links.

Tests inject failures before and between leaf creates and before sidecar replacement/removal; prove exact recovery, byte-identical retry, conflicting-file refusal, create-before-retire, no partial authority loss, field preservation, section override retention, empty-sidecar deletion, old-generation chaining through generation 9, and actionable diagnostics naming entry and link. Changelog migration guidance explains the new source directory, relative-link refusal, retry action, and sections-only survivor in the same schema transaction.

### Task 1.6: Convert the dogfood corpus, publish every leaf, and apply the non-CLI claims
Kind: batch
Latitude: exact
Applying: ["split-pitfall-corpus-and-generated-index:per-entry-authored-corpus", "split-pitfall-corpus-and-generated-index:indexed-leaf-publication", "split-pitfall-corpus-and-generated-index:index-only-convention-framing", "split-pitfall-corpus-and-generated-index:cohesive-pitfall-model", "split-pitfall-corpus-and-generated-index:complete-generated-family", "split-pitfall-corpus-and-generated-index:safe-forward-migration", "split-pitfall-corpus-and-generated-index:preserve-validation-and-guidance", "split-pitfall-corpus-and-generated-index:bounded-navigation-scope", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: [".awf/docs/pitfalls.yaml", "glob:.awf/docs/pitfalls/*.md", "glob:docs/pitfalls/*.md", "docs/pitfalls.md", "internal/project/pitfalls_dogfood_test.go", ".awf/domains/rendering.yaml", ".awf/topics/metadata/rendering/doc-outputs.yaml", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/code-design/single-home/current-state.md", "internal/configspec/spec.go", "internal/configspec/spec_test.go", "docs/config-reference.md", "docs/topics/rendering/doc-outputs.md", "docs/topics/config/migrations-and-locks.md", "docs/topics/code-design/single-home.md", "docs/topics/rendering/index.md", "docs/topics/config/index.md", "docs/topics/code-design/index.md", "docs/domains/rendering.md", "docs/domains/config.md", "docs/domains/code-design.md", ".awf/awf.lock", "docs/decisions/split-pitfall-corpus-and-generated-index.md", "docs/decisions/INDEX.md"]
Representative: ["Convert the first sidecar entry into one canonical frontmatter source and verify its generated table row, domain links, metadata block, and verbatim leaf body.", "Convert the punctuation-heavy titles containing backticks, brackets, pipes, or placeholder syntax and verify their displayed titles remain literal.", "Convert domainless and multi-domain entries and verify Unassigned and duplicated domain-index links without duplicated bodies."]
Edge: ["Retain `.awf/docs/pitfalls.yaml` only if non-data section configuration remains after conversion.", "Assign collision suffixes from the shared allocator in original authored order; never derive output identity from later title sorting."]
Post-check: Add permanent `TestPitfallDogfoodSourceOutputParity` in `internal/project/pitfalls_dogfood_test.go`; it opens the repository fixture through production project loading, compares the sorted authored source slug set exactly with the generated `docs/pitfalls/*.md` stem set, resolves every index leaf link to that same set, and fails with the symmetric difference. Run `go test ./internal/project -run '^TestPitfallDogfoodSourceOutputParity$' -count=1` and require Go's PASS sentinel. Then run `./x render`, require `git diff --exit-code` after the intended outputs are staged, run `./x check`, and inspect the index plus representative/edge leaves for intended meaning and absence of contradictory monolith guidance.

Convert every old sidecar entry into one canonical source without changing its semantic title, metadata, or body. Remove `data.pitfalls`; retain no second registry. Expand the rendering domain and `rendering/doc-outputs` selectors to own `internal/pitfall/**` while keeping the `code-design/single-home` claim globally applicable rather than creating overlapping code-design path ownership. Remove the obsolete configspec `data.pitfalls` entry and document the per-file source contract and optional sections-only sidecar.

Transition the ADR from Proposed to Implementing with the staged checker-provided content digest and append one Applied event containing exactly these six operations in the same transaction as their claim mutations:

- remove `rendering/doc-outputs:pitfall-data-validated`;
- add `rendering/doc-outputs:pitfall-corpus-validated` with `Backing: test` and its parser/corpus proof marker;
- add `rendering/doc-outputs:pitfall-output-complete` with `Backing: test` and its complete-family integration proof marker;
- add `code-design/single-home:pitfall-model-single-home` with `Backing: test` and a structural ownership proof that the semantic declarations have one production home;
- update `rendering/doc-outputs:opaque-doc-source-guidance` with its source-marker proof;
- add `config/migrations-and-locks:pitfall-corpus-migration` with `Backing: test` and its migration retry/preflight proof marker.

The updated and added prose must state the ADR's exact active contracts without embedding implementation counts. Regenerate all topic/domain/index/config-reference outputs and the root lock with their sources.

### Phase close

The source schema, migration, dogfood corpus, generated index/leaves, complete output lifecycle, config reference, changelog, and six claim operations form one project-atomic activation. Close them as:

```commit
feat(code-design): split pitfall corpus (applies ADR batch)
```

## Phase 2: Add supported pitfall scaffolding and author guidance

**Execution mode: subagent-driven.**

Completes: ["pitfall-scaffold", "workflow-guidance", "governed-state", "verified-system"]

### Task 2.1: Add `awf new pitfall` through the existing new-command boundary
Applying: ["split-pitfall-corpus-and-generated-index:path-derived-scaffolding", "split-pitfall-corpus-and-generated-index:cohesive-pitfall-model", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["cmd/awf/new.go", "cmd/awf/new_test.go", "cmd/awf/gate_test.go", "cmd/awf/run_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "internal/project/scaffold.go", "internal/project/scaffold_test.go", "internal/project/presentation_boundary_test.go", "internal/pitfall"]

Add the `pitfall` child to the static `awf new` specification and dispatch. Version-gate before project reads or writes, parse exactly one complete title positional under existing interspersed argument rules, load the current corpus, ask `internal/pitfall` for the canonical serialized source and first-free slug, create exactly one file with exclusive semantics, and report its repo-relative authored path through project-owned presentation. A race that occupies the selected path must refuse without trying a new suffix behind the caller's back; an ordinary retry recomputes from current state. Do not render automatically, mutate the sidecar, or introduce rollback for a one-file exclusive create.

Tests cover help/usage/parity, gate ordering, empty/reserved titles, Unicode and punctuation slugging, case/whitespace-equivalent duplicate titles, occupied suffix gaps, exclusive race refusal, exact scaffold bytes, reported path, and zero generated-output mutation. Add the new child to `gate_test.go`'s exhaustive probe and update `run_test.go`'s command-source `project.Open` census. Update command-source structural expectations rather than duplicating kind facts in `cmd/awf`.

### Task 2.2: Route authoring guidance and apply the CLI claims
Kind: batch
Latitude: exact
Applying: ["split-pitfall-corpus-and-generated-index:path-derived-scaffolding", "split-pitfall-corpus-and-generated-index:preserve-validation-and-guidance", "split-pitfall-corpus-and-generated-index:backed-pitfall-contracts"]
Paths: ["templates/skills/retrospective/SKILL.md.tmpl", "templates/skills/bugfix/SKILL.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/skills/parts/retrospective", ".awf/skills/parts/bugfix", ".awf/parts/working-with-awf", ".awf/docs/glossary.yaml", ".awf/agents-doc.yaml", ".awf/topics/parts/tooling/cli/current-state.md", "glob:.pi/skills/awf-*/SKILL.md", "glob:.claude/skills/awf-*/SKILL.md", "AGENTS.md", "docs/working-with-awf.md", "docs/glossary.md", "docs/topics/tooling/cli.md", "docs/topics/tooling/index.md", "docs/domains/tooling.md", "docs/config-reference.md", "README.md", "changelog/CHANGELOG.md", ".awf/awf.lock", "docs/decisions/split-pitfall-corpus-and-generated-index.md", "docs/decisions/INDEX.md"]
Representative: ["Retrospective guidance creates a pitfall through `awf new pitfall` and then edits the reported authored source.", "Bugfix guidance follows the compact generated index to a leaf instead of demanding a whole-corpus read.", "Working-with-awf and glossary distinguish authored sources, the generated index, and generated leaves."]
Edge: ["Unset layout variables and generic templates remain coherent and token-free.", "No durable guidance tells authors to edit `docs/pitfalls.md`, a generated leaf, or `data.pitfalls`."]
Post-check: Run separate checked greps with success sentinels over authoring and guidance surfaces in `templates/`, `.awf/skills/`, `.awf/parts/`, `.awf/agents-doc.yaml`, generated skills, `AGENTS.md`, `README.md`, and current generated guides: retired instructions to author generated pitfalls or `data.pitfalls` have an empty terminal set, while the new command/source/index vocabulary appears in each named representative surface. Exclude append-only ADR/plan history, frozen migration tests, and pitfall entry bodies whose semantic historical examples accurately mention `data.pitfalls`; no exclusion permits a live authoring instruction. Run `./x render`, `./x check`, and focused semantic reads of the rendered retrospective, bugfix, working-with-awf, glossary, and pitfalls index guidance.

Update the exhaustive `tooling/cli:cli-creation-and-inventory` claim to include pitfalls while preserving its no-render-selection clause, and add `tooling/cli:pitfall-scaffold` with `Backing: test` and the focused CLI proof marker. Append one Applied event containing exactly those two operations in the same transaction. Keep the ADR Implementing and the plan Proposed; terminal status-only closure is deferred until implementation assurance settles.

Update changelog and user guidance with command behavior and deletion-as-retirement. Regenerate every affected target skill and neutral documentation output from its authored source. Do not edit historical ADRs or Implemented plans to replace their accurate old-schema references.

### Phase close

The command, rendered guidance, and final two claim operations are one independently useful transaction:

```commit
feat(tooling): add pitfall scaffolding (applies ADR batch)
```

## Definition of done

- `dod: pitfall-family` Every valid authored pitfall has exactly one generated leaf and one compact index row; index/domain navigation, metadata, title escaping, source guidance, working/staged declarations, hashes, lock membership, drift, backup, and deletion pruning agree, while malformed source shapes fail through one shared corpus model.
- `dod: pitfall-migration` A generation-42 or older adopter upgrades through the new absolute generation without field loss or authority ambiguity; relative links and conflicts refuse before retirement, byte-identical partial leaves retry safely, and section-only sidecar configuration survives.
- `dod: pitfall-scaffold` `awf new pitfall "<Title>"` uses the shared deterministic allocator, creates one canonical source exclusively, reports its authored path, and never renders or mutates a second registry.
- `dod: workflow-guidance` Current docs and workflow guidance distinguish authored sources, generated index, and generated leaves, point authors to the command/source corpus, retain no live `data.pitfalls` instruction, and render coherently with empty values.
- `dod: governed-state` All eight declared ADR operations are Applied in two atomic batches with matching test-backed claim mutations; the ADR remains Implementing and this plan remains Proposed pending settled implementation assurance.
- `dod: verified-system` `./x check` is clean, `./x gate` reports 100% statement coverage and no production dead code, deterministic source/output set comparison is empty, and focused semantic rendering review records the inspected index and representative leaves.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- Plan review replaced Task 1.6's ad hoc parity script with permanent `TestPitfallDogfoodSourceOutputParity` and its exact focused invocation. This makes the source/output/index set proof reproducible from the selected snapshot.
- Plan review corrected Phase 1's closing scope from `rendering` to `code-design`. The active dependency-composition commit-classification claim, not a settled user choice, owns cross-package structure that introduces `internal/pitfall` and its consumers.
- Phase 1 review found thirteen authority-determined gaps: presence-aware optional-list validation, complete Markdown code/autolink masking, explicit output dependencies, empty-registry retirement, atomic leaf and sidecar publication, preflighted sidecar remainders, four complete invariant proofs, and stale glossary and retrospective guidance. The post-review settlement corrected all thirteen through existing semantic and publication boundaries, expanded their focused proofs, and introduced no design deviation. The implementation owner's truncated return omitted its deviation field; parent inventory reviewed every omitted path as a necessary consumer, fixture, template, or generated-output consequence and found no unjustified scope. The baseline sidecar contained 46 entries despite the ADR's earlier dated measurement of 47; exact pre-commit comparison confirmed all 46 titles, ordered metadata values, and bodies survived conversion.
