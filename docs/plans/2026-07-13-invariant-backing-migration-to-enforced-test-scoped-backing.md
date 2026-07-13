---
date: 2026-07-13
adrs: [105, 106]
status: Proposed
---
# Plan: Invariant-Backing Migration to Enforced Test-Scoped Backing

Flips the ADR-0105/0106 mechanism (shipped additively by the
`2026-07-13-enforced-test-backing-and-two-marker-mechanism` plan) to **hard teeth**: every existing
awf invariant is migrated to the proof/touches marker model, `invariants.testGlobs` is set to
`['**/*_test.go']`, and both ADRs flip to `Implemented`. Design rationale lives in ADR-0105 and
ADR-0106 — this plan is the execution record and does not re-argue it.

## Goal

Reach enforced test-scoped backing with a green gate at every step: reclassify the one genuinely
untestable invariant to `unbacked-invariant:`, add a test-file proof marker for the other 72
production-only-backed slugs (downgrading their production markers to `touches-invariant:`), turn on
`testGlobs`, then flip ADR-0105 and ADR-0106 to `Implemented` — backing their 12 new slugs on the
mechanism's existing tests and retiring `context-tier1-governs` for its union-aware successor
`context-tier1-marker-union`.

## Architecture summary

Five phases, each a self-contained commit that passes `./x gate` (`invariants.testGlobs` stays unset
through Phases 1–2, so the source-only fallback keeps every migrated slug backed; teeth switch on in
Phase 3):

1. **Reclassify the sole unbacked invariant** (`adr-new-no-overwrite`) — its refusal guard is a
   `// coverage-ignore` unreachable branch (`internal/adr/adr.go`), so no test can prove it. ADR-0042
   declaration → `unbacked-invariant:` + `Verify:` note; its production marker → `touches-invariant:`.
2. **Add test proof markers for the 72 backed slugs** — each gets a `// invariant: <slug>` marker on
   the test that already asserts it, and its production `// invariant:` marker becomes
   `// touches-invariant: <slug> — <note>`. One slug (`skills-set-in-confighash`) needs a new proof
   test written first; one (`lock-atomic-save`) has three production sites and one proof site.
3. **Turn on `testGlobs`** — set `invariants.testGlobs: ['**/*_test.go']` in `.awf/config.yaml`;
   backing now means an executed test line.
4. **Flip ADR-0105 `Implemented`** — add proof markers for its 9 mechanism slugs on the existing
   mechanism tests, amend its one inaccurate illustrative example, rewrite the invariants domain
   narrative, regenerate `ACTIVE.md`.
5. **Flip ADR-0106 `Implemented`** — add proof markers for its 3 context slugs, retire
   `context-tier1-governs` (remove its markers; add the `context-tier1-marker-union` marker), swap the
   AGENTS.md context bullet and the tooling domain narrative, regenerate `ACTIVE.md` + `AGENTS.md`.

## Tech stack

Go 1.26. Packages/files touched: `.awf/config.yaml`, `docs/decisions/{0042,0092,0105,0106}-*.md`,
production `.go` files across `cmd/awf`, `cmd/{releasecheck,repoaudit,pincheck}`, and `internal/**`,
their `_test.go` siblings, `.awf/agents-doc.yaml`, `.awf/domains/parts/{invariants,tooling}/current-state.md`,
and the regenerated rendered surfaces (`AGENTS.md`, `docs/decisions/ACTIVE.md`, `docs/domains/*.md`,
`docs/config-reference.md`). Gate: `./x gate` before every commit; `./x check` for drift. The
migration's completeness is verified by running the invariants checker with `testGlobs` set and
requiring zero issues.

## File structure

- **Created:** none (the `skills-set-in-confighash` proof test extends an existing `_test.go`).
- **Modified:** `.awf/config.yaml`; `docs/decisions/0042-adr-scaffolding-command.md`,
  `docs/decisions/0092-read-only-context-query-command.md`,
  `docs/decisions/0105-enforced-test-backing-and-the-proof-touches-invariant-marker-split.md`,
  `docs/decisions/0106-backed-aware-two-marker-context-surfacing.md`; every production `.go` file in
  the marker table below and each named `_test.go`; `.awf/agents-doc.yaml`;
  `.awf/domains/parts/invariants/current-state.md`, `.awf/domains/parts/tooling/current-state.md`; and
  the regenerated `AGENTS.md`, `docs/decisions/ACTIVE.md`, `docs/domains/{invariants,tooling}.md`,
  `docs/config-reference.md`.
