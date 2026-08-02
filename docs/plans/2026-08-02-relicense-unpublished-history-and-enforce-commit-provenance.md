---
format: plan-v2
date: 2026-08-02
adrs:
  - agpl-3-0-only-cutover-at-the-unpublished-history-boundary
  - opt-in-commit-identity-and-signature-enforcement
status: Proposed
---
# Plan: Relicense unpublished history and enforce commit provenance

## Goal

Recreate the complete unpublished Git DAG beneath one pinned AGPL-3.0-only boundary commit, correct fixture identities and signatures without changing published MIT history, ship reusable opt-in exact-commit provenance enforcement, and activate it for this repository with recoverable local and remote publication controls.

The plan does not revoke MIT rights already granted for published versions, rewrite any advertised remote object, treat local hooks as a hostile-owner security boundary, add GPG or X.509 trust semantics, infer relicensing authority from Git metadata, or begin a live ref move before a complete copied-repository rehearsal succeeds.

## Architecture summary

Execution separates reusable policy capability from the one-time repository migration. `internal/config` and `internal/configspec` own the optional policy shape; `internal/commitpolicy` owns exact policy facts, violations, operational refusals, and human rendering; `internal/git` owns revision expansion, tag peeling, commit facts, and native Git SSH verification; `internal/project` composes one operation-scoped verifier; and `cmd/awf` only routes `awf check commit-policy`, emits the model-owned rendering, and maps exits. Generated reference-transaction and pre-push payloads call that same operation and resolve policy from the invoking worktree.

The first execution phase accepts both reviewed ADRs and answers the migration-mechanism question by running the complete real ref/object universe in a mode-0700 external copy. It freezes a machine-readable manifest, contributor-rights and notice dispositions, recovery artifacts, the exact signing public key, and a deterministic old-to-new map without touching live refs. Subsequent green phases add config/schema support and the common verifier/CLI. An inline owner then stops all writers, repeats the census, creates fresh external recovery artifacts, removes leaking local identity overrides, and executes the proven expected-old transaction over live active refs. Only after the rewritten DAG is conforming does the plan render hooks, activate this repository's exact policy, and terminalize the policy ADR in one legal transaction. The final implementation phases prove and terminalize the AGPL project-license claim, obtain explicit acceptance, retire only temporary local backup refs, and freeze the reviewed plan; the external recovery bundle remains until separately retired.

No one-off migration state is stored in `internal/manifest`, which remains the rendered-output lock model. Operational manifests, copied repositories, rights evidence, patches, trust material, and rewrite maps live outside the checkout under the Phase 1 protected state root. Only sanitized conclusions and commands are recorded in this plan's Notes. All implementation phases close green; live-history phases are `inline` and cannot be delegated.

## Phase 1: Accept authority and prove the complete migration in a copied repository

**Execution mode: inline.**
Completes: ["copied-migration-proven", "relicensing-authority-cleared", "recovery-proven"]

