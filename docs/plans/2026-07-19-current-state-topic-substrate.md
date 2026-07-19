---
date: 2026-07-19
adrs: [133, 134, 136]
status: Proposed
---
# Plan: Current-State Topic Substrate

## Goal

Implement the bridge-safe topic substrate required by ADR-0133, ADR-0134, and ADR-0136: strict
current-state configuration, a parsed topic/claim corpus, deterministic rendered topic outputs and
indexes, `awf new topic`, and the read-only `awf topic` query. The existing ADR-derived context,
invariant, supersession, ACTIVE.md, and domain-decision authority remains unchanged; migration,
attestation, the new ADR lifecycle, staged state-impact checks, and project cutover are non-goals.

## Architecture summary

Add `internal/topic` as the single parsed view of path-derived topic metadata, constrained Markdown
claims, direct claim references, provenance, invariant backing, and configured marker sites. Load it
once through `internal/project`, use it as a discovered output producer, and route its rendered topic
documents and per-domain indexes through the existing output plan, manifest, pruning, and drift
machinery. Add `currentState` as an optional strict bridge-preparation config beside the still-active
`invariants` config. Expose the corpus through one two-file scaffold command and one read-only query
command without changing `awf context` or `awf invariants`.

Plans 1 and 2 of the approved four-plan program are one **unreleased bridge tranche**. No tag,
package publication, adopter release, or claim that schema 14 is bridge-complete may occur after this
plan and before the bridge-migration plan lands its inventory, normalization, readiness, attestation,
ordinary-command refusal, and recovery machinery. The transient source state may exercise topic
rendering and queries in tests, but Plan 2 must make every ordinary command, including `sync`, `check`,
`topic`, `context`, and `invariants`, refuse while a bridge lock remains before the bridge release is
cut. This sequencing constraint resolves the apparent shadow-validation conflict without authorizing
a second authority engine: normal context and invariant enforcement remain legacy-only throughout.

**Path and diff notation:** every repository-relative path below is rooted at
`/home/hypno/Projects/agentic-workflows`; this declaration makes, for example,
`internal/topic/topic.go` exactly `/home/hypno/Projects/agentic-workflows/internal/topic/topic.go`.
Each production task is an exhaustive symbol-and-branch diff contract: add only the named declarations
and behavior, preserve every explicitly retained path, and make no unrelated edit. Each new-file task
owns the complete contents of that file family through the named types, functions, validation
branches, and tests; formatting/import details are determined by `gofmt` and the compiler rather than
copied as brittle pre-implementation boilerplate. During review, the user explicitly approved two
narrow plan-convention exceptions for this effort: symbol-and-branch contracts replace implementation-
sized literal Go patch hunks, and tightly coupled parser/render/test file families may share one task
when they close under one package test and one phase commit. These exceptions do not permit vague
behavior, deferred branches, unrelated edits, or multi-commit batching.

## File structure

- **Created:** `internal/topic/{topic,corpus,markers,coverage,render,scaffold,query}.go` and matching `_test.go`
  files; `internal/project/topics.go`, `internal/project/topics_test.go`;
  `templates/topics/{topic,index}.md.tmpl`; `cmd/awf/{topic,topic_test}.go`.
- **Modified:** `internal/config/{config,config_test}.go`, `internal/configspec/{spec,spec_test}.go`,
  `internal/project/{configreference,configreference_test,project,render,output_plan,output_plan_test,
  sweep,sweep_test,domains_test,project_test,drift_test,render_tree_test,version_test}.go`,
  `internal/migrate/{migrate,migrate_test}.go`, `cmd/releasecheck/{main,main_test}.go`,
  `.github/workflows/release.yml`, `templates/{embed.go,domains/domain.md.tmpl}`,
  `internal/clispec/{clispec,clispec_test}.go`, `cmd/awf/{run_test,new,new_test,dispatch}.go`,
  `.awf/docs/parts/architecture/{components,data-flow}.md`,
  `.awf/domains/parts/{adr-system,config,invariants,rendering,tooling}/current-state.md`,
  `templates/docs/working-with-awf.md.tmpl`, `templates/agents-doc/AGENTS.md.tmpl`, create
  `.awf/parts/agents-doc/commands.md`, `README.md`, and
  `changelog/CHANGELOG.md`.