- **Deleted:** none.

## Phase 1 — Reclassify the sole unbacked invariant (`adr-new-no-overwrite`)

`adr-new-no-overwrite` is the one migrated slug with no honest proof: its guard
(`internal/adr/adr.go`, `if _, err := os.Stat(path); err == nil {`) carries a `// coverage-ignore`
comment because `NextNumber` always returns a fresh name, so the refusal branch is unreachable and no
test exercises it. It becomes an `unbacked-invariant:` with a hand-written `Verify:` note. The
symmetric checks (`unbacked-refuses-proof`, `unbacked-requires-verify-note`) are already live from the
mechanism plan and are independent of `testGlobs`, so this reclassification must convert the
production marker and add the `Verify:` note in the same commit.

- [ ] **Task 1.1 — Reclassify the ADR-0042 declaration.** In
  `docs/decisions/0042-adr-scaffolding-command.md`, change the `adr-new-no-overwrite` bullet (locate
  with `grep -n adr-new-no-overwrite docs/decisions/0042-adr-scaffolding-command.md`):

  ```diff
  -- `invariant: adr-new-no-overwrite` — `awf new adr` refuses to overwrite an existing file at its computed
  -  target path rather than silently clobbering it.
  +- `unbacked-invariant: adr-new-no-overwrite` — `awf new adr` refuses to overwrite an existing file at its
  +  computed target path rather than silently clobbering it. **Verify:** `NewFile` (`internal/adr/adr.go`)
  +  stats the target and returns an "already exists" error before any write; the branch is
  +  `// coverage-ignore` (unreachable, since `NextNumber` always returns one past every existing
  +  `NNNN-*.md`), so confirm the guard precedes the write by reading `NewFile`.
  ```

- [ ] **Task 1.2 — Downgrade the production marker to `touches-invariant:`.** In
  `internal/adr/adr.go`, on the `NewFile` doc-comment block (locate with
  `grep -n 'invariant: adr-new-no-overwrite' internal/adr/adr.go`):

  ```diff
  -// invariant: adr-new-no-overwrite
  +// touches-invariant: adr-new-no-overwrite — refuse-overwrite guard; unbacked (unreachable), see ADR-0042 Verify note
  ```

- [ ] **Task 1.3 — Verify and commit.** `./x gate` && `./x check` (both clean; the reclassified slug
  now satisfies `unbacked-refuses-proof` — no proof marker remains — and `unbacked-requires-verify-note`;
  the note-bearing `touches-invariant:` raises no bare-touches advisory). `git add
  docs/decisions/0042-adr-scaffolding-command.md internal/adr/adr.go`. Commit:
  `refactor(invariants): reclassify adr-new-no-overwrite as unbacked (unreachable guard)`.

## Phase 2 — Add test proof markers for the 72 backed slugs

Every other production-only-backed slug has a test that asserts it (the repo's 100%-coverage gate,
ADR-0012). Each is migrated identically: add a `// invariant: <slug>` proof marker on the asserting
test, and rewrite the production `// invariant: <slug>` marker to
`// touches-invariant: <slug> — <note>` (the note briefly names what the production site does, so no
bare-touches advisory fires). With `testGlobs` still unset, the source-only fallback keeps each slug
backed (the new test-file marker is a source file), so the gate stays green.

Two slugs are handled first because they are not pure marker-moves:

- [ ] **Task 2.1 — Write the missing `skills-set-in-confighash` proof test, then mark it.**
  `skills-set-in-confighash` (`internal/project/confighash.go`, the `render.ReferencesSkills` fold)
  has no dedicated test asserting its drift consequence, unlike its siblings `scopes-in-confighash`
  and `invariant-markers-in-confighash`. Add a test to `internal/project/confighash_test.go` mirroring
  `internal/project/drift_test.go :: TestScopesEditReflagsReferencingArtifacts`: it must build a
  project whose enabled set renders at least one `.skills`-referencing artifact, compute its
  `ConfigHash`, toggle the `skills` enable array, and assert the hash changes (the enable-set edit
  reflags a `.skills`-referencing artifact stale). Read `TestScopesEditReflagsReferencingArtifacts`
  and `internal/project/confighash.go` first to match the existing harness and the exact
  `render.ReferencesSkills` condition. Place `// invariant: skills-set-in-confighash` on the asserting
  line of the new test, and downgrade the production marker:

  ```diff
  -		// invariant: skills-set-in-confighash
  +		// touches-invariant: skills-set-in-confighash — folds the effective skills set into ConfigHash; proof in confighash_test.go
  ```

