---
format: plan-v2
date: 2026-08-26
adrs: [serialize-project-mutations-by-physical-root, bind-repository-and-release-acceptance-to-exact-revisions, bind-implementation-subagent-execution-to-the-selected-checkout, require-complete-and-unambiguous-mutable-authority, bind-terminal-plan-closure-to-selected-implementation-evidence]
status: Proposed
---
# Plan: Close v0.40.0 Review Baseline

## Goal

Close the credible safety, acceptance, execution-binding, and mechanical workflow defects from the
v0.40.0 full review, with durable adversarial evidence and honest adopter-facing residual-risk
statements. Preserve focused package ownership, linked-worktree tracked concurrency, the stable main
Pi session root, revision-independent handshake-gated adopter pi-tools, existing release targets, and
verification strength. Do not redesign coverage, add optional assurance lanes, broaden plan grammar,
add Core delegation, redesign effort-free consent transfer, retire punctuation machinery, build a
new unrelated-defect router, expand platform support or provenance, or claim the remaining parent
session and runtime reproducibility risks are closed.

## Architecture summary

Extend `internal/filesystem` as the single neutral owner of root confinement and the shared
persistent advisory-lock mechanism. Tracked-tree mutation leases bind to the selected checkout;
primary-resident leases bind to the shared physical resident root; operations that touch both acquire
canonical identities in deterministic order. Focused operation packages continue to own their use
cases and recovery policy, while Publisher continues to own output planning and lock-last
publication. Every operation acquires its applicable lease before loading mutable authority and
holds it through a typed complete or partial outcome. Existing upgrade journaling remains the
crash-recovery owner rather than being replaced by the lease.

Make the materialized staged candidate the local behavioral-test authority and carry its original
HEAD-to-index selection evidence explicitly. Release verification selects mutations over the prior
tag through the candidate revision and binds publication to successful required CI conclusions for
the exact tagged SHA. Stabilize repository workflow identities before applying and verifying hosted
required-check and release-protection settings; repository tests prove wiring, while operator
evidence alone proves live GitHub state.

Use the already validated optional `verificationCheckout` as both the implementation child's base
CWD and its before-and-after commit-policy identity. Omission keeps root execution and verification;
no profile infers an effort. The parent Pi session stays rooted at the project root, so its
pre-integration mutations remain explicitly path-targeted and the known issue remains open. The
adopter pi-tools runtime remains unpinned and available only after a successful protocol handshake;
the pinned `pi-tools/testing` dependency remains test-only.

Close the remaining approved defects at their current owners: strict authority parsers and recovery,
terminal plan freeze enforcement, structural workflow pin parsing, NUL-safe Git path transport,
portable release metadata, and reviewer/workflow contract contradictions. The review surface evaluates protected contract, Definition of done, current
authority, and reconciled material instructions rather than original plan choreography. Broader
policy choices identified by the review remain explicitly deferred.

Before the first application transaction for each linked ADR, move it through Accepted to
Implementing by the ordinary lifecycle handshake. Apply each State change only with the transaction
that makes it true. Keep all five linked ADRs Implementing and this plan Proposed through
implementation assurance; effort finalization owns terminal status-only closure after integration
and verified live settings.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Make mutable authority fail closed

**Execution mode: subagent-driven.**

Advances: ["strict-authority-inputs"]

### Task 1.1: Bind upgrade journals to their committed lock and complete input
Applying: ["require-complete-and-unambiguous-mutable-authority:mutable-authority-fails-closed"]
Paths: ["internal/upgrade/journal.go", "internal/upgrade/journal_test.go"]
Post-check: "Observe focused adversarial recovery tests fail before production changes, then require `go test ./internal/upgrade` to exit zero. The terminal matrix rejects trailing JSON, a final operation that is not the lock replacement, a missing or mismatched final-lock digest, invalid operation ordering, and a forged committed hash before any recovery mutation. Rejected recovery preserves quarantined and project bytes; valid precommit rollback and postcommit cleanup remain convergent. Run the journal parser fuzz target over phase, digest, operation-order, truncation, and trailing-document mutations without a panic, mutation, or accepted unbound journal."

Make strict EOF and final-operation/hash binding part of `ParseJournal` validation before recovery can
trust `FinalLockSHA256`. Exercise the real quarantine and cleanup boundary rather than only the pure
parser. Preserve generic supported-upgrade rollback, lock-last commit, and unusable-journal Git
recovery guidance.

### Task 1.2: Reject ambiguous live locks and multi-document YAML
Applying: ["require-complete-and-unambiguous-mutable-authority:mutable-authority-fails-closed"]
Paths: ["internal/manifest/manifest.go", "internal/manifest/manifest_test.go", "internal/config/config.go", "internal/config/config_test.go", "internal/config/tree_reader_test.go", "internal/publisher"]
Post-check: "Observe focused parser tests fail before implementation, then require manifest, config, tree-reader, and publisher tests to exit zero. Live locks reject unknown fields, trailing JSON, empty permanent inventory, malformed or duplicate inventory entries, and a misspelled `files` key before Publisher backs up or changes any output. Lock absence remains the only adoption path. Working-tree and snapshot config and every sidecar accept exactly one YAML document, reject a valid second document and trailing non-comment content, retain strict known-field behavior, and leave source bytes unchanged on refusal."

Centralize strict one-document YAML decoding at the config owner and reuse it for working and snapshot
config and sidecars. Keep historical audit decoding separate from the strict live-lock contract.
Require a nonempty permanent live inventory without turning lock absence into corruption.

### Phase close

Land strict journal, manifest, config, and sidecar authority parsing with adversarial no-mutation
recovery evidence.

```commit
fix(awf): reject ambiguous mutable authority
```

## Phase 2: Settle strict mutable-authority documentation

**Execution mode: inline.**

Completes: ["strict-authority-inputs"]

### Task 2.1: Apply strict mutable-authority claims after Phase 1 review
Applying: ["require-complete-and-unambiguous-mutable-authority:mutable-authority-fails-closed"]
Paths: ["internal/config/config_test.go", "internal/manifest/manifest_test.go", ".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/tooling/upgrade-runtime/current-state.md", "docs/decisions/require-complete-and-unambiguous-mutable-authority.md", "changelog/CHANGELOG.md"]
Post-check: "Move the reviewed ADR through Accepted to Implementing, update exactly `config/configuration:root-sidecar-keys-rejected`, `config/migrations-and-locks:corrupt-lock-refuses`, and `tooling/upgrade-runtime:upgrade-failure-is-recoverable`, add named test proof markers for the two test-backed claims, retain the upgrade claim as unbacked with strengthened verification, append one matching Applied event, and render. Current claims state complete one-document config and sidecars, closed nonempty live locks, and journal-to-final-lock binding before recovery mutation. The Unreleased changelog names the fail-closed authority boundary; drift, staged transition checks, and the full gate pass."

This is the focused post-review settlement for the already landed parser implementation. Keep all
three operations in one lifecycle transaction so current authority and the ADR history agree.
Renewed Phase 1 assurance must settle after this transaction before Phase 3 begins.

### Phase close

Land ADR-backed strict-authority claims, proof markers, and release-note currency.

```commit
docs(awf): settle strict mutable authority (applies ADR batch)
```

## Phase 3: Establish root-scoped mutation transactions

**Execution mode: subagent-driven.**

Advances: ["root-scoped-mutation-safety"]
Completes: ["shared-lease-owner"]