- **Generated/updated by migration and sync:** `.awf/awf.lock`, `docs/{architecture,
  config-reference,working-with-awf}.md`, `docs/domains/{adr-system,config,invariants,rendering,
  tooling}.md`, the corresponding enabled rendered standard documents under `examples/sundial/`,
  and `examples/sundial/.awf/awf.lock`. Use `git diff --name-only` after each sync to stage the exact
  generated fan-out; do not hand-edit generated files.
- **Deleted:** none. In particular, keep `docs/decisions/ACTIVE.md`, domain ADR indexes, legacy
  invariant configuration, and every legacy authority consumer.

## Phase 1: Add strict bridge-preparation configuration and schema generation

- [ ] **Task 1.1: Add the `currentState` config model beside legacy invariants.** In
  `internal/config/config.go`, add `CurrentState *CurrentStateConfig `yaml:"currentState"`` to
  `Config` without modifying `Invariants`. Define `CurrentStateConfig` with `Sources
  []CurrentStateSource`, `TestGlobs []string`, `TopicCoverage string`, `TopicFanout string`, and
  `MaxTopicsPerPath *int`; define `CurrentStateSource` with exactly `Globs []string`, `Marker
  string`, and optional `Close string`. Keep the pointer presence-aware through strict YAML decoding,
  expose an effective-value helper that returns 8 only when it is nil, and never replace an explicit
  value during validation. In `Config.Validate`, normalize absent severities to `error` and `warn`;
  accept only `error|warn|off`; reject an explicitly nonpositive maximum;
  reject duplicate or invalid anchored source/test globs, an empty source glob list, an empty marker,
  and an explicitly empty close token. Preserve the YAML decoder's `KnownFields(true)` rejection of
  unknown nested fields. Do not derive, copy, remove, or consult `Invariants` in this phase.

- [ ] **Task 1.2: Publish every config key from the single configspec authority.** In
  `internal/configspec/spec.go`, add descriptors for `currentState.sources`,
  `currentState.sources[].globs`, `.marker`, `.close`, `currentState.testGlobs`,
  `currentState.topicCoverage`, `currentState.topicFanout`, and
  `currentState.maxTopicsPerPath`. Describe them as topic validation and bridge-preparation inputs,
  explicitly saying they do not switch normal context or invariant authority. Extend
  `internal/project/configreference.go`'s `currentValue` projection so a nil maximum displays the
  documented default 8 while an explicit positive pointer value renders deterministically. Extend
  `internal/configspec/spec_test.go` parity coverage and
  `internal/project/configreference_test.go` golden/current-value cases; never add a second
  hand-maintained config-reference table.

- [ ] **Task 1.3: Add the unreleased bridge-tranche schema generation.** In
  `internal/migrate/migrate.go`, append generation 14 named `current-state-topic-substrate` with a
  no-op apply function: this generation recognizes prepared topic inputs but performs no semantic
  conversion. Update the current-generation pin and registry coverage in
  `internal/migrate/migrate_test.go`. In `internal/project/project.go`, set `Version` to `0.18.0` and
  add `14: "0.18.0"` to `minVersionBySchema`; update the exact pins in
  `internal/project/version_test.go` and update the terminal upgraded-lock schema expectation in
  `cmd/awf/run_test.go`. Do not add attestation lock fields, invariant conversion, or migration
  journaling here. Add `project.BridgeTrancheComplete = false`; make `cmd/releasecheck`
  refuse while false; test the false/true cases and pin release-workflow ordering before GoReleaser.
  Treat 0.18.0 as unreleased until Plan 2 flips the sentinel after bridge completion and publishes it.

- [ ] **Task 1.4: Test strict configuration before rendering consumes it.** Add table cases in
  `internal/config/config_test.go` for omission/defaults, both severities at every legal value,
  nonpositive maximum, duplicate/empty/malformed globs, empty marker/close, and unknown fields.
  Add config-reference cases proving the default and explicit projections. Run:

  ```sh
  go test ./internal/config ./internal/configspec ./internal/project ./internal/migrate
  ```

  Expected: every named package reports `ok`.