- [ ] **Task 2.2 — Add the 71 remaining proof markers (batch).** For each slug in the **affected-site
  set** table below, apply the two edits. Locate the production marker with
  `grep -rn 'invariant: <slug>' --include='*.go' .` (line numbers below are as of base commit
  `e18b05c` and shift as edits land — always grep) and the proof test by the named function.

  **Representative** (`include-splice`) — add the proof marker on the asserting test, downgrade the
  production marker:

  ```diff
  # internal/render/include_test.go — in func TestExpandIncludesSplices:
   	// (on or just above the assertion that the partial body is spliced verbatim)
  +	// invariant: include-splice
  ```
  ```diff
  # internal/render/include.go:
  -// invariant: include-splice
  +// touches-invariant: include-splice — include-directive splice site; proof in include_test.go
  ```

  **Edge — `context-read-only` (two production markers, marker lands on an already-named snapshot
  test).** Its property is *already* test-backed (ADR-0092 documents the snapshot test); the migration
  only relocates the marker. Add `// invariant: context-read-only` inside
  `cmd/awf/context_test.go :: TestRunContextReadOnly` (the test that snapshots tree mtimes and
  `.awf/awf.lock` bytes before/after), and downgrade **both** production markers
  (`cmd/awf/context.go`, on `runContext` and `runUncovered`):

  ```diff
  # cmd/awf/context.go (two sites):
  -// invariant: context-read-only
  +// touches-invariant: context-read-only — read-only command entry point (no writer dependency); proof in context_test.go
  ```

  **Edge — `lock-atomic-save` (three production markers, one proof site).** Add
  `// invariant: lock-atomic-save` inside
  `internal/manifest/manifest_test.go :: TestWriteFileAtomicFailureLeavesTargetUntouched`, and
  downgrade all three production markers — `internal/manifest/manifest.go`,
  `internal/migrate/configedit.go`, `internal/migrate/singletonstandarddocs.go`:

  ```diff
  -// invariant: lock-atomic-save
  +// touches-invariant: lock-atomic-save — atomic temp-file+rename write site; proof in manifest_test.go
  ```

  **Affected-site set** (71 slugs; `skills-set-in-confighash` is Task 2.1; `context-read-only` and
  `lock-atomic-save` follow the edge shapes above). `test file :: TestFunc` names the proof site — if
  a `_test.go` in the same package holds the named function, that is the file; otherwise the table
  gives the path:

  | slug | production marker (base `e18b05c`) | proof test :: function |
  |---|---|---|
  | add-skill-pairs-agent | cmd/awf/list_add.go:210 | cmd/awf/list_add_test.go :: TestRunAddAppliesClosurePlan |
  | adr-new-heading-matches-file | internal/adr/adr.go:259 | internal/adr/adr_test.go :: TestNewFileHappyPath |
  | adr-new-sequential-numbering | internal/adr/adr.go:212 | internal/adr/adr_test.go :: TestNextNumberSkipsGapToMaxPlusOne |
  | adr-new-strips-markers | internal/adr/adr.go:258 | internal/adr/adr_test.go :: TestNewFileHappyPath |
  | adr-new-version-gated | cmd/awf/new.go:19 | cmd/awf/gate_test.go :: TestNewGatesInHandler |
  | audit-domain-doc-staleness | internal/audit/audit.go:280 | internal/audit/audit_test.go :: TestRuleDomainDocStaleness |
  | audit-uncommitted-changes | internal/audit/git.go:24 | internal/audit/git_test.go :: TestRuleUncommittedChanges |
  | audit-undocumented-domain | internal/audit/audit.go:318 | internal/audit/audit_test.go :: TestRuleUndocumentedDomain |
  | awf-bak-flagged | internal/project/sweep.go:116 | internal/project/sweep_test.go :: TestSweepFlagsUnclaimedEntries |
  | changelog-embed-decodes | internal/changelog/changelog.go:25 | internal/changelog/changelog_test.go :: TestLoadFromEmbed |
  | changelog-flags-exclusive | cmd/awf/changelog.go:17 | cmd/awf/changelog_test.go :: TestRunChangelogFlagsExclusive |
  | changelog-range-chronological | internal/changelog/changelog.go:114 | internal/changelog/changelog_test.go :: TestRange |
  | cli-command-spec-single-source | internal/clispec/clispec.go:38 | internal/clispec/clispec_test.go :: TestNamesAndUsageLine |
  | cli-config-kinds | internal/project/kind.go:29 | internal/project/kind_test.go :: TestKindLookups |
  | commit-gate-shared-rule | internal/audit/audit.go:137 | cmd/awf/commitgate_test.go :: TestRunCommitGateRejectsNonConventional |
  | config-mutation-roundtrip | internal/config/edit.go:58 | internal/config/edit_test.go :: TestSetArrayMember |
  | config-serialization-owned | internal/config/edit.go:286 | internal/config/edit_test.go :: TestMarshalSkeleton |
  | configspec-var-derivation | internal/configspec/spec.go:47 | internal/configspec/spec_test.go :: TestConfigspecVarDerivation |
  | context-output-parity | cmd/awf/context.go:126 | cmd/awf/context_test.go :: TestRunContextJSONParity |
  | context-static-fallback | cmd/awf/context.go:48 | cmd/awf/context_test.go :: TestRunContextStaticFallback |
  | cursor-no-bridge | internal/project/render.go:344 | internal/project/target_test.go :: TestMultiTargetRender |
  | glob-migration-anchored | internal/config/edit.go:257 | internal/config/edit_test.go :: TestAnchorNoSlashGlobs |
  | include-in-templatehash | internal/project/render.go:519 | internal/project/golden_test.go :: TestTemplateHashCoversExpandedSource |
  | include-missing-fails | internal/render/include.go:22 | internal/render/include_test.go :: TestExpandIncludesMissingPartialFails |
  | include-no-nested | internal/render/include.go:23 | internal/render/include_test.go :: TestExpandIncludesNestedFails |
  | include-no-sections | internal/render/include.go:24 | internal/render/include_test.go :: TestExpandIncludesSectionMarkerFails |
  | include-splice | internal/render/include.go:21 | internal/render/include_test.go :: TestExpandIncludesSplices |
  | init-force-backs-up | internal/project/install.go:58 | cmd/awf/run_test.go :: TestInitGuardBlocksAndForceOverrides |
  | init-set-closed | internal/project/scaffold.go:111 | internal/project/scaffold_test.go :: TestScaffoldDefaultIsClosed |
  | local-catalog-clone | internal/project/local.go:48 | internal/project/local_test.go :: TestLocalSynthesisDoesNotMutateStandard |
  | local-doc-catalog-clone | internal/project/local.go:57 | internal/project/local_test.go :: TestLocalDocSynthesisDoesNotMutateStandard |
  | local-doc-map-fields | internal/project/local.go:195 | internal/project/local_test.go :: TestLocalDocSubfolderPathAndMap |
  | local-doc-name-path-validated | internal/config/config.go:319 | internal/config/docname_test.go :: TestValidateDocName |
  | local-doc-no-shadow | internal/project/local.go:165 | internal/project/local_test.go :: TestLocalDocNameShadowingStandardStaysStandard |
  | local-doc-renders-from-base | internal/project/render.go:242 | internal/project/local_test.go :: TestLocalDocRendersFromBase |
  | local-doc-requires-declaration | internal/project/local.go:174 | internal/project/local_test.go :: TestLocalDocUndeclaredNameFailsOpen |
  | local-name-validated | internal/config/config.go:300 | internal/config/config_test.go :: TestValidateArtifactName |
  | local-no-shadow | internal/project/local.go:82 | internal/project/local_test.go :: TestLocalNameShadowingStandardStaysStandard |
  | local-renders-from-base | internal/project/render.go:222 | internal/project/local_test.go :: TestLocalSkillRendersFromBasePerTarget |
  | local-requires-declaration | internal/project/local.go:92 | internal/project/local_test.go :: TestLocalUndeclaredNameFailsOpen |
  | multi-target-render | internal/project/render.go:295 | internal/project/target_test.go :: TestMultiTargetRender |
  | no-replacewith | internal/config/config.go:21 | internal/config/config_test.go :: TestSidecarRejectsReplaceWith |
  | no-residual-section-marker | internal/render/section.go:106 | internal/render/section_test.go :: TestCheckResidualMarkersBareTokenLegal |
  | no-section-marker-leak | internal/render/render.go:72 | internal/render/render_test.go :: TestRenderDefault |
  | part-marker-advisory | internal/render/section.go:84 | internal/render/section_test.go :: TestHasMarkerLine |
  | part-placeholder-sandboxed | internal/project/placeholders.go:127 | internal/project/placeholders_test.go :: TestSubstitutePlaceholders |
  | parts-raw | internal/render/render.go:167 | internal/render/render_test.go :: TestPartBodyIsRawNeverTemplated |
  | pathglob-anchored | internal/pathglob/pathglob.go:24 | internal/pathglob/pathglob_test.go :: TestMatchAnchored |
  | pitfall-data-validated | internal/project/pitfalls.go:32 | internal/project/pitfalls_test.go :: TestPitfallEntriesErrors |
  | plan-new-unnumbered | internal/plan/plan.go:97 | internal/plan/plan_test.go :: TestNewFileHappyPath |
  | provenance-banner | internal/project/banner.go:16 | internal/project/banner_test.go :: TestInjectBannerPlain |
  | release-changelog-pin | cmd/releasecheck/main.go:26 | cmd/releasecheck/main_test.go :: TestRunFailsStaleNewestEntry |
  | remove-block-scoped | internal/config/edit.go:82 | internal/config/edit_test.go :: TestSetArrayMember |
  | repo-audit-error-exit | cmd/repoaudit/main.go:112 | cmd/repoaudit/main_test.go :: TestErrorMissingEntry |
  | scaffold-core-only | internal/project/scaffold.go:97 | internal/project/scaffold_test.go :: TestScaffoldEnablesCoreTargets |
  | scaffold-seeds-all-vars | internal/project/scaffold.go:32 | internal/project/scaffold_test.go :: TestScaffoldVarsCoverAllReferenced |
  | section-edit-pointer | internal/render/render.go:41 | internal/render/render_test.go :: TestEditPointerStub |
  | sidecar-key-overrides-default | internal/project/datamerge.go:10 | internal/project/datamerge_test.go :: TestWithDefaultData |
  | singleton-doc-migration-relocates-parts | internal/migrate/singletonstandarddocs.go:18 | internal/migrate/singletonstandarddocs_test.go :: TestSingletonStandardDocsRelocatesSidecarAndParts |
  | single-version-authority | cmd/awf/version.go:37 | cmd/awf/version_test.go :: TestAwfVersionSingleAuthority |
  | stub-notes-path-keyed | internal/project/check.go:92 | internal/project/notes_test.go :: TestStubNotesPathKeyedAcrossTargets |
  | stub-part-verbatim | internal/render/section.go:62 | internal/render/section_test.go :: TestHasStubMarker |
  | sync-backs-up-foreign | internal/project/project.go:170 | internal/project/project_test.go :: TestSyncReportBacksUpForeignIndexNotManaged |
  | uncovered-collapses-directories | internal/project/context.go:298 | internal/project/context_test.go :: TestUncovered |
  | uncovered-lists-unowned-only | internal/project/context.go:297 | internal/project/context_test.go :: TestUncovered |
  | uncovered-output-parity | cmd/awf/context.go:102 | cmd/awf/context_test.go :: TestRunContextUncoveredJSONParity |
  | uninstall-removes-lock-tracked | internal/project/install.go:102 | internal/project/install_test.go :: TestUninstallSkipsEscapingLockPaths |
  | version-compat-gate | cmd/awf/gate.go:35 | cmd/awf/gate_test.go :: TestGateBehindVersionErrors |
  | workflow-actions-sha-pinned | cmd/pincheck/main.go:27 | cmd/pincheck/main_test.go :: TestRunFailsTagPinnedAction |

  Placement rule: the proof marker must open its line (after optional leading whitespace) and sit on
  or immediately above the assertion that exercises the property — not merely anywhere in the file —
  so it reads as a claim about *that* test. If a named test does not, on reading, actually assert the
  slug's property, stop and record it in Notes rather than planting a hollow marker (this is the
  authorized escape valve — see Notes).