### Task 3.1: Extract the shared physical-root lease mechanism
Applying: ["serialize-project-mutations-by-physical-root:lease-by-physical-root", "serialize-project-mutations-by-physical-root:lease-covers-authority-to-outcome", "serialize-project-mutations-by-physical-root:preserve-focused-policy-owners"]
Paths: ["internal/filesystem", "internal/adr/scaffold_lock.go", "internal/adr/scaffold_lock_test.go", "internal/testsupport/fsfixture", ".awf/topics/parts/tooling/filesystem-access/current-state.md", "docs/decisions/serialize-project-mutations-by-physical-root.md"]
Post-check: "Focused filesystem and subprocess tests prove canonical-root and symlink-alias identity, restrictive persistent lock-file creation, same-root cross-process exclusion, independent linked-checkout tracked leases, deterministic dual-root acquisition without deadlock, context-aware waiting, explicit release, and process-exit release. A terminal source census proves production advisory-lock acquisition has one neutral implementation and ADR allocation only configures it. Add the filesystem claim with direct proof and its matching ADR Applied event in the same transaction; render and drift checks pass."

Use the existing canonical identity, persistent lock file, process-held advisory lock, and release
mechanics rather than creating a second lock protocol. Keep fault injection in the existing
`internal/testsupport/fsfixture` owner. In this first application transaction, move the mutation ADR
through Accepted to Implementing and add
`tooling/filesystem-access:root-scoped-project-mutation-leases` with direct invariant proof.

### Task 3.2: Add confined transaction primitives without absorbing policy
Applying: ["serialize-project-mutations-by-physical-root:confined-transaction-primitives", "serialize-project-mutations-by-physical-root:preserve-focused-policy-owners"]
Paths: ["internal/filesystem/handle.go", "internal/filesystem", "internal/filepublication", "internal/config"]
Post-check: "Focused tests prove confined observation and stat, expected-identity atomic replacement, exclusive authored creation, rename and removal, parent-symlink and parent-swap refusal, same-directory replacement, wrapped error identity, and no outside-root mutation. Production reachability proves every new primitive has a real operation consumer in this phase or Task 3.3 and no policy-specific result type moved into `internal/filesystem`."

Extend the existing `filesystem.Handle` and `filepublication` seam only as required by actual
operations. Keep domain, local-document, ADR, upgrade, effort, and publication recovery policy at
their focused owners.