- [ ] **Task 1.5: Document the config/schema behavior in the same commit.** Update `README.md` and
  the Unreleased section of `changelog/CHANGELOG.md` with optional `currentState`, schema 14, version
  0.18.0, and the explicit statement that Plans 1 and 2 must both land before a bridge release. Update
  `.awf/domains/parts/config/current-state.md` and the architecture component source in the same
  behavior commit, then render their generated outputs. Do not describe migration readiness as
  available yet.

- [ ] **Task 1.6: Upgrade both adopted fixtures, assert the locks, regenerate, and commit.** Build the
  source binary once, use it to run `upgrade` at the repository root and in `examples/sundial`, then
  parse both JSON locks and assert their schema/version before sync:

  ```sh
  tmp="$(mktemp -d)" && go build -o "$tmp/awf" ./cmd/awf
  "$tmp/awf" upgrade
  (cd examples/sundial && "$tmp/awf" upgrade)
  python3 - <<'PY'
  import json
  for path in ('.awf/awf.lock', 'examples/sundial/.awf/awf.lock'):
      with open(path, encoding='utf-8') as handle:
          lock = json.load(handle)
      assert lock['schemaVersion'] == 14, (path, lock['schemaVersion'])
      assert lock['awfVersion'] == '0.18.0', (path, lock['awfVersion'])
  PY
  ./x sync
  ./x check
  ./x gate
  ```

  Expected: the Python assertion emits no output; `./x check` is drift-free; and the gate reports
  100% coverage, no production dead code, and clean prose. Stage only the Phase 1 Go sources, tests,
  authored docs, README/changelog entries, generated config references/version outputs, and both locks
  reported by `git diff --name-only`; commit:

  ```commit
  feat(config): add current-state preparation schema
  ```

## Phase 2: Parse, validate, render, lock, and prune topics

- [ ] **Task 2.1: Implement strict topic metadata and claim parsing.** Create
  `internal/topic/topic.go` with path-derived `TopicID {Domain, Slug}`, `Metadata {Title, Summary,
  Paths, Applies}`, `Topic`, `Claim`, `ClaimType`, and `Backing` types plus parsing helpers. Decode
  `.awf/topics/metadata/<domain>/<topic>.yaml` with a strict local YAML struct containing only
  `title`, `summary`, `paths`, and `applies`; require nonempty title/one-line summary, kebab domain and
  topic components, and exactly one applicability form: nonempty duplicate-free anchored `paths` or
  `applies: global`. Reuse `internal/pathglob.Validate`.

  Parse `.awf/topics/parts/<domain>/<topic>/current-state.md` as explanatory prose followed by exactly
  one final `## Claims`. Accept only ``### `rule: <slug>` `` and
  ``### `invariant: <slug>` `` headings; reject level-one through level-three headings inside the
  claim region, duplicate local slugs, empty claim prose, and malformed reserved metadata. Parse the
  terminal contiguous metadata block in the exact order `Origin`, optional `Revised-by`, optional
  `References`, then invariant-only `Backing` and conditional `Verify`. Rules reject backing fields;
  invariants require either `Backing: test` with no Verify or `Backing: unbacked` followed by one
  nonempty Verify. Canonical full IDs are `<domain>/<topic>:<local-slug>`.

- [ ] **Task 2.2: Build the validated corpus.** Create `internal/topic/corpus.go` with `LoadCorpus`,
  `All`, `ByTopicID`, `ByClaimID`, and `ForDomain` plus deterministic incoming/outgoing direct-reference
  indexes. Discover metadata and parts from the two fixed trees, require a one-to-one pair, require the
  owning domain in `Config.Domains`, allow the same local slug in different topics, and reject duplicate
  full IDs, dangling/self references, and invalid Origin/Revised-by ADRs. Resolve provenance from the
  injected `adr.Corpus`; require every cited ADR to exist and be Implemented, and preserve Revised-by
  order without duplicates. Reference cycles remain legal and are never expanded.