- [ ] **Task 2.3 — Verify and commit.** Post-check (deterministic completeness proof): temporarily add
  `testGlobs` to `.awf/config.yaml`, run the checker, then revert:

  ```
  # add under invariants: →  testGlobs:\n    - '**/*_test.go'
  go run ./cmd/awf invariants   # expect: exit 0, no "invariant issue(s)" line
  git checkout .awf/config.yaml  # revert — testGlobs is turned on for real in Phase 3
  ```

  Then `./x gate` (100% coverage holds; the new `skills-set-in-confighash` test is covered) && `./x
  check` (clean — every `touches-invariant:` names a declared slug and carries a note, so no
  dangling-marker and no bare-touches advisory appears). `git add` the changed production `.go` files,
  their `_test.go` siblings, and `internal/project/confighash_test.go`. Commit:
  `feat(invariants): add test proof markers for 72 backed invariants`.

## Phase 3 — Turn on `testGlobs`

- [ ] **Task 3.1 — Set `invariants.testGlobs` and regenerate.** In `.awf/config.yaml`, add the glob
  list under `invariants:` (the `TestGlobs` field and its validation shipped with the mechanism plan):

  ```diff
   invariants:
     disabled: false
     sources:
       - globs:
           - '**/*.go'
         marker: //
  +  testGlobs:
  +    - '**/*_test.go'
  ```

  Run `./x sync` (regenerates `docs/config-reference.md`, whose live-state projection now shows awf's
  `testGlobs` value).

