---
date: 2026-08-01
adrs:
  - sanction-the-seal-crossing-integration-transition
status: Proposed
---
# Plan: Intrinsic ADR Formats and Stale-Merge Authorization

## Goal

Implement [ADR-sanction-the-seal-crossing-integration-transition](../decisions/sanction-the-seal-crossing-integration-transition.md): route ADRs by their authored format, retire permanent cutoff and legacy-gap authority, and authorize exact incoming-parent stale ADRs through shared `commit-msg` and audit validation.

This plan does not add plan versions, retrofit an ADR to a newer format, add an allowance ledger or preparation command, or begin implementation before this plan and the linked Proposed ADR complete review.

## Architecture summary

`internal/adr` becomes the single owner of format activation, intrinsic parsing, and current-format scaffolding. `internal/manifest`, `internal/migrate`, `internal/project`, and `internal/upgrade` remove permanent routing authority while retaining the frozen bridge-attestation input long enough to verify and consume it. `internal/currentstate` owns one parent-qualification model built on its existing retained-slug, unique-slugless-digest, and number pairing rules. A new leaf `internal/commitmsg` package owns Git cleaning and authorization trailers; `cmd/awf` and `internal/audit` orchestrate live and historical snapshots through that shared parser and the shared qualification policy. Each implementation transaction applies the linked ADR's State changes in declaration order, updates authoritative `.awf/` prose, and renders generated outputs.

## File structure

- **Created:** `internal/commitmsg/commitmsg.go`, `internal/commitmsg/commitmsg_test.go`, `internal/currentstate/qualification.go`, `internal/currentstate/qualification_test.go`, `internal/migrate/intrinsicadrformat.go`, and `internal/migrate/intrinsicadrformat_test.go`.
- **Modified:** `internal/adr/{adr.go,adr_test.go,corpus.go,corpus_test.go,format.go,format_test.go,pending_test.go}`; `internal/currentstate/{check_test.go,load.go,load_test.go,transition.go,transition_test.go,renumber_test.go,aggregate_test.go}`; `internal/manifest/{manifest.go,manifest_test.go}`; `internal/migrate/{migrate.go,migrate_test.go,workflowtelemetry_test.go}`; `internal/project/{project.go,project_test.go,currentstate.go,currentstate_test.go,staged_test.go,mergeaggregate_test.go,configreference.go,configreference_test.go,topics_test.go}`; `internal/upgrade/{upgrade.go,upgrade_test.go,journal.go,journal_test.go}`; `internal/git/{walk.go,walk_test.go,handle.go,git_test.go,entrypoints_test.go,merge.go,merge_test.go}`; `internal/snapshot/{commit.go,commit_test.go,range.go,range_test.go}`; `internal/audit/{audit.go,audit_test.go,git_context_test.go}`; `cmd/awf/{commitgate.go,commitgate_test.go,check.go,check_test.go,checkgroup_test.go,run_test.go,failure_paths_test.go,init.go,topic_test.go,context_test.go,upgrade_test.go}`; bridge fixtures in `internal/contextq/render_test.go`; `templates/adr-template/template.md.tmpl`, `templates/adr-readme/README.md.tmpl`, `templates/hooks/{commit-msg.sh.tmpl,pre-merge-commit.sh.tmpl}`, and affected documentation templates; `.awf/awf.lock`, `.awf/topics/parts/{adr-system/adr-lifecycle,config/migrations-and-locks,invariants/current-state-authority,tooling/upgrade-runtime,tooling/audit-and-snapshots}/current-state.md`, `.awf/domains/parts/{adr-system,config,invariants,tooling}/current-state.md`, `.awf/parts/workflow/local-hooks.md`, `.awf/parts/working-with-awf/{commands,overview}.md`, `.awf/docs/parts/{architecture,pitfalls,releasing}/**`, `.awf/docs/parts/roadmap/deferred.md`, `.awf/docs/pitfalls.yaml`, `README.md`, and the linked ADR and plan lifecycle fields; rendered `.awf/hooks/*`, `docs/{architecture,config-reference,pitfalls,releasing,roadmap,workflow,working-with-awf}.md`, `docs/domains/*.md`, `docs/topics/**/*.md`, `docs/decisions/INDEX.md`, and example-adopter outputs selected by `./x render`.
- **Deleted:** `internal/migrate/adrformatv2.go`, `internal/migrate/adrformatv2_test.go`, `internal/migrate/adrformatv3.go`, and `internal/migrate/adrformatv3_test.go` after their historical activation generations move into the registry and their cutoff-writing behavior is replaced by the generation-31 migration. No historical ADR or bridge-attestation field is deleted.