- [ ] **Task 2.3: Build the configured marker index.** Create `internal/topic/markers.go` to scan only
  configured `currentState.sources` and parse a marker
  on its own comment line as exactly `state: <qualified-id>`, `invariant: <qualified-id>`, or
  `touches-state: <qualified-id> - <nonempty-note>`, honoring optional closing tokens. Reject unknown
  claim IDs; reject state/touches sites outside the effective topic scope; require proof sites to
  match both a configured source and `testGlobs`; require every test-backed invariant to have a proof
  and forbid a proof for an unbacked invariant. Effective path scope is the intersection of topic
  selectors and the owning domain's sidecar paths; only `applies: global` bypasses it.

- [ ] **Task 2.4: Add the focused topic-coverage evaluator used by queries.** Create
  `internal/topic/coverage.go` with `CoverageForTopic` and `TopicCoverage` result types. Given one
  parsed topic, its owning domain's anchored paths, and its marker index, return declared applicability,
  effective domain-intersected selectors, whether the topic contains at least one claim, and sorted
  marker sites. `applies: global` reports global effective scope but never scoped-coverage
  satisfaction; a path-scoped zero-claim shell reports its selectors but
  `SatisfiesScopedCoverage: false`; a nonempty path-scoped topic reports true only for paths in the
  domain/topic intersection. This evaluator does not enumerate the working tree, alter
  `awf context --uncovered`, or apply configured severity; Plan 3 owns snapshot path universes and
  repository-wide coverage diagnostics. Call it from the Phase 2 render-model builder to populate the
  generated topic document's applicability summary, from `awf topic --coverage` in Phase 4, and from
  the scaffold pipeline test in Phase 3; it is therefore production-reachable at the Phase 2 gate.

- [ ] **Task 2.5: Add focused parser/corpus/coverage tests.** Create
  `internal/topic/{topic,corpus,markers,coverage}_test.go` with table fixtures for every accepted and
  rejected grammar branch, paired-tree discovery, domain resolution, path bounding/global scope,
  provenance statuses, duplicate identities, direct references/cycles, source comment forms,
  proof/test-glob scope, empty/global/nonempty scoped-coverage results, and deterministic diagnostics.
  Use temporary repositories and the real ADR parser rather than mocks. Run
  `go test ./internal/topic`; expected: `ok`.

- [ ] **Task 2.6: Add topic templates and deterministic render models.** Create
  `templates/topics/topic.md.tmpl` with the generated title, summary, and `CoverageForTopic`
  applicability summary followed by the authored `current-state` part, and
  `templates/topics/index.md.tmpl` with a title/summary-sorted list linking
  every topic in one domain. Embed `topics/**` in `templates/embed.go`. Create
  `internal/topic/render.go` with deterministic models for the individual document, per-domain index,
  and compact domain-navigation list. Route authored Markdown through the existing raw convention-part
  assembly and authoring-comment stripping path; do not interpolate it as YAML data.

  Modify `templates/domains/domain.md.tmpl` to add a compact Topics link/list while retaining its
  current Decisions section and per-domain ADR index. Empty domains render coherent topic navigation
  rather than a no-value token.

- [ ] **Task 2.7: Integrate the producer with project loading and output planning.** Create
  `internal/project/topics.go` with one lazily cached topic corpus per invocation, reset alongside the
  ADR corpus in `beginInvocation`. Extend `internal/project/render.go` and
  `internal/project/output_plan.go` so each paired topic produces
  `<docsDir>/topics/<domain>/<topic>.md`, each domain with topics produces
  `<docsDir>/topics/<domain>/index.md`, and domain docs receive the compact topic model. Add these as
  normal managed Markdown plan nodes with exact metadata/part/template dependencies, so existing sync,
  manifest hashing, regeneration comparison, brownfield collision handling, pruning, and stale-output
  checks consume them without topic-specific lock fields. Update collision diagnostics for a local
  document or topic named `index` that competes with a generated topic index.

  In `internal/project/sweep.go`, explicitly claim `.awf/topics`, its `metadata` and `parts` roots,
  domain directories, each metadata YAML file, and the single matching current-state part. Reject
  orphan metadata, orphan parts, unexpected sections/files, invalid path components, and unconfigured
  domain directories. Do not add topics to `kindDescriptors` or any enable array.