- [ ] **Task 3.2 — Verify and commit.** `./x gate` && `./x check` (clean: backing now means an executed
  test line; every backed slug has its Phase-2 test marker, `adr-new-no-overwrite` is unbacked, the
  12 ADR-0105/0106 slugs are not yet required — those ADRs are still `Proposed` — and
  `context-tier1-governs` remains backed by `internal/project/context_test.go`). `git add
  .awf/config.yaml docs/config-reference.md`. Commit:
  `feat(config): enforce test-scoped invariant backing via testGlobs`.

## Phase 4 — Flip ADR-0105 to `Implemented`

Flipping ADR-0105 makes its 9 declared slugs enforced; each is `Backed` and needs a proof marker in a
`testGlobs` file. The mechanism plan already wrote the tests — this phase only annotates them. It also
corrects ADR-0105's one now-inaccurate illustrative example (per the authorization to keep ADR data
correct) and discharges ADR-0105's doc-currency obligations.

- [ ] **Task 4.1 — Add proof markers for the 9 ADR-0105 slugs.** Place `// invariant: <slug>` on the
  asserting line of each named test (all in `internal/invariants/invariants_test.go` except the last,
  in `internal/config/config_test.go`). Grep each function to confirm it asserts the property before
  marking; if two slugs map to one function, mark two distinct assertion lines within it.

  | slug | proof test :: function |
  |---|---|
  | proof-marker-test-scoped | invariants_test.go :: TestCheckProofInNonTestFileScoped |
  | absent-testglobs-source-fallback | invariants_test.go :: TestCheckProofInNonTestFileScoped (the fallback assertion — the block that re-checks with `TestGlobs` empty) |
  | backed-requires-proof | invariants_test.go :: TestCheckBackedNoProof |
  | unbacked-refuses-proof | invariants_test.go :: TestCheckUnbackedWithProof |
  | unbacked-requires-verify-note | invariants_test.go :: TestCheckUnbackedWithoutVerify |
  | touches-marker-advisory | invariants_test.go :: TestMarkersUnderTwoMarkers (the assertion that a `touches-invariant:` marker does not back / raises no finding) |
  | dangling-marker-advisory | invariants_test.go :: TestCheckDanglingMarkerNote |
  | bare-touches-note | invariants_test.go :: TestCheckBareTouchesNote |
  | testglobs-anchored-validated | internal/config/config_test.go :: TestInvariantTestGlobsValidation |

