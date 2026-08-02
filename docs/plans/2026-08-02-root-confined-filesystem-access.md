---
date: 2026-08-02
adrs:
  - test-support-exports-earn-test-consumers
  - consumer-local-contracts-over-single-home-filesystem-access
status: Proposed
---
# Plan: Root-confined filesystem access

## Goal

Implement [ADR-test-support-exports-earn-test-consumers](../decisions/test-support-exports-earn-test-consumers.md)
and [ADR-consumer-local-contracts-over-single-home-filesystem-access](../decisions/consumer-local-contracts-over-single-home-filesystem-access.md): correct production-versus-test consumer authority, add one `os.Root`-backed production filesystem handle and one kernel-backed fault source, and convert upgrade attestation digest traversal away from direct OS calls and its mutable `lstat` seam.

Non-goals: converting project sync, migration, snapshot capture, upgrade journal mutation, or any clock, environment, working-directory, subprocess, and unrelated filesystem effect.

## Architecture summary

Four implementation phases keep every commit green while respecting both ADRs' lifecycle order. Phase 1 accepts the prerequisite ADR. Phase 2 applies its three operations atomically, adds the one-way production-import proof, and implements it. Phase 3 accepts the filesystem ADR only after the prerequisite is terminal. Phase 4 lands the complete concrete-first vertical slice in one transaction: production handle, shared fault source, outside-package test consumer, upgrade-local structural contract and policy helpers, domain ownership, and architecture documentation. The filesystem ADR's five claim operations remain unapplied through implementation review, then land together with their proof markers in the deferred direct Accepted-to-Implemented transaction after terminal review settles; that transaction also freezes this plan.

Both records remain slug-identified in the managed branch. Before integration, run `awf adr number test-support-exports-earn-test-consumers consumer-local-contracts-over-single-home-filesystem-access` after merging the integration branch so the prerequisite receives the earlier number. Until then, status history and prose cite the stable ADR slugs.

## File structure

- **Created:** `internal/filesystem/handle.go`, `internal/filesystem/handle_test.go`, `internal/testsupport/fsfixture/fsfixture.go`, `internal/testsupport/fsfixture/fsfixture_test.go`, `internal/upgrade/digest_test.go`.
- **Modified:** `docs/decisions/test-support-exports-earn-test-consumers.md`, `docs/decisions/consumer-local-contracts-over-single-home-filesystem-access.md`, `internal/testsupport/deps_test.go`, `internal/upgrade/digest.go`, `internal/upgrade/upgrade.go`, `internal/upgrade/upgrade_test.go`, `.awf/domains/tooling.yaml`, `.awf/topics/parts/code-design/package-composition/current-state.md`, `.awf/topics/parts/code-design/dependency-composition/current-state.md`, `.awf/topics/parts/tooling/test-infrastructure/current-state.md`, `.awf/topics/parts/tooling/filesystem-access/current-state.md`, `.awf/docs/parts/architecture/components.md`, this plan at its deferred freeze, and rendered `.awf/awf.lock`, `docs/architecture.md`, `docs/decisions/INDEX.md`, `docs/domains/**`, and `docs/topics/**` outputs changed by those authored inputs.
- **Deleted:** none. Phase 4 deletes the `lstat` declaration and swap test, not a whole file.

## Phase 1: Accept the test-support consumer authority

**Execution mode: inline.** This is one docs-only green lifecycle transaction. It must precede acceptance of the filesystem ADR.

- [ ] **Task 1.1: Accept ADR-test-support-exports-earn-test-consumers.** In `docs/decisions/test-support-exports-earn-test-consumers.md`, change frontmatter `status: Proposed` to `status: Accepted` and append an `Accepted` Status history event dated on execution day with the record's computed `content-sha256`. Use the established digest workflow: temporarily place 64 zeros, run `./x check`, copy the reported digest exactly, replace the zeros, and rerun `./x check` until clean. Do not alter the Decision or State changes.
- [ ] **Task 1.2: Render and verify the acceptance.** Run `./x render`; stage the ADR, `docs/decisions/INDEX.md`, and `.awf/awf.lock` if changed. `git diff --check` produces no output, `./awf check staged` exits zero, and `./x gate` exits zero.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage only the complete acceptance transaction and create its one commit. The wired pre-commit hook must remain configured at `.githooks`; otherwise run the staged check and gate manually immediately before committing.