- [ ] **Task 2.8: Prove the complete render lifecycle.** Create
  `internal/topic/render_test.go` and `internal/project/topics_test.go`; extend
  `internal/project/{output_plan,sweep,domains,project,drift,render_tree}_test.go`. Cover a valid
  path-scoped and global topic, deterministic index order, compact domain links with Decisions still
  present, raw-part/publication-safe rendering, output collisions, lock membership, metadata and part
  drift, add/remove/rename pruning, invalid-corpus refusal, and closed-tree orphan diagnostics. Use a
  batch fixture helper for repeated topic-tree setup, but keep expected output paths literal. Run:

  ```sh
  go test ./internal/topic ./internal/project
  ```

  Expected: both packages report `ok`.

- [ ] **Task 2.9: Document the unreleased producer, sync, verify, and commit.** Update the authored
  architecture component/data-flow parts and the `adr-system`, `config`, `invariants`, and `rendering`
  current-state parts listed in File structure. State precisely that topics are parsed and rendered
  preparation artifacts in this unreleased tranche, that legacy ADR and invariant authority still
  governs normal context/enforcement, and that Plan 2 will place ordinary commands behind bridge-lock
  refusal before release. Document topic output-plan/lock/prune/drift participation as implementation
  substrate, not an adopter-ready shadow authority mode. Update
  `templates/docs/working-with-awf.md.tmpl`, `README.md`, and the Unreleased changelog in this same
  behavior commit with the two input paths, strict metadata and claim grammar, output paths, and the
  no-intermediate-release limitation. Run `./x sync`, `./x check`, and `./x gate`; expected:
  drift-free output, clean invariant checking, 100% coverage, no production dead code, and clean prose.
  Stage exactly the Phase 2 source/test/template/authored-doc paths and sync-generated fan-out; commit:

  ```commit
  feat(rendering): add current-state topic producer
  ```

## Phase 3: Scaffold paired topic inputs without syncing

- [ ] **Task 3.1: Add collision-safe scaffold primitives.** Create
  `internal/topic/scaffold.go` with title-to-kebab slug derivation, configured-domain validation, and
  collision allocation against both metadata and part trees. Reserve `index`, reject a title that
  cannot produce a kebab slug, and never overwrite either half of an existing or orphaned pair. The
  returned paths must be repository-relative and deterministically ordered.

  The metadata scaffold must contain the supplied title, a coherent generic one-line summary, and an
  anchored path placeholder that is valid syntax but explicitly needs adopter editing. The authored
  part must contain an explanatory authoring comment and an empty final `## Claims` section; it must
  contain no invented claim heading, prose, Origin, or backing metadata. Empty shells are valid for
  preparation and rendering but are recorded as not satisfying later coverage checks.

- [ ] **Task 3.2: Add `awf new topic <domain> <title...>`.** In
  `internal/clispec/clispec.go`, add `topic` beneath the existing `new` group with two positional
  components (domain and joined title) and in-handler gating. In `cmd/awf/new.go`, add the `topic`
  dispatch arm, validate usage before the version gate, open the project, call the scaffold helper,
  create parent directories and both files atomically enough that a second-write failure removes the
  first, and print both paths. Do not mutate `.awf/config.yaml`, the lock, or rendered docs, and do not
  invoke sync. Keep all existing ADR/plan/local scaffold behavior unchanged.

- [ ] **Task 3.3: Test scaffold and command behavior through the real pipeline.** Create
  `internal/topic/scaffold_test.go`; extend `internal/clispec/clispec_test.go`,
  `cmd/awf/new_test.go`, and `internal/project/topics_test.go` for success, exact two-file content, no
  invented claims, no sync/lock/config mutation, missing arguments, unknown/noncanonical domain, slug
  collision suffixing, reserved index, either-side orphan refusal, rollback after the second write
  fails, version gating, help/dispatch parity, and the unknown-kind diagnostic. The project integration
  case must call the real scaffold, load the resulting zero-claim shell through `topic.LoadCorpus`,
  build the output plan and render it successfully, then call the coverage evaluator and assert that
  the empty shell does not satisfy scoped coverage. This pins the ADR-0134/ADR-0136 empty-shell seam
  without inventing claim content. Run:

  ```sh
  go test ./internal/topic ./internal/clispec ./cmd/awf
  ```

  Expected: every named package reports `ok`.