## Literal claim mutations

The executor lands these blocks verbatim in the phase named below. Removed claims disappear whole. `ADR-sanction-the-seal-crossing-integration-transition` is the pending record's exact provenance token until integration numbers it; governed numbering may substitute the assigned number mechanically.

**Phase 1 add `intrinsic-format-routing`:**

```markdown
### `invariant: intrinsic-format-routing`

A numbered ADR is parsed by its authored format marker, independent of its number: `current-state-v1`, `current-state-v2`, and `current-state-v3` select their matching frozen parser, while marker absence selects the legacy parser. An unknown, duplicate, empty, or malformed marker is refused rather than treated as legacy. A pending ADR is valid only in the running binary's current authoring format, and `awf new adr` emits that format from the activation registry.
Origin: ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 1 update `adr-amendable-until-terminal`:**

```markdown
### `invariant: adr-amendable-until-terminal`

A current-state-v2 or current-state-v3 ADR has digest-covered content that is amendable while Proposed, Accepted, or Implementing and freezes permanently at a terminal status, independent of its assigned number. Post-Accepted amendment is recorded as a stamp chain: only an Amended event introduces a new digest, which must differ from the preceding stamp; a status event repeats the preceding stamp or establishes the first; the latest stamp must equal the computed content digest; and an amendment never alters or removes an operation already referenced by an Applied event.
Origin: ADR-0188
Revised-by: ADR-0202, ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 1 update `adr-status-enum-and-matrix`:**

```markdown
### `invariant: adr-status-enum-and-matrix`

Every governed ADR is routed by its intrinsic declared format: V1 retains its four statuses and five legal edges, while V2 and V3 recognize Proposed, Accepted, Implementing, Implemented, and Abandoned, recognize status, Applied, and Amended history events, and accept only the format-specific status, history-event, digest-chain, and application-cardinality transitions. A numberless record is valid only when it declares the running binary's current authoring format and satisfies that format's pending-identity rules.
Origin: ADR-0135
Revised-by: ADR-0143, ADR-0188, ADR-0202, ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 2 update `initial-adoption-version-immutable`:**

```markdown
### `invariant: initial-adoption-version-immutable`

The first-adoption binary version is sealed once and preserved unchanged by ordinary sync, zero-migration upgrade, staged authority checks, and forced initialization. ADR format is authored in each record and no cutoff or legacy-gap set forms permanent lock authority.
Origin: ADR-0139
Revised-by: ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 2 add `bridge-attestation-cutoff-payload-discarded`:**

```markdown
### `invariant: bridge-attestation-cutoff-payload-discarded`

Final upgrade continues to parse and verify a resident version-1 bridge attestation and its approval artifact, including the historical `adrFormatV1From` and `legacyADRGaps` payload, but the journaled cutover discards those routing values instead of promoting them. Approval deletion and final lock replacement retain the existing atomic commit point, and no ADR byte is rewritten.
Origin: ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 3 update `merge-transition-ordered-aggregate`:**

```markdown
### `invariant: merge-transition-ordered-aggregate`

A merge transition is validated as an ordered aggregate rather than one authoring step: several application batches are legal in ascending ADR-number and intra-ADR history order, a claim's operations across the pair must form a legal ordered chain of at most one leading add, any number of updates, at most one remove, and after the remove any number of dominated updates, and an appended Status history must preserve the prior history as an exact prefix. A non-merge transition keeps the stricter per-step contract of one new batch per ADR, one operation per claim, and the fixed status-event shape. A newly introduced ADR in an older intrinsic format is provisional at the staged boundary that lacks merge-parent and message evidence; every other derivable transition check remains blocking, and definitive admission requires exact incoming-parent qualification at commit-msg.
Origin: ADR-0182
Revised-by: ADR-0191, ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 4 add `older-format-incoming-parent-sanction`:**

```markdown
### `invariant: older-format-incoming-parent-sanction`

An ordinary commit may introduce only the running binary's current ADR format. A real merge may introduce an older-format result ADR only when an incoming parent carries the paired record in that same format, the result differs only by the deterministic numbering or renumbering substitutions sanctioned for that format, and the cleaned final commit-message trailer block carries an adjacent complete `AWF-Allow-Version` and nonempty `AWF-Allow-Reason` pair for that format. `commit-msg` is definitive, a refusal preserves the staged merge for retry, redundant complete pairs are harmless, and malformed reserved trailers refuse.
Origin: ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

**Phase 6 add `stale-merge-trailer-replay`:**

```markdown
### `invariant: stale-merge-trailer-replay`

