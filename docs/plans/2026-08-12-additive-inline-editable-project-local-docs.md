---
format: plan-v2
date: 2026-08-12
adrs: [additive-inline-editable-project-local-docs]
status: Implemented
---
# Plan: Additive Inline-Editable Project-Local Docs

## Goal

Ship additive `localDocs` declarations whose free-form Markdown bodies are authored inline, checked
and discoverable like other managed documentation, scaffolded by `awf new doc`, and backed up before
declaration removal or uninstall. Do not restore catalog selection, local sidecars, convention-part
bodies, custom templates, or broader staged-check semantics.

## Architecture summary

`config.Config` owns a strictly decoded, validated, name-sorted projection of local document
metadata. A separate project producer translates that projection into neutral Markdown output
nodes rendered through one shared embedded template with one in-place body; it does not modify the
standard catalog or `Layout.Docs`. The same normalized projection feeds output declarations,
per-document hashes, and an explicit union in the agent-guide document map. The command layer owns
only CLI grammar and presentation, delegating structured YAML mutation to `internal/config` and
rendering to `internal/project`.

Before local documents become reachable, confined complete-file backup naming, reading, and
exclusive publication move into one `internal/filesystem` operation consumed by the existing sync
backup path. The feature transaction then makes rendering, prune preservation, and uninstall
preservation live together. `internal/project` owns recognition of the shared local-document
template identity; outer command composition passes that policy into resident uninstall, while
`internal/resident` uses the shared filesystem operation and never imports project or spells the
identity. Current-state claims land pair-atomically with their behavior. Architecture and adopter
guidance travel with each transaction that makes their facts true; lifecycle statuses freeze only
after assurance.

## Phase 1: Shared confined backup mechanism

**Execution mode: subagent-driven.**

Completes: ["backup-single-home"]

### Task 1.1: Move sibling backup publication below its consumers
Applying: ["additive-inline-editable-project-local-docs:preserve-local-doc-on-removal"]
Paths: ["internal/filesystem/handle.go", "internal/filesystem/handle_test.go", "internal/project/install.go", "internal/project/install_test.go", "internal/project/project.go", "internal/project/project_test.go", ".awf/docs/parts/architecture/dependencies.md", "docs/architecture.md", ".awf/awf.lock"]

Move the existing source-read, mode-preserving `.awf-bak[.N]` naming loop and exclusive publication
into one `internal/filesystem` operation over the minimal confined read-and-publish capability.
Retain project-owned error context and result mapping at its caller. Route the existing foreign-file
sync backup through the new operation as its first production consumer, leaving externally visible
behavior byte-for-byte unchanged. The lower package must not learn manifest, template, project,
resident, or uninstall concepts, and no second suffix algorithm remains.

Prove source read errors, permission preservation, occupied suffix retry, non-collision publication
failure, and successful path selection through `go test ./internal/filesystem ./internal/project`.
The command must exit successfully, and `rg -n 'awf-bak|backupPath' internal/project internal/resident`
must show no independent numbered-suffix implementation outside the shared filesystem owner. Update
the architecture dependency section in the same transaction, then run `./x render && ./x check` and
require clean generated state.

### Phase close

```commit
refactor(rendering): centralize confined backup publication
```

## Phase 2: Safe configured local-document outputs

**Execution mode: subagent-driven.**

Advances: ["local-doc-guidance"]
Completes: ["local-doc-core", "local-doc-safety"]

### Task 2.1: Add the typed declaration, validation, schema, and reference model
Kind: batch
Applying: ["additive-inline-editable-project-local-docs:additive-local-doc-declarations", "additive-inline-editable-project-local-docs:local-doc-name-boundary"]
Paths: ["internal/config/config.go", "internal/config/config_test.go", "internal/configspec/spec.go", "internal/configspec/spec_test.go", "internal/migrate/migrate.go", "internal/migrate/changes.go", "glob:internal/migrate/*_test.go", "internal/project/project.go", "internal/project/configreference.go", "internal/project/configreference_test.go", "docs/config-reference.md"]
Representative: Decode and validate one `localDocs` item with exactly `name`, `title`, and `description`; normalize only for deterministic projection, not by rewriting authored config.
Edge: Reject duplicates, empty or multiline metadata, `.md` suffixes, invalid or escaping segments, reserved top-level families, and exact standard-output collisions while an absent list remains valid.
Post-check: Run `go test ./internal/config ./internal/configspec ./internal/migrate ./internal/project`; require successful strict-field parity, validation, byte-identical no-op generation advance, minimum-version registration, and config-reference tests.