- [ ] **Task 4.2 — Amend ADR-0105's inaccurate example and flip its status.** In
  `docs/decisions/0105-...marker-split.md`: (a) the Context example naming `context-read-only` as
  "genuinely-untestable" is now wrong (it is backed via its snapshot test, as is the structural
  single-source trio) — rewrite it to the one property class awf actually leaves unbacked:

  ```diff
  -  ADR-0008 Decision item 4 ... forces a genuinely-untestable but
  -  load-bearing property — e.g. `context-read-only`, structural single-source properties — to shed
  -  its slug and become prose.
  +  ADR-0008 Decision item 4 ... forces a genuinely-untestable but
  +  load-bearing property — e.g. a defensive guard whose refusal branch is unreachable by construction —
  +  to shed its slug and become prose.
  ```

  (Read the exact surrounding lines first; preserve wording outside the example.) (b) Flip the
  frontmatter `status: Proposed` → `status: Implemented`.

- [ ] **Task 4.3 — Rewrite the invariants domain narrative.** In
  `.awf/domains/parts/invariants/current-state.md`, replace the single-marker description with the
  two-marker model: a `invariant: <slug>` (backed) or `unbacked-invariant: <slug>` + `Verify:` note
  (unbacked) ADR declaration; a proof `invariant:` source marker that backs only in an
  `invariants.testGlobs` file (`['**/*_test.go']` here; source-only fallback when unset); the advisory
  `touches-invariant:` context marker; and the symmetric checks (backed-requires-proof,
  unbacked-refuses-proof, unbacked-requires-verify-note) plus the dangling-marker/bare-touches
  advisories. Keep the retirement (ADR-0031) and the `DeclaringADRs` join description. Preserve ADR-0077
  glob wording. Then `./x sync` (regenerates `docs/decisions/ACTIVE.md` for the status flip and
  `docs/domains/invariants.md`).