- [ ] **Task 3.4: Document, sync, verify, and commit.** Update the tooling current-state authored
  part, `templates/docs/working-with-awf.md.tmpl`, `README.md`, and the Unreleased changelog in this
  behavior commit with the exact command, two outputs, no-sync behavior, manual-authoring requirement,
  and the unreleased-tranche warning. Create `.awf/parts/agents-doc/commands.md` by extending
  `sectionDefault` with `awf new topic` authoring guidance. Run `./x sync`, `./x check`, and `./x gate`; expected:
  no drift, 100% coverage, no production dead code, and clean prose. Stage only Phase 3 paths and the
  generated docs/locks changed by sync; commit:

  ```commit
  feat(tooling): scaffold current-state topics
  ```

## Phase 4: Add the read-only topic and claim query

- [ ] **Task 4.1: Assemble one query result model.** Create `internal/topic/query.go` with a single
  deterministic `QueryResult` used by both human and JSON presentation. Resolve either
  `<domain>/<topic>` or `<domain>/<topic>:<claim>` from the active corpus. Default topic results show
  title, summary, claims, claim types, prose, and backing state; default claim results show that claim
  and its backing state. Hide Origin/Revised-by and reference edges by default.

  `--history` adds only direct active Origin/Revised-by ADR number, title, and status;
  `--references` adds sorted direct incoming and outgoing claim IDs; `--coverage` adds declared scope,
  effective domain intersection, and configured marker sites; combined flags union those fields
  without transitive traversal. `--json` changes presentation only and uses stable field names and
  deterministic arrays. Since State changes and removed-claim history belong to Plan 3, an absent
  active identity remains not found even with `--history`; do not invent tombstones or parse legacy
  supersession as claim history.

- [ ] **Task 4.2: Add the version-gated static-fallback CLI for the unreleased tranche.** In
  `internal/clispec/clispec.go`, add top-level `topic` with exactly one selector and independent
  boolean flags `--history`, `--references`, `--coverage`, and `--json`, gated in its handler. Register
  the handler in `cmd/awf/dispatch.go`; create `cmd/awf/topic.go`. Validate syntax first; outside an
  adopted tree print static usage/reference text without gating; inside an adopted tree run the binary
  version gate, open the project, assemble one result, and render human or JSON output. The command
  must have no writer dependency and must not change the worktree, index, config, or lock. Plan 2 must
  insert bridge-lock refusal ahead of corpus loading before any release; the final current-state
  upgrade later makes the already-tested query reachable to upgraded projects.

- [ ] **Task 4.3: Test every projection and read-only boundary.** Create
  `internal/topic/query_test.go` and `cmd/awf/topic_test.go` for topic/claim selectors, malformed and
  missing identities, every flag independently and in combination, deterministic order, human/JSON
  semantic parity, hidden-by-default provenance/references, direct-only traversal, global and bounded
  coverage, configured marker sites, static fallback, version refusal, and before/after tree and index
  digests proving read-only behavior. Add `TestTopicSubstrateEndToEnd` with literal metadata
  `title: Scheduling`, `summary: Current scheduling contracts.`, and `paths: ["internal/**"]`; a part
  containing final `## Claims`, rule `deterministic-order`, invariant `stable-output`, both with
  `Origin: ADR-0001`, and the invariant with `Backing: test`; `internal/schedule.go` with
  `// state: schedule/contracts:deterministic-order`; and `internal/schedule_test.go` with
  `// invariant: schedule/contracts:stable-output`. The fixture must create Implemented ADR-0001, a
  `schedule` domain owning `internal/**`, matching Go comment sources/test globs, and one unchanged
  legacy invariant. Assert scaffold, completed-corpus load, sync, human/JSON query parity, topic/index/
  lock presence, removal pruning, and byte-identical legacy context/invariant output. Extend
  clispec/dispatch/gated-command pinned tests rather than creating a second command list. Run:

  ```sh
  go test ./internal/topic ./internal/clispec ./cmd/awf
  ```

  Expected: every named package reports `ok`.