### Task 3.3: Put Publisher preparation and lock-last publication inside the lease
Applying: ["serialize-project-mutations-by-physical-root:lease-by-physical-root", "serialize-project-mutations-by-physical-root:lease-covers-authority-to-outcome", "serialize-project-mutations-by-physical-root:preserve-focused-policy-owners", "serialize-project-mutations-by-physical-root:explicit-partial-outcomes"]
Paths: ["internal/publisher", "internal/resident", "internal/testsupport/publishing_ownership_test.go", "cmd/awf/sync.go", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", "docs/decisions/serialize-project-mutations-by-physical-root.md"]
Post-check: "Publisher owner and fault tests prove the applicable tracked and resident leases are held before mutable lock, output, backup, prune, and resident observation; remain held through output planning, mutation, result construction, and final lock replacement; preserve independent tracked-only worktrees; and acquire both roots deterministically when required. A failure after any committed effect returns and presents the complete stable partial result and recovery action. The final lock remains last, and no public mutate-from-stale-Preparation path bypasses the lease. Render and drift checks pass with the sync claim updated and the matching ADR Applied event appended."

Preserve Publisher as the owner of output planning, tracked/resident routing, backup, prune, stable
result ordering, and lock-last policy. Do not make the lease crash-atomic and do not replace the
upgrade journal.

### Phase close

Land the shared root lease, confined mutation primitives, and leased Publisher transaction boundary.

```commit
feat(awf): serialize root-scoped publication (applies ADR batch)
```

## Phase 4: Move project operations under the transaction boundary

**Execution mode: subagent-driven.**

Completes: ["root-scoped-mutation-safety", "complete-mutation-outcomes"]

### Task 4.1: Convert tracked authority and authored-source operations
Kind: batch
Applying: ["serialize-project-mutations-by-physical-root:lease-by-physical-root", "serialize-project-mutations-by-physical-root:lease-covers-authority-to-outcome", "serialize-project-mutations-by-physical-root:confined-transaction-primitives", "serialize-project-mutations-by-physical-root:explicit-partial-outcomes"]
Paths: ["internal/domainop", "internal/localdocop", "internal/topicop", "internal/topic", "internal/adr", "internal/plan", "internal/currentstatecoord", "internal/initop", "internal/project", "internal/upgrade", "cmd/awf"]
Representative: "A domain or local-document operation acquires the selected-checkout lease before loading `.awf/config.yaml`, checks the loaded identity before atomic replacement, exclusively creates authored input, synchronizes while still leased, and returns every committed effect if a later step fails."
Edge: "ADR numbering that publishes a destination and then cannot retire the pending source reports the exact assignment, both path states, and the safe retry or recovery action instead of losing the committed destination."
Post-check: "For the complete production call population under the named packages, a source and call-graph census leaves no direct host-path mutation of selected-checkout config, authored sources, or generated outputs outside the confined transaction boundary. Focused red-then-green tests cover same-authority concurrent updates, stale config identity, authored-file collision, parent symlink swap, destination-success/source-remove-failure numbering, Publisher late failure, init cleanup, topic rollback, and supported-upgrade journal recovery. Every refusal either preserves the pre-command tree digest or returns a typed complete partial outcome; command output retains every committed fact."

Acquire before config, corpus, template, destination, or publication planning. Preserve focused
rollback semantics and operation result models. Configure ADR allocation through the shared lease
rather than retaining a parallel lock implementation.

### Task 4.2: Convert shared-resident and dual-root lifecycle operations
Kind: batch
Applying: ["serialize-project-mutations-by-physical-root:lease-by-physical-root", "serialize-project-mutations-by-physical-root:lease-covers-authority-to-outcome", "serialize-project-mutations-by-physical-root:confined-transaction-primitives", "serialize-project-mutations-by-physical-root:explicit-partial-outcomes"]
Paths: ["internal/effort", "internal/effortop", "internal/worktree", "internal/resident", "internal/git", "cmd/awf/effort.go", "cmd/awf/uninstall.go"]
Representative: "A memory edit or archive reservation takes only the shared resident lease; a worktree operation that changes tracked and resident state takes both canonical leases in the shared order."
Edge: "A partially completed effort finish or worktree removal retains its focused typed recovery action and exact committed topology facts; the neutral filesystem package never decides lifecycle recovery."
Post-check: "Cross-process integration tests exercise same-checkout tracked contention, independent linked-worktree tracked mutation, shared-resident contention from distinct worktrees, deterministic dual-root lifecycle mutation, and process death while holding either lease. Memory publication, effort finish, worktree add/remove, resident uninstall, archive uncertainty, and retry tests remain green. A production mutation census classifies every selected-checkout or resident writer under tracked, resident, dual-root, intentionally Git-owned integration, or read-only behavior with no unexplained direct writer."

Keep Git integration as the reconciliation authority for independent checkout trees. Do not route
governed primary-checkout integration, deferred lifecycle closure, topology removal, retrospective,
or finish through an effort worktree.

### Task 4.3: Apply operation and CLI authority changes
Applying: ["serialize-project-mutations-by-physical-root:explicit-partial-outcomes"]
Paths: [".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", "docs/decisions/serialize-project-mutations-by-physical-root.md", "changelog/CHANGELOG.md"]
Post-check: "Apply the two remaining mutation ADR updates to `tooling/cli:cli-creation-and-inventory` and `tooling/cli:domain-lifecycle-commands`, append their matching Applied event, and render. Semantically recheck the already-applied `rendering/sync-and-drift:sync-mutations-root-confined` claim without duplicating its operation. Current claims and changelog describe typed complete or partial results without claiming crash atomicity. Focused presentation tests, semantic review of every changed command example, `./awf context --show pending` for the ADR, render, drift, and staged checks report no unapplied operation or contradictory generated prose."

### Phase close

Land leased tracked, resident, and dual-root operations with confined writes and complete outcomes.

```commit
feat(awf): transact project mutations (applies ADR batch)
```

## Phase 5: Bind local and release acceptance to exact candidates

**Execution mode: subagent-driven.**

Completes: ["staged-candidate-authority", "release-revision-authority", "acceptance-input-integrity"]

### Task 5.1: Run selected behavior against the materialized staged candidate
Applying: ["bind-repository-and-release-acceptance-to-exact-revisions:staged-candidate-is-test-authority", "bind-repository-and-release-acceptance-to-exact-revisions:evidence-based-test-selection"]
Paths: ["x", "internal/project/gate_runner_test.go", "cmd/awf/checkstaged.go", ".awf/docs/parts/testing/gate.md", "docs/testing.md", ".awf/topics/parts/tooling/quality-gates/current-state.md"]
Post-check: "Before production changes, a staged broken candidate plus an unstaged compensating edit makes the new regression fail for the right reason. After implementation, the gate runs affected behavioral lanes against materialized staged bytes and preserves the original parent-to-index changed set explicitly through selection. Add, modify, delete, rename, empty, malformed, and unavailable-index matrices fail closed. README, architecture, changelog, generated documentation, and every repository input consumed by Go or Pi tests select the owning behavioral lane; only dependency-proven inputs skip it. Focused tests, staged checks, and semantic review of testing guidance pass."

In the first application transaction, move the exact-revision ADR through Accepted to Implementing.
Do not commit the temporary candidate in a way that erases selection evidence. Remove false broad
documentation-family exemptions rather than replacing them with another unproved allowlist.

### Task 5.2: Make release mutation and publication evidence exact-revision based
Applying: ["bind-repository-and-release-acceptance-to-exact-revisions:exact-main-revision", "bind-repository-and-release-acceptance-to-exact-revisions:exact-tagged-revision", "bind-repository-and-release-acceptance-to-exact-revisions:release-range-mutation-selection"]
Paths: [".github/workflows/ci.yml", ".github/workflows/release.yml", "x", "internal/project/gate_runner_test.go", "cmd/releasecheck", ".awf/docs/parts/releasing/content.md", ".awf/topics/parts/tooling/changelog-and-release/current-state.md"]
Post-check: "Workflow contract tests prove stable `CI / gate` and `CI / release-config` identities, explicit event base/head selection, previous-release-tag-to-candidate mutation selection, refusal when endpoints or exact-SHA conclusions cannot be established, tag checkout identity, verification before credential-bearing publication, and no ancestry-only acceptance path. A release fixture whose tagged SHA lacks either required successful conclusion refuses before GoReleaser; the exact successful SHA proceeds. Release guidance names the same identities and sequence. Focused workflow/release tests and releasecheck pass."

Keep local staged and CI range selection as two inputs to one fail-conservative selector. Split or
narrow credential lifetime where required for the exact publication boundary, but do not expand this
phase into attestation, immutable-release, or platform-support redesign.

### Task 5.3: Parse workflow pins and changed paths as structured data
Paths: ["cmd/pincheck/main.go", "cmd/pincheck/main_test.go", "internal/git/walk.go", "internal/git/walk_test.go", "cmd/repoaudit"]
Post-check: "Observe focused fixtures fail before implementation, then require pincheck, Git, and repoaudit tests to pass. Pincheck structurally associates action `uses` and GoReleaser `version` with the same workflow step across quoted keys, legal spacing, nested `with`, unrelated versions, comments, and malformed YAML. Range path transport uses `git diff -z` and exact NUL parsing; newline, tab, quote, backslash, Unicode, and leading-dash filenames round-trip byte-exactly and trigger the same repoaudit ownership rules as ordinary names."

Use the repository's existing YAML owner rather than another line parser. Keep path-prefix policy at
repoaudit and transport decoding at the Git boundary.

### Task 5.4: Apply repository-wiring current-state claims
Applying: ["bind-repository-and-release-acceptance-to-exact-revisions:exact-main-revision", "bind-repository-and-release-acceptance-to-exact-revisions:staged-candidate-is-test-authority", "bind-repository-and-release-acceptance-to-exact-revisions:evidence-based-test-selection", "bind-repository-and-release-acceptance-to-exact-revisions:exact-tagged-revision", "bind-repository-and-release-acceptance-to-exact-revisions:release-range-mutation-selection"]
Paths: [".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/metadata/tooling/quality-gates.yaml", ".awf/topics/parts/tooling/changelog-and-release/current-state.md", "docs/decisions/bind-repository-and-release-acceptance-to-exact-revisions.md", "changelog/CHANGELOG.md"]
Post-check: "Apply the updates to `tooling/quality-gates:covercheck-mutation-regression`, `tooling/quality-gates:staged-test-selection`, and `tooling/changelog-and-release:release-gate-on-tag`, plus the test-backed add `tooling/quality-gates:exact-revision-repository-acceptance`; append matching Applied events and render. Every invariant claim resolves to direct workflow-contract tests and fails under a controlled wiring falsification. `./awf context --show pending` reports only the two hosted-setting operations reserved for Phase 9."

### Phase close

Land staged-candidate behavioral authority, exact-SHA repository and release wiring, structural pin
validation, and NUL-safe range paths.

```commit
feat(tooling): bind acceptance to exact revisions (applies ADR batch)
```

## Phase 6: Bind implementation children to the selected checkout

**Execution mode: subagent-driven.**

Advances: ["adopter-status-currency"]
Completes: ["selected-checkout-execution"]

### Task 6.1: Use one validated checkout for child execution and verification
Applying: ["bind-implementation-subagent-execution-to-the-selected-checkout:selected-checkout-governs-child-execution", "bind-implementation-subagent-execution-to-the-selected-checkout:omitted-checkout-retains-root-default"]
Paths: ["templates/pi/awf-subagents/index.ts.tmpl", "internal/publisher/target_test.go", "tools/pi-extension-test"]
Post-check: "Before the template change, a recorder test fails because an explicit managed `verificationCheckout` still starts the child at the project root. After rendering, explicit-checkout calls validate one canonical live checkout, use it as both child base CWD and before/after commit snapshot identity, refuse invalid or runtime-inaccessible paths before dispatch, and retain root execution plus root verification when omitted. Tests cover relative writes, deliberately targeted outside paths without a false confinement claim, role loading, diagnostics, failed child results, unchanged-HEAD policy, and linked-worktree identity. Missing optional data renders coherently with no no-value token."

In this first application transaction, move the selected-checkout ADR through Accepted to
Implementing. Preserve the current exact checkout validator and the runtime's existing descendant
confinement rather than adding a second execution-checkout field.

### Task 6.2: Make effort-backed callers supply the managed checkout explicitly
Kind: batch
Applying: ["bind-implementation-subagent-execution-to-the-selected-checkout:effort-workflows-supply-managed-checkout", "bind-implementation-subagent-execution-to-the-selected-checkout:parent-session-root-remains-stable"]
Paths: ["templates/skills/effort-workflow/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/partials/checkpoint-routine.md", ".awf/skills", "internal/project/plan_execution_workflow_template_test.go", "internal/publisher/target_test.go"]
Representative: "An effort-backed implementation dispatch passes the exact managed-worktree path as `verificationCheckout`; a root-owned integration or effort-free call omits it intentionally."
Edge: "The main Pi session stays at repository root, so parent shell and file mutations name the managed-worktree path explicitly and never claim child CWD alignment confines an absolute target."
Post-check: "A deterministic source and rendered-output sweep classifies every implementation dispatch as explicit managed checkout or intentional root default, with no inferred effort, activity, branch, or topology routing. Pi and Claude workflow prose preserves the governed primary-checkout transition for integration, terminal closure, topology removal, retrospective, and finish. Focused semantic tests reject any claim that the parent session moved or all mutations are confined."

### Task 6.3: Apply checkout execution authority and document the residual
Applying: ["bind-implementation-subagent-execution-to-the-selected-checkout:selected-checkout-governs-child-execution", "bind-implementation-subagent-execution-to-the-selected-checkout:omitted-checkout-retains-root-default", "bind-implementation-subagent-execution-to-the-selected-checkout:effort-workflows-supply-managed-checkout", "bind-implementation-subagent-execution-to-the-selected-checkout:parent-session-root-remains-stable"]
Paths: [".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", ".awf/parts/pi-runtime-reference", "docs/known-issues.md", "docs/decisions/bind-implementation-subagent-execution-to-the-selected-checkout.md", "changelog/CHANGELOG.md"]
Post-check: "Apply updates to all four State-change claims named by the ADR, append matching Applied events, and render. Current authority and adopter guidance distinguish child CWD alignment from confinement, state the explicit parent-path mitigation prominently, retain the wrong-checkout known issue because its full completion criteria do not hold, and retain the governed primary-checkout lifecycle transition. Semantic review covers the profile, runtime reference, workflow skills, known issue, changelog, and both target renderings; drift and staged checks pass."

Retain the unpinned adopter pi-tools runtime and its protocol handshake. Preserve the pinned
`pi-tools/testing` source-only test dependency and prevent any runtime import or claimed adopter pin.

### Phase close

Land explicit-checkout child execution, caller propagation, current authority, and honest residual
risk guidance.

```commit
feat(rendering): bind child checkout (applies ADR batch)
```

## Phase 7: Reconcile reviewer, plan, and workflow contracts

**Execution mode: subagent-driven.**

Completes: ["review-contract-consistency", "terminal-plan-freeze", "workflow-wording-consistency"]

### Task 7.1: Make code review follow the protected contract instead of original choreography
Paths: ["templates/agents/code-reviewer.md.tmpl", "internal/catalog/standard.go", ".awf/agents/code-reviewer.yaml", "internal/evals/plan_flexibility_test.go", "internal/publisher/target_test.go"]
Post-check: "Semantic negative evals fail before correction for the shipped `all plan tasks` and `stated file paths and content` synonyms, then pass after both are absent from canonical sources and every rendered Pi and Claude reviewer. Positive evals require review of protected contract, Definition of done, current authority, reconciled material instructions, unexplained outcome or authorization drift, coherent landed transactions, and incomplete work without requiring original task, path, content, phase, or commit identity. A complete rendered-output sweep has no equivalent choreography requirement."

Remove only reviewer contradictions from AWF2-004. Preserve green checkpoints, ownership,
verification, and protected-contract outcomes. Broader plan-authoring grammar and evidence-based
conditional tasks remain deferred rather than being changed indirectly here.

### Task 7.2: Require complete findings and explicit review authority
Kind: batch
Paths: ["templates/partials/review-spine-tail.md", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "internal/evals", "internal/project", "internal/publisher/target_test.go"]
Representative: "A review with findings emits the bounded digest followed by the complete structured finding array; supporting narrative may remain optional."
Edge: "Standalone ADR review without an explicit selected path refuses instead of choosing by modification time; this task does not redesign effort-free plan consent transfer."
Post-check: "Semantic fixtures fail before correction for an omitted non-user-decision finding and implicit most-recent ADR selection, then pass with every mechanical, reasoned, and user-decision finding present exactly once in the structured array and an explicit ADR path in the review brief. Pi and Claude outputs contain no modification-time authority selection or optional-exhaustive-inventory wording. Existing one-verify-pass and lifecycle routing contracts remain green."

### Task 7.3: Enforce terminal plan immutability and reconciliation
Applying: ["bind-terminal-plan-closure-to-selected-implementation-evidence:implemented-plan-is-frozen-history", "bind-terminal-plan-closure-to-selected-implementation-evidence:closure-is-bound-to-selected-implementation-evidence", "bind-terminal-plan-closure-to-selected-implementation-evidence:terminal-validation-fails-closed"]
Paths: ["internal/plan", "internal/plancheck", "cmd/awf/checkstaged.go", "cmd/awf", "docs/known-issues.md", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md"]
Post-check: "Observe staged transition tests fail before implementation for an edited Implemented body and an unreconciled terminal flip. After correction, a selected before/after history proves an already Implemented plan is byte-stable, permits the authorized Proposed-to-Implemented status transaction only after actual touched-path and material-deviation reconciliation, rejects malformed or unavailable comparison evidence, and preserves ordinary Proposed-plan amendments. `awf check staged`, direct plancheck tests, and transition fixtures cover both boundaries without relying only on prose markers. Close the known issue only if its exact completion criteria and durable oracle both hold."

Implement at the plan and staged-transition owner, not as another writing-skill promise. Use existing
selected-tree evidence and artifact parsers rather than test-shaped production seams.

### Task 7.4: Align advisory and incidental-defect wording
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "internal/prosegate", "templates/docs/workflow.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "templates/skills/bugfix/SKILL.md.tmpl", "internal/evals", "README.md"]
Post-check: "Before production or prose correction, observe the focused CLI status/exit regression and semantic incidental-defect eval fail for the shipped blocking and unconditional-ownership wording. After correction, generated CLI help, README, workflow, and agent guidance consistently describe punctuation findings as advisory Warning/zero-exit behavior; focused command tests prove the status and exit code. Semantic workflow evals require immediate repair for defects caused by the transaction or blocking its safe completion, while unrelated concrete defects are recorded and routed separately without silently expanding the change. No new unrelated-defect system or punctuation retirement appears in the diff."

### Phase close

Land protected-contract code review, complete review output, explicit ADR selection, terminal plan
freeze enforcement, and mechanically consistent workflow wording.

```commit
fix(rendering): reconcile review and workflow contracts
```

## Phase 8: Make release archives portable

**Execution mode: subagent-driven.**

Completes: ["release-archive-portability"]

### Task 8.1: Normalize archive metadata for restricted extraction
Paths: [".goreleaser.yaml", ".github/workflows/release.yml", "cmd/releasecheck", "internal/project", ".awf/docs/parts/releasing/content.md", "changelog/CHANGELOG.md", "README.md"]
Post-check: "Before metadata correction, observe a restricted-rootless extraction fixture fail for the released ownership metadata rather than an unrelated container or permission setup defect. After correction, GoReleaser configuration and release-contract tests prove Linux archives carry normalized portable ownership and expected modes; the same fixture extracts the snapshot archive and finds exactly the expected binary, LICENSE, and README paths. Darwin and Windows archive shape, checksums, notes, and target matrix remain unchanged. Release guidance describes portable extraction, exact-SHA gating from Phase 5, and unchanged same-channel checksum limitations without claiming independent authenticity, behavioral cross-platform CI, immutable releases, attestations, or a pinned adopter pi-tools runtime. Releasecheck, snapshot smoke, render, drift, and focused prose checks pass."

This corrects metadata on an existing Linux release asset; it does not expand the release target or
platform-support boundary. Bootstrap cache hardening and broader platform and provenance work remain
deferred under the approved scope.

### Phase close

Land portable release archive metadata and accurate distribution security boundaries.

```commit
fix(tooling): normalize release archive metadata
```

## Phase 9: Apply hosted acceptance settings and prove the complete baseline

**Execution mode: inline.**

Completes: ["hosted-acceptance-enforcement", "clean-baseline-assurance", "adopter-status-currency"]

### Task 9.1: Apply and verify exact live GitHub acceptance settings
Applying: ["bind-repository-and-release-acceptance-to-exact-revisions:documented-hosted-state-is-verified"]
Paths: [".github/workflows/ci.yml", ".github/workflows/release.yml", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/parts/tooling/changelog-and-release/current-state.md", ".awf/docs/parts/releasing/content.md", "docs/decisions/bind-repository-and-release-acceptance-to-exact-revisions.md"]
Post-check: "Using authenticated GitHub API reads and writes against `hypnotox/agentic-workflows`, first capture the current ruleset, tag, environment, and required-check state. Apply only the stabilized Phase 5 identities: exact `CI / gate` and `CI / release-config` success for main candidates, plus the documented tag or protected-environment policy that prevents publication of a tagged SHA without its exact required conclusions. Read back every changed setting and require exact normalized equality with the intended policy; exercise a safe non-publishing verification of workflow/ref identity. On failure, preserve captured before/after evidence, report the unapplied setting, and do not claim or commit enforcement."

Hosted settings are an operator transaction, not repository-test backing. Do not weaken signed
commits or fast-forward policy. Do not perform destructive tag or release tests against a real
published version.

### Task 9.2: Apply hosted-state claims and publish final status
Applying: ["bind-repository-and-release-acceptance-to-exact-revisions:documented-hosted-state-is-verified", "bind-repository-and-release-acceptance-to-exact-revisions:exact-main-revision", "bind-repository-and-release-acceptance-to-exact-revisions:exact-tagged-revision"]
Paths: [".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/metadata/tooling/quality-gates.yaml", ".awf/topics/parts/tooling/changelog-and-release/current-state.md", ".awf/docs/parts/releasing/content.md", ".awf/parts/pi-runtime-reference", "docs/known-issues.md", "README.md", "changelog/CHANGELOG.md", "docs/decisions/bind-repository-and-release-acceptance-to-exact-revisions.md", "docs/plans/2026-08-26-close-v0-40-0-review-baseline.md"]
Post-check: "Only after Task 9.1 read-back succeeds, add `tooling/quality-gates:hosted-main-acceptance-settings` and `tooling/changelog-and-release:hosted-release-protection` as operator-verified rules with no false test backing, append the final Applied event, and render. Adopter-facing status names every material residual: unpinned handshake-gated pi-tools, explicitly path-targeted parent-session mutations and the still-open wrong-checkout issue, same-channel release integrity, deferred platform/provenance and coverage work, and any known issue whose completion criteria remain unmet. No deferred item is described as closed."

If live settings cannot be applied, leave the two operations Remaining, keep the ADR Implementing,
record the blocker in plan Notes and effort memory, and stop terminal closure.

### Task 9.3: Verify the staged final transaction
Paths: ["pathspec::(top)**", "docs/decisions/serialize-project-mutations-by-physical-root.md", "docs/decisions/bind-repository-and-release-acceptance-to-exact-revisions.md", "docs/decisions/bind-implementation-subagent-execution-to-the-selected-checkout.md", "docs/decisions/require-complete-and-unambiguous-mutable-authority.md", "docs/decisions/bind-terminal-plan-closure-to-selected-implementation-evidence.md", "docs/plans/2026-08-26-close-v0-40-0-review-baseline.md"]
Post-check: "Stage the complete Phase 9 transaction and materialize that exact index candidate while retaining its parent-to-index changed set and implementation-range endpoints. Against that candidate, run focused parser, recovery, filesystem, concurrency, operation, Publisher, gate-runner, workflow, release, profile, plancheck, archive, and semantic-render tests; `go test ./...`; fuzz smoke targets; `./x render`; `./x check`; `./awf check staged`; `./x gate`; releasecheck and GoReleaser config/snapshot verification; production reachability; binary-version validation; the complete-range repo audit; and the selected coverage and mutation blockers. All blocking checks exit zero, generated drift is empty, no new unreviewed coverage identity remains, every controlled falsification has been restored, live GitHub settings still match Task 9.1, and only explicitly documented nonblocking advisories remain. Re-read every staged mutation target before phase close and require the staged tree identity to remain unchanged from verification through commit."

Record candidate-independent commands, intended ranges, fuzz parameters, generated meaning-review
boundaries, rootless archive environment, live-setting normalized read-back, and residual
dispositions in Notes before final staging. After staging, retain the final tree identity, command
outputs, audit evidence, and unchanged-through-commit proof in ignored effort-local execution
evidence for postcommit phase and implementation review; do not mutate the verified candidate to
record its own identity. All five linked ADRs remain Implementing and the plan remains Proposed
through the closing commit.

### Phase close

Land operator-verified hosted claims, adopter status, release notes, and staged-candidate assurance
evidence without prematurely terminalizing the linked artifacts.

```commit
docs(awf): record v0.40 review baseline assurance (applies ADR batch)
```

After this commit exists, ordinary phase review and independent implementation assurance inspect the
exact complete implementation range against all five linked ADRs, this Definition of done, current
authority, and reconciled Notes. Any settlement commit receives its own required checks and renewed
review. Effort workflow performs integration and terminal lifecycle closure only after that
postcommit assurance settles.

## Definition of done

- `dod: strict-authority-inputs` Upgrade recovery accepts only EOF-complete journals whose final lock operation and digest agree, live locks are closed and nonempty, config and sidecars are exactly one YAML document in working and snapshot trees, and adversarial refusals mutate nothing.
- `dod: shared-lease-owner` One neutral canonical persistent advisory-lock mechanism provides checkout-local tracked and shared primary-resident leases, deterministic dual-root acquisition, cross-process serialization, and process-exit release without suppressing independent linked-worktree tracked mutation.
- `dod: root-scoped-mutation-safety` Every selected-checkout or resident mutation acquires all applicable physical-root leases before mutable authority loading and planning, uses confined identity-checked and exclusive primitives, holds the lease through publication, and leaves no unexplained direct writer.
- `dod: complete-mutation-outcomes` A failed mutation either preserves its pre-command tree or returns and presents every committed effect plus a safe retry or recovery action; Publisher remains lock-last and upgrade journaling remains the crash-recovery owner.
- `dod: staged-candidate-authority` Staged behavioral verification executes against materialized staged bytes with original selection evidence preserved, and only dependency-proven inputs skip their owning suites.
- `dod: release-revision-authority` Main and release acceptance bind required successful conclusions and mutation selection to the exact candidate SHA and previous-tag range, with structural workflow-pin validation and no ancestry-only publication path.
- `dod: hosted-acceptance-enforcement` Live main required checks and release tag or environment protection match documented stable identities by operator read-back, and repository documentation never claims an unverified hosted control.
- `dod: selected-checkout-execution` An explicit verification checkout is one validated identity for child base CWD and commit snapshots, omission retains root behavior, effort workflows supply the managed worktree, and the main session remains stable without a false confinement claim.
- `dod: review-contract-consistency` Code review evaluates protected contract, Definition of done, current authority, reconciled material instructions, and landed coherence rather than original plan task, path, content, phase, or commit choreography; all findings are structured and ADR selection is explicit.
- `dod: terminal-plan-freeze` Implemented plan bodies cannot change silently, and the terminal transition proves actual touched-path and material-deviation reconciliation through durable selected-tree evidence.
- `dod: workflow-wording-consistency` Punctuation is advisory everywhere, and incidental defect ownership distinguishes caused or transaction-blocking repairs from separately recorded unrelated defects without adding deferred policy machinery.
- `dod: acceptance-input-integrity` Workflow pins are parsed by YAML step identity and Git range paths are NUL-safe for adversarial filenames, preserving fail-closed audit and acceptance selection.
- `dod: release-archive-portability` Release archives preserve their target matrix and contents while normalized metadata extracts successfully under a restricted rootless user.
- `dod: adopter-status-currency` Current authority, workflow guidance, known issues, release documentation, runtime reference, README, and changelog state implemented behavior and every material residual without claiming closure of deferred reproducibility, parent-session, platform, provenance, coverage, or policy work.
- `dod: clean-baseline-assurance` Focused adversarial tests, full tests, fuzz smoke, render and drift, staged and full gates, release verification, rootless archive smoke, exact live-settings read-back, range audit, coverage policy, mutation selection, semantic generated-output review, and independent implementation assurance all settle at the exact candidate tip.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material
cross-owner revisions rather than editing the plan; the parent supplies the report to phase review
and reconciles required plan changes with findings in one focused post-review settlement commit
before checkpointing or later execution. Record ADR lifecycle and Applied batches, material route
deviations, red-then-green and falsification evidence, concurrency and recovery results, generated
meaning-review boundaries, hosted-settings before/after and read-back evidence, residual known-issue
dispositions, and terminal review findings here as execution proceeds.

Approved scope retains revision-independent handshake-gated adopter pi-tools and the pinned test-only
`pi-tools/testing` dependency. It mitigates but does not close parent-session wrong-checkout risk.
Broader plan-authoring grammar, evidence-based conditional work, Core delegation, effort-free consent
transfer, punctuation retirement, unrelated-defect routing, coverage redesign, optional assurance
lanes, bootstrap cache hardening, platform expansion, immutable releases, attestations, and other
provenance work remain outside this plan.

Phase 1 reconstructed red-first evidence by applying the complete test-only diff to detached baseline
`6ce178c9d`: focused config, manifest, Publisher, and recovery tests failed on ignored second YAML
documents, ambiguous or empty locks, mutation before lock refusal, and unbound journals. The green
implementation passes `go test ./...`; `FuzzParseJournal` passed 357,099 executions over six seconds
without a panic or invalid acceptance. Dirty-tree review found that the initial implementation limited
nonempty inventory enforcement to Publisher and did not independently prove duplicate JSON fields or
recovery immutability. Settlement moved the complete contract to the ordinary live-lock seam, rejected
duplicate outer and entry fields, compared the full recovery tree before and after refusal, added a
forged committed-hash case, strengthened the independent fuzz oracle, and updated synthetic live-lock
fixtures to carry inventory. Replacing the production digest with a constant made both the recovery
case and fuzz seeds fail against an independently computed SHA-256. This is an authority-determined
route correction, not a protected-contract change.

Phase 1 review found no implementation defect but identified three stale current-state claims,
missing direct proof markers, and a missing Unreleased changelog entry. Because current-state updates
require an ADR operation, the reviewed pending ADR
`require-complete-and-unambiguous-mutable-authority` was added after the code commit. At initial
review, Phase 1 was the only completed affected phase, at commit `517f8aade`; Phases 2 through 9 were
unstarted. Task 2.1 owns one focused lifecycle and documentation settlement, and renewed Phase 1
assurance must pass before Phase 3 progression. During Phase 2, the marker gate corrected the recorded route from impossible
production proof markers to named test-scoped proofs for the two backed claims; the unbacked upgrade
claim instead retains its strengthened `Verify:` evidence. Renewed Phase 1 and Phase 2 review found
that `manifest.Parse` and `LoadOptional` still bypassed permanent-inventory validation for live
consumers. The settlement makes the ordinary parser own that validation, adds an Uninstall regression
that proves ambiguous or empty inventory refuses without removing the generated file or lock, and
adds the missing snapshot-config proof marker. The Uninstall regression failed against the reviewed
range and passes with the seam correction. Renewed exact-range review then found two remaining
unambiguous-authority gaps: non-local lock inventory paths were skipped while other entries could
mutate, and duplicate fields at nested journal levels survived typed JSON decoding. Mechanical review
settlement rejects non-local or non-canonical inventory paths at the manifest owner, proves Publisher,
render, and Uninstall preserve state on refusal, rejects duplicate fields recursively before journal
decoding, and adds journal-level, operation-level, image-level, and fuzz evidence. All new regressions
failed against `0d0ad25fe` and pass with the settlement.

Initial plan review corrected one scope error and one lifecycle-ordering ambiguity. Bootstrap cache
hardening was removed because the approved outline explicitly deferred it. Archive metadata
normalization remains because it corrects restricted extraction of the existing Linux release asset
without expanding the supported target set. Final verification now binds to the staged Phase 9
candidate; ordinary phase review and independent implementation assurance bind to the resulting
postcommit range before integration. The verify pass added the filesystem claim's authored source
and removed a self-referential Notes requirement: final staged identity and outputs stay in ignored
execution evidence for postcommit review rather than changing the candidate they identify.

Phase 3 review settlement keeps expected-identity commit rooted at the selected filesystem after a
prepared parent is relocated, uses exclusive scaffold-directory creation so Init claims only its own
winner, and preserves complete Publisher plus Init effects through advisory and lease-release
failures. Direct claim markers bind the ADR adapter contention proof and the sole production lock
owner census. A Linux destination-disappearance regression now reaches the native exchange refusal,
replacing its prior coverage admission; the remaining native cleanup admissions name their exact
uninjectable branches. These changes preserve the approved filesystem and operation ownership
boundaries and require no protected-contract deviation.

The single Phase 3 verify pass found two remaining ownership races. Native expected mutation could
follow a symlink installed where a prepared parent had been relocated, transiently exchanging
outside-root entries before rollback; an inotify-backed regression observed both replacement and
removal touch the outside directory. Commit now resolves the immediate parent through the selected
`os.Root` and gives native exchange only leaf names. Init could also record a replacement directory
observed after its own `Mkdir`; directory creation now records the temporary directory's opened
identity before exclusive no-replace publication, with `internal/filepublication` retaining the
single released-platform publication owner. The scaffold keeps that returned identity without a
post-publication lookup. Both regressions pass, released-platform filesystem packages compile, and
this reasoned settlement preserves the approved confinement and focused-owner boundaries.

Phase 4 grounding added the omitted authored-source owners `internal/plan` and `internal/topic`, the
focused effort command-operation boundary `internal/effortop`, and `internal/git` for explicit
classification of intentionally Git-owned topology effects. The ADR already applied the filesystem
lease add and sync claim update in Phase 3, so Phase 4 applies only the two remaining CLI operations
and semantically rechecks the sync claim. These corrections preserve the approved phase outcome and
ownership boundary.

Phase 4 review found that its initial commits closed only part of the writer population and released
or acquired leases outside several authority-to-presentation boundaries. Settlement centralizes
effort scope selection, removes parallel unleased plan, ADR, topic, domain, and uninstall writers,
classifies exact topology and upgrade recovery effects, and keeps applicable leases through actual
success or diagnostic presentation. Plan and ADR parent-swap regressions and the uninstall fault
matrix prove confined effects; blocking diagnostic writers prove competing effort and upgrade
transactions cannot begin before presentation completes. ADR numbering and scaffolding load their
corpora and mutate records and provenance through the selected-root handle. Upgrade presentation
labels planned migrations separately until journal commit, and worktree removal and effort-creation
rollback retain exact known topology axes without inventing certainty. These authority-determined
corrections preserve Publisher, upgrade-journal, Git-integration, and focused lifecycle ownership
rather than introducing parallel policy.

Renewed Phase 4 review found six remaining authority and presentation gaps. Settlement binds plan and
ADR scaffold leases to the identity of their selected-root handles, inspects uninstall preservation
through its already-open confined capability, and replaces raw resident worktree preparation and
cleanup with the shared resident capability. Failed effort-creation rollback now detaches and retires
only the observed identity while preserving a same-name successor and reporting an exact reservation
or committed cleanup residue. Worktree failures report the exact managed path and branch. Root-swap,
same-name successor, mismatched-handle, confined-preservation, and exact diagnostic regressions pass;
these corrections preserve the approved transaction model without widening lifecycle policy.

The Phase 4 verify-pass remediation restores a retirement reservation after native cleanup refusal
while retaining and reporting concurrent residue, binds topic authority loading and publication to
one opened selected-root identity, and closes the resident capability before worktree Git mutation.
Focused red-first regressions prove all three defects and exact residue presentation. Parent semantic
inspection additionally found that a selected-root close failure after successful topic publication
lost the created-path outcome; a red-first operation regression now requires the typed partial result
to retain every created path and prohibit unsafe retry. The verify pass then removed an unsupported
claim about which identity occupies each retained rollback path and directly rendered the close
outcome to prove its exact path groups and recovery guidance. These corrections remain inside the
approved confinement and complete-outcome boundaries.

Phase 5 post-review settlement corrected conservative mutation selection and validation without changing the protected boundary. CI dispatch now runs the existing mutation blocker unconditionally when no trustworthy range exists; release selection admits only prior `vX.Y.Z` release tags and first release runs the same blocker unconditionally. Pincheck rejects malformed YAML shapes, aliases or merges, and unpinned reusable-workflow uses. Releasecheck now dispatches testably, refuses trailing API data, and gains injectable HTTP refusal coverage. The real hook regression exercises staged add, modify, delete, rename, and executable-mode candidates from an alternate index in a linked worktree, including a rejected staged broken file with an unstaged repair; it proves payload execution, cleanup, and unchanged source, indexes, and HEAD. YAML workflow-contract mutations prove the named CI identities, exact-SHA verification and checkout/tag identity, read-only verification before needs-bound publication, publication-only GoReleaser, release selector and fallback, and dispatch fallback. The exact-revision claim is Reapplied with its original operation to narrow it to landed repository wiring; hosted enforcement remains explicitly pending Phase 9. Focused Go tests passed before render; rendered output and broader gates remain for parent settlement verification. Parent coverage settlement moved the explicit-index Git fixture helper after the established fixture corpus, directly exercised every pincheck parser helper and every injectable releasecheck dispatch, transport, and evidence branch, and retained only the terminating process wrapper as exact visible coverage debt.

The single Phase 5 verify pass found that direct, unaliased YAML merge keys could still hide an
unpinned action. A red-first regression proves the bypass, and pincheck now rejects the YAML merge
tag itself before workflow inspection. This mechanical correction completes fail-closed structured
pin validation without changing the phase boundary.

Phase 6 corrected the route: pi-tools v0.3.0 accepts a prepared CWD and does not supply the required runtime confinement refusal. Preparation now resolves and caches the one canonical accessible descendant checkout before dispatch, returns it as child CWD, and snapshots that same identity before and after both completed and failed outcomes. The recorder red test failed with prepared CWD `/repo` instead of `/repo/.awf/worktrees/w`; it passes after the change. Root omission remains root/root, and explicit outside paths remain possible rather than falsely refused as confinement.

Phase 6 review settlement replaces callback-only evidence with the pinned pi-tools toolkit lifecycle.
Overlapping root and managed-worktree invocations make real relative and deliberate outside writes,
return completed and failed outcomes, and prove each prepared CWD remains paired with its own commit
snapshots. The Pi workflow clauses now state the effort-backed pre-integration requirement, root
omission, stable parent root, explicit parent paths, non-confinement boundary, and governed primary
lifecycle transition without contradictory CWD wording. The runtime smoke is the named backing for
the lifecycle claim, while focused rendered-output assertions falsify malformed guidance.

The single Phase 6 verify pass found platform-neutral containment, proof-selection, and testing-guide
residuals. Checkout containment now uses the native separator and rejects absolute relative results,
with Windows parent-escape and cross-drive cases. The gate-enabled smoke requires the exact pinned
toolkit lifecycle proving unit rather than accepting any green Pi suite. Testing guidance now names
the narrow test-only scheduler and prepared-CWD composition while preserving revision-independent
adopter handshake compatibility and pi-tools ownership of general runtime mechanics. These
corrections preserve the selected-checkout and independent-runtime decisions.

Phase 7 review found that terminal-plan enforcement lacked lawful current-state authority and used
only the status-flip diff plus prose substrings rather than selected implementation history and parsed
reconciliation. The focused ADR `bind-terminal-plan-closure-to-selected-implementation-evidence`
records the already approved terminal-freeze boundary and adds its one plan-artifact claim. ADR review
removed an unsupported permanent path-identity guarantee, retaining only byte stability, selected
implementation evidence, fail-closed reconciliation, and Proposed-plan mutability. The ADR moved
through Implementing with its sole Applied event. The settlement records a plan-owned terminal
reconciliation with an explicit implementation range, exact parsed touched-path list, and parsed
material-deviation list. The staged-transition owner resolves the selected range from Git rather than
the status-flip index, rejects missing, malformed, or empty evidence, and freezes every byte of an
Implemented record including status. Red-first terminal tests rejected substring-only reconciliation
and status regression; a repository-backed range fixture rejected missing selectors and empty ranges.
The green semantic oracle accepts an actual unplanned path when fully reconciled, proving closure does
not require original task, path, phase, or commit choreography. Task 7.3 applies that reviewed
decision without terminalizing the plan or ADR. The implementation settlement must receive renewed
exact-range Phase 7 assurance from settled Phase 6 boundary `5f9c68ba3` through the corrected
settlement tip, resolved from Git, and that assurance must settle before Phase 8 begins.

Phase 7 settlement grounding found four mechanical gaps in that first settlement. Terminal evidence now
unions old and new paths across every commit in the selected ancestor history, so modified-then-restored
and delete-recreate-identical paths remain visible rather than vanishing from an endpoint tree diff.
The reconciliation selector is exactly two lowercase full 40-hex object IDs; staged validation resolves
both, requires base ancestry, and binds its head to current checkout HEAD, refusing symbolic,
abbreviated, missing, reversed, divergent, empty, and stale selections. Parsed touched paths use
canonical reversible quoted strings and compare as a duplicate-free set, so authored Git order is not
policy; fences make example headings inert. Material deviations remain a required nonempty prose list
or exact `none`, but their substantive truth is review judgment rather than mechanically derived.
Focused parser, set-order, repository-history, stale-tip, ancestry, fenced-example, and adversarial-path
tests cover the correction and fail under direct mutations that restore endpoint-only selection,
movable selectors, ordered comparison, or fence-blind parsing. This retains the ADR's deliberate absence of permanent
plan-path, deletion, or rename guarantees and leaves multiple-plan aggregation advisory.

Renewed Phase 7 review found four mechanical settlement gaps. Terminal closure guidance now tells
both effort-backed and effort-free owners to author the parsed reconciliation from a full-SHA range
ending at current HEAD, every old and new path accumulated across selected commits and merge
resolutions, and the complete material-deviation record rather than endpoint diffs or prose markers.
The plan-format reference states the same authoring grammar. Repository selector failures retain their
underlying Git error identity and operation context. Independent fixtures preserve an intermediate
add-then-delete path and a merge-resolution-only path, while a marked repository-backed `CheckStaged`
test proves matching selected history reaches terminal policy and a guessed path is rejected. Rendered
Pi, Claude, and plan-reference boundaries carry the same meaning.

Phase 8 review found that the first archive metadata correction also normalized Darwin tar headers,
left checksum preservation implicit, returned an unwrapped structured-file decode error, and ignored
snapshot cleanup failure. The settlement splits Linux and non-Linux archive definitions over the
same build so only Linux receives portable ownership while names, formats, contents, and the target
matrix remain stable. The snapshot oracle now rejects non-Linux ownership normalization, requires
`checksums.txt` to name exactly all six archives, and checks cleanup. Structured archive decoding
wraps its cause. These corrections preserve the existing release boundary without adding platforms
or provenance machinery. Renewed review found that the structured-entry test did not pin the new
error context; a focused mutation proved the strengthened assertion fails when wrapping is removed.

Phase 9 captured the live GitHub ruleset, branch-protection, environment, tag-protection, workflow,
and exact-main check state before mutation. The existing active `main` ruleset retained deletion,
non-fast-forward, and required-signature rules and had no bypass actors, but required no checks. The
operator transaction added app-bound `gate` and `release-config` required-status rules to that
ruleset and created an active `release tags` ruleset for `refs/tags/v*` with the same GitHub Actions
app-bound checks and no bypass actors. Exact normalized API read-back matched both intended policies.
A safe non-publishing probe confirmed active remote `CI` and `Release` workflow identities, successful
current-main conclusions from GitHub Actions, applicable main rules, and the release ref pattern
without creating a ref, release, or publication. The release-tag rule deliberately adds no deletion
or non-fast-forward rule because immutable releases remain outside this plan.

Before final staging, the candidate-independent verification command set is the focused owner suite
for `cmd/awf`, `cmd/releasecheck`, `cmd/pincheck`, `internal/config`, `internal/manifest`,
`internal/publisher`, `internal/upgrade`, `internal/currentstate`, `internal/currentstatecoord`,
`internal/filesystem`, `internal/project`, `internal/topic`, `internal/evals`, and
`internal/coverage`; `go test ./...`; `go test ./internal/upgrade -run=^$ -fuzz=FuzzParseJournal
-fuzztime=10s`; `go test ./cmd/releasecheck -run '^TestReleaseArchivesPortableSnapshot$' -count=1`;
`./x render`; `./x check`; `./awf check staged`; `./x gate`; `goreleaser check`; and a clean
GoReleaser snapshot. The gate owns version validation, released-platform compilation, production
reachability, exact coverage policy, staged lane selection, and the targeted mutation blocker when
its exact owner paths are selected. The complete implementation audit and review range begins at
`6ce178c9d3db433b246dde8ddc6b89517b84ceec`; Phase 9 review begins after
`456639859091df2d54cc5341ca8c5762f7d01415`; both end at the closing commit resolved from Git after
it exists. Run `./awf audit` and `./x audit-local` over the complete range. Do not transcribe a
prospective closing SHA.

The pre-stage journal fuzz smoke ran for ten seconds with the default twelve workers and completed
705,425 executions without a failure. The restricted archive oracle runs on Linux through
`unshare --user --map-root-user` and extracts with `tar` without `--no-same-owner`; it requires
exactly `awf`, `LICENSE`, and `README.md`. The generated meaning review covers release guidance,
testing guidance, local-hook workflow guidance, both changed tooling topic pages, and the lock that
binds each source to its rendered output. `TestSelfHostedRemotePolicyDocumentation` pins the hosted
main and tag checks and rejects the stale claim that a local gate itself blocks merges. The focused
archive oracle, complete Go suite, render, drift check, GoReleaser validation, six-target snapshot,
and normalized live-settings read-back were green before staging; the exact staged candidate must
repeat its applicable checks.

Final live-setting verification compares normalized authenticated API reads of rulesets `18766557`
and `21631403` with their intended documents and requires exact equality immediately before commit.
Residual dispositions remain explicit rather than closed: adopter pi-tools stays revision-independent
and handshake-gated; parent-session mutation stays path-targeted and non-confined; archives and
checksums retain same-channel integrity; non-Linux targets retain compile and archive rather than
hosted behavioral assurance; immutable releases, attestations, broader provenance and platform work,
coverage redesign, and broader mutation sampling remain deferred; and every known issue whose
completion criteria are unmet remains open.

The first final staged check correctly refused removal of the now-satisfied hosted-settings pending
sentence from `exact-revision-repository-acceptance` because the transaction initially recorded only
the two new rule additions. The settlement appends a distinct Reapplied occurrence for that existing
add operation, preserving its first-application provenance while authorizing the material claim
correction. This is the lifecycle handshake required by current authority, not a protected-contract
change.

Complete-range implementation assurance found two acceptance-input residuals. Pincheck accepted a
second YAML document after a valid workflow, and releasecheck treated multiple successful exact-SHA
CI runs as ambiguous even when one run carried complete required-job evidence. Both regressions were
observed red against `d2104ef0e`. Pincheck now requires EOF after one document. Releasecheck treats
same-revision successful runs as equivalent candidates and accepts only a candidate whose `gate` and
`release-config` jobs are complete and successful; incomplete candidates remain refusals. This
reasoned selection correction follows the exact-revision authority without weakening required-job
proof. The single verify pass found that per-candidate API failures lost their error identity during
aggregation. Settlement retains each cause through wrapping and joins failed-candidate evidence; a
red-first transport sentinel proves `errors.Is` survives the complete refusal.
