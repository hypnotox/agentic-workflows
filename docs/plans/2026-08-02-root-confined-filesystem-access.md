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

- [ ] **Task 2.1: Add a repository production-import proof.** In `internal/testsupport/deps_test.go`, retain `dependencyViolations` and `TestZeroInternalDeps` unchanged in purpose, then add a helper that parses imports from non-test `.go` files under repository `cmd/**` and `internal/**`, excluding the entire `internal/testsupport/**` subtree, and returns a violation whenever the unquoted import equals `github.com/hypnotox/agentic-workflows/internal/testsupport` or starts with that string plus `/`. Use `testsupport.RepoRoot(t)` from the external `testsupport_test` package to anchor the walk; skip `_test.go`, `testdata`, hidden VCS/resident directories, and non-Go files. Parsing failures are test failures, not ignored files.

  Add `TestProductionNeverImportsTestSupport` with the proof marker:

  ```go
  // invariant: tooling/test-infrastructure:production-never-imports-test-support (TestProductionNeverImportsTestSupport)
  ```

  The test walks the live repository and reports every path/import pair. Add focused table cases proving that a production source importing the root testsupport package and one importing `internal/testsupport/fsfixture` are rejected, while a standard-library import, another repository import, and a `_test.go` consumer are not production-import violations. The live scan must find zero violations. Forbidden: invoking `go list`, Git, or another repository package from production testsupport code; the proof itself may import the root testsupport package because it is an external test file.

- [ ] **Task 2.2: Update export eligibility exactly.** Replace the body of `code-design/package-composition:export-earns-consumer` in `.awf/topics/parts/code-design/package-composition/current-state.md` with authority that distinguishes the declaring package:

  - a new or deliberately converted export in a production package requires an outside-package production consumer in the same green transaction;
  - `export_test.go` remains legal and a black-box test does not earn a production export;
  - an export in a dedicated shared package under `internal/testsupport/**` instead requires an outside-package test consumer in the same green transaction, and a compile-only reference does not count;
  - composition capabilities and error identities remain governed by their named sibling claims.

  Preserve `Origin: ADR-0200`, append `ADR-test-support-exports-earn-test-consumers` to `Revised-by`, retain `Backing: unbacked`, and replace Verify prose with a declaring-package classification followed by the corresponding production or outside-package test consumer trace.

- [ ] **Task 2.3: Update concrete-first symmetry exactly.** Replace `code-design/dependency-composition:concrete-first-consumer` in `.awf/topics/parts/code-design/dependency-composition/current-state.md` so a production composition capability requires one named concrete production first consumer, while a capability exported from dedicated `internal/testsupport/**` requires one named outside-package test first consumer. Both capability and consumer land in the same green transaction, the consumer uses the whole introduced capability, and anticipated reuse adds no adapter, constructor field, interface method, option, helper, or fault operation. Preserve `Origin: ADR-0178`, append `ADR-test-support-exports-earn-test-consumers` to `Revised-by`, retain `Backing: unbacked`, and make Verify inspect the declaring package, corresponding caller kind, whole-capability use, and same-transaction diff.