For a merge whose result tree is at or after the intrinsic-format schema generation, `awf audit` derives the merge-time current authoring format from the shared activation registry and replays the shared cleaned-message parser and incoming-parent qualification against the result, first parent, and every incoming parent. It reports an Error for malformed reserved trailers or an older-format import lacking its complete authorization pair, while pre-epoch merges, historical non-merges, valid or redundant pairs, and true fast-forwards produce no stale-merge authorization finding.
Origin: ADR-sanction-the-seal-crossing-integration-transition
Backing: test
```

## Phase 1: Make ADR format intrinsic and centralize activation

**Execution mode: inline.** This phase is one independently green transaction and applies State changes 1 through 4 in their declared order.

- [ ] **Task 1.1: Write the intrinsic-routing regression first.** In `internal/adr/format_test.go`, add `TestParseRecordRoutesByIntrinsicFormat` with the exact proof marker `// invariant: adr-system/adr-lifecycle:intrinsic-format-routing (TestParseRecordRoutesByIntrinsicFormat)`. Build numbered records at numbers on both sides of every former cutoff and assert that markerless bytes use `Legacy`, exact `current-state-v1`, `current-state-v2`, and `current-state-v3` markers use their matching parsers independent of number, and pending V3 remains valid. Assert refusals for unknown, duplicate, malformed, or empty `format:` keys and for markerless pending, V1 pending, and V2 pending files. Run `go test ./internal/adr -run TestParseRecordRoutesByIntrinsicFormat`; it must fail against cutoff routing before production edits and pass afterward.
- [ ] **Task 1.2: Add the activation registry and intrinsic parser.** In `internal/adr/format.go` and `internal/adr/adr.go`, replace `FormatBoundaries` and `ParseRecord(name, data, boundaries)` with `ParseRecord(name, data)` that parses the frontmatter marker before choosing a parser. Marker absence is the only legacy route; YAML/frontmatter parse failure, a duplicate key, an unknown marker, or a malformed governed marker must return an error and never fall back. Add one ordered package-private registry whose entries contain `Format`, exact marker, and activation generation: V1 at generation 14, V2 at 15, V3 at 29. In this phase expose only `CurrentFormat() Format`, `CurrentFormatMarker() string`, and `KnownFormatMarker(string) bool`, all consumed by parsing or scaffolding; defer the generation-indexed query to Phase 5. Keep V3 as current. Remove `AdoptionBoundary`; retain identity validation through the existing `loadIdentityCorpus` caller in scaffolding without computing gaps or a cutoff. Update `internal/adr/corpus.go` so every full-body load calls intrinsic `ParseRecord` with no authority parameter.
- [ ] **Task 1.3: Make scaffolding registry-driven.** Change `internal/adr.NewFile` so callers no longer pass a freely chosen `Format`; numbered and pending scaffolds both use the registry's current format, with the current V3 slug and heading rules. Update `internal/project/project.go` and `cmd/awf/init.go` to stop deriving a scaffold format or brownfield boundary from `.awf/awf.lock`; initialization still validates duplicate ADR identities and existing authored formats but does not rewrite them. Update `templates/adr-template/template.md.tmpl` and render-data construction so the exact current marker comes from the registry-backed project data and missingkey=zero or empty data still renders coherent generic prose with no `<no value>` or unresolved token. Update golden and scaffold tests in `internal/adr/adr_test.go`, `internal/adr/pending_test.go`, and `internal/project/{project_test.go,golden_test.go}`; prove `awf new adr` cannot request V1/V2 and continues to emit V3 numbered or pending identity according to branch state.
- [ ] **Task 1.4: Remove boundary arguments and cutoff-era parser expectations.** Mechanically update every `ParseRecord`, corpus-load, and current-state-load call in `internal/adr`, `internal/currentstate`, `internal/project`, `internal/audit`, and their tests. Replace assertions about a record being above or below a number cutoff with intrinsic-marker assertions. Run `rg -n 'FormatBoundaries|AdoptionBoundary' internal cmd` and require no output; compilation and `go test ./...` are the deterministic check that every `ParseRecord` caller uses the new two-argument signature.
- [ ] **Task 1.5: Apply the first claim batch and current documentation.** Change the linked ADR from `Proposed` to `Implementing`; append the required Implementing status event and one Applied event naming operations 1-4 in declaration order, using the ADR's computed canonical content stamp. In `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`, remove `fresh-adoption-v1-cutoff` and land verbatim the Phase 1 add and two update blocks from Literal claim mutations. Update `.awf/domains/parts/adr-system/current-state.md`, ADR authoring guidance, `templates/adr-readme/README.md.tmpl`, and exact affected source parts found by `rg -n 'cutoff|format marker|current-state-v3' .awf templates README.md`; do not edit generated topic/domain docs directly. Run `./x render && ./x check`; both must finish clean, and `docs/decisions/INDEX.md` must show the ADR as Implementing.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `gofmt` on changed Go files, `go test ./internal/adr ./internal/currentstate ./internal/project ./cmd/awf`, `git diff --check`, `./x render`, and `./x check`; each must succeed. Stage only this transaction, require `./awf check --staged` and `./x gate` to succeed, then commit:

```commit
refactor(adr-system): route ADRs by authored format
```

## Phase 2: Retire permanent routing authority and consume the bridge safely

**Execution mode: inline.** This phase removes the permanent lock state only after Phase 1 can load every ADR intrinsically; it applies State changes 5 through 8 in order and closes with resident bridge recovery green.

- [ ] **Task 2.1: Write migration and bridge-discard regressions first.** Add `internal/migrate/intrinsicadrformat_test.go` for schema 30 to 31 and `internal/upgrade/upgrade_test.go:TestFinalUpgradeDiscardsBridgeADRRoutingPayload` with `// invariant: tooling/upgrade-runtime:bridge-attestation-cutoff-payload-discarded (TestFinalUpgradeDiscardsBridgeADRRoutingPayload)`. The migration fixture must start with all four permanent routing JSON keys, run generation 31, assert those keys are absent, schema is 31, `initializedWithVersion` and unrelated manifest entries are byte-semantically preserved, and every ADR blob digest is unchanged. The final-upgrade test must use a version-1 bridge attestation carrying `adrFormatV1From` and `legacyADRGaps`, verify its approval and tree digest, complete the journal, delete the approval, replace the lock without permanent routing keys, preserve `initializedWithVersion` behavior, and leave ADR bytes unchanged. Add failure injections before and after lock replacement and assert existing rollback/postcommit recovery semantics. Run the two named tests and require them to fail before production edits.
- [ ] **Task 2.2: Remove permanent manifest authority but retain the frozen bridge shape.** In `internal/manifest/manifest.go`, remove `Lock.ADRFormatV1From`, `ADRFormatV2From`, `ADRFormatV3From`, `LegacyADRGaps`, presence bookkeeping, ordering/gap validation, and JSON emission. Preserve `BridgeAttestation.ADRFormatV1From` and `BridgeAttestation.LegacyADRGaps` with their exact historical JSON spellings and version-1 validation. Make strict parsing reject permanent retired keys at current schema, while the generation-31 migration parses old lock bytes through a migration-local compatibility struct and writes the current `manifest.Lock`. Update `internal/manifest/manifest_test.go` to distinguish rejected current permanent keys from accepted bridge payload.
- [ ] **Task 2.3: Replace cutoff migrations with the intrinsic generation.** Register generation 31 as `intrinsic-adr-format` in `internal/migrate/migrate.go`, with `OwnsSchemaStamp: true`. Move the activation facts for generations 15 and 29 into `internal/adr` and delete the V2/V3 cutoff migration files after no registry entry calls them. Generation 31 removes only the four permanent lock keys and stamps schema 31 without reading or rewriting ADRs. Update minimum-version/schema tests and `ConfigForCurrentSchema` fixtures; the existing binary-version gate must make an older binary refuse a generation-31 lock. Verify `rg -n 'adr-format-v[23]-cutoff|ADRFormatV[123]From|LegacyADRGaps' internal/migrate internal/manifest` returns only migration compatibility fixtures and bridge-attestation fields explicitly allowed by this task.
- [ ] **Task 2.4: Remove cutoff production flow end to end.** In `internal/project/project.go`, stop computing, preserving, rendering, syncing, or selecting by permanent boundaries. In `internal/project/currentstate.go`, replace `validatePermanentLockTransition` with validation of remaining immutable `initializedWithVersion` plus the established schema/bridge transition; delete `attestationBoundaries`, `nextADRIdentityFromTree`, V2/V3 sealing, inherited cutoff, and resealing edges. In `internal/currentstate/load.go`, remove boundary and gap parameters and legacy gap/contiguity routing checks. In `internal/upgrade/upgrade.go`, verify the resident attestation and approval exactly as before but omit its cutoff/gap payload from `cutoverOperations`; preserve the journal commit point and approval deletion. Update `cmd/awf` and all listed fixtures so no current lock literal contains retired fields. A repository-wide `rg -n 'ADRFormatV[123]From|LegacyADRGaps|adrFormatV[123]From|legacyAdrGaps' --glob '*.go'` must return only the frozen `BridgeAttestation`, generation-31 compatibility decoder, and tests explicitly proving those inputs; `.awf/awf.lock` must contain none of the retired keys after render.
- [ ] **Task 2.5: Apply the lock and upgrade claim batch and documentation.** Append one Applied event for operations 5-8. Remove `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable` and `tooling/upgrade-runtime:legacy-format-set-is-closed`, then land verbatim the Phase 2 update and add blocks from Literal claim mutations. Update `.awf/topics/parts/config/migrations-and-locks/current-state.md`, `.awf/topics/parts/tooling/upgrade-runtime/current-state.md`, their domain narratives, `.awf/docs/parts/architecture/**`, `.awf/docs/parts/releasing/content.md`, `.awf/docs/pitfalls.yaml`, `.awf/parts/working-with-awf/*.md`, `README.md`, and config-reference inputs wherever they state cutoff, gap, migration, first-adoption, or bridge behavior. Keep historical research and implemented ADR bytes unchanged. Run `./x render && ./x check` and require clean drift and no stale current-state cutoff claim.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `go test ./internal/manifest ./internal/migrate ./internal/upgrade ./internal/currentstate ./internal/project ./cmd/awf`, `git diff --check`, `./x render`, and `./x check`; all must succeed. Stage only the phase, require `./awf check --staged` and `./x gate`, then commit:

```commit
refactor(config): retire ADR cutoff authority
```

## Phase 3: Remove the incident relaxations and identify provisional imports

**Execution mode: inline.** This phase restores ordinary transition rules, gives staged checking a production-consumed provisional result, and applies State change 9 without landing an unused parent-qualification entrypoint.

- [ ] **Task 3.1: Write relaxation-removal and provisional-introduction tests first.** In `internal/currentstate/transition_test.go`, invert the former `isRenumberRetrofit` admission to a refusal; in `renumber_test.go`, assert unique digest pairing considers only slugless records on both ends and refuses an after-side newly slugged record and ambiguous digests. Add tests for `OlderIntroductions(before, after Universe, current adr.Format) []Introduction`: current-format additions produce none, existing older records undergoing their legal lifecycle produce none, and a newly introduced legacy/V1/V2 record produces an `Introduction{Identity, Format}`. Add `internal/project/staged_test.go` cases proving such an introduction is provisional while every unrelated static, lifecycle, claim-handshake, coverage, and aggregate violation remains blocking. Run the named focused tests and require failure before production edits.
- [ ] **Task 3.2: Restore the declared ordinary pairing and transition rules.** In `internal/currentstate/transition.go`, delete `isRenumberRetrofit`, require paired governed records to keep the same format, and restore `uniqueSluglessDigests` so only a slugless before and slugless after record can form a unique canonical-content-digest pair. Keep retained slug first and number last. Do not add a format retrofit, newly-slugged after-side digest path, deletion exception, or caller-specific pairing implementation.
- [ ] **Task 3.3: Add the exact provisional model and consume it.** In `internal/currentstate/transition.go`, define `type Introduction struct { Identity string; Format adr.Format }` and `OlderIntroductions(before, after Universe, current adr.Format) []Introduction`. It must use the existing pairing resolver, include only result records with no before-side pair whose format is older than `current`, and sort by identity. Extend `CurrentStateReport` in `internal/project/currentstate.go` with `Provisional []currentstate.Introduction`; `CheckStaged` populates it from HEAD/index after ordinary `CheckPair`, and the staged report renderer treats it as informational rather than a finding. This is the only suppressed condition; no `CheckPair` finding changes rank or disappears.
- [ ] **Task 3.4: Apply the aggregate claim update and document the boundary.** Append one Applied event for operation 9 and land the exact `merge-transition-ordered-aggregate` block in the Literal claim mutations section. Update the invariants domain narrative and architecture source for staged provisional introductions and the future first-parent/result/incoming-parent evidence boundary. Render and require `./x check` clean.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `go test ./internal/adr ./internal/currentstate ./internal/project -run 'Test.*Intrinsic|Test.*Pair|Test.*Renumber|Test.*Aggregate|Test.*Staged|Test.*Qualif'`, then the full tests for those packages, `git diff --check`, `./x render`, and `./x check`. Stage only this phase, require `./awf check --staged` and `./x gate`, then commit:

```commit
refactor(invariants): mark stale ADR imports provisional
```

## Phase 4: Enforce stale-merge authorization at commit-msg