- [ ] **Task 4.4: Document, sync, verify, and commit.** Update the tooling current-state authored
  part, working-with-awf template, `README.md`, and the Unreleased changelog in this behavior commit
  with selector grammar, default fields, independent detail flags, JSON behavior, and the active-only
  unreleased limitation. Extend the commands part with `awf topic` and its flags. In
  `templates/agents-doc/AGENTS.md.tmpl`, change the binary-version invariant
  sentence from `` `config` and `context` degrade`` to `` `config`, `context`, and `topic` degrade``;
  `./x sync` must update the root `AGENTS.md` and every enabled adopter rendering, including
  `examples/sundial/AGENTS.md`. Run `./x sync`, `./x check`, and `./x gate`; expected: no drift, 100%
  coverage, no production dead code, and clean prose. Stage only Phase 4 paths and generated fan-out;
  commit:

  ```commit
  feat(tooling): query current-state topics
  ```

## Phase 5: Verify the unreleased boundary and freeze this plan

- [ ] **Task 5.1: Run whole-effort acceptance checks.** Run:

  ```sh
  go test ./internal/topic ./internal/config ./internal/configspec ./internal/project ./internal/clispec ./cmd/awf ./internal/migrate
  ./x sync
  ./x check
  ./x gate
  git diff --check
  ```

  Expected: all packages report `ok`; sync leaves no follow-up drift; check and gate are clean; the
  diff check emits no output. Then run the literal end-to-end fixture alone:

  ```sh
  go test ./internal/project -run '^TestTopicSubstrateEndToEnd$' -count=1
  ```

  Expected: `ok github.com/hypnotox/agentic-workflows/internal/project`; this test owns the exact
  scaffold/content/query/prune and unchanged-legacy assertions specified in Task 4.3.

- [ ] **Task 5.2: Freeze only this plan and commit.** Record actual implementation findings under
  Notes, change this plan's `status` to `Implemented`, and leave ADR-0133, ADR-0134, and ADR-0136
  Proposed because later plans still implement their migration, authority, lifecycle, and cutover
  commitments. Add a Notes entry that Plans 1 and 2 are an unreleased tranche and that releasing is
  forbidden until the bridge-migration plan settles. Run `./x sync`, `./x check`, and `./x gate`;
  stage exactly this plan and any sync-generated index/lock files changed by its status; commit:

  ```commit
  docs(awf): publish current-state topic substrate
  ```

## Verification

- A prepared topic is parsed once, rendered as a managed document, listed in its domain topic index
  and domain navigation, hashed in the lock, detected by drift checking, and pruned on removal.
- Strict metadata, claim, provenance, reference, backing, and marker failures block topic loading with
  deterministic diagnostics; path-scoped topics cannot escape their owner, while explicit global
  topics remain owned and globally visible.
- `awf new topic` writes exactly the two authored inputs and invents no normative claim; `awf topic`
  is deterministic, direct-only, and read-only.
- The source tree contains no call from `awf context` or `awf invariants` into `internal/topic`, and
  no removal of legacy supersession, ACTIVE.md, domain decision indexes, or invariant authority.
- `./x gate` passes at every commit boundary.

## Notes

- User decisions during plan review: preserve the approved decomposition by treating this plan and
  the bridge-migration plan as one unreleased tranche; allow the two narrow exact-patch and coupled-
  file-task exceptions recorded under Path and diff notation; add the focused Plan 1 coverage
  evaluator; and use a presence-aware `*int` for `maxTopicsPerPath`. No release may occur between
  Plans 1 and 2; Plan 2 must add bridge-lock refusal before publication.
- Migration inventory, readiness, attestation, recovery, transaction journal, deterministic
  `upgrade --check --json` (`ready`, `findings`, exhaustive adjudication records, and exact planned
  mutation path/hash/mode records), and v0.18.0 publication belong to the bridge-migration plan.
- Only runtime Plan 3 Phase 1 closes independently. Plan 3 Phases 2-5 and Plan 4's authored corpora,
  marker/ADR preparation, two attestations, final upgrade, legacy deletion, shared cutover commit, and
  separately gated v0.19.0 release form one coupled slice after published v0.18.0.