- [ ] **Task 2.4: Add the one-way test-infrastructure claim.** Append this claim to `.awf/topics/parts/tooling/test-infrastructure/current-state.md` and replace the topic preamble only as needed to mention both directions of the boundary:

  ```markdown
  ### `invariant: production-never-imports-test-support`

  Non-test Go files outside `internal/testsupport/**` never import the root test-support package or any of its subpackages; shared test fixtures remain a test-only dependency in the direction from outside-package tests into test support.
  Origin: ADR-test-support-exports-earn-test-consumers
  Backing: test
  ```

  Place the named proof marker from Task 2.1 on the test. Do not add `Verify:` to the backed claim.

- [ ] **Task 2.5: Implement the prerequisite ADR atomically.** In `docs/decisions/test-support-exports-earn-test-consumers.md`, change frontmatter to `status: Implemented`. Append one Applied event listing, in exact declaration order, update `code-design/package-composition:export-earns-consumer`, update `code-design/dependency-composition:concrete-first-consumer`, and add `tooling/test-infrastructure:production-never-imports-test-support`, with every qualified id in an inline code span. Append the terminal `Implemented` event after Applied, repeating the Accepted content stamp because the ADR body did not change. Run `./x render` and stage the ADR, all three authored claim parts, `internal/testsupport/deps_test.go`, generated topic/domain/index outputs, and `.awf/awf.lock`.

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

**Execution mode: subagent-driven.** Baseline: start from a clean managed worktree with Phases 1-3 committed; `git status --short` returns empty; both ADR files report the prerequisite `Implemented` and filesystem ADR `Accepted`; `./x check` is clean; and `./x gate` exits zero. This phase is one indivisible concrete-first transaction. The phase owner may order tasks internally but must not commit a provider, fixture export, fault operation, or consumer separately.

- [ ] **Task 4.1: Implement the production handle and its contract tests.** Create `internal/filesystem/handle.go` with a one-sentence package comment and concrete `Handle` owning an `*os.Root`. Export and document `Open(root string) (*Handle, error)`, `Close() error`, `Walk(subtree string, visit func(path string, info fs.FileInfo) (bool, error)) error`, `Read(path string) ([]byte, error)`, `Info(path string) (fs.FileInfo, error)`, and `LinkInfo(path string) (fs.FileInfo, error)`. `Open` calls `os.OpenRoot`. Every operation accepts only `.` or `fs.ValidPath(name)`, rejects empty/absolute/parent paths before delegation, uses slash-relative names, and wraps operation plus path with `%w`. `Walk` uses the root's `fs.FS`, resolves `DirEntry.Info` before the consumer callback, supplies metadata for the entry itself without following a final symlink, treats callback `true` as descend and `false` as skip only for directories, ignores the boolean for nondirectories, and propagates callback identity. It never returns absolute paths or exposes `filepath.SkipDir`. It does not follow directory symlinks. Keep any genuinely race-only adapter branch behind a specific `coverage-ignore`; do not copy digest exclusions.

  Create `internal/filesystem/handle_test.go`. `TestHandleConfinesPaths` covers `.`, valid nested reads, empty, absolute, `..`, an internal symlink, and a symlink whose target escapes the root, using platform skips only when symlink creation itself is unsupported. `TestRootConfinedFilesystemSingleHome` AST-scans non-test repository Go files for `os.OpenRoot`: production use is allowed only under `internal/filesystem/**`, with the separately decided `internal/testsupport/fsfixture/**` test source allowed by explicit path; a new production home fails. Additional tests cover Open failure, Read/Info/LinkInfo distinction, slash-relative Walk paths, false directory descent, ignored file booleans, callback error identity, and use-after-close behavior. Do not add invariant proof-marker comments yet; the deferred claim transaction owns them.

- [ ] **Task 4.2: Implement the one kernel-backed fault source.** Create `internal/testsupport/fsfixture/fsfixture.go` as a standard-library-only package. Export and document a concrete root-backed handle plus only the capability used this phase. Define an `Operation` type with exact operations for walk traversal, walk entry-info, read, info, and link-info; define `Fault{Operation, Path, Err}`; and construct the fixture from an OS root plus zero or more faults. Export `Close() error`, and require every outside-package fixture use to register it with `t.Cleanup`. Reject nil errors, invalid paths, and duplicate operation/path keys. A matching operation/path returns the caller error through operation/path `%w` wrapping; every other call delegates to the real `os.Root`. Its `Walk`, `Read`, `Info`, and `LinkInfo` signatures structurally match the upgrade consumer interface and the production handle. Package and implementation comments cite ADR-consumer-local-contracts-over-single-home-filesystem-access and explain that the standard-library-only leaf forces this distinct kernel-backed controlled-fault source rather than a second production adapter.

  Create `internal/testsupport/fsfixture/fsfixture_test.go`. Cover validation, delegation, every introduced fault operation, exact path selection, nonmatching delegation, callback polarity/error behavior, and `errors.Is` preservation. Add `TestFilesystemFaultSourceSingleHome` as a structural scan proving this package is the only non-test `internal/testsupport/**` source that both opens `os.Root` and defines filesystem fault operations. Do not add its invariant marker until the deferred claim transaction.

- [ ] **Task 4.3: Convert digest policy to a consumer-local structural contract.** In `internal/upgrade/digest.go`, delete `var lstat`, remove direct `os` and `filepath` mechanism calls that the new handle owns, and declare a private `attestationTree` interface with exactly `Walk`, `Read`, `Info`, and `LinkInfo` signatures. Keep `treeDigest(root string, tree attestationTree)`, `collectUnder(tree, ...)`, `collectADRs(tree, ...)`, and `collectMarkerSources(tree, ...)` as readable consumer-local policy helpers. Read `.awf/config.yaml` through `tree.Read`, map `fs.ErrNotExist` to the existing `not an awf project (run `awf init`)` context, map other read failures to `read config`, preserve each cause with `%w`, then call `config.Parse(config.RootDir(root), bytes)`. Use slash-path operations for config-derived paths. Keep universe selection, ADR identity, pathglob matching, nested `.git` `LinkInfo`, nested `.awf` `Info`, optional subtree behavior, selected-file initial-missing behavior, mode recording, sorting, and digest encoding in upgrade.

  In `internal/upgrade/upgrade.go`, make public `Verify` open `filesystem.Handle`, defer its close, and call an unexported `verifyWithFilesystem(ctx, root, att, tree)` that retains version, Git head, digest comparison, and restoration guidance policy. The read-only deferred close result is deliberately ignored per the ADR. `FinalUpgrade` continues to call public `Verify` unchanged.

- [ ] **Task 4.4: Convert and expand upgrade tests with the real first consumer.** Update every `treeDigest`, `collectMarkerSources`, and related helper call in `internal/upgrade/upgrade_test.go` to pass a production handle or shared fault fixture; delete `TestCollectMarkerSourcesPropagatesGitBoundaryStatError`'s `testsupport.SwapVar` use and remove now-unused imports. Create `internal/upgrade/digest_test.go` with `TestVerifyUsesInjectedFilesystem` and table-driven cases that instantiate `fsfixture`, thereby providing the required outside-package test consumer of every exported fixture capability and every fault operation.

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

1. Add the four named invariant proof-marker comments to the already-existing tests: `TestVerifyUsesInjectedFilesystem`, `TestRootConfinedFilesystemSingleHome`, `TestHandleConfinesPaths`, and `TestFilesystemFaultSourceSingleHome`.
2. Update `.awf/topics/parts/code-design/dependency-composition/current-state.md`'s `consumer-owned-contracts` claim to say that a substituting consumer owns the smallest cohesive local structural interface over a shared concrete implementation; provider-neutral values may cross; consumer-local helpers may express business policy but never reimplement the shared concern; and direct concrete dependencies remain legal when substitution is unnecessary. Preserve `Origin: ADR-0178`, append the filesystem ADR to `Revised-by`, retain `Backing: unbacked`, and make Verify inspect provider surface, local interface, helper policy, and absence of renamed function-field indirection.
3. Add `code-design/dependency-composition:upgrade-attestation-filesystem-wiring` with `Origin` set to the filesystem ADR and `Backing: test`: public `Verify` opens and closes the production root-confined handle, passes it to the private structural consumer, and no digest helper discovers or defaults the dependency. Point its marker at `TestVerifyUsesInjectedFilesystem`.
4. Replace `.awf/topics/parts/tooling/filesystem-access/current-state.md`'s shell prose with the ownership summary and add, in State changes declaration order:
   - `single-production-handle`, backed by `TestRootConfinedFilesystemSingleHome`: `internal/filesystem` is the only production home for deliberately composed root-confined filesystem access; it exports a concrete handle and no provider-owned interface, while historical direct effects remain bounded candidates until converted.
   - `root-confined-paths`, backed by `TestHandleConfinesPaths`: the handle accepts only valid slash-relative paths beneath its `os.Root`, refuses lexical and symlink escape, returns slash-relative walks, and preserves wrapped error identity.
   - `single-fault-source`, backed by `TestFilesystemFaultSourceSingleHome`: `internal/testsupport/fsfixture` is the one standard-library-only kernel-backed controlled fault source, production never imports it, caller errors preserve identity, and the distinct-source reasoning is referenced at the site.
5. In the filesystem ADR, append one Applied event containing all five State changes in exact declaration order, then append `Implemented` with the Accepted content stamp and change frontmatter to `status: Implemented`. This is a direct Accepted-to-Implemented transition; never append `Implementing`.
6. Change this plan's frontmatter to `status: Implemented` and record any actual exclusion survivors or implementation deviations in Notes before freezing it.
7. Run `./x render`; stage all authored claims, markers, ADR, plan, `docs/decisions/INDEX.md`, generated topic/domain/architecture outputs, and `.awf/awf.lock`. `./awf check staged` and `./x gate` must both pass before the deferred commit.

Before integration, merge the integration branch into the managed branch and run:

```bash
./awf adr number test-support-exports-earn-test-consumers consumer-local-contracts-over-single-home-filesystem-access
./x render
git status --short
# Inspect the numbering-only path set, then stage each reported path explicitly; do not use git add -A.
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