Add the root list and closed item type to the strict config schema. Keep validation in `internal/config`
for intrinsic item grammar and reserved roots, and use project output authority for collisions that
depend on catalog and generated paths. Register the next schema generation as a no-byte config
migration whose successful upgrade advances the lock, and describe the list plus item leaves in
configspec so reflection parity, live-state classification, and the generated reference stay closed.
Advance `internal/project.Version` with the schema generation and register that version as the
minimum compatible binary. Do not forward-port the new optional field into historical bytes and do
not alter retired selection key handling.

### Task 2.2: Render local documents through a separate in-place producer
Kind: batch
Applying: ["additive-inline-editable-project-local-docs:inline-freeform-local-doc", "additive-inline-editable-project-local-docs:local-doc-managed-coverage"]
Paths: ["glob:templates/**", "internal/project/singleton.go", "internal/project/render.go", "internal/project/output_plan.go", "internal/project/confighash.go", "internal/project/layout.go", "glob:internal/project/*_test.go", "glob:internal/render/*_test.go"]
Representative: `runbooks/incident-response` declares `docs/runbooks/incident-response.md`, renders the configured H1 and one `awf:edit-in-place` body, and reads that body back verbatim on render and check.
Edge: Keep `catalog.Standard` and `Layout.Docs` independent of local declarations; reject output recipe collisions before rendering and preserve staged-reader isolation without adding staged removal or link checks.
Post-check: Run `go test ./internal/render ./internal/project -run 'Test(LocalDoc|.*InPlace|.*OutputPlan|.*OutputDeclaration|.*ConfigHash|.*Staged.*LocalDoc)'`; require declaration/render parity, name-order invariance, publication-safe empty-value defense, body preservation, awf-owned-heading drift, normalized per-entry hashing, full-catalog regression, and no failing selected test.

Declare one shared Markdown template identity beside the other noncatalog render identities and include
it in the live template census. Build one project-owned normalized projection and reuse it for
output declaration and rendering so paths, policies, inputs, declarers, hashes, and ordering cannot
drift. Each node uses the ordinary Markdown link and skill-reference policy, reads the existing
managed output only for its in-place body, and folds exactly its normalized entry into its config
hash. Collision validation covers catalog docs, singleton and generated docs, and other local
entries without inserting local names into catalog layout.

### Task 2.3: Make prune and uninstall preservation live with the output
Kind: batch
Applying: ["additive-inline-editable-project-local-docs:preserve-local-doc-on-removal"]
Paths: ["internal/project/project.go", "internal/project/install.go", "internal/project/singleton.go", "glob:internal/project/*sync*_test.go", "internal/project/install_test.go", "internal/project/runner_test.go", "internal/resident/resident.go", "internal/resident/resident_test.go", "cmd/awf/uninstall.go", "cmd/awf/uninstall_test.go", "internal/filesystem/handle.go"]
Representative: Removing one declaration and rendering publishes the complete prior document to its first free sibling backup, removes the managed source, and advances the lock only after both operations succeed.
Edge: An absent output needs no backup; unsafe or unreadable sources, final symlinks, escaping parents, publication failures, and removal failures retain the old lock. Ordinary generated outputs retain existing prune and uninstall behavior.
Post-check: Run `go test ./internal/filesystem ./internal/project ./internal/resident ./cmd/awf -run 'Test(LocalDocPrune|Uninstall.*LocalDoc|Backup)'`; require the selected fault matrix to pass, including numbered suffix retry, mixed ordinary/local entries, complete-file recovery, and old-lock retention.