**Execution mode: inline.** This phase adds the definitive live authorization boundary and applies State change 10.

- [ ] **Task 4.1: Specify Git cleaning and trailers with failing tests.** Create `internal/commitmsg/commitmsg_test.go`. Assert CRLF normalization; removal of lines whose first nonblank character is `#`; stop at a scissors line; trailing blank removal; first surviving nonblank subject; and preservation of other body bytes. Assert the final trailer block is the maximal nonempty suffix after a blank separator whose every line is unindented `Key: value`. Parse only adjacent exact-case pairs `AWF-Allow-Version: <value>` then `AWF-Allow-Reason: <value>`, trim ASCII whitespace from values, accept known governed markers or literal `legacy`, allow surrounding ordinary trailers, duplicate complete pairs, and redundant complete pairs. Refuse reserved-key body lookalikes, unknown reserved keys, unknown versions, continuation lines, orphan/reversed/interleaved pairs, empty reasons, and malformed final blocks. These tests must fail before the new package exists.
- [ ] **Task 4.2: Move message policy into `internal/commitmsg`.** Define `type Message struct { Text string; Subject string }`, `type Authorization struct { Version string; Reason string }`, `func Clean(raw []byte) Message`, and `func ParseAuthorizations(msg Message, validVersion func(string) bool) ([]Authorization, error)`. The callback supplied by callers returns true only for `legacy` or `adr.KnownFormatMarker(value)`. Return a typed `*SyntaxError` carrying the offending cleaned line and reason. Move `cleanCommitLines`, `cleanCommitSubject`, and `cleanCommitBody` out of `cmd/awf/commitgate.go`; Conventional Commit and memory-citation callers consume `Message`. The leaf package imports none of `internal/adr`, `cmd/awf`, `internal/project`, `internal/audit`, or `internal/currentstate`.
- [ ] **Task 4.3: Add exact Git/snapshot evidence seams.** Add `git.MergeHeads(projectRoot string) ([]string, error)`, returning nil when `MERGE_HEAD` is absent and all trimmed hashes in file order when present; it resolves the worktree-private Git dir through `containingGitDir`. Add `(*git.Repo).CommitParents(ctx, rev) ([]string, error)` and `(*git.Repo).CommitMessage(ctx, rev) (string, error)`, preserving full hashes/message and context cancellation, and register both object-read contracts in `internal/git/entrypoints_test.go`. Add `snapshot.CommitTrees(ctx, repo, revs) ([]*Tree, error)` as an ordered wrapper over `CommitTree`; the caller uses existing `IndexTree`, `CommitTree("HEAD")`, `MergeHeads`, and `CommitTrees` for result, first parent, and incoming parents. Test linked worktrees, absent/multiple MERGE_HEAD lines, missing objects, rerooting, and cancellation.
- [ ] **Task 4.4: Add exact byte qualification and make `check commit` definitive.** Extend `currentstate.Loaded` and `Universe` with `Sources map[string][]byte`, keyed by parsed ADR identity and populated with cloned snapshot bytes in `load.go`; this is operation-local evidence, not package/global state. Create `internal/currentstate/qualification.go` with `type Qualification struct { Introduction Introduction; Qualified bool }` and `func QualifyIncoming(first, result Universe, incoming []Universe, current adr.Format) []Qualification`. Reuse `newPairing`; compare source bytes after applying only deterministic number/filename/heading and, for governed records, `Origin:`/`Revised-by:` substitutions. Legacy receives no provenance substitution. Reject every other byte change, and sort by identity. In `internal/project/currentstate.go`, define `type CommitAuthorizationResult struct { Condition string; ChangedIndex bool; NextActions []string }` and `CheckCommitAuthorization(ctx, msg commitmsg.Message) (CommitAuthorizationResult, error)`. It loads the exact snapshots through Task 4.3, parses trailers, calls `QualifyIncoming`, and returns `ChangedIndex: false`; a deficiency uses `NextActions: []string{"correct the message trailers", "run git commit to finish the existing merge"}`. `cmd/awf/commitgate.go` renders that result after existing memory and Conventional Commit checks. No provisional introduction plus valid redundant pairs succeeds; malformed reserved syntax always refuses; provisional non-merge always refuses; a merge requires parent qualification and every required version. No path writes the index, message, or `MERGE_HEAD`.
- [ ] **Task 4.5: Add the required end-to-end proof.** In `cmd/awf/commitgate_test.go`, add `TestCheckCommitAuthorizesOlderFormatIncomingParent` with `// invariant: adr-system/adr-lifecycle:older-format-incoming-parent-sanction (TestCheckCommitAuthorizesOlderFormatIncomingParent)`. Use real Git fixtures to prove admission and refusal halves: qualifying V2 and legacy imports with pairs; absent, malformed, or wrong-version pairs; evil-merge edits; non-merge provisional introduction; redundant valid pairs; octopus qualification; and recoverability after refusal with staged bytes and `MERGE_HEAD` unchanged. Keep `templates/hooks/commit-msg.sh.tmpl` a thin `awf check commit "$1"` delegate and keep `pre-merge-commit` limited to evidence available before the message/parents exist. Update hook golden tests.
- [ ] **Task 4.6: Apply the authorization claim and user guidance.** Append one Applied event for operation 10 and land verbatim the Phase 4 `older-format-incoming-parent-sanction` block from Literal claim mutations. Update `.awf/parts/workflow/local-hooks.md`, `.awf/parts/working-with-awf/{commands,overview}.md`, `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`, `.awf/topics/parts/rendering/singletons-and-payloads/current-state.md` if its hook contract wording changes, architecture/pitfall sources, `README.md`, hook templates, CLI help/spec text, and adopter-facing examples. Document the recoverable unstamped conflict-free merge flow, optional proactive `git merge --no-commit --no-ff`, exact trailer pair, malformed reserved syntax, true fast-forward non-event, and prohibition on ADR retrofit or allowance state. Render; `./x check` and the example adopter check must be clean.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `go test ./internal/commitmsg ./internal/git ./internal/snapshot ./internal/currentstate ./internal/project ./cmd/awf -run 'Test.*Commit|Test.*Merge|Test.*Authoriz|Test.*Trailer|Test.*Message'`, then full tests for those packages, `git diff --check`, `./x render`, and `./x check`. Stage only this phase, require `./awf check --staged` and `./x gate`, then commit:

```commit
feat(tooling): authorize stale ADR merges at commit-msg
```

## Phase 5: Replay stale-merge authorization in audit

**Execution mode: inline.** This phase reuses the live parser and qualification policy for history. State change 11 deliberately remains unapplied until the deferred post-review transaction.

- [ ] **Task 5.1: Write historical replay regressions first.** Add `internal/audit/audit_test.go:TestAuditReplaysStaleMergeTrailers` without its invariant proof marker yet. Build committed merge fixtures for valid, missing, malformed, wrong-version, duplicate, and redundant pairs; exact V1/V2/legacy incoming records; evil-merge mutation; octopus qualification; a pre-generation-31 merge; a generation-31 non-merge; and a true fast-forward range. Assert errors only for applicable malformed authorization or unauthorised qualifying imports, and no finding for pre-epoch, non-merge, valid, redundant, or fast-forward cases. Require the test to fail before audit orchestration is added.
- [ ] **Task 5.2: Extend committed evidence without changing first-parent semantics.** Extend `internal/git.Commit` with `Revision string` (full object hash), `Message string` (full raw committed message), and `Parents []string` (full hashes in Git order), while retaining the existing short `Hash`, `Subject`, `Body`, `IsMerge`, and `Changes` fields. Populate those fields in `toCommit`; preserve `RangeCommits` ordering, rerooting, and non-merge diff calculation, and do not load trees for rules that do not ask. Audit loads the result through `snapshot.CommitTree(repo, c.Revision)` and parents through `snapshot.CommitTrees(repo, c.Parents)`. Keep `snapshot.RangePair` first-parent behavior unchanged. Cover root, one-parent, two-parent, octopus, subdirectory, shallow/missing-parent, and cancellation paths.
- [ ] **Task 5.3: Reuse the shared authorization validator in audit.** Add `adr.FormatAtGeneration(generation int) (Format, bool)` in `internal/adr/format.go`; it selects the last registry entry whose activation is at or below the generation and is consumed immediately by audit. In `internal/audit/audit.go`, add `replayStaleMergeAuthorizations(ctx, repo, commits, in)` called from `Run` after pure `evaluate`. For each merge, read the result lock and skip absent locks or `schemaVersion < 31`; otherwise derive the current format through `FormatAtGeneration`, load result/first-parent/all-incoming universes, parse `c.Message` with `commitmsg`, and call `currentstate.QualifyIncoming`. Emit Error findings for malformed reserved trailers or each required format without a complete pair. Pre-epoch merges and historical non-merges stay outside the rule; a fast-forward has no commit to inspect.
- [ ] **Task 5.4: Complete audit documentation without applying operation 11.** Update audit/snapshot topic narrative without adding the pending claim, the tooling-domain source, architecture data flow, workflow and working-with-awf audit guidance, release/pitfall material, CLI/help text, and any current roadmap item that describes the now-shipped cutoff workaround; preserve historical incident records. Keep ADR status `Implementing`, keep operation 11 Remaining, and do not add its proof marker. Run the repository-wide searches `rg -n 'adrFormatV[123]From|legacyAdrGaps|FormatBoundaries|isRenumberRetrofit' --glob '!docs/decisions/**' --glob '!docs/plans/**'` and `rg -n 'AWF-Allow-Version|AWF-Allow-Reason' internal cmd templates .awf README.md`; the first may show only frozen bridge compatibility and intentional migration vocabulary, while the second must show one shared syntax owner plus callers, tests, and rendered guidance. Run `./x render && ./x check` and require clean output.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `go test ./internal/commitmsg ./internal/git ./internal/snapshot ./internal/audit ./internal/project ./internal/currentstate`, `git diff --check`, `./x render`, and `./x check`; all must succeed. Stage only this phase, require `./awf check --staged` and `./x gate`, then commit:

```commit
feat(tooling): replay stale ADR merge authorization
```

## Phase 6: Apply the final claim and freeze the reviewed records

**Execution mode: inline.** This deferred transaction runs in the integrated primary checkout only after terminal implementation review, managed-worktree integration, any renewed review required by a divergent merge, and managed-worktree removal have settled. It applies State change 11 and freezes the ADR and plan together.

- [ ] **Task 6.1: Settle review and integration before freezing.** Run the independent implementation-review workflow over Phases 1-5. Resolve findings in new green commits and repeat review when behavior changes. Integrate the managed worktree through the governed effort workflow; if integration creates a divergent merge, run the required renewed review over the integrated result. Remove the managed worktree only after terminal review. Stop for any user-decision finding. The ADR remains `Implementing` and operation 11 remains Remaining through this boundary.
- [ ] **Task 6.2: Apply the last operation and both terminal states.** In the integrated primary checkout, add the exact `stale-merge-trailer-replay` claim block from Literal claim mutations to `.awf/topics/parts/tooling/audit-and-snapshots/current-state.md` and add `// invariant: tooling/audit-and-snapshots:stale-merge-trailer-replay (TestAuditReplaysStaleMergeTrailers)` immediately above the already passing named test. In the linked ADR, append the final Applied event for operation 11 and then the `Implemented` status event with the current canonical content stamp. Change this plan's `status:` to `Implemented`, record actual deviations under Notes without changing the ADR's meaning, and run `./x render`; `docs/decisions/INDEX.md` must place the ADR in history and no operation may remain.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `go test ./internal/audit -run TestAuditReplaysStaleMergeTrailers`, `git diff --check`, `./x render`, and `./x check`; stage only this claim/lifecycle transaction, require `./awf check --staged` and `./x gate`, then commit:

```commit
feat(tooling): govern stale merge replay
```

## Verification

- `go test ./...` and `./x gate` finish successfully with 100% statement coverage, dead-code, lint, cross-compile, generated Pi, and supply-chain checks green.
- `./x render && ./x check` is clean, `git diff --exit-code` is clean after commits, and the example adopter reports no notes.
- `./awf check --staged` accepts every implementation batch only when the linked Applied event and exact claim mutation are staged together.
- Mixed numbered legacy, V1, V2, and V3 ADRs parse by their own markers at arbitrary numbers; `awf new adr` emits only the registry current format.
- Current locks and generation-31 migration output contain no permanent ADR cutoff/gap keys; a resident version-1 bridge attestation remains consumable and its routing payload is discarded without changing ADR bytes.
- An older-format introduction cannot land in an ordinary commit. A real merge can import only an exact qualifying incoming-parent ADR and only with the complete required trailer pair; refusal preserves the staged merge for `git commit` retry.
- Audit applies the same parser and qualification policy from generation 31, reports invalid applicable merges, and ignores pre-epoch history, non-merges, and true fast-forwards.
- `rg -n 'FormatBoundaries|isRenumberRetrofit' internal cmd` returns no output, and production `AWF-Allow-` syntax is defined only in `internal/commitmsg` while command and audit code consume it.

## Notes

- The historical version-1 bridge-attestation fields remain deliberately readable compatibility input. Their presence is not permanent lock authority and must not be removed by a broad symbol sweep.
- Plans remain unversioned and receive no stale-merge authorization path.
- The plan and ADR stay Proposed throughout plan review. Implementation begins only after the user authorizes execution; the first implementation transaction, not this plan commit, moves the ADR to Implementing.