### Task 1.1: Accept both reviewed decisions without applying state changes
Latitude: exact
Paths: ["docs/decisions/agpl-3-0-only-cutover-at-the-unpublished-history-boundary.md", "docs/decisions/opt-in-commit-identity-and-signature-enforcement.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Apply `awf-adr-lifecycle` to each reviewed Proposed ADR. Change each status to Accepted and append its canonical Accepted history event without changing Decision or State changes prose and without applying any operation. Run `./x render`. `./awf context --show pending` for both ADR paths must report every declared state operation Remaining and each ADR frozen according to its retained pre-V4 lifecycle semantics.

### Task 1.2: Spike the complete copied-repository migration
Kind: spike
Question: Which available Git rewrite mechanism and exact command sequence can, in a mode-0700 copy containing the source repository's complete object/ref universe, derive the final published boundary from freshly advertised remote heads/tags/releases, fetch and verify the pinned SPDX AGPL bytes, insert exactly one license commit, recreate and SSH-sign every selected unpublished commit and moving annotated tag through one topology-preserving map, correct only audited fixture identities, classify every ref/worktree/pseudoref/index/dirty-path disposition, update selected refs with expected-old checks, preserve remote-tracking evidence, and prove all structural, content, identity, signature, linked-worktree, refusal, recovery, and cleanup assertions without changing the live repository; record the selected mechanism, tool versions, exact rehearsal commands, sanitized manifest/evidence locations, and pass/fail answer in this plan's Notes, and reject every candidate that misses an assertion?

### Task 1.3: Freeze authority, notice, recovery, and signing inputs outside the checkout
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Create a timestamped mode-0700 state root at `${XDG_STATE_HOME:-$HOME/.local/state}/awf/relicense-unpublished-history-and-enforce-signatures/` and keep it outside every Git worktree. Store the rehearsal copy, fetched SPDX file, tool-version capture, complete ref and worktree census, source object-format/full-OID-width declaration, remote advertisement snapshots, pseudoref/index/dirty-path dispositions, contributor identity census, contributor-rights and retained-notice dispositions, public signing-key record, old-to-new object/ref map, external bundle, and per-worktree patches or archives there. Store permissions or other sensitive evidence outside the repository and put only a sanitized cleared/blocking conclusion in Notes.

Fetch `AGPL-3.0-only.txt` only from SPDX license-list-data commit `d46e94e2c78ceede1cfc63cfa0396472d2798d4c`; require exactly 34,020 bytes, one terminal newline, and SHA-256 `d8a6cc31abc16b6748c7a21f21611f5a1ec33f67d22ca23d7da1c19b95496bee`. Enumerate every configured remote's advertised heads and tags and record release evidence through the GitHub API; derive rather than assume the final common published boundary. Classify every author, committer, imported file, and retained notice selected for rewriting. Any unresolved authority, unrelated history, moving remote, unsupported ref class, incomplete dirty-state archive, key mismatch, or failed rehearsal assertion blocks Phase close for user disposition.

Independently verify the external bundle with `git bundle verify`, restore it into a second empty repository, apply each retained dirty-state artifact to its recorded base, and rerun the structural verifier there. Exercise expected-old mismatch, remote movement, missing object, invalid boundary, unrelated selected history, signing failure, tag-peel failure, incomplete backup, prepared and non-prepared reference-transaction states, dirty and unmerged indexes, detached and linked worktrees, absolute and relative `core.hooksPath`, commit/amend/merge/rebase/reset/branch creation/deletion, and multi-ref transactions. Refusal must leave every selected ref unchanged and retain recoverable staged state. Record exact successful commands and terminal assertions in Notes; do not commit the external artifacts or any secret key material.

### Phase close

Update only this plan's Notes with the sanitized spike answer, selected exact command sequence, evidence-root identifier, authority result, and deviations. Run `./x render`, `./x check`, and `git diff --check`; each must exit zero. Stage the two Accepted ADR transitions, generated index/lock, and plan Notes. Require `./awf check staged` and `./x gate`, then commit:

```commit
docs(plans): prove copied history migration
```

## Phase 2: Add the optional policy configuration and schema contract

**Execution mode: subagent-driven.**
Completes: ["policy-config-valid"]

### Task 2.1: Parse and validate the commit-policy configuration
Latitude: exact
Paths: ["internal/config/config.go", "internal/config/config_test.go", "internal/configspec/spec.go", "internal/configspec/spec_test.go"]

Before dispatch, require `git status --short` to be empty and `./x check` plus `./x gate` to exit zero from the Phase 1 commit. Retain `go test ./internal/config ./internal/configspec ./internal/migrate` as additional focused evidence.

Add an optional top-level `commitPolicy` pointer with required `grandfatheredThrough`, optional nonempty `allowedIdentities`, default-false `requireSignedCommits`, and conditionally required nonempty `allowedSigners`. Model exact identity and signer records in `internal/config`; do not import the future policy package into config. Validate nonempty trimmed UTF-8 identity fields without controls, pair uniqueness, lowercase hexadecimal syntax at one supported full object-ID width rather than accepting abbreviations, principal grammar `[A-Za-z0-9._@+-]+`, record uniqueness, and the signing-option relationships. Structural validation must not open a repository or resolve a commit; Task 3.2 compares the configured width to the actual repository object format and resolves the commit at runtime.

Validate each key as exactly one option-free and comment-free OpenSSH public-key record with no newline or trailing record, accepted by `ssh-keygen` and restricted to the algorithms the runtime verifier supports. Put subprocess-backed semantic validation behind an operation seam rather than a package global. Failures name the complete config key and offending list element without echoing secret material. Add `TestCommitPolicyValidation` and its `// invariant: config/validation:commit-policy (TestCommitPolicyValidation)` marker immediately above the complete proving test; keep the test name on a separate line.

Document every field through `internal/configspec`, including absence behavior, exact-match semantics, the full-OID baseline, allowed signer principal/key relationship, and invalid combinations. Existing configurations must decode and validate unchanged.

### Task 2.2: Advance schema generation once without inventing policy
Latitude: exact
Paths: ["internal/migrate/migrate.go", "internal/migrate/commitpolicy.go", "internal/migrate/commitpolicy_test.go", "internal/migrate/forwardport_test.go", "internal/migrate/migrate_test.go", ".awf/awf.lock", "examples/sundial/.awf/awf.lock"]

Register one new generation after the then-current generation in `internal/migrate/commitpolicy.go`. Its migration is byte-preserving for config trees without `commitPolicy` and never invents a baseline, identity, signer, key, hook activation, or repository setting. Update current-generation, forward-port, stale-lock, ahead-binary, and no-mutation tests so the registry, lock generation, and gated-command refusal cannot diverge. Do not hard-code the author-time generation number in assertions that can derive it from the registry.

### Task 2.3: Apply the configuration validation state operation and document the schema
Kind: batch
Latitude: exact
Paths: ["docs/decisions/opt-in-commit-identity-and-signature-enforcement.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", "templates/docs/config-reference.md.tmpl", "docs/architecture.md", "docs/config-reference.md", "docs/topics/config/validation.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "examples/sundial/docs/architecture.md", "examples/sundial/docs/config-reference.md", "examples/sundial/.awf/awf.lock"]
Representative: Add the commitPolicy validation claim and describe its optional schema in the authored root sources, then render the exact root and Sundial destinations listed in Paths.
Edge: Preserve absent-policy output and never invent a baseline, identity, signer, key, or hook activation in either adopter.
Post-check: Run `./x render && ./x check`, then require `git diff --exit-code -- docs/architecture.md docs/config-reference.md docs/topics/config/validation.md docs/decisions/INDEX.md .awf/awf.lock examples/sundial/docs/architecture.md examples/sundial/docs/config-reference.md examples/sundial/.awf/awf.lock` after staging; no generated destination may remain unstaged or drifted.

Transition the Accepted policy ADR to Implementing and append one Applied event for exactly its first declared state operation, `add config/validation:commit-policy`. Author the claim in `.awf/topics/parts/config/validation/current-state.md` with this ADR as Origin, `Backing: test`, and the Task 2.1 proof marker. Update authored architecture and config-reference sources for parsing, validation, absence behavior, and schema migration; do not claim runtime enforcement yet. Run `./x render` and stage all selected root and Sundial generated outputs, including the lock and decision index.

### Phase close

Run `gofmt -w internal/config/config.go internal/config/config_test.go internal/configspec/spec.go internal/configspec/spec_test.go internal/migrate/migrate.go internal/migrate/commitpolicy.go internal/migrate/commitpolicy_test.go internal/migrate/forwardport_test.go internal/migrate/migrate_test.go`, `go test ./internal/config ./internal/configspec ./internal/migrate`, `./x render`, `./x check`, and `git diff --check`; each must exit zero and absent-policy golden output must remain coherent. Stage the complete schema/config transaction, first Applied batch, claim/proof, docs, and rendered outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(config): add commit provenance policy
```

## Phase 3: Implement one exact-commit verifier and CLI

**Execution mode: subagent-driven.**
Completes: ["common-verifier-complete"]

### Task 3.1: Model exact policy evaluation and actionable outcomes
Latitude: exact
Paths: ["internal/commitpolicy/doc.go", "internal/commitpolicy/policy.go", "internal/commitpolicy/evaluate.go", "internal/commitpolicy/outcome.go", "internal/commitpolicy/render.go", "internal/commitpolicy/policy_test.go", "internal/commitpolicy/evaluate_test.go", "internal/commitpolicy/outcome_test.go", "internal/commitpolicy/render_test.go", ".awf/domains/tooling.yaml"]

Before dispatch, require `git status --short` to be empty and `./x check` plus `./x gate` to exit zero from the Phase 2 commit. Focused package tests below supplement rather than replace that full-green baseline.

Add `internal/commitpolicy/**` to the tooling domain in `.awf/domains/tooling.yaml`. Create `internal/commitpolicy` as the only policy home, with `doc.go` stating in one sentence that it owns exact commit-policy facts, evaluation outcomes, and human rendering. Every exported declaration receives a Go doc comment naming its semantic contract and must have a production consumer in this Phase 3 transaction; keep all other declarations private. Define typed policy values, author/committer facts, signature verdicts, stable commit-keyed violations, and operational refusals. Evaluate both author and committer byte-for-byte against complete allowed identity pairs. Require successful allowed-signer verification when signing is enabled; a signature header alone is insufficient. Deduplicate commits before evaluation and return every violation in stable commit/field order.

Operational refusals must preserve causes and distinguish config, baseline, revision resolution, tag peel, linked-worktree, temporary trust-file, and signature-process failures. Each carries category, observed condition, whether refs changed, whether the index changed, and ordered reconciliation actions. Human rendering stays in this package and prints the exact required identity and signature phrases, complete relevant allowlists, affected commit, author-versus-committer distinction, unchanged-state facts, and configuration/rerun guidance. It must not mutate refs, indexes, config, history, or trust data.

Add `TestExactCommitEnforcement` with `// invariant: tooling/commit-policy:exact-commit-enforcement (TestExactCommitEnforcement)` immediately above it. Cover allowed and disallowed authors/committers independently, mixed and duplicate violations, unsigned, malformed, wrong-key and valid SSH signatures, disabled signing, stable rendering, preserved causes, and every operational outcome branch.

### Task 3.2: Extend the Git boundary for commit facts, reachability, peeling, and SSH verification
Latitude: exact
Paths: ["internal/git/handle.go", "internal/git/walk.go", "internal/git/lifecycle.go", "internal/git/runner.go", "internal/git/commitpolicy.go", "internal/git/commitpolicy_test.go", "internal/git/entrypoints_test.go"]

Add consumer-oriented `internal/git.Repo` methods for full-OID object resolution, commit fact loading, revision/range expansion, commits reachable from a target but not the grandfathered baseline, recursive annotated-tag peeling, target-type diagnostics, and native `git verify-commit`. Keep go-git/native representations private. Baseline runtime resolution must require one full object ID of the repository's actual object-format width and a commit object.

For SSH verification, create a mode-0600 temporary allowed-signers file from the configured principal/public-key pairs, set only the Git configuration needed for ordinary SSH verification, invoke the existing deadline-bound native runner, and remove the file on every return path. Do not alter persistent Git or environment configuration, accept GPG/X.509, parse `git log` display prose, or collapse process failure into a policy violation. Add the new native entrypoint to the centralized entrypoint contract.

Integration fixtures must prove SHA-1 and supported alternate object-format width handling where the installed Git supports it, baseline exclusion, overlapping target deduplication, range grammar, lightweight and recursively annotated tags, non-commit targets, missing objects, invalid baselines, valid/wrong/unknown/malformed SSH signatures, allowed-signers cleanup, and deterministic errors.

### Task 3.3: Compose the verifier at the project boundary
Latitude: exact
Paths: ["internal/project/project.go", "internal/project/commitpolicy.go", "internal/project/commitpolicy_test.go", "internal/git/controlroot.go", "internal/git/controlroot_test.go", "internal/worktree/wiring_test.go"]

Add one operation-scoped project verifier that translates validated config into `internal/commitpolicy.Policy`, obtains facts through a consumer-local interface backed by `internal/git`, and returns typed package outcomes. It accepts explicit revision/range targets, expands and deduplicates them after the baseline, and emits one explicit disabled-policy note when the block is absent. The verifier must never parse CLI output or allow hooks/tests to implement a parallel walk.

Resolve config, generated payload paths, and temporary trust material from the invoking worktree root reported by Git, while stable executable hook wiring may resolve through the shared control root. Add linked-worktree tests with deliberately different primary/linked configs and absolute/relative `core.hooksPath`; each invocation must select its own branch worktree's policy. Missing/moved worktrees and unsafe root resolution return typed refusals.

### Task 3.4: Expose `awf check commit-policy` without presentation duplication
Latitude: exact
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/dispatch.go", "cmd/awf/checkgroup.go", "cmd/awf/commitpolicy.go", "cmd/awf/commitpolicy_test.go", "cmd/awf/check_test.go", "cmd/awf/help_test.go"]

Register exact grammar `awf check commit-policy <revision-or-range>...` with at least one explicit target. Route through one new handler that opens the ordinary gated project, calls the project verifier once, selects the model-owned renderer, writes stable stdout/stderr, and maps typed success, violation, disabled, and operational-refusal exits. It must not reconstruct messages, silently default to HEAD, rewrite a revision, or mutate the repository.

End-to-end tests cover arity/help, absent policy, one/multiple/range targets, overlapping deduplication, complete violation output, operational errors, linked-worktree selection, stale-binary refusal, and clean stdout/stderr separation. Use actual signed fixture commits for cryptographic cases rather than mocking a valid signature verdict at the command boundary.

### Task 3.5: Apply the exact-enforcement state operation and document ownership
Kind: batch
Latitude: exact
Paths: ["docs/decisions/opt-in-commit-identity-and-signature-enforcement.md", ".awf/topics/parts/tooling/commit-policy/current-state.md", ".awf/topics/metadata/tooling/commit-policy.yaml", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/testing/layout.md", "templates/docs/working-with-awf.md.tmpl", "README.md", "changelog/CHANGELOG.md", "docs/architecture.md", "docs/testing.md", "docs/working-with-awf.md", "docs/topics/tooling/commit-policy.md", "docs/domains/tooling.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "examples/sundial/docs/architecture.md", "examples/sundial/docs/testing.md", "examples/sundial/docs/working-with-awf.md", "examples/sundial/.awf/awf.lock"]
Representative: Document the single commit-policy package, common CLI verifier, typed rendering, and preview-before-activation flow in authored sources and their exact generated root/Sundial destinations.
Edge: Do not claim hook enforcement is available, duplicate human rendering in cmd/awf, or present a disabled policy as an error.
Post-check: Run `./x render && ./x check`, then require `git diff --exit-code -- docs/architecture.md docs/testing.md docs/working-with-awf.md docs/topics/tooling/commit-policy.md docs/domains/tooling.md docs/decisions/INDEX.md .awf/awf.lock examples/sundial/docs/architecture.md examples/sundial/docs/testing.md examples/sundial/docs/working-with-awf.md examples/sundial/.awf/awf.lock` after staging; the listed generated set must be staged and drift-free.

Append one Applied event for exactly the policy ADR's second declared operation, `add tooling/commit-policy:exact-commit-enforcement`. Author the invariant with this ADR as Origin, `Backing: test`, and the Task 3.1 proof. Update authored architecture, testing, command, README, and Unreleased changelog surfaces for package ownership, exact CLI behavior, disabled policy, actionable refusals, and the preview-before-activation workflow. Render root and Sundial outputs. Do not document hook enforcement as available until Phase 4.

### Phase close

Run `find internal/commitpolicy internal/git -name '*.go' -type f -print0 | sort -z | xargs -0 gofmt -w` and `gofmt -w internal/project/commitpolicy.go internal/project/commitpolicy_test.go internal/clispec/clispec.go internal/clispec/clispec_test.go cmd/awf/dispatch.go cmd/awf/checkgroup.go cmd/awf/commitpolicy.go cmd/awf/commitpolicy_test.go cmd/awf/check_test.go cmd/awf/help_test.go`, `go test ./internal/commitpolicy ./internal/git ./internal/project ./internal/clispec ./cmd/awf`, `./x render`, `./x check`, and `git diff --check`; each must exit zero. Stage the common verifier, CLI, second Applied operation, claim/proof, docs, and generated outputs. Require `./awf check staged` and `./x gate`, then commit:

```commit
feat(tooling): verify exact commit provenance
```

## Phase 4: Execute the proven live history cutover

**Execution mode: inline.**
Advances: ["project-license-agpl", "repository-policy-active"]
Completes: ["published-history-unchanged", "unpublished-dag-recreated", "identity-and-signatures-correct"]

### Task 4.1: Stop writers and freeze the live transaction manifest
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Do not dispatch this phase or run it while any other agent, user process, rebase, merge, release, fetch, or push may mutate the repository. Stop every agent and record the quiescence boundary. Set `TXN_ROOT` to a fresh mode-0700 child of `${XDG_STATE_HOME:-$HOME/.local/state}/awf/relicense-unpublished-history-and-enforce-signatures/`. Repeat Phase 1's exact remote advertisements, GitHub releases/protection query, object/ref/worktree/pseudoref/index/dirty-path census, contributor-rights/notice audit, effective identities, and signature census into `$TXN_ROOT/live-manifest.json`. Re-derive the final common published boundary and require it and every advertised ref to match the accepted Phase 1 inputs or stop for explicit disposition.

Require one manifest disposition for every local branch, linked/detached worktree HEAD, tag, custom ref, remote-tracking ref, `refs/original/*`, pseudoref, index, and uncommitted path. Active selected refs map; remote-tracking refs stay frozen evidence; recovery-only refs/pseudorefs are archived and marked update-or-remove; unrelated or outside-boundary history blocks. Record every expected-old OID at full object width. Reconfirm contributor authority and retained notices; do not infer clearance from an unchanged author label.

### Task 4.2: Create recovery artifacts and remove leaking identity overrides
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Create a fresh external `git bundle --all` artifact and a separate patch/archive for every retained index and worktree state before changing refs or config. Run `git bundle verify "$TXN_ROOT/recovery.bundle"`, restore it into a second empty repository, restore each dirty-state artifact onto its recorded base, and execute the exact recovery verifier recorded by Phase 1. Hash and record each artifact in the protected manifest. An incomplete, unverified, checkout-resident, or non-restorable recovery set blocks mutation. Keep the external bundle and planned temporary backup refs; do not remove old objects or expire reflogs.

After recovery is proven, enumerate `user.name` and `user.email` with `git config --show-origin --show-scope --get-regexp '^(user\.name|user\.email)$'` from every path emitted by `git worktree list --porcelain`. Remove repository-local values with `git config --local --unset-all user.name` and `git config --local --unset-all user.email`, and remove each worktree-local value with `git -C <worktree> config --worktree --unset-all user.name` and the corresponding email command; tolerate only the specific nonzero result meaning the key was absent. Do not change global config. Before any new commit, require every worktree's effective `git var GIT_AUTHOR_IDENT` and `git var GIT_COMMITTER_IDENT` to begin exactly `Josua Müller <hypnotox@pm.me>` and require `git config --get gpg.format`, `git config --bool --get commit.gpgSign`, and the public key derived from `git config --get user.signingKey` to match the Phase 1 manifest. Preserve test-fixture identities only inside temporary test repositories.

### Task 4.3: Recreate and atomically present the unpublished graph
Latitude: exact
Paths: ["LICENSE", "README.md", "docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Execute exactly the selected Phase 1 mechanism and command sequence recorded in Notes against `$TXN_ROOT/live-manifest.json`, changing only its transaction-root arguments. Fetch and reverify the canonical SPDX bytes; never reuse unchecked or manually edited license text. Insert one dedicated commit immediately after the unchanged published boundary whose tree changes only `LICENSE` plus the README license badge and footer. Recreate the complete selected unpublished graph through one old-to-new map, preserving parent order, messages, timestamps, genuine audited identities, retained notices, and every tree byte except the intended per-snapshot license/README transformation. Replace every exact `T <t@example.com>` author or committer occurrence with `Josua Müller <hypnotox@pm.me>` and freshly SSH-sign every recreated commit. Recreate and sign every annotated tag whose target moves; map lightweight tags by target.

Before presentation, run all copied-tree assertions against candidate objects. Create the complete temporary-backup set through one `git update-ref --stdin` transaction whose `create` commands require zero old OIDs, so an existing backup aborts without partial creation. Then feed every mapped branch/tag/custom-ref update and every recovery-only removal classified for the live cutover to one separate `git update-ref --stdin` transaction using exact expected-old OIDs. Any mismatch or hook refusal must leave that whole selected presentation transaction unchanged; if backup creation succeeded but presentation failed, retain and record the backups rather than deleting them. Keep remote-tracking refs frozen. Reconcile linked/detached worktree HEADs and restore retained index/worktree state only through recorded maps. Do not push, force-update a remote, delete temporary backups, run GC, or amend a rewritten object.

### Task 4.4: Validate the live mapping before any cleanup
Latitude: exact
Paths: ["LICENSE", "README.md", ".goreleaser.yaml", "docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Prove every selected old commit has exactly one new mapping; every active selected ref resolves through it; published ancestry and advertised remote refs remain byte-for-byte unchanged; contracting the inserted license node yields identical topology and parent order; all rewritten trees differ only in intended license/README bytes; no fixture identity remains in policy-era reachable commits; every recreated commit and moved annotated tag verifies with the recorded SSH key; all genuine identities and required notices remain; and every worktree/ref/pseudoref/index/path has its classified terminal state. Require the pinned LICENSE hash/size/newline and exact README AGPL badge/footer.

Run these exact commands after setting `TXN_ROOT` from Task 4.1: `go test ./...`; `rm -rf dist && go run github.com/goreleaser/goreleaser/v2@v2.17.0 check`; `go run github.com/goreleaser/goreleaser/v2@v2.17.0 release --snapshot --clean`; `"$TXN_ROOT/rehearsal/bin/assert-release-license" --dist dist --license LICENSE`; `./x render`; `./x check`; `./x gate`; `git fsck --full`; and `"$TXN_ROOT/rehearsal/bin/verify-mapping" --manifest "$TXN_ROOT/live-manifest.json" --repository .`. Create `$TXN_ROOT/policy-preview`, detach it at the rewritten HEAD, invoke `"$TXN_ROOT/rehearsal/bin/materialize-policy-config" --manifest "$TXN_ROOT/live-manifest.json" --checkout "$TXN_ROOT/policy-preview"`, derive the target argv with `mapfile -t POLICY_TARGETS < <("$TXN_ROOT/rehearsal/bin/list-policy-targets" "$TXN_ROOT/live-manifest.json")`, and require `(cd "$TXN_ROOT/policy-preview" && ./awf check commit-policy "${POLICY_TARGETS[@]}")` to report zero violations. Remove only the preview checkout after success. Every command must exit zero, no live ref may point to superseded selected history except classified temporary backup refs, and `git status` must match the restored-state manifest. Record sanitized map hashes and terminal assertions in Notes.

### Phase close

Create no implementation commit before all mapping assertions and effective-identity checks pass. Update only this plan's Notes with sanitized live transaction evidence, the unchanged published boundary, map hash, remaining temporary backups, and any approved deviations. The resulting note commit is a new AGPL-era commit authored and committed as `Josua Müller <hypnotox@pm.me>`, freshly SSH-signed, and not rewritten. Run `./x render`, `./x check`, and `git diff --check`; stage only plan/generated-note effects, require `./awf check staged` and `./x gate`, then commit:

```commit
docs(plans): record live license cutover
```

## Phase 5: Render hooks, activate repository policy, and terminalize the policy ADR

**Execution mode: inline.**
Completes: ["generic-hook-enforcement-complete", "repository-policy-active"]

### Task 5.1: Render the reference-transaction payload and complete pre-push expansion
Latitude: exact
Paths: ["templates/hooks/reference-transaction.sh.tmpl", "templates/hooks/pre-push.sh.tmpl", "internal/project/render.go", "internal/project/output_plan.go", "internal/project/hooks_test.go", "internal/project/executable_test.go", "internal/project/output_declarations_test.go", "internal/project/descriptor_parity_test.go", ".awf/hooks/reference-transaction.sh", ".awf/hooks/pre-push.sh", "examples/sundial/.awf/hooks/reference-transaction.sh", "examples/sundial/.awf/hooks/pre-push.sh", ".awf/awf.lock", "examples/sundial/.awf/awf.lock"]

Before any Phase 5 action, resolve `TXN_ROOT` to the accepted protected live-transaction directory recorded by Phase 4 Notes and require `test -f "$TXN_ROOT/live-manifest.json"` plus `test -x "$TXN_ROOT/rehearsal/bin/list-policy-targets"` to succeed. Do not infer the directory by choosing the newest filesystem entry.

Add `reference-transaction.sh` to the hooks singleton, executable output declarations, render loop, descriptors, and lock. In `prepared`, buffer and validate every `<old-oid> <new-oid> <ref>` input record, select only nonzero new local branch targets, and invoke the common verifier over commits reachable from each new target but not its old target and not the grandfathered baseline. New branches include every post-baseline reachable commit. Deletion-only and backward-only changes add no commit. Non-prepared states exit without reevaluation. A refusal must abort the complete transaction, say refs did not move, preserve the index fact, and print exact reconciliation guidance.

Extend pre-push to parse every standard update record before running the configured gate. Skip deletion targets; expand local branch commits; recursively peel annotated tags; handle lightweight tags by target type; diagnose non-commit targets in verbose output; and fail closed on baseline, missing-object, or peel failures. Check the safe superset reachable from each local target without using remote-tracking refs as freshness authority. Invoke the project gate only after policy succeeds. Buffer stdin before invoking subprocesses so multi-ref input is never partially consumed.

Keep absent-policy rendering coherent and successful without unresolved tokens. Extend `TestHookPayloadsRendered` for the fifth payload and add `TestCommitPolicyHookPayloads` with `// invariant: rendering/singletons-and-payloads:commit-policy-hook-payloads (TestCommitPolicyHookPayloads)`. Integration tests cover every state/update/tag branch, multi-ref deduplication, invalid input, linked worktrees, and gate ordering with exact unchanged-ref assertions.

### Task 5.2: Wire stable stubs and close the generated documentation set
Kind: batch
Latitude: exact
Paths: [".githooks/reference-transaction", ".githooks/pre-push", "examples/sundial/.githooks/reference-transaction", "examples/sundial/.githooks/pre-push", "examples/sundial/.awf/config.yaml", ".awf/parts/workflow/local-hooks.md", ".awf/parts/agents-doc/awf-setup.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/development/command-runner.md", ".awf/docs/parts/testing/gate.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/rendering/companion-scripts/current-state.md", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", "templates/docs/testing.md.tmpl", "templates/docs/architecture.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "README.md", "changelog/CHANGELOG.md", "AGENTS.md", "docs/architecture.md", "docs/testing.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/topics/rendering/singletons-and-payloads.md", "docs/topics/rendering/companion-scripts.md", ".awf/hooks/reference-transaction.sh", ".awf/hooks/pre-push.sh", ".awf/awf.lock", "examples/sundial/AGENTS.md", "examples/sundial/docs/architecture.md", "examples/sundial/docs/testing.md", "examples/sundial/docs/workflow.md", "examples/sundial/docs/working-with-awf.md", "examples/sundial/.awf/hooks/reference-transaction.sh", "examples/sundial/.awf/hooks/pre-push.sh", "examples/sundial/.awf/awf.lock"]
Representative: Add thin executable reference-transaction stubs and document the five inert payloads, explicit wiring, preview-before-enable sequence, invoking-worktree resolution, and GitHub final boundary across authored sources and their listed generated outputs.
Edge: Keep Sundial unconditionally absent-policy; do not add commitPolicy, invent a baseline, activate hooks for adopters, imply hostile-owner protection, or emit unresolved/no-value tokens when policy data is absent.
Post-check: Run `./x render && ./x check`; run `rg -n 'four inert|four payload|reference-transaction|commitPolicy|commit-policy|pre-push|core\.hooksPath' .awf templates README.md changelog examples/sundial` and require every current hook-count/activation statement to agree; after staging, run `git diff --exit-code -- AGENTS.md docs/architecture.md docs/testing.md docs/workflow.md docs/working-with-awf.md docs/topics/rendering/singletons-and-payloads.md docs/topics/rendering/companion-scripts.md .awf/hooks/reference-transaction.sh .awf/hooks/pre-push.sh .awf/awf.lock examples/sundial/AGENTS.md examples/sundial/docs/architecture.md examples/sundial/docs/testing.md examples/sundial/docs/workflow.md examples/sundial/docs/working-with-awf.md examples/sundial/.awf/hooks/reference-transaction.sh examples/sundial/.awf/hooks/pre-push.sh examples/sundial/.awf/awf.lock` so the closed generated set is staged and drift-free.

The root and Sundial stubs locate the invoking worktree and delegate to that worktree's generated payload without reading policy from the primary checkout. Keep pre-push stubs thin delegates. Prove absolute/relative `core.hooksPath` and distinct primary/linked configs through native Git integration tests. awf still never edits `core.hooksPath` for an adopter.

### Task 5.3: Materialize and exercise this repository's exact policy
Latitude: exact
Paths: [".awf/config.yaml", ".awf/awf.lock", ".awf/hooks/reference-transaction.sh", ".awf/hooks/pre-push.sh", ".githooks/reference-transaction", ".githooks/pre-push", "docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Set `commitPolicy.grandfatheredThrough` to the full unchanged final published MIT boundary from `$TXN_ROOT/live-manifest.json`. Set exactly one allowed identity, `Josua Müller <hypnotox@pm.me>`. Enable signed commits and set one signer with principal `hypnotox@pm.me` and the exact option-free OpenSSH public key proven in Phase 1. Compare `ssh-keygen -y -f "$(git config --get user.signingKey)"` to the manifest key before writing; stop on mismatch. Do not store private material, abbreviate the baseline, tolerate the fixture identity, or move the baseline to hide a failure.

Run `mapfile -t POLICY_TARGETS < <("$TXN_ROOT/rehearsal/bin/list-policy-targets" "$TXN_ROOT/live-manifest.json")` and require `./awf check commit-policy "${POLICY_TARGETS[@]}"` to report zero violations before wiring is exercised. Run `./x render`. Execute `"$TXN_ROOT/rehearsal/bin/verify-worktree-config" --manifest "$TXN_ROOT/live-manifest.json" --repository .` and require every worktree's effective identity, SSH format, signing flag, public key, and hook resolution to match. Run `gh api repos/hypnotox/agentic-workflows/branches/main/protection/required_signatures --jq '.enabled == true'` and `gh api repos/hypnotox/agentic-workflows/branches/main/protection --jq '(.allow_force_pushes.enabled == false) and (.allow_deletions.enabled == false)'`; each must print `true`. Do not add PR or status-check requirements.

Run the native hook integration tests that create a disposable allowed signed commit, reject unsigned and disallowed-identity commits before ref movement, bypass reference-transaction only inside a disposable fixture, and prove pre-push then refuses it before the configured gate. Require cleanup assertions to leave no active nonconforming fixture ref and never push to a remote. Record only sanitized terminal results in Notes.

### Task 5.4: Apply the rendering operations and terminalize the policy ADR
Latitude: exact
Paths: ["docs/decisions/opt-in-commit-identity-and-signature-enforcement.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/parts/agents-doc/awf-setup.md", ".awf/parts/workflow/local-hooks.md", ".awf/docs/parts/development/setup.md", "README.md", "changelog/CHANGELOG.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Append the policy ADR's final Applied event containing exactly its declaration-ordered remaining operations: update `rendering/singletons-and-payloads:hook-payloads-rendered`, then add `rendering/singletons-and-payloads:commit-policy-hook-payloads`. Preserve the existing hook-payload claim's Origin and proof unit, append this ADR once to Revised-by, and update it from four to five exact outputs. Add the policy-hook invariant with this ADR as Origin, `Backing: test`, and `TestCommitPolicyHookPayloads`.

In this same transaction, after repository activation and every Decision commitment is true, append the canonical Implemented event and status/digest required by `awf-adr-lifecycle`. Update authored project-specific setup, local-hook, development, README, and changelog guidance for effective identity/signing policy, exact reconciliation commands, worktree-local resolution, preview, and GitHub boundary. Do not amend frozen Decision meaning. Run `./x render` so the index and lock agree.

### Phase close

Run `gofmt -w internal/project/render.go internal/project/output_plan.go internal/project/hooks_test.go internal/project/executable_test.go internal/project/output_declarations_test.go internal/project/descriptor_parity_test.go`; `go test ./internal/project ./internal/commitpolicy ./internal/git ./cmd/awf`; `find .githooks .awf/hooks examples/sundial/.githooks examples/sundial/.awf/hooks -maxdepth 1 -type f -print0 | sort -z | xargs -0 -n1 bash -n`; `mapfile -t POLICY_TARGETS < <("$TXN_ROOT/rehearsal/bin/list-policy-targets" "$TXN_ROOT/live-manifest.json") && ./awf check commit-policy "${POLICY_TARGETS[@]}"`; `./x render`; `./x check`; and `git diff --check`. Every command must exit zero, syntax checking must visit the complete listed stub/payload set, and policy preview must report no violation. Stage hook behavior, stubs, root policy activation, final policy Applied/Implemented history, claims/proofs, docs/changelog, and generated outputs. Require `./awf check staged` and `./x gate`, then commit a freshly SSH-signed commit:

```commit
feat(rendering): activate signed commit provenance
```

## Phase 6: Prove project licensing and terminalize the license ADR

**Execution mode: inline.**
Completes: ["project-license-agpl"]

### Task 6.1: Prove the AGPL project-license contract and close its generated surfaces
Kind: batch
Latitude: exact
Paths: ["LICENSE", "README.md", ".goreleaser.yaml", "cmd/releasecheck/project_license_test.go", ".awf/topics/metadata/tooling/project-license.yaml", ".awf/topics/parts/tooling/project-license/current-state.md", ".awf/docs/parts/releasing/content.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", "changelog/CHANGELOG.md", "docs/releasing.md", "docs/architecture.md", "docs/topics/tooling/project-license.md", "docs/decisions/agpl-3-0-only-cutover-at-the-unpublished-history-boundary.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "examples/sundial/docs/architecture.md", "examples/sundial/.awf/awf.lock"]
Representative: Add TestProjectLicenseAGPL and the dedicated project-license claim, then document and render the exact project-license, release, architecture, decision-index, and lock destinations listed in Paths.
Edge: Exclude dependency-license metadata and retained third-party notices from obsolete-project-MIT findings, and do not move project-license presentation into changelog ownership.
Post-check: Run `./x render && ./x check`; after staging, run `git diff --exit-code -- docs/releasing.md docs/architecture.md docs/topics/tooling/project-license.md docs/decisions/INDEX.md .awf/awf.lock examples/sundial/docs/architecture.md examples/sundial/.awf/awf.lock` so the complete generated set is staged and drift-free.

Resolve `TXN_ROOT` to the accepted protected live-transaction directory recorded by Phase 4 Notes and require `test -x "$TXN_ROOT/rehearsal/bin/assert-release-license"` before using its evidence in this phase.

Add `TestProjectLicenseAGPL` under `cmd/releasecheck` and its `// invariant: tooling/project-license:project-license-agpl (TestProjectLicenseAGPL)` marker. Pin exact LICENSE SHA-256, byte length, terminal newline, README badge/footer, GoReleaser archive inclusion, and absence of obsolete project MIT references while applying the explicit exclusions above. Extend topic applicability only as required for the proving production/check surface.

Transition the Accepted license ADR directly through Implementing: append one Applied event for its sole operation, `add tooling/project-license:project-license-agpl`, author the claim with this ADR as Origin and `Backing: test`, then append the canonical Implemented event in this same transaction as required by Decision 10. Update authored release, architecture, README, and changelog sources only where project-license truth changed. Do not change frozen Decision meaning. Render index, topic, docs, and lock.

### Phase close

Run `gofmt -w cmd/releasecheck/project_license_test.go`; `go test ./cmd/releasecheck`; `rm -rf dist && go run github.com/goreleaser/goreleaser/v2@v2.17.0 check`; `go run github.com/goreleaser/goreleaser/v2@v2.17.0 release --snapshot --clean`; `"$TXN_ROOT/rehearsal/bin/assert-release-license" --dist dist --license LICENSE`; `./x render`; `./x check`; and `git diff --check`. Every command must exit zero and the archive verifier must find the exact pinned LICENSE bytes in every release archive. Stage the project-license claim/proof, Applied/Implemented history, authored docs/changelog, and generated outputs. Require `./awf check staged` and `./x gate`, then commit a freshly SSH-signed commit:

```commit
docs(adr): implement AGPL project licensing
```

## Phase 7: Obtain acceptance and retire temporary local presentation

**Execution mode: inline.**
Completes: ["recovery-cleanup-accepted"]

### Task 7.1: Present the complete validation digest for explicit acceptance
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Resolve `TXN_ROOT` to the accepted protected live-transaction directory recorded by Phase 4 Notes and require its manifest, mapping verifier, policy-target enumerator, recovery hashes, and cleanup command generator to match the hashes recorded at Phase 4 close. Rerun the exact manifest, structural, content, identity, signature, policy, render, gate, release-package, Git fsck, worktree, hook, and GitHub-protection commands from Phases 4 through 6. Present the unchanged published boundary, active old-to-new ref map, backup-ref set, external recovery hashes/location, retained notices, effective identity/key fingerprint, policy target coverage, and every approved deviation to the user without exposing private or sensitive rights evidence. Stop and obtain explicit acceptance before deleting any temporary backup ref or active recovery presentation.

### Task 7.2: Atomically remove accepted temporary refs and prove the final active universe
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

After acceptance, generate the complete cleanup command set from the accepted manifest and feed it to one `git update-ref --stdin` transaction. Every delete carries its exact expected-old OID; any mismatch aborts the entire cleanup set without partial deletion. Remove only temporary backup and recovery-only refs classified for deletion. Do not delete remote-tracking evidence, the external bundle, retained dirty-state archives, or any operational custom ref. Do not expire reflogs or run aggressive GC.

Re-enumerate all refs, pseudorefs, worktrees, indexes, and uncommitted paths; require every item to match its accepted disposition and no ordinary active ref to present the superseded unpublished MIT graph. Run `git fsck --full`; `"$TXN_ROOT/rehearsal/bin/verify-mapping" --manifest "$TXN_ROOT/live-manifest.json" --repository . --accepted-cleanup`; `mapfile -t POLICY_TARGETS < <("$TXN_ROOT/rehearsal/bin/list-policy-targets" "$TXN_ROOT/live-manifest.json") && ./awf check commit-policy "${POLICY_TARGETS[@]}"`; `./x render`; `./x check`; and `./x gate`. Every command must exit zero. Record acceptance provenance, the atomically removed expected-old set, surviving external recovery artifacts, and terminal assertions in Notes.

### Phase close

Stage only the plan Notes and any generated index/lock effect. Require `./awf check staged` and `./x gate`, then commit a freshly SSH-signed commit:

```commit
docs(plans): record migration acceptance
```

## Phase 8: Settle terminal review and freeze the plan

**Execution mode: inline.**
Completes: ["terminal-records-frozen"]

### Task 8.1: Run terminal implementation review over mapped and post-cutover history
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md"]

Invoke `awf-reviewing-impl` over every implementation commit and the rewritten commit range identified by the accepted old-to-new map. Review the mapped tree/content/topology/signature evidence rather than assuming pre-rewrite OIDs still identify reviewed objects. Route findings under the review skill: apply mechanical and reasoned fixes in new green commits, stop for user-decision findings, and renew review after any integration or corrective change that alters behavior or mapped history. This task completes only when terminal review settles with every Definition-of-done item established.

### Task 8.2: Freeze the reviewed plan and generated lifecycle index
Latitude: exact
Paths: ["docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

After Task 8.1 settles, change this plan to `status: Implemented` and record final deviations in Notes. Both linked ADRs must already be Implemented from Phases 5 and 6 with every state operation Applied; do not append another ADR history event or change frozen Decision meaning. Run `./x render` and explicitly stage the plan, generated decision index, and lock.

### Phase close

Run `./x render`, `./x check`, `git diff --check`, and `git status --short`; render/check/diff must exit zero and status may list only the intended plan/index/lock transaction before staging. Stage exactly `docs/plans/2026-08-02-relicense-unpublished-history-and-enforce-commit-provenance.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock`; require `./awf check staged` and `./x gate`, then commit a freshly SSH-signed commit:

```commit
docs(plans): complete licensed provenance migration
```

## Definition of done

- `dod: copied-migration-proven` A mode-0700 copied repository containing the complete real object/ref universe has executed the full cutover, hook refusal, linked-worktree, recovery, and restore paths with an exact successful mechanism and command sequence recorded in Notes; the live repository was untouched by that rehearsal.
- `dod: relicensing-authority-cleared` Every selected contribution, author/committer identity, imported file, and retained notice has a recorded cleared disposition, and no unresolved authority or notice case is inferred from Git metadata.
- `dod: recovery-proven` A verified external full bundle plus restorable per-worktree/index artifacts exists outside the checkout, a second restore passed the complete verifier, and retention continues beyond temporary-ref cleanup.
- `dod: policy-config-valid` Optional commitPolicy config, exact validation, public field documentation, and one byte-preserving schema migration work for absent and enabled adopters without inventing policy values.
- `dod: common-verifier-complete` One project-composed verifier and `awf check commit-policy` deduplicate explicit target/range commits after the baseline, evaluate exact author/committer and allowed SSH signatures, and return stable complete typed diagnostics without mutation.
- `dod: generic-hook-enforcement-complete` Five inert hook payloads render publication-safely; reference-transaction rejects nonconforming introduced branch commits before ref movement, pre-push checks branches and tags before the gate, and linked worktrees use their invoking-worktree policy.
- `dod: published-history-unchanged` Every advertised remote object and all ancestry through the derived final published MIT boundary remain byte-for-byte unchanged and no remote ref was force-moved by this plan.
- `dod: unpublished-dag-recreated` Exactly one pinned AGPL boundary commit precedes one topology-preserving old-to-new mapping of every selected unpublished commit/ref, with intended LICENSE/README differences only and every ref/worktree/pseudoref/index/path disposition resolved.
- `dod: identity-and-signatures-correct` No policy-era reachable author or committer uses `T <t@example.com>`, audited genuine third-party identities remain unchanged, and every recreated commit plus moved annotated tag verifies with the approved SSH signer.
- `dod: project-license-agpl` Canonical LICENSE bytes, README badge/footer, release archives, project references, retained third-party notices, current-state claim, and `TestProjectLicenseAGPL` all prove AGPL-3.0-only without recategorizing dependency metadata.
- `dod: repository-policy-active` Root commitPolicy uses the unchanged published boundary, exact owner identity, and proven public SSH key; reference-transaction and pre-push are wired; every worktree resolves approved identity/signing settings; and GitHub main still requires signed commits with force pushes/deletion disabled.
- `dod: recovery-cleanup-accepted` The user accepted the complete validation digest, only classified temporary local backup/recovery refs were removed with expected-old checks, no active ref presents superseded unpublished history, and the external recovery bundle remains retained.
- `dod: terminal-records-frozen` Independent terminal implementation review settled the mapped and post-cutover history, both ADRs are Implemented with every operation Applied, and this plan is frozen as Implemented in a green signed transaction.

## Notes

Task-level `Applying` references are intentionally omitted. Both linked ADRs predate current-state-v4, remain mutable while Proposed/Accepted under their retained format, and therefore expose no stable Decision selector that plan-v2 may resolve; their plan-level `adrs:` links and the exact numbered commitments stated in each task preserve reviewable scope until the ADRs freeze. Do not manufacture V4 slugs or use mutable `#N` selectors merely to silence Proposed-plan assignment notes.

Phase 1 records the copied-repository spike answer here before any dependent implementation phase starts. The answer must include the selected mechanism and exact command sequence, sanitized evidence-root identifier, tool versions, published-boundary derivation method, ref/worktree disposition schema, expected-old transaction form, old-to-new map verification command, SPDX verification command, signature/tag verification command, recovery-restore command, and every passed or failed assertion. Later phases may replace no part of that sequence silently; a changed mechanism or failed live precondition is material authority drift and requires a user check-in.

The live migration, hook/policy activation, project-license terminalization, acceptance, and terminal-freeze phases are deliberately inline. Subagents and helpers may implement or review bounded capability code only where a phase explicitly permits it; they must not own quiescence, external rights evidence, signing material, live ref movement, policy activation, user acceptance, backup-ref deletion, terminal lifecycle, or the effort memory. No implementation begins while this plan is being authored or reviewed.

Phase 8 owns terminal review and the plan freeze. The policy ADR terminalizes in Phase 5 only after its repository-activation commitment is true; the license ADR terminalizes with its sole Applied operation in Phase 6, before acceptance. Phase 8 must not append duplicate ADR events. After its closing commit, follow the governed managed-worktree integration path, renew implementation review if integration changes behavior or mapped history, remove managed topology only after integration settles, and run `awf-retrospective` last. The external recovery bundle is retired only by a later deliberate user action, not by plan completion.