- [ ] **Task 4.4 — Verify and commit.** `./x gate` && `./x check` (clean: ADR-0105's 9 slugs are now
  enforced and backed in test scope; `ACTIVE.md` shows `0105` Implemented). `git add
  internal/invariants/invariants_test.go internal/config/config_test.go
  docs/decisions/0105-*.md docs/decisions/ACTIVE.md
  .awf/domains/parts/invariants/current-state.md docs/domains/invariants.md`. Commit:
  `feat(invariants): enforce ADR-0105 test-backing (status Implemented)`.

## Phase 5 — Flip ADR-0106 to `Implemented` and retire `context-tier1-governs`

Flipping ADR-0106 enforces its 3 context slugs and applies its `retires_invariants:
[context-tier1-governs]`. The retired slug's markers are removed and its union-aware successor
`context-tier1-marker-union` is marked in the same commit (ADR-0106 requires this), and the AGENTS.md
context bullet + tooling narrative swap to the successor.

- [ ] **Task 5.1 — Add proof markers for the 3 ADR-0106 slugs.** Place `// invariant: <slug>` on the
  asserting line of each test (grep to confirm the assertion first; `context-invariant-backed-labeled`
  and `context-surfaces-marker-notes` share `TestContextForLabelsAndNotes` — mark two distinct lines).

  | slug | proof test :: function |
  |---|---|
  | context-tier1-marker-union | internal/invariants/invariants_test.go :: TestMarkersUnderUnionScan |
  | context-invariant-backed-labeled | internal/project/context_test.go :: TestContextForLabelsAndNotes |
  | context-surfaces-marker-notes | internal/project/context_test.go :: TestContextForLabelsAndNotes |

- [ ] **Task 5.2 — Retire `context-tier1-governs` markers.** Remove the proof marker
  `// invariant: context-tier1-governs` from `internal/project/context.go` (on the Tier-1 join, ~:136)
  and from `internal/project/context_test.go` (~:102). At the `context.go` production site, add a
  successor touches marker recording the production context:

  ```diff
  -	// invariant: context-tier1-governs
  +	// touches-invariant: context-tier1-marker-union — Tier-1 union-scan join site; proof in invariants_test.go
  ```

- [ ] **Task 5.3 — Flip ADR-0106 status and swap the context wording.** (a) In
  `docs/decisions/0106-...surfacing.md`, flip `status: Proposed` → `status: Implemented`. (b) In
  `.awf/agents-doc.yaml`, rewrite the `**Context Tier 1 governs.**` bullet (locate with
  `grep -n context-tier1-governs .awf/agents-doc.yaml`): rename the trailing
  `(invariant: context-tier1-governs)` to `(invariant: context-tier1-marker-union)` and reword the
  sentence so "present as a marker under a queried path" reflects the union scan of
  `invariants.sources` + `testGlobs` recognising both the proof `invariant:` and the
  `touches-invariant:` marker, and note that each governing invariant is labelled backed/unbacked.
  (c) In `.awf/domains/parts/tooling/current-state.md`, update the `awf context` narrative for the
  union scan (`MarkersUnder` over `sources` ∪ `testGlobs`, both marker kinds), the backed/unbacked
  labelling of governing invariants, and the surfaced `Verify:`/touches site notes. Then `./x sync`
  (regenerates `AGENTS.md`, `docs/decisions/ACTIVE.md`, `docs/domains/tooling.md`).