Sync recognizes outgoing local documents from the project-owned shared template identity and invokes
the common filesystem backup operation before prune. For uninstall, expose only the bounded
project-owned recognition policy needed by outer composition; `cmd/awf` supplies it to resident's
uninstall operation. Resident opens the applicable confined tracked or resident handle, invokes the
shared mechanism for matching entries, and reports backups without importing project or duplicating
template identity. No committed state may contain reachable sole-source local documents without
both removal paths preserving them.

### Task 2.4: Apply the core and preservation claims
Kind: batch
Latitude: exact
Applying: ["additive-inline-editable-project-local-docs:additive-local-doc-declarations", "additive-inline-editable-project-local-docs:inline-freeform-local-doc", "additive-inline-editable-project-local-docs:preserve-local-doc-on-removal"]
Paths: ["docs/decisions/additive-inline-editable-project-local-docs.md", ".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/rendering/inplace-and-placeholders/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", "docs/topics/config/configuration.md", "docs/topics/rendering/inplace-and-placeholders.md", "docs/topics/rendering/project-output-plan.md", "docs/topics/rendering/sync-and-drift.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: Add `local-doc-declarations` with focused proof and revise `no-artifact-selection-surface` without weakening unconditional catalog rendering.
Edge: Apply no document-map or CLI operation early; the Phase 2 event contains only the seven named core and preservation operations.
Post-check: Run `./x render && ./x check`; require clean generated drift and a valid ADR history whose Applied event contains exactly the seven Phase 2 operations.

Use the ADR lifecycle procedure to enter Implementing and apply exactly: update
`config/configuration:config-expresses-repo-facts-only`, update
`config/configuration:no-artifact-selection-surface`, add
`config/configuration:local-doc-declarations`, add
`rendering/inplace-and-placeholders:local-doc-body-inline`, update
`rendering/project-output-plan:output-plan-complete`, add
`rendering/sync-and-drift:local-doc-prune-preserved`, and update
`rendering/sync-and-drift:uninstall-removes-lock-entries`. Author each mechanical claim with focused
production and test markers; retain `config-expresses-repo-facts-only` as a rule.

### Task 2.5: Ship declaration, inline-authoring, recovery, and architecture guidance
Applying: ["additive-inline-editable-project-local-docs:local-doc-guidance-travels"]
Paths: ["templates/docs/working-with-awf.md.tmpl", "templates/docs/doc-standard.md.tmpl", "templates/skills/writing-docs/SKILL.md.tmpl", "templates/skills/using-awf/SKILL.md.tmpl", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", "internal/project/local_doc_guidance_test.go", "docs/working-with-awf.md", "docs/doc-standard.md", "docs/architecture.md", ".claude/skills/awf-writing-docs/SKILL.md", ".claude/skills/awf-using-awf/SKILL.md", ".pi/skills/awf-writing-docs/SKILL.md", ".pi/skills/awf-using-awf/SKILL.md", ".awf/awf.lock"]
Post-check: Run `./x render && ./x check && go test ./internal/project -run TestLocalDocAuthoringGuidance`; require per-surface assertions for declaration, reserved namespaces, the sole editable body, ordinary checks, removal and uninstall recovery, and a negative assertion that no rendered skill permits direct edits outside `awf:edit-in-place`. Inspect the rendered passages in both docs and both target skill pairs for coherent meaning and record the inspected boundaries and result in Notes.

Update shipped template defaults, this repository's applicable working-guide override, architecture
parts, and their generated outputs. `writing-docs` routes repository-specific facts to local docs
when no standard doc owns them; `using-awf` names the local-doc body as the narrow exception to its
general generated-output prohibition. Do not mention the not-yet-landed scaffold command or the
not-yet-landed agent-guide projection in this phase.

### Phase close

The phase owner closes one green transaction containing the typed schema, no-byte generation,
separate rendered output family, both preservation paths, and the matching Applied claim batch.

```commit
feat(rendering): add safe local docs (applies ADR batch)
```

## Phase 3: Agent-guide discovery and complete documentation checks

**Execution mode: subagent-driven.**

Advances: ["local-doc-guidance"]
Completes: ["local-doc-discovery"]

### Task 3.1: Union local documents into the agent-guide map
Applying: ["additive-inline-editable-project-local-docs:local-doc-managed-coverage"]
Paths: ["internal/project/layout.go", "internal/project/render.go", "internal/project/confighash.go", "templates/agents-doc/AGENTS.md.tmpl", "glob:internal/project/*agents*_test.go", "internal/project/render_test.go", "internal/project/check_test.go", "AGENTS.md", ".awf/awf.lock"]
Post-check: Run `go test ./internal/project -run 'Test(LocalDoc.*Agent|Agent.*LocalDoc|LocalDoc.*Reference|LocalDoc.*GuideSize)'`; require every selected union, hash, link, skill-reference, and guide-size test to pass.

Build an explicit project-owned document-map union that retains the catalog-only meaning of
`Layout.Docs` while appending local name, output path, title, and description rows sorted by name.
Fold the complete sorted local projection into only the agent-guide config hash. Prove an empty list
is inert, YAML reordering is inert, metadata changes affect the matching local output and agent
guide, every row has a live link, and ordinary working-tree checks scan link and skill references
inside the preserved inline body. Exercise the existing agent-guide size advisory without changing
its threshold or severity.

### Task 3.2: Apply complete-output and document-map claims
Kind: batch
Latitude: exact
Applying: ["additive-inline-editable-project-local-docs:local-doc-managed-coverage"]
Paths: ["docs/decisions/additive-inline-editable-project-local-docs.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", "docs/topics/rendering/doc-outputs.md", "docs/topics/rendering/guide-and-doc-templates.md", "docs/decisions/INDEX.md", "AGENTS.md", ".awf/awf.lock"]
Representative: Add the complete-output invariant and revise the existing document-map claim to cover the explicit catalog-plus-local union.
Edge: Preserve `layout-docs-full-catalog`; the claim update describes agent-guide discovery, not local membership in `Layout.Docs`.
Post-check: Run `./x render && ./x check && go test ./internal/project -run TestLocalDocAgentGuideProjection`; require clean drift and the fixture test to prove one exact link, title, and description with no duplicate row.

Apply exactly: add `rendering/doc-outputs:local-doc-output-complete` and update
`rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`. The new invariant covers the
whole local output path from declaration through lock, drift, reference scanning, and discovery;
attach proof markers to the focused integration tests rather than restating catalog-layout claims.

### Task 3.3: Add discovery and reference-check guidance
Applying: ["additive-inline-editable-project-local-docs:local-doc-guidance-travels"]
Paths: ["templates/docs/working-with-awf.md.tmpl", ".awf/parts/working-with-awf/config-and-overrides.md", "internal/project/local_doc_guidance_test.go", "docs/working-with-awf.md", ".awf/awf.lock"]
Post-check: Run `./x render && ./x check && go test ./internal/project -run TestLocalDocDiscoveryGuidance`; require the shipped default and self-hosted guide to state agent-guide discovery plus ordinary link and skill-reference checks without implying catalog membership or broader staged checks. Inspect and record the rendered section result in Notes.

### Phase close

```commit
feat(rendering): map local docs (applies ADR batch)
```

## Phase 4: Local-document scaffolding command

**Execution mode: subagent-driven.**

Completes: ["local-doc-scaffold", "local-doc-guidance"]

### Task 4.1: Add config-owned structured list mutation
Applying: ["additive-inline-editable-project-local-docs:scaffold-local-doc-command"]
Paths: ["internal/config/edit.go", "internal/config/edit_test.go"]

Add the narrow config mutation that appends one typed local-document mapping while preserving
unrelated values, comments, key order, and the serializer's indentation contract. It refuses a
duplicate or malformed existing list and reads back to the same typed value accepted by strict
parsing. Keep YAML node construction and encoding out of `cmd/awf`.

### Task 4.2: Expose `awf new doc`
Kind: batch
Applying: ["additive-inline-editable-project-local-docs:scaffold-local-doc-command"]
Paths: ["internal/clispec/clispec.go", "glob:internal/clispec/*_test.go", "cmd/awf/new.go", "cmd/awf/new_test.go", "glob:cmd/awf/*gate*_test.go", "cmd/awf/main.go", "README.md"]
Representative: `awf new doc runbooks/api-v2 "How to operate API v2" --title "API v2"` appends the metadata, renders the file, and reports `docs/runbooks/api-v2.md`.
Edge: Without `--title`, derive `Api V2` from the final segment; refuse duplicates, reserved or colliding names, empty metadata, a repeated flag, and any existing destination before mutation.
Post-check: Run `go test ./internal/config ./internal/clispec ./cmd/awf -run 'Test(.*LocalDoc|.*NewDoc|.*GatedCommand|.*README)'`; require all selected mutation, grammar, gate, presentation, README projection, derived-title, and explicit-title tests to pass.

Add `new doc` as a first-class clispec child with exactly two positionals and one optional,
nonrepeatable value flag. Follow existing composition boundaries: CLI grammar and presentation stay
in the command package, intrinsic validation and config bytes stay in config, collision and render
authority stay in project. Reload the written config before rendering and report only the created
repository-relative output through the typed presentation boundary.

### Task 4.3: Apply the CLI inventory claim
Latitude: exact
Applying: ["additive-inline-editable-project-local-docs:scaffold-local-doc-command"]
Paths: ["docs/decisions/additive-inline-editable-project-local-docs.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/tooling/cli.md", "docs/decisions/INDEX.md", "README.md", ".awf/awf.lock"]
Post-check: Run `./x render && ./x check`; require clean generated drift and a valid ADR history whose Applied event contains exactly update `tooling/cli:cli-creation-and-inventory`.

Update `tooling/cli:cli-creation-and-inventory` in the same transaction as the command, preserving
its prohibition on catalog membership selection, and append the matching Applied event.

### Task 4.4: Add scaffold-command guidance
Applying: ["additive-inline-editable-project-local-docs:local-doc-guidance-travels"]
Paths: ["templates/docs/working-with-awf.md.tmpl", ".awf/parts/working-with-awf/commands.md", "internal/project/local_doc_guidance_test.go", "docs/working-with-awf.md", "README.md", ".awf/awf.lock"]
Post-check: Run `./x render && ./x check && go test ./internal/project -run TestLocalDocCommandGuidance`; require the shipped default, self-hosted guide, and generated README command projection to agree on the two positionals, optional `--title`, derived-title behavior, and output path. Inspect and record the rendered command passage in Notes.

### Phase close

```commit
feat(tooling): add new doc (applies ADR batch)
```

## Phase 5: Deferred lifecycle freeze

**Execution mode: inline.**

Completes: ["lifecycle-frozen"]

### Task 5.1: Freeze the settled ADR and plan after assurance
Latitude: exact
Paths: ["docs/decisions/0272-additive-inline-editable-project-local-docs.md", "docs/plans/2026-08-12-additive-inline-editable-project-local-docs.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Begin only after implementation review has settled or its omission has been explicitly justified by
the governing workflow. Reconcile actual reasoned deviations and implementation findings into this
plan's Notes. Use the ADR lifecycle procedure to append only the status-only Implemented event after
all ten declared operations are Applied, then change this plan from Proposed to Implemented. Run
`./x render && ./x check`; require a clean index and lock, an ADR history with no Remaining or
Canceled operation, and an Implemented plan whose Notes reflect the executed work.

### Phase close

```commit
docs(adr): implement additive inline-editable local docs
```

## Definition of done

- `dod: backup-single-home` Confined complete-file sibling backup naming, mode preservation, suffix
  retry, and publication have one lower-level production home consumed by sync without changing
  existing collision behavior.
- `dod: local-doc-core` A valid declaration deterministically produces one collision-safe,
  publication-safe, in-place-editable managed Markdown output without changing the standard catalog
  or `Layout.Docs`; invalid declarations fail strict validation and existing configs advance through
  the no-byte schema generation.
- `dod: local-doc-safety` The first reachable local-doc implementation also preserves every present
  document before declaration-removal prune or uninstall, retains the old lock on failure, and
  leaves ordinary generated-output behavior unchanged.
- `dod: local-doc-discovery` Every local document appears once in the agent-guide map and participates
  in ordinary working-tree drift, link, and skill-reference checks with confined metadata hashes and
  no staged-contract expansion.
- `dod: local-doc-scaffold` `awf new doc` creates the exact config entry and rendered output through
  the version-gated, config-owned, project-rendered, typed-presentation path for derived and explicit
  titles, while refusing collisions before mutation.
- `dod: local-doc-guidance` Adopter docs, architecture, and both rendered skill targets consistently
  teach when and how to declare, scaffold, edit, check, remove, recover, and uninstall local docs.
- `dod: lifecycle-frozen` Independent assurance is settled, every ADR operation is Applied, actual
  findings are recorded, and the ADR and plan are both Implemented with clean generated state.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.
Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- Plan review required the sole-source preservation behavior to land in the same transaction that
  first makes local-document outputs reachable. Disposition: added a behavior-preserving shared
  backup phase, folded prune and uninstall preservation into the core feature phase, and moved the
  reusable mechanism into `internal/filesystem` with project-owned recognition and outer uninstall
  composition to preserve dependency direction.
- Plan review required architecture and adopter guidance to travel with the behavior each passage
  describes and to update shipped defaults rather than only self-hosted overrides. Disposition:
  distributed backup architecture to Phase 1, declaration, inline, recovery, skill, and producer
  guidance to Phase 2, discovery guidance to Phase 3, and command guidance to Phase 4; each phase
  renders and semantically inspects its applicable outputs.
- Plan review found that broad positive grep could not prove coherent edit boundaries. Disposition:
  added per-surface guidance tests with explicit negative direct-edit assertions plus recorded
  semantic inspection of the rendered docs and both skill targets.
- Phase 2 post-review settlement: output collision validation now consults the complete normalized
  declaration inventory before producer rendering, rather than only catalog documents. This is a
  reasoned correction under the ADR's collision authority: target-owned and generated outputs are
  equally ambiguous with a local path, while the local declaration excludes its own declarer.
- Phase 2 post-review settlement: explicit `localDocs: null` and every non-sequence are rejected
  by root-node presence inspection because YAML bypasses the collection decoder for null; omission
  and an empty sequence remain valid and authored list order is not rewritten.
- Phase 2 post-review settlement: resident uninstall owns structured local backup results and
  presentation, retaining the exact `.awf-bak[.N]` selected by the shared confined backup operation.
  A private dependency-composed uninstall seam tests inspection, backup-publication, and
  post-backup-removal faults without importing project or spelling the local template identity.
- Phase 2 post-review settlement: dedicated prune and uninstall fault matrices cover complete
  recovery bytes, first-free suffixes, absence, unsafe/symlink refusal, backup publication and
  removal failures, reporting, and old-lock retention. Reachable local prune and uninstall fault
  branches no longer claim structural coverage impossibility.
- Phase 2 post-review settlement: the complete output-plan invariant now attaches to
  `TestLocalDocsOutputPlan`, which proves plural sorted local nodes, separate template/declarer
  identity, ordinary Markdown policy, regeneration, and catalog and layout independence. The
  local-prune invariant attaches to its dedicated preservation fault matrix, not runner pruning.
- Phase 2 post-review settlement: staged drift regression proves a coherent staged local config,
  output, and lock universe is unchanged when the working-tree local output and declaration are
  removed. It adds no staged removal, new-output, or reference semantics.
- Phase 2 post-review settlement: the Unreleased changelog now tells adopters about `localDocs`,
  its sole inline-editable body, ordinary managed checks, and `.awf-bak` prune/uninstall recovery.
- Second Phase 2 post-review settlement: restored collective proof backing for the complete output-plan claim; strengthened recovery tests with real backup bytes; completed prune fault clauses (including injected inspection); added selected-reader local and config-reference render faults; covered resident open and lock-removal failures; and removed the resulting false reachability ignores. The preservation matrices retain exact backup reports, source and lock safety, suffix behavior, absence, unsafe refusal, publication, and removal outcomes.
- Final Phase 2 assurance settlement: marker-backed local prune testing injects an unreadable source after successful inspection and proves error identity, unchanged source and old lock, and no backup or prune result. Removed false coverage ignores from the reachable runner conditional-unit render failure and corrupt-lock resident local-template preservation; focused tests now exercise both paths.
- Phase 3 semantic inspection covered the rendered local-document guidance passages in `docs/working-with-awf.md` and the self-hosted `AGENTS.md` document map. The guide states agent-guide discovery and ordinary working-tree Markdown-link and skill-reference checks, preserves the catalog and staged-drift boundaries, and does not introduce Phase 4 scaffold guidance; the map remains catalog-only in this repository because it has no local declarations.
- Semantic inspection after settlement covered `docs/working-with-awf.md`, `docs/doc-standard.md`,
  `docs/architecture.md`, `.claude/skills/awf-writing-docs/SKILL.md`,
  `.claude/skills/awf-using-awf/SKILL.md`, `.pi/skills/awf-writing-docs/SKILL.md`, and
  `.pi/skills/awf-using-awf/SKILL.md`. The passages remain coherent, preserve the sole editable
  body and recovery boundary, and do not project Phase 3 agent-guide discovery or Phase 4 scaffold
  command guidance.
- Phase 3 added-path deviation: `internal/project/catalog_sweep_test.go` was added because template
  catalog sweep contexts need the new local-document data; it is governed by the agent-guide union
  and verified by the full gate.
- Phase 3 post-review settlement: moved the normalized local-document union outside the replaceable
  `document-map` catalog section while retaining its ordinary convention-part override, added
  structural custom-section, guide-size, complete hash-census, ordinary reference-report, and
  collective invariant-proof coverage, and amended the existing Unreleased changelog entry. The
  correction preserves catalog and Layout independence, the exact Applied two-operation batch, and
  the Proposed plan and Implementing ADR boundary; it introduces no Phase 4 scaffold guidance or
  operation.
- Final Phase 3 review settlement: the content-sensitive approach attempted to produce exactly one
  Document map heading across default, heading-bearing custom, headingless custom, and dropped
  catalog variants while preserving local rows outside the replaceable catalog body. ADR-0237
  superseded that approach: the template-owned structural heading remains, and a convention part's
  authored leading heading is body content that may coexist with it. The local-doc hash census now
  compares both lock path sets before its exact changed-hash assertion, so added or removed
  after-only entries cannot be ignored.
- Phase 3 authority correction: ADR-0237 requires the template-owned structural heading independent
  of convention-part body content. The local suffix consults only the loaded section drop state for
  its fallback heading; the content-sensitive duplicate part read was removed. The document-map
  claim is narrowed accordingly, and a pre-Phase-3 golden proves empty-default byte inertia.
  Staged lifecycle validation required a separate Reapplied event for that already-Applied claim
  correction; this is a mechanical authority disposition, not an outcome deviation.
- Phase 3 commit-subject deviation: already-landed authority correction commit
  `584aeb7582b7710e15df43bb9717d3254fba4c91` omitted `(applies ADR batch)` from its subject. Its
  Reapplied history/event and staged lifecycle validation are correct; the omission is a
  presentation/audit convention only. The committed settlement transaction is not rewritten, and
  future application commits must use the suffix.
- Phase 4 semantic inspection covered the rendered command passage in `docs/working-with-awf.md`.
  It teaches `awf new doc <name> <description> [--title <title>]`, `Api V2` derivation,
  `docs/<name>.md`, inline body editing, ordinary checking, declaration-removal backup, and
  uninstall recovery.
- Phase 4 post-review settlement: explicit empty titles now remain distinguishable from an absent
  flag and fail intrinsic config validation before mutation. A project-owned candidate preflight
  replaces command-side config mutation, CLI tests cover derived and explicit titles plus every
  required pre-mutation refusal, and injected dependency tests record the retryable config-only
  state after a post-publication render failure. The settlement also restores the discovery proof
  marker and adds the scaffold command to the Unreleased changelog.
- Integration reconciliation merged the current integration branch, preserved Implemented ADR-0270
  retired-key handling and ADR-0271 unconditional `./awf` guidance, composed both Unreleased entries
  with local docs, and numbered the pending record as ADR-0272. The empty-local-doc guide golden
  advanced only for ADR-0271's independent command-prefix change, so it continues to prove local-doc
  omission and an empty list are byte-inert against the current non-local baseline.