```commit
docs(adr): accept test-support export consumers
```

## Phase 2: Apply symmetric test-support authority and its one-way import proof

**Execution mode: inline.** This phase starts from Phase 1 committed with `git status --short` empty, `./x check` clean, and `./x gate` passing. It applies all three prerequisite operations in declaration order through one direct Accepted-to-Implemented transaction.

- [ ] **Task 2.1: Add a repository production-import proof.** In `internal/testsupport/deps_test.go`, retain `dependencyViolations` and `TestZeroInternalDeps` unchanged in purpose. Add a `productionTestsupportImportViolations(path string, source any) ([]string, error)` parser helper and a live walker anchored with `testsupport.RepoRoot(t)` from the external `testsupport_test` package. The walker covers every production `.go` source in this repository, including the root package, `cmd/**`, production packages under `internal/**`, `changelog/**`, `templates/**`, `tools/**`, and the separate `examples/sundial` module. It excludes the entire `internal/testsupport/**` source boundary, which is test-only and remains governed by `TestZeroInternalDeps`; `_test.go`; every directory named `testdata`, `vendor`, or `node_modules`; `.git/**`; and resident `.awf/efforts/**` and `.awf/worktrees/**` trees because none is outside-testsupport production source. Do not exclude another nested module merely because it has its own `go.mod`. Parsing failures are test failures, not ignored files.

  A violation occurs whenever an unquoted import equals `github.com/hypnotox/agentic-workflows/internal/testsupport` or starts with that string plus `/`. Add `TestProductionNeverImportsTestSupport` with the proof marker:

  ```go
  // invariant: tooling/test-infrastructure:production-never-imports-test-support (TestProductionNeverImportsTestSupport)
  ```

  The test reports every path/import pair and the live scan reaches zero findings. Table cases pass synthetic path/source pairs and prove rejection for the root testsupport package, `internal/testsupport/fsfixture`, and an example-adopter production file; allowance for a standard-library import, another repository import, `_test.go`, and a `testdata` fixture; and parse-error propagation. Forbidden: invoking `go list`, Git, or another repository package from production testsupport code; the proof itself may import the root testsupport package because it is an external test file.