- [ ] **Task 5.4 — Verify and commit.** `./x gate` && `./x check` (clean: `context-tier1-governs` is
  retired — dropped from the required set with no residual marker — `context-tier1-marker-union` and
  the other two ADR-0106 slugs are enforced and backed; `AGENTS.md` and `ACTIVE.md` regenerated; no
  `context-tier1-governs` reference survives outside the historical ADR-0104/0106 bodies). `git add
  internal/invariants/invariants_test.go internal/project/context.go internal/project/context_test.go
  docs/decisions/0106-*.md docs/decisions/ACTIVE.md .awf/agents-doc.yaml AGENTS.md
  .awf/domains/parts/tooling/current-state.md docs/domains/tooling.md`. Commit:
  `feat(invariants): enforce ADR-0106 union-scan context surfacing, retire context-tier1-governs`.

## Verification

- `./x gate` and `./x check` clean at every phase boundary and at the end.
- With Phase 3 landed, `go run ./cmd/awf invariants` reports **zero** issues, and temporarily removing
  any single Phase-2 proof marker makes it fail (the teeth are real).
- `awf context <a production path in the marker table>` labels the governing invariant `backed`; a
  query touching `internal/adr/adr.go` labels `adr-new-no-overwrite` `unbacked` and surfaces its
  `Verify:` note.
- `docs/decisions/ACTIVE.md` shows ADR-0105 and ADR-0106 `Implemented`.
- `grep -rn 'context-tier1-governs' AGENTS.md docs/domains/ .awf/` returns nothing (only the historical
  ADR-0104/0106 bodies retain it).
- 100% coverage holds throughout (the sole new test is `skills-set-in-confighash`'s; every other change
  is a comment marker or doc edit).
- The changelog `[Unreleased]` entry (added by the mechanism plan) already describes this effort's
  user-facing behaviour and needs no change.

## Notes

- **Authorized escape valve — a named test that does not truly assert its slug.** If, on reading a
  proof-test function named in the tables, it does not actually exercise the slug's property, do not
  plant a hollow marker. Either (a) move the marker to a sibling test that does assert it, or (b) if no
  test does, treat the slug like `adr-new-no-overwrite` — reclassify its ADR declaration to
  `unbacked-invariant:` with a `Verify:` note and downgrade its markers to `touches-invariant:` —
  recording the decision here. The classification was analysed per-slug, but execution is the final
  check.
- **ADR data corrections.** ADR text discovered inaccurate under the new model is amended so the data
  is correct (Task 4.2 fixes ADR-0105's `context-read-only` example). ADR-0092's `context-read-only`
  bullet ("Backed by a test that snapshots the working tree …") stays accurate — the migration backs
  it via exactly that snapshot test. ADR-0105 and ADR-0106 are `Proposed` and freely amendable; edits
  to Implemented ADRs (0042 in Phase 1, 0092's marker relocation) are the mechanical migration ADR-0105
  authorises, mirroring the mechanism plan's `inv:`→`invariant:` corpus rewrite.
- **Classification decisions (both confirmed with the user).** The borderline single-source trio
  (`cli-command-spec-single-source`, `config-serialization-owned`, `commit-gate-shared-rule`) and
  `context-read-only` are all **backed** — each has a real asserting test; `unbacked-invariant:` is
  reserved for the genuinely untestable, of which awf has exactly one (`adr-new-no-overwrite`, an
  unreachable defensive guard).
- **Out of scope.** No adopter migration is emitted (the two-front `inv:`→`invariant:` + marker
  relocation break is documented in the mechanism plan's ADRs as un-auto-migratable); `examples/sundial`
  keeps no `testGlobs`, so its source-only fallback is unchanged.
- **Resync.** This plan links `Proposed` ADRs 0105/0106; `awf-reviewing-plan` hands off to
  `awf-reviewing-plan-resync` to reconcile the plan against the finalised ADRs (in particular the
  Task 4.2 example amendment) before implementation.