- [ ] **Task 2.2: Update export eligibility exactly.** Replace the complete `export-earns-consumer` block in `.awf/topics/parts/code-design/package-composition/current-state.md` with this literal Markdown:

  ```markdown
  ### `invariant: export-earns-consumer`

  A new or deliberately converted exported symbol declared by a production package ships with an outside-package production consumer in the same green transaction; an `export_test.go` seam stays legal and a black-box `_test` package does not earn that production export. An exported symbol declared by a dedicated shared test-support package under `internal/testsupport/**` instead ships with an outside-package test consumer in the same green transaction, and a compile-only reference does not count. Composition capabilities remain governed by `code-design/dependency-composition:concrete-first-consumer`, and exported error identities remain governed by `code-design/outcome-modeling:consumed-identity` including its documented-consumer escape hatch.
  Origin: ADR-0200
  Revised-by: ADR-test-support-exports-earn-test-consumers
  Backing: unbacked
  Verify: For each new or deliberately converted export, classify its declaring package; trace an outside-package production consumer for a production-package export or a real outside-package test consumer for a dedicated `internal/testsupport/**` export in the same commit, then apply the named composition-capability or error-identity specialization where relevant.
  ```

- [ ] **Task 2.3: Update concrete-first symmetry exactly.** Replace the complete `concrete-first-consumer` block in `.awf/topics/parts/code-design/dependency-composition/current-state.md` with this literal Markdown:

  ```markdown
  ### `invariant: concrete-first-consumer`

  Every new production composition capability lands in the same green transaction as exactly one named concrete production first consumer. A composition capability exported by a dedicated shared test-support package under `internal/testsupport/**` instead lands with exactly one named outside-package test first consumer. In either case the consumer uses the whole introduced capability, and no adapter, constructor field, interface method, option, helper, fault operation, or other composition surface is added only for anticipated reuse.
  Origin: ADR-0178
  Revised-by: ADR-test-support-exports-earn-test-consumers
  Backing: unbacked
  Verify: For each newly exported or shared composition symbol, classify its declaring package, trace the corresponding production or outside-package test caller in the same commit, confirm exactly one named first consumer uses the whole introduced capability, and reject every introduced member without that consumer use.
  ```

- [ ] **Task 2.4: Add the one-way test-infrastructure claim.** Append this claim to `.awf/topics/parts/tooling/test-infrastructure/current-state.md` and replace the topic preamble only as needed to mention both directions of the boundary:

  ```markdown
  ### `invariant: production-never-imports-test-support`

  Non-test Go files outside `internal/testsupport/**` never import the root test-support package or any of its subpackages; shared test fixtures remain a test-only dependency in the direction from outside-package tests into test support.
  Origin: ADR-test-support-exports-earn-test-consumers
  Backing: test
  ```

  Place the named proof marker from Task 2.1 on the test. Do not add `Verify:` to the backed claim.

- [ ] **Task 2.5: Implement the prerequisite ADR atomically.** In `docs/decisions/test-support-exports-earn-test-consumers.md`, change frontmatter to `status: Implemented` and append only the terminal `Implemented` event, repeating the Accepted content stamp because the ADR body did not change. This direct Accepted-to-Implemented transition implicitly applies all three State changes in declaration order; do not append an explicit Applied event. Land all three claim mutations and the proof marker in this same checked transaction. Run `./x render` and stage the ADR, all three authored claim parts, `internal/testsupport/deps_test.go`, generated topic/domain/index outputs, and `.awf/awf.lock`.

- [ ] **Task 2.6: Verify authority and proof behavior.** Run `go test ./internal/testsupport`, which exits zero. In a scratch edit, add `github.com/hypnotox/agentic-workflows/internal/testsupport/fsfixture` to a non-test Go file under `internal/` and confirm `go test ./internal/testsupport` fails naming that file and import; restore with `git restore` and rerun until clean. Run `./awf topic code-design/package-composition`, `./awf topic code-design/dependency-composition`, and `./awf topic tooling/test-infrastructure`; each renders the updated or added claim with correct provenance and backing. `./x check` and `./x gate` both exit zero.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete code, authority, proof, lifecycle, and rendered-output transaction. Confirm `./awf check staged` and `./x gate` exit zero, then create the one closing commit.

```commit
refactor(code-design): align test-support consumer authority
```

## Phase 3: Accept the root-confined filesystem architecture

**Execution mode: inline.** This docs-only transaction starts only after ADR-test-support-exports-earn-test-consumers is `Implemented` and Phase 2 is committed.

- [ ] **Task 3.1: Accept ADR-consumer-local-contracts-over-single-home-filesystem-access.** In `docs/decisions/consumer-local-contracts-over-single-home-filesystem-access.md`, change frontmatter `status: Proposed` to `status: Accepted` and append an execution-day `Accepted` event with the computed content digest. Use the same zero-digest/check/copy/recheck workflow as Phase 1. Confirm the prerequisite ADR's latest status is already `Implemented`; do not accept out of order.
- [ ] **Task 3.2: Render and verify the acceptance.** Run `./x render`; stage the ADR, `docs/decisions/INDEX.md`, and `.awf/awf.lock` if changed. `git diff --check` is silent, `./awf check staged` exits zero, and `./x gate` exits zero.
- [ ] **Phase-close: stage, check, gate, and commit.** Commit the complete acceptance transaction only.

```commit
docs(adr): accept root-confined filesystem access
```

## Phase 4: Land the root-confined handle, shared fault source, and upgrade consumer

**Execution mode: subagent-driven.** Baseline: start from a clean managed worktree with Phases 1-3 committed; `git status --short` returns empty; both ADR files report the prerequisite `Implemented` and filesystem ADR `Accepted`; `./x check` is clean; and `./x gate` exits zero. This phase is one indivisible concrete-first transaction. Execute Tasks 4.1-4.6 in their declared order, but do not commit a provider, fixture export, fault operation, or consumer separately.

- [ ] **Task 4.1: Implement the production handle and its contract tests.** Create `internal/filesystem/handle.go` with a one-sentence package comment and concrete `Handle` owning an `*os.Root`. Export and document `Open(root string) (*Handle, error)`, `Close() error`, `Walk(subtree string, visit func(path string, info fs.FileInfo) (bool, error)) error`, `Read(path string) ([]byte, error)`, `Info(path string) (fs.FileInfo, error)`, and `LinkInfo(path string) (fs.FileInfo, error)`. `Open` calls `os.OpenRoot`. Every operation accepts only `.` or `fs.ValidPath(name)`, rejects empty/absolute/parent paths before delegation, uses slash-relative names, and wraps operation plus path with `%w`. `Walk` uses the root's `fs.FS`, resolves `DirEntry.Info` before the consumer callback, supplies metadata for the entry itself without following a final symlink, treats callback `true` as descend and `false` as skip only for directories, ignores the boolean for nondirectories, and propagates callback identity. It never returns absolute paths or exposes `filepath.SkipDir`. It does not follow directory symlinks. Keep any genuinely race-only adapter branch behind a specific `coverage-ignore`; do not copy digest exclusions.

  Create `internal/filesystem/handle_test.go`. `TestHandleConfinesPaths` proves every future `root-confined-paths` clause in one named unit: `.`, valid nested reads, empty, absolute, `..`, repeated-separator invalidity, and `errors.Is` preservation for callback and root-operation failures. It creates an internal directory symlink and an escaping directory symlink, walks their parent, observes each symlink entry itself, and proves no child beneath either target is visited; direct access through the internal link succeeds while direct access through the escaping link is refused. It also proves every emitted Walk path is slash-relative and directory descent polarity is honored. Use platform skips only when symlink creation itself is unsupported. Add separate focused tests for Open failure, Read/Info/LinkInfo distinction, ignored nondirectory booleans, and use-after-close behavior.

  Build an AST scanner over every repository non-test Go source. `TestRootConfinedFilesystemSingleHome` proves every future `single-production-handle` clause: calls to `os.OpenRoot` and stored `*os.Root` production values occur only under `internal/filesystem/**`; the explicitly distinct `internal/testsupport/fsfixture/**` path is classified as test source rather than production; `internal/filesystem` declares exactly one exported concrete handle type, `Handle`, and no interface; and every outside-package production use imports the concrete package. Synthetic refuting sources must prove detection of an extra `os.OpenRoot` call, a stored `*os.Root`, a second exported concrete handle type, and a provider-owned exported interface. Do not add invariant proof-marker comments yet; the deferred claim transaction owns them.

- [ ] **Task 4.2: Implement the one kernel-backed fault source.** Create `internal/testsupport/fsfixture/fsfixture.go` with the one-sentence package comment `Package fsfixture provides the repository's kernel-backed controlled filesystem fault source as the distinct test source authorized by ADR-consumer-local-contracts-over-single-home-filesystem-access.` and these exact exported declarations, each with a doc comment:

  ```go
  type Operation string

  const (
      OperationWalk     Operation = "walk"
      OperationWalkInfo Operation = "walk-info"
      OperationRead     Operation = "read"
      OperationInfo     Operation = "info"
      OperationLinkInfo Operation = "link-info"
  )

  type Fault struct {
      Operation Operation
      Path      string
      Err       error
  }

  type Handle struct { /* unexported os.Root and fault map fields */ }

  func Open(root string, faults ...Fault) (*Handle, error)
  func (h *Handle) Close() error
  func (h *Handle) Walk(subtree string, visit func(string, fs.FileInfo) (bool, error)) error
  func (h *Handle) Read(path string) ([]byte, error)
  func (h *Handle) Info(path string) (fs.FileInfo, error)
  func (h *Handle) LinkInfo(path string) (fs.FileInfo, error)
  ```

  `Open` validates faults in slice order before opening the root: reject an unknown operation as `fsfixture: fault <index>: unknown operation %q`; a nil error as `fsfixture: fault <index>: nil error`; a path other than `.` or `fs.ValidPath(path)` as `fsfixture: fault <index>: invalid path %q`; and a duplicate operation/path key as `fsfixture: fault <index>: duplicate <operation> fault for %q`. Then call `os.OpenRoot`; wrap failure as `fsfixture: open root %q: %w`. Every method applies the same path validation before fault lookup. Method failures wrap as `fsfixture: <operation> %q: %w`, preserving `errors.Is`. `OperationWalk` matches a visited path before entry metadata is resolved; `OperationWalkInfo` matches immediately before `DirEntry.Info`; both abort traversal. Walk metadata describes the entry without following its final symlink, callback true/false and error behavior exactly match `filesystem.Handle`, and directory symlinks are not followed. Nonmatching calls delegate to the real root. `Close` delegates to `os.Root.Close`; each outside-package test registers a cleanup that reports a close error through `t.Error`.

  Add the required ADR-slug comment beside the implementation: `ADR-consumer-local-contracts-over-single-home-filesystem-access permits this distinct test source because the standard-library-only testsupport leaf cannot import the production handle.`

  Create `internal/testsupport/fsfixture/fsfixture_test.go`. Cover every validation branch in its declared precedence, root-open failure, Close, successful delegation, every operation constant, exact-path matching, nonmatching delegation, walk metadata and descent, callback error, and `errors.Is` preservation. Implement reusable AST scanner helpers plus refuting synthetic-source cases. `TestFilesystemFaultSourceSingleHome` independently proves: no non-test file under `internal/testsupport/**` outside `fsfixture/**` calls `os.OpenRoot` or stores `*os.Root`; no other testsupport package declares a filesystem `Operation`/`Fault` source; every non-test fixture import is from the standard library or its own testsupport subtree; the live fixture source contains the required ADR slug; a caller sentinel injected through every operation remains `errors.Is`-matchable; and every handle operation with a nonmatching fault delegates successfully to the real root. Synthetic refuting sources must cover a partial duplicate that opens a root without fault declarations, one that declares fault operations without opening a root, and a non-standard-library import; each must fail the scanner. Do not add its invariant marker until the deferred claim transaction.

- [ ] **Task 4.3: Characterize, then convert digest policy to a consumer-local structural contract.** Before changing `internal/upgrade/digest.go`, create `internal/upgrade/digest_test.go` and add `TestTreeDigestCharacterization` against the old signatures. Build one fixed temporary repository fixture with `.awf/config.yaml`, the approval file, one domain sidecar, one topic metadata file, one topic part, one ADR, one matching marker-source file, one unmatched ordinary file, and one nested awf project; write explicit contents and chmod every digest member to `0o644`. Construct the expected universe as a literal sorted path slice, use the existing collection helpers to produce the actual set, and compare path-for-path. Run the old `treeDigest`, capture its full `sha256:<hex>` result as a literal `wantDigest`, and run the focused test successfully before modifying production code. Once captured, the literal path slice and digest are frozen for the rest of the phase: never regenerate or update either from the converted implementation. Record the pre-conversion focused-test command and successful terminal state in the phase Notes.

  Then, in `internal/upgrade/digest.go`, delete `var lstat`, remove direct `os` and `filepath` mechanism calls that the new handle owns, and declare a private `attestationTree` interface with exactly `Walk`, `Read`, `Info`, and `LinkInfo` signatures. Keep `treeDigest(root string, tree attestationTree)`, `collectUnder(tree, ...)`, `collectADRs(tree, ...)`, and `collectMarkerSources(tree, ...)` as readable consumer-local policy helpers. After the signature conversion, update only the characterization test's dependency construction and helper arguments; its fixture, literal selected-path slice, and literal digest stay byte-for-byte unchanged and the focused test must still pass. Read `.awf/config.yaml` through `tree.Read`, map `fs.ErrNotExist` to the existing `not an awf project (run `awf init`)` context, map other read failures to `read config`, preserve each cause with `%w`, then call `config.Parse(config.RootDir(root), bytes)`. Use slash-path operations for config-derived paths. Keep universe selection, ADR identity, pathglob matching, nested `.git` `LinkInfo`, nested `.awf` `Info`, optional subtree behavior, selected-file initial-missing behavior, mode recording, sorting, and digest encoding in upgrade.

  In `internal/upgrade/upgrade.go`, make public `Verify` open `filesystem.Handle`, defer its close, and call an unexported `verifyWithFilesystem(ctx, root, att, tree)` that retains version, Git head, digest comparison, and restoration guidance policy. The read-only deferred close result is deliberately ignored per the ADR. `FinalUpgrade` continues to call public `Verify` unchanged.

- [ ] **Task 4.4: Convert and expand upgrade tests with the real first consumer.** In `internal/upgrade/upgrade_test.go`, update the complete current call-site set: `sealedRepo`; `TestTreeDigestIsStableAndSensitive`'s stable and moved calls; all three `TestTreeDigestBranches` calls; both `TestCollectMarkerSourcesPrunesNestedGitRoots` subtests; and the old global-seam failure test. Each opens either `filesystem.Handle` or `fsfixture.Handle`, registers Close cleanup, and passes the tree to the new signature. Delete `TestCollectMarkerSourcesPropagatesGitBoundaryStatError` and replace its behavior in the new fault table; remove `testsupport.SwapVar` and now-unused `io/fs` imports. `upgrade.go`'s sole production `treeDigest` call moves into `verifyWithFilesystem`. `collectUnder` and `collectADRs` currently have no direct external call sites; new fault tests may call them only through `treeDigest`. After conversion, run `rg -n 'treeDigest\(|collectUnder\(|collectADRs\(|collectMarkerSources\(' internal/upgrade`; every nondefinition result must be one of the named converted sites or a new `digest_test.go` case, and `go test ./internal/upgrade` must reject any old-arity survivor.

  Extend `internal/upgrade/digest_test.go` with `TestVerifyUsesInjectedFilesystem` and table-driven cases that instantiate `fsfixture`, thereby providing the required outside-package test consumer of `Open`, `Close`, `Fault`, every operation constant, and every handle method. The named wiring test has two halves. Its behavioral half injects a digest read failure after constructing a repository whose HEAD matches the attestation and proves `verifyWithFilesystem` returns the caller identity. Its structural half parses `upgrade.go` and `digest.go` and asserts that public `Verify` calls `filesystem.Open`, defers the returned concrete handle's `Close`, passes that same value to `verifyWithFilesystem`, and that only public `Verify` constructs the production handle; `verifyWithFilesystem`, `treeDigest`, `collectUnder`, `collectADRs`, and `collectMarkerSources` each receive the dependency and contain no constructor, mutable default, or `filesystem.Open` call. Include synthetic refuting snippets for an inner constructor and a missing deferred Close.

  Exhaustively cover: absent `.awf/config.yaml` with `not an awf project` and `errors.Is(fs.ErrNotExist)`; non-missing config read fault with `read config` and caller identity; optional missing domains/topics; walk failure under authored sidecars; decisions walk failure; root marker walk failure; walk entry-info failure; nonregular authored entry skipped; selected-file initial `fs.ErrNotExist` skipped; selected-file custom read error propagated; post-read `Info` error propagated; nested `.git` `LinkInfo` error; nested `.awf` `Info` error; successful nested Git and awf pruning; and injected digest failure propagated through `verifyWithFilesystem` after a matching Git head. Assert `errors.Is` for every caller-supplied sentinel. Preserve existing successful digest stability, mode sensitivity, content sensitivity, and seal acceptance/rejection tests.

  Run `rg -n 'coverage-ignore' internal/upgrade/digest.go` and classify every remaining line in the plan Notes before phase close. Delete every exclusion whose branch the fixture reaches. Retain zero or more only when the branch remains unrelated, genuinely unreachable, or race-only, and rewrite each surviving justification to name that reason. The acceptance condition is evidence for each survivor, never a predetermined count.

- [ ] **Task 4.5: Assign package ownership and architecture documentation.** Add `internal/filesystem/**` to `.awf/domains/tooling.yaml` adjacent to `internal/git/**`. In `.awf/docs/parts/architecture/components.md`, add one bullet saying `internal/filesystem` owns the one production `os.Root`-backed handle, consumers own local structural contracts and policy helpers, and `internal/testsupport/fsfixture` is the distinct standard-library-only kernel-backed fault source. Do not author the pending claims yet. Run `./x render` and stage `docs/architecture.md`, domain/topic navigation, and `.awf/awf.lock` changes caused by these inputs.

- [ ] **Task 4.6: Verify the full vertical slice.** Run `go test ./internal/filesystem ./internal/testsupport/fsfixture ./internal/upgrade`; all pass. Run `rg -n 'var lstat|SwapVar\(t, &lstat|os\.(ReadFile|Stat|Lstat)|filepath\.WalkDir' internal/upgrade/digest.go internal/upgrade/*_test.go`; production `digest.go` and the old mutable seam produce no matches, while direct OS assertions in tests are acceptable only outside the injected policy path. Run `go test ./internal/testsupport` to prove production does not import the fixture. Run `./awf context internal/filesystem internal/testsupport/fsfixture internal/upgrade`; the new package and fixture are owned by tooling, and upgrade sees global composition authority. Run `./x render && ./x check`; drift and state are clean. Run `./x gate`; it exits zero with 100% coverage and no dead production code.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete provider, fixture, outside-package consumer, tests, domain map, architecture source, and rendered-output transaction. Confirm `./awf check staged` and `./x gate` exit zero. Create exactly one closing commit; splitting it would violate both concrete-first claims.

```commit
refactor(code-design): compose root-confined upgrade filesystem
```

## Verification

Before terminal review, verify the four phase commits are present in order, the worktree is clean, both ADR lifecycle states are prerequisite `Implemented` and filesystem `Accepted`, the plan remains `Proposed`, `./x check` is clean, and `./x gate` passes. Run `go test ./internal/filesystem ./internal/testsupport/fsfixture ./internal/upgrade ./internal/testsupport` and confirm every package passes. Re-run the scratch production import violation from Phase 2 and one scratch `..` or escaping-symlink handle access from Phase 4, confirm the intended test/refusal, restore the files, and return to a clean gate.

After implementation review settles, the deferred transaction owned by `awf-reviewing-impl` does all of the following atomically:

1. Add these exact proof-marker comments immediately above the named tests, while leaving each test declaration on its own non-marker line so named-proof validation succeeds:

   ```go
   // invariant: code-design/dependency-composition:upgrade-attestation-filesystem-wiring (TestVerifyUsesInjectedFilesystem)
   // invariant: tooling/filesystem-access:single-production-handle (TestRootConfinedFilesystemSingleHome)
   // invariant: tooling/filesystem-access:root-confined-paths (TestHandleConfinesPaths)
   // invariant: tooling/filesystem-access:single-fault-source (TestFilesystemFaultSourceSingleHome)
   ```

   Place each marker only in the file containing its named test; the four lines are shown together solely to pin literal spelling.

2. Replace the complete `consumer-owned-contracts` block in `.awf/topics/parts/code-design/dependency-composition/current-state.md` with:

   ```markdown
   ### `invariant: consumer-owned-contracts`

   When substitution is needed around a shared concrete implementation, the consumer declares the smallest cohesive structural interface locally and names its dependency for the semantic operation it needs. The provider exports the concrete implementation and neutral values its mechanism yields, never a universal consumer interface. Consumer-local helpers and values may translate the imported capability into readable business policy but never reimplement the shared concern; a direct concrete dependency remains legal when substitution is unnecessary.
   Origin: ADR-0178
   Revised-by: ADR-consumer-local-contracts-over-single-home-filesystem-access
   Backing: unbacked
   Verify: For each changed dependency boundary, inspect the provider's exported surface, the consumer-local interface and helpers, and production wiring; confirm the interface is the consumer's narrow cohesive view, policy remains local, no provider-owned universal interface or function-field renaming layer appears, and a direct concrete dependency is used when no substitution boundary is needed.
   ```

3. Append this literal block to the same dependency-composition topic:

   ```markdown
   ### `invariant: upgrade-attestation-filesystem-wiring`

   Public upgrade attestation `Verify` opens and closes the production root-confined filesystem handle at its outer boundary, passes that handle through the private consumer-owned structural contract, and no digest or collection helper constructs, discovers, or defaults the dependency.
   Origin: ADR-consumer-local-contracts-over-single-home-filesystem-access
   Backing: test
   ```

4. Replace `.awf/topics/parts/tooling/filesystem-access/current-state.md`'s shell prose with `The filesystem package owns production root-confined access, while the dedicated testsupport fixture owns the distinct kernel-backed controlled fault source.` followed by `## Claims` and these literal blocks in declaration order:

   ```markdown
   ### `invariant: single-production-handle`

   `internal/filesystem` is the only production home for deliberately composed root-confined filesystem access; it exports one concrete handle and no provider-owned interface, while historical direct filesystem effects remain bounded candidates until a concrete conversion adopts the handle.
   Origin: ADR-consumer-local-contracts-over-single-home-filesystem-access
   Backing: test

   ### `invariant: root-confined-paths`

   The production handle accepts only valid slash-relative paths beneath its selected `os.Root`, refuses absolute, parent, and escaping-symlink access, returns slash-relative walk paths without following directory symlinks, and preserves wrapped error identity.
   Origin: ADR-consumer-local-contracts-over-single-home-filesystem-access
   Backing: test

   ### `invariant: single-fault-source`

   `internal/testsupport/fsfixture` is the only standard-library-only kernel-backed controlled filesystem fault source; it delegates unselected operations to its real root, preserves caller-supplied error identity, and cites the durable distinct-source decision at its implementation site. Production import exclusion remains governed by `tooling/test-infrastructure:production-never-imports-test-support`.
   Origin: ADR-consumer-local-contracts-over-single-home-filesystem-access
   Backing: test
   ```
5. In the filesystem ADR, change frontmatter to `status: Implemented` and append only the terminal `Implemented` event with the Accepted content stamp. This direct Accepted-to-Implemented transition implicitly applies all five declaration-ordered State changes in the same claim-and-marker transaction; append neither an Applied event nor `Implementing`.
6. Change this plan's frontmatter to `status: Implemented` and record any actual exclusion survivors or implementation deviations in Notes before freezing it.
7. Run `./x render`; stage all authored claims, markers, ADR, plan, `docs/decisions/INDEX.md`, generated topic/domain/architecture outputs, and `.awf/awf.lock`. `./awf check staged` and `./x gate` must both pass before the deferred commit.
8. Commit the deferred transaction exactly with `git commit -m "docs(code-design): apply filesystem access authority"`. The subject is the one closing subject for claim application, ADR terminal state, and plan freeze.

Before integration, merge the integration branch into the managed branch and run:

```bash
./awf adr number test-support-exports-earn-test-consumers consumer-local-contracts-over-single-home-filesystem-access
./x render
git status --short
# Stop unless every reported path is a numbering substitution or its rendered output.
git add -u
git add -- docs/decisions/[0-9][0-9][0-9][0-9]-test-support-exports-earn-test-consumers.md
git add -- docs/decisions/[0-9][0-9][0-9][0-9]-consumer-local-contracts-over-single-home-filesystem-access.md
./awf check staged
./x gate
git commit -m "docs(adr): number filesystem access decisions"
./x check
./x gate
```

The command must assign the prerequisite the lower number and substitute both stable slug references.
The explicitly staged path set must contain only numbering substitutions and rendered outputs caused
by numbering. Run terminal review over the committed numbered result, and integrate only after that
review settles according to the managed-worktree workflow.

## Notes

- Existing `internal/snapshot/filesystem.go` remains out of scope because it captures ordinary-tree symlink representation rather than providing root-confined repository access.
- Phase 4 records any surviving `internal/upgrade/digest.go` coverage exclusions here with their final line and evidence. An empty survivor list is valid.
- The fixture is deliberately not an in-memory filesystem and never becomes a production dependency.
